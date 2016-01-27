package tty

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"text/template"

	"github.com/braintree/manners"
	"github.com/elazarl/go-bindata-assetfs"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/kr/pty"
	"github.com/yubo/gotty/hcl"
)

type InitMessage struct {
	Arguments string `json:"Arguments,omitempty"`
	AuthToken string `json:"AuthToken,omitempty"`
}

type Tty struct {
	options       *Options
	upgrader      *websocket.Upgrader
	titleTemplate *template.Template
	server        *manners.GracefulServer
	//onceMutex     *umutex.UnblockingMutex
}

type Session struct {
	tty     *Tty
	command []string
}

type Options struct {
	Address             string                 `hcl:"address"`
	Port                string                 `hcl:"port"`
	PermitWrite         bool                   `hcl:"permit_write"`
	EnableBasicAuth     bool                   `hcl:"enable_basic_auth"`
	Credential          string                 `hcl:"credential"`
	EnableRandomUrl     bool                   `hcl:"enable_random_url"`
	RandomUrlLength     int                    `hcl:"random_url_length"`
	IndexFile           string                 `hcl:"index_file"`
	EnableTLS           bool                   `hcl:"enable_tls"`
	TLSCrtFile          string                 `hcl:"tls_crt_file"`
	TLSKeyFile          string                 `hcl:"tls_key_file"`
	EnableTLSClientAuth bool                   `hcl:"enable_tls_client_auth"`
	TLSCACrtFile        string                 `hcl:"tls_ca_crt_file"`
	TitleFormat         string                 `hcl:"title_format"`
	EnableReconnect     bool                   `hcl:"enable_reconnect"`
	ReconnectTime       int                    `hcl:"reconnect_time"`
	Once                bool                   `hcl:"once"`
	PermitArguments     bool                   `hcl:"permit_arguments"`
	CloseSignal         int                    `hcl:"close_signal"`
	Preferences         HtermPrefernces        `hcl:"preferences"`
	RawPreferences      map[string]interface{} `hcl:"preferences"`
}

var (
	Version        = "0.0.12"
	tty            *Tty
	session        *Session
	DefaultOptions = Options{
		Address:             "",
		Port:                "8080",
		PermitWrite:         false,
		EnableBasicAuth:     false,
		Credential:          "",
		EnableRandomUrl:     false,
		RandomUrlLength:     8,
		IndexFile:           "",
		EnableTLS:           false,
		TLSCrtFile:          "/etc/gotty/gotty.crt",
		TLSKeyFile:          "/etc/gotty/gotty.key",
		EnableTLSClientAuth: false,
		TLSCACrtFile:        "/etc/gotty/gotty.ca.crt",
		TitleFormat:         "GoTTY - {{ .Command }} ({{ .Hostname }})",
		EnableReconnect:     false,
		ReconnectTime:       10,
		Once:                false,
		CloseSignal:         1, // syscall.SIGHUP
		Preferences:         HtermPrefernces{},
	}
)

func Init(command []string, options *Options) error {
	titleTemplate, err := template.New("title").Parse(options.TitleFormat)
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
	}

	session = &Session{
		tty:     tty,
		command: command,
	}
	return nil
}

