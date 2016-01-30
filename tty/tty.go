package tty

import (
	"container/list"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
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

func Parse() {
	flags.Parse() //for glog

	_, err := os.Stat(expandHomeDir(configFile))
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
		session:       make(map[ConnKey]*session),
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
							sess.key.Name, sess.key.Addr)
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
	filePath = expandHomeDir(filePath)
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
		crtFile := expandHomeDir(GlobalOpt.TLSCrtFile)
		keyFile := expandHomeDir(GlobalOpt.TLSKeyFile)
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
			sess.key.Name, sess.key.Addr)
	}

}

func makeServer(tty *Tty, addr string, handler *http.Handler) (*http.Server, error) {
	server := &http.Server{
		Addr:    addr,
		Handler: *handler,
	}

	if GlobalOpt.EnableTLSClientAuth {
		caFile := expandHomeDir(GlobalOpt.TLSCACrtFile)
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

func ws_clone(sess *session, r *http.Request,
	query *url.URL, conn *websocket.Conn, cip string) error {
	key := ConnKey{Addr: cip}
	if err := keyGenerator(&key); err != nil {
		return err
	}
	sess.linkNb += 1
	opt := *sess.options
	if !(opt.PermitWrite && opt.PermitShare && opt.All) {
		opt.PermitWrite = false
	}
	s := &session{
		key:        key,
		linkTo:     sess,
		linkNb:     1,
		status:     CONN_S_CONNECTED,
		method:     CONN_M_SHARE,
		createTime: time.Now().Unix(),
		connTime:   time.Now().Unix(),
		options:    &opt,
		command:    sess.command,
		context: &clientContext{
			request:     r,
			connection:  &webConn{conn: conn},
			connections: sess.context.connections,
			command:     sess.context.command,
			pty:         sess.context.pty,
			connRx:      sess.context.connRx,
		},
	}
	s.context.session = s
	tty.session[key] = s
	return s.context.goHandleClientJoin()
}

func ws_connect(session *session, r *http.Request,
	query *url.URL, conn *websocket.Conn) {
	argv := session.command[1:]
	if params := query.Query()["arg"]; len(params) != 0 {
		argv = append(argv, params...)
	}

	cmd := exec.Command(session.command[0], argv...)
	ptyIo, err := pty.Start(cmd)
	if err != nil {
		glog.Errorln("Failed to execute command", err)
		delete(tty.session, session.key)
		conn.Close()
		return
	}
	tty.server.StartRoutine()
	session.connTime = time.Now().Unix()
	session.status = CONN_S_CONNECTED

	glog.Infof("Command is running for client %s with PID %d (args=%q)",
		r.RemoteAddr, cmd.Process.Pid, strings.Join(argv, " "))

	conns := make(map[ConnKey]*webConn)

	session.context = &clientContext{
		session:     session,
		request:     r,
		connection:  &webConn{conn: conn},
		connections: &conns,
		command:     cmd,
		pty:         ptyIo,
		connRx:      make(chan *connRx),
	}
	session.context.goHandleClient()
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	var init InitMessage
	var key ConnKey
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
		key.Name = params[0]
	}
	if params := query.Query()["addr"]; len(params) != 0 {
		key.Addr = params[0]
	}
	//}

	if cip, _, err = net.SplitHostPort(r.RemoteAddr); err != nil {
		glog.Infof("Failed to authenticate websocket connection")
		conn.Close()
		return
	}

	if session, ok = tty.session[key]; !ok {
		glog.Infof("name:%s addr:%s is not exist\n", key.Name, key.Addr)
		conn.Close()
		return
	}

	if !ipFilter(cip, session.nets) {
		glog.Infof("RemoteAddr:%s is not allowed to access name:%s addr:%s\n",
			cip, key.Name, key.Addr)
		conn.Close()
		return
	}

	session.Lock()
	defer session.Unlock()

	if session.method == CONN_M_EXEC {
		if session.status == CONN_S_CONNECTED &&
			session.options.PermitShare {
			ws_clone(session, r, query, conn, cip)
		} else if session.status == CONN_S_WAITING {
			ws_connect(session, r, query, conn)
			return
		} else {
			glog.Infof("name:%s addr:%s status is %s, not allow to connect\n",
				key.Name, key.Addr, session.status)
			conn.Close()
			return
		}
	} else if session.method == CONN_M_ATTACH {
		if session.status != CONN_S_WAITING {
			glog.Infof("name:%s addr:%s status is %s, not waiting\n",
				key.Name, key.Addr, session.status)
			conn.Close()
			return
		}
		session.linkTo.Lock()
		defer session.linkTo.Unlock()

		session.connTime = time.Now().Unix()
		session.status = CONN_S_CONNECTED
		session.context = &clientContext{
			session:     session,
			request:     r,
			connection:  &webConn{conn: conn},
			connections: session.linkTo.context.connections,
			command:     session.linkTo.context.command,
			pty:         session.linkTo.context.pty,
			connRx:      session.linkTo.context.connRx,
		}
		session.context.goHandleClientJoin()
	}
}

func (tty *Tty) handleCustomIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, expandHomeDir(GlobalOpt.IndexFile))
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
		//glog.Infof("%s %d %s %s", r.RemoteAddr, rw.status, r.Method, r.URL.Path)
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

func expandHomeDir(path string) string {
	if path[0:2] == "~/" {
		return os.Getenv("HOME") + path[1:]
	} else {
		return path
	}
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
