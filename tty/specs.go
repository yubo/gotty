package tty

import (
	"sync"
	"text/template"

	"github.com/braintree/manners"
	"github.com/gorilla/websocket"
)

type InitMessage struct {
	Arguments string `json:"Arguments,omitempty"`
	AuthToken string `json:"AuthToken,omitempty"`
}

type connKey struct {
	name, addr string
}

type Tty struct {
	options       *Options
	upgrader      *websocket.Upgrader
	titleTemplate *template.Template
	server        *manners.GracefulServer
	session       map[connKey]*session
	waitingConn   *Slist
	//onceMutex     *umutex.UnblockingMutex
}

type Session_info struct {
	Name       string
	Addr       string
	Command    []string
	RemoteAddr string
	ConnTime   int64
}

type session struct {
	sync.RWMutex
	key        connKey
	status     string
	remoteAddr string
	createTime int64
	connTime   int64
	options    *CmdOptions
	context    *clientContext
	command    []string
}

type Options struct {
	Address             string                 `hcl:"address"`
	Port                string                 `hcl:"port"`
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
	WaitingConnTime     int                    `hcl:"waiting_conn_time"`
}

type CallOptions struct {
	Opt  CmdOptions
	Args []string
}

type CmdOptions struct {
	Name        string
	Addr        string
	PermitWrite bool
}

const (
	Version = "0.0.12"
	//UNIX_SOCKET = "/var/run/gotty.sock"
	UNIX_SOCKET      = "/tmp/gotty.sock"
	CONN_S_WAITING   = "waiting"
	CONN_S_CONNECTED = "connected"
	CONN_S_CLOSED    = "closed"
)

var (
	DefaultOptions = Options{
		Address:             "",
		Port:                "8080",
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
		WaitingConnTime:     10,
	}
	DefaultCmdOptions = CmdOptions{
		Name:        "",
		PermitWrite: false,
	}
)
