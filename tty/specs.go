package tty

import (
	"fmt"
	"sync"
	"text/template"

	"github.com/braintree/manners"
	"github.com/gorilla/websocket"
)

type InitMessage struct {
	Arguments string `json:"Arguments,omitempty"`
	AuthToken string `json:"AuthToken,omitempty"`
}

type ConnKey struct {
	Name string
	Addr string
}

func (k ConnKey) String() string {
	if k.Name == "" && k.Addr == "" {
		return "NULL"
	}
	return fmt.Sprintf("%s/%s", k.Name, k.Addr)
}

type Tty struct {
	options       *Options
	upgrader      *websocket.Upgrader
	titleTemplate *template.Template
	server        *manners.GracefulServer
	session       map[ConnKey]*session
	waitingConn   *Slist
}

type Session_info struct {
	Key        ConnKey
	PKey       ConnKey
	Method     string
	Status     string
	Command    []string
	RemoteAddr string
	ConnTime   int64
	LinkNb     int32
}

type session struct {
	sync.Mutex
	key        ConnKey
	linkTo     *session
	linkNb     int32
	method     string
	status     string
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
	SName       string
	SAddr       string
	All         bool
	PermitWrite bool
	PermitShare bool
}

type connRx struct {
	key         ConnKey
	messageType int
	p           []byte
	err         error
}

type webConn struct {
	sync.Mutex
	conn *websocket.Conn
}

type connErr struct {
	key ConnKey
	err error
}

const (
	Version = "0.0.12"
	//UNIX_SOCKET = "/var/run/gotty.sock"
	UNIX_SOCKET      = "/tmp/gotty.sock"
	CONN_S_WAITING   = "waiting"
	CONN_S_CONNECTED = "connected"
	CONN_S_CLOSED    = "closed"
	CONN_M_EXEC      = "exec"
	CONN_M_SHARE     = "share"
	CONN_M_ATTACH    = "attach"
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
		PermitShare: false,
	}
)
