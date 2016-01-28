package tty

import (
	"container/list"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/template"
	"time"

	"github.com/braintree/manners"
	"github.com/elazarl/go-bindata-assetfs"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/kr/pty"
	"github.com/yubo/gotool/flags"
	"github.com/yubo/gotty/hcl"
)

var (
	tty *Tty
	//session   *Session
	GlobalOpt Options = DefaultOptions
)

func init() {
	flags.CommandLine.Usage = "Share your terminal as a web application"
	flags.CommandLine.Name = "gotty"

	// daemon
	cmd := flags.NewCommand("daemon", "Enable daemon mode",
		daemon_handle, flag.ExitOnError)
	cmd.StringVar(&configFile, "c",
		"/etc/gotty/gotty.conf", "Config file path")

}

func daemon_handle(arg interface{}) {
	args := arg.(CallOptions).Args

	if err := checkConfig(&GlobalOpt); err != nil {
		exit(err, 6)
	}

	err := tty_init(&GlobalOpt, args)
	if err != nil {
		exit(err, 3)
	}

	registerSignals()
	if err = run(); err != nil {
		exit(err, 4)
	}
}

func Parse() {

	flags.Parse() //for glog

	_, err := os.Stat(ExpandHomeDir(configFile))
	if !os.IsNotExist(err) {
		if err := applyConfigFile(&GlobalOpt, configFile); err != nil {
			glog.Errorln(err)
			os.Exit(2)
		}
	}

}

func tty_init(options *Options, command []string) error {
	// called after Parse()

	titleTemplate, err := template.New("title").Parse(GlobalOpt.TitleFormat)
	if err != nil {
		return errors.New("Title format string syntax error")
	}

	tty = &Tty{
		options: options,
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			Subprotocols:    []string{"gotty"},
		},
		titleTemplate: titleTemplate,
		session:       make(map[connKey]*session),
		waitingConn:   &Slist{list: list.New()},
	}

	/*
		session = &Session{
			tty:     tty,
			command: command,
		}
	*/

	// waiting conn clean routine
	go func() {
		var n, e *list.Element
		var sess *session
		var now int64
		t := time.NewTicker(time.Second).C
		for {
			select {
			case <-t:
				now = time.Now().Unix()
				e = tty.waitingConn.list.Front()
				for e != nil {
					if e.Value.(*session).createTime+
						int64(options.WaitingConnTime) > now {
						break
					}
					n = e.Next()
					sess = tty.waitingConn.Remove(e).(*session)

					if sess.status == CONN_S_WAITING {
						sess.Lock()
						glog.Infof("name[%s] addr[%s] waiting conntion timeout\n",
							sess.key.name, sess.key.addr)
						//remove from tty.session
						sess.status = CONN_S_CLOSED
						delete(tty.session, sess.key)
						sess.Unlock()
					}

					e = n
				}

			}
		}
	}()

	return rpc_init()
}

func applyConfigFile(options *Options, filePath string) error {
	filePath = ExpandHomeDir(filePath)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return err
	}

	fileString := []byte{}
	glog.Infof("Loading config file at: %s", filePath)
	fileString, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	if err := hcl.Decode(options, string(fileString)); err != nil {
		return err
	}

	return nil
}

func checkConfig(options *Options) error {
	if GlobalOpt.EnableTLSClientAuth && !GlobalOpt.EnableTLS {
		return errors.New("TLS client authentication is enabled, " +
			"but TLS is not enabled")
	}
	return nil
}