func ApplyConfigFile(options *Options, filePath string) error {
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

func CheckConfig(options *Options) error {
	if options.EnableTLSClientAuth && !options.EnableTLS {
		return errors.New("TLS client authentication is enabled, but TLS is not enabled")
	}
	return nil
}

func Run() error {
	if tty.options.PermitWrite {
		glog.Infof("Permitting clients to write input to the PTY.")
	}

	if tty.options.Once {
		glog.Infof("Once option is provided, accepting only one client")
	}

	path := ""
	if tty.options.EnableRandomUrl {
		path += "/" + generateRandomString(tty.options.RandomUrlLength)
	}

	endpoint := net.JoinHostPort(tty.options.Address, tty.options.Port)

	customIndexHandler := http.HandlerFunc(tty.handleCustomIndex)
	authTokenHandler := http.HandlerFunc(tty.handleAuthToken)
	staticHandler := http.FileServer(
		&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, Prefix: "static"},
	)

	var siteMux = http.NewServeMux()

	if tty.options.IndexFile != "" {
		glog.Infof("Using index file at " + tty.options.IndexFile)
		siteMux.Handle(path+"/", customIndexHandler)
	} else {
		siteMux.Handle(path+"/", http.StripPrefix(path+"/", staticHandler))
	}
	siteMux.Handle(path+"/auth_token.js", authTokenHandler)
	siteMux.Handle(path+"/js/", http.StripPrefix(path+"/", staticHandler))
	siteMux.Handle(path+"/favicon.png", http.StripPrefix(path+"/", staticHandler))

	siteHandler := http.Handler(siteMux)

	if tty.options.EnableBasicAuth {
		glog.Infof("Using Basic Authentication")
		siteHandler = wrapBasicAuth(siteHandler, tty.options.Credential)
	}

	siteHandler = wrapHeaders(siteHandler)

	wsMux := http.NewServeMux()
	wsMux.Handle("/", siteHandler)
	wsMux.Handle(path+"/ws", http.HandlerFunc(wsHandler))
	siteHandler = (http.Handler(wsMux))

	siteHandler = wrapLogger(siteHandler)

	scheme := "http"
	if tty.options.EnableTLS {
		scheme = "https"
	}
	glog.Infof(
		"Server is starting with command: %s\n",
		strings.Join(session.command, " "),
	)
	if tty.options.Address != "" {
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
					Host:   net.JoinHostPort(address, tty.options.Port),
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

	if tty.options.EnableTLS {
		crtFile := ExpandHomeDir(tty.options.TLSCrtFile)
		keyFile := ExpandHomeDir(tty.options.TLSKeyFile)
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

func makeServer(tty *Tty, addr string, handler *http.Handler) (*http.Server, error) {
	server := &http.Server{
		Addr:    addr,
		Handler: *handler,
	}

	if tty.options.EnableTLSClientAuth {
		caFile := ExpandHomeDir(tty.options.TLSCACrtFile)
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
	glog.Infof("New client connected: %s", r.RemoteAddr)
	s := session

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
	var init InitMessage

	err = json.Unmarshal(stream, &init)
	if err != nil {
		glog.Infof("Failed to parse init message %v", err)
		conn.Close()
		return
	}
	if init.AuthToken != s.tty.options.Credential {
		glog.Infof("Failed to authenticate websocket connection")
		conn.Close()
		return
	}
	argv := s.command[1:]
	if s.tty.options.PermitArguments {
		if init.Arguments == "" {
			init.Arguments = "?"
		}
		query, err := url.Parse(init.Arguments)
		if err != nil {
			glog.Infof("Failed to parse arguments")
			conn.Close()
			return
		}
		params := query.Query()["arg"]
		if len(params) != 0 {
			argv = append(argv, params...)
		}
	}

	s.tty.server.StartRoutine()
	/*
		if tty.options.Once {
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
	cmd := exec.Command(s.command[0], argv...)
	ptyIo, err := pty.Start(cmd)
	if err != nil {
		glog.Errorln("Failed to execute command", err)
		return
	}
	glog.Infof("Command is running for client %s with PID %d (args=%q)", r.RemoteAddr, cmd.Process.Pid, strings.Join(argv, " "))

	context := &clientContext{
		session:    s,
		request:    r,
		connection: conn,
		command:    cmd,
		pty:        ptyIo,
		writeMutex: &sync.Mutex{},
	}

	context.goHandleClient()
}

func (tty *Tty) handleCustomIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, ExpandHomeDir(tty.options.IndexFile))
}

func (tty *Tty) handleAuthToken(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("var gotty_auth_token = '" + tty.options.Credential + "';"))
}

func Exit() (firstCall bool) {
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