func run() error {

	if GlobalOpt.Once {
		glog.Infof("Once option is provided, accepting only one client")
	}

	path := ""
	if GlobalOpt.EnableRandomUrl {
		path += "/" + generateRandomString(GlobalOpt.RandomUrlLength)
	}

	endpoint := net.JoinHostPort(GlobalOpt.Address, GlobalOpt.Port)

	customIndexHandler := http.HandlerFunc(tty.handleCustomIndex)
	authTokenHandler := http.HandlerFunc(tty.handleAuthToken)
	staticHandler := http.FileServer(
		&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, Prefix: "static"},
	)

	var siteMux = http.NewServeMux()

	if GlobalOpt.IndexFile != "" {
		glog.Infof("Using index file at " + GlobalOpt.IndexFile)
		siteMux.Handle(path+"/", customIndexHandler)
	} else {
		siteMux.Handle(path+"/", http.StripPrefix(path+"/", staticHandler))
	}
	siteMux.Handle(path+"/auth_token.js", authTokenHandler)
	siteMux.Handle(path+"/js/", http.StripPrefix(path+"/", staticHandler))
	siteMux.Handle(path+"/favicon.png", http.StripPrefix(path+"/", staticHandler))

	siteHandler := http.Handler(siteMux)

	if GlobalOpt.EnableBasicAuth {
		glog.Infof("Using Basic Authentication")
		siteHandler = wrapBasicAuth(siteHandler, GlobalOpt.Credential)
	}

	siteHandler = wrapHeaders(siteHandler)

	wsMux := http.NewServeMux()
	wsMux.Handle("/", siteHandler)
	wsMux.Handle(path+"/ws", http.HandlerFunc(wsHandler))

	siteHandler = wrapLogger(http.Handler(wsMux))

	scheme := "http"
	if GlobalOpt.EnableTLS {
		scheme = "https"
	}
	/*
		glog.Infof(
			"Server is starting with command: %s\n",
			strings.Join(session.command, " ")
		)
	*/
	if GlobalOpt.Address != "" {
		glog.Infof(
			"URL: %s",
			(&url.URL{Scheme: scheme, Host: endpoint, Path: path + "/"}).String(),
		)
	} else {
		for _, address := range listAddresses() {
			glog.Infof(
				"URL: %s",
				(&url.URL{
					Scheme: scheme,
					Host:   net.JoinHostPort(address, GlobalOpt.Port),
					Path:   path + "/",
				}).String(),
			)
		}
	}

	server, err := makeServer(tty, endpoint, &siteHandler)
	if err != nil {
		return errors.New("Failed to build server: " + err.Error())
	}
	tty.server = manners.NewWithServer(server)

	if GlobalOpt.EnableTLS {
		crtFile := ExpandHomeDir(GlobalOpt.TLSCrtFile)
		keyFile := ExpandHomeDir(GlobalOpt.TLSKeyFile)
		glog.Infof("TLS crt file: " + crtFile)
		glog.Infof("TLS key file: " + keyFile)

		err = tty.server.ListenAndServeTLS(crtFile, keyFile)
	} else {
		err = tty.server.ListenAndServe()
	}
	if err != nil {
		return err
	}

	glog.Infof("Exiting...")

	return nil
}

func (tty *Tty) newWaitingConn(sess *session) error {
	sess.Lock()
	defer sess.Unlock()

	if _, exsit := tty.session[sess.key]; !exsit {
		tty.session[sess.key] = sess
		tty.waitingConn.Push(sess)
		return nil
	} else {
		return fmt.Errorf("the key name[%s] addr[%s] is exsit",
			sess.key.name, sess.key.addr)
	}

}

func makeServer(tty *Tty, addr string, handler *http.Handler) (*http.Server, error) {
	server := &http.Server{
		Addr:    addr,
		Handler: *handler,
	}

	if GlobalOpt.EnableTLSClientAuth {
		caFile := ExpandHomeDir(GlobalOpt.TLSCACrtFile)
		glog.Infof("CA file: " + caFile)
		caCert, err := ioutil.ReadFile(caFile)
		if err != nil {
			return nil, errors.New("Could not open CA crt file " + caFile)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, errors.New("Could not parse CA crt file data in " + caFile)
		}
		tlsConfig := &tls.Config{
			ClientCAs:  caCertPool,
			ClientAuth: tls.RequireAndVerifyClientCert,
		}
		server.TLSConfig = tlsConfig
	}

	return server, nil
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	var init InitMessage
	var key connKey
	var session *session
	var ok bool
	var cip string

	glog.Infof("New client connected: %s", r.RemoteAddr)

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	conn, err := tty.upgrader.Upgrade(w, r, nil)
	if err != nil {
		glog.Infof("Failed to upgrade connection: " + err.Error())
		return
	}

	_, stream, err := conn.ReadMessage()
	if err != nil {
		glog.Infof("Failed to authenticate websocket connection")
		conn.Close()
		return
	}

	err = json.Unmarshal(stream, &init)
	if err != nil {
		glog.Infof("Failed to parse init message %v", err)
		conn.Close()
		return
	}
	if init.AuthToken != GlobalOpt.Credential {
		glog.Infof("Failed to authenticate websocket connection")
		conn.Close()
		return
	}

	//if GlobalOpt.PermitArguments {
	if init.Arguments == "" {
		init.Arguments = "?"
	}
	query, err := url.Parse(init.Arguments)
	if err != nil {
		glog.Infof("Failed to parse arguments")
		conn.Close()
		return
	}

	if params := query.Query()["name"]; len(params) != 0 {
		key.name = params[0]
	}
	//}

	if cip, _, err = net.SplitHostPort(r.RemoteAddr); err != nil {
		glog.Infof("Failed to authenticate websocket connection")
		conn.Close()
		return
	}

	addrs := []string{cip, "0.0.0.0"}
	for _, key.addr = range addrs {
		if session, ok = tty.session[key]; ok {
			break
		}
	}
	if !ok {
		glog.Infof("name:%s addr:%s is not exist\n", key.name, cip)
		conn.Close()
		return
	}

	argv := session.command[1:]
	if params := query.Query()["arg"]; len(params) != 0 {
		argv = append(argv, params...)
	}
	session.Lock()
	defer session.Unlock()

	if session.status != CONN_S_WAITING {
		glog.Infof("name:%s addr:%s status is %s, not waiting\n",
			key.name, key.addr, session.status)
		conn.Close()
		return
	}

	session.status = CONN_S_CONNECTED
	tty.server.StartRoutine()
	/*
		if GlobalOpt.Once {
			if tty.onceMutex.TryLock() { // no unlock required, it will die soon
				glog.Infof("Last client accepted, closing the listener.")
				s.server.Close()
			} else {
				glog.Infof("Session is already closing.")
				conn.Close()
				return
			}
		}
	*/
	cmd := exec.Command(session.command[0], argv...)
	ptyIo, err := pty.Start(cmd)
	if err != nil {
		glog.Errorln("Failed to execute command", err)
		return
	}
	glog.Infof("Command is running for client %s with PID %d (args=%q)",
		r.RemoteAddr, cmd.Process.Pid, strings.Join(argv, " "))

	session.context = &clientContext{
		session:    session,
		request:    r,
		connection: conn,
		command:    cmd,
		pty:        ptyIo,
		writeMutex: &sync.Mutex{},
	}
	session.remoteAddr = r.RemoteAddr
	session.connTime = time.Now().Unix()

	session.context.goHandleClient()
}

func (tty *Tty) handleCustomIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, ExpandHomeDir(GlobalOpt.IndexFile))
}

func (tty *Tty) handleAuthToken(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("var gotty_auth_token = '" + GlobalOpt.Credential + "';"))
}

func Exit() (firstCall bool) {

	rpc_done()

	if tty.server != nil {
		firstCall = tty.server.Close()
		if firstCall {
			glog.Infof("Received Exit command, waiting for all clients to close sessions...")
		}
		return firstCall
	}
	return true
}

func wrapLogger(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWrapper{w, 200}
		handler.ServeHTTP(rw, r)
		glog.Infof("%s %d %s %s", r.RemoteAddr, rw.status, r.Method, r.URL.Path)
	})
}

func wrapHeaders(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "GoTTY/"+Version)
		handler.ServeHTTP(w, r)
	})
}

func wrapBasicAuth(handler http.Handler, credential string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.SplitN(r.Header.Get("Authorization"), " ", 2)

		if len(token) != 2 || strings.ToLower(token[0]) != "basic" {
			w.Header().Set("WWW-Authenticate", `Basic realm="GoTTY"`)
			http.Error(w, "Bad Request", http.StatusUnauthorized)
			return
		}

		payload, err := base64.StdEncoding.DecodeString(token[1])
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if credential != string(payload) {
			w.Header().Set("WWW-Authenticate", `Basic realm="GoTTY"`)
			http.Error(w, "authorization failed", http.StatusUnauthorized)
			return
		}

		glog.Infof("Basic Authentication Succeeded: %s", r.RemoteAddr)
		handler.ServeHTTP(w, r)
	})
}

func generateRandomString(length int) string {
	const base = 36
	size := big.NewInt(base)
	n := make([]byte, length)
	for i, _ := range n {
		c, _ := rand.Int(rand.Reader, size)
		n[i] = strconv.FormatInt(c.Int64(), base)[0]
	}
	return string(n)
}

func listAddresses() (addresses []string) {
	ifaces, _ := net.Interfaces()

	addresses = make([]string, 0, len(ifaces))

	for _, iface := range ifaces {
		ifAddrs, _ := iface.Addrs()
		for _, ifAddr := range ifAddrs {
			switch v := ifAddr.(type) {
			case *net.IPNet:
				addresses = append(addresses, v.IP.String())
			case *net.IPAddr:
				addresses = append(addresses, v.IP.String())
			}
		}
	}

	return
}

func ExpandHomeDir(path string) string {
	if path[0:2] == "~/" {
		return os.Getenv("HOME") + path[1:]
	} else {
		return path
	}
}

func exit(err error, code int) {
	if err != nil {
		glog.Errorln(err)
	}
	os.Exit(code)
}

func registerSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(
		sigChan,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	go func() {
		for {
			s := <-sigChan
			switch s {
			case syscall.SIGINT, syscall.SIGTERM:
				Exit()
				os.Exit(1)
			}
		}
	}()
}
