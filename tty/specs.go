package tty

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"os/user"
	"sync"
	"text/template"

	"github.com/braintree/manners"
	"github.com/gorilla/websocket"
	"github.com/yubo/gotty/rec"
)

type clientContext struct {
	session     *session
	request     *http.Request
	connection  *webConn
	connections *map[ConnKey]*webConn
	command     *exec.Cmd
	pty         io.ReadWriteCloser
	fd          uintptr
	writeMutex  *sync.Mutex
	connRx      chan *connRx
}

type argResizeTerminal struct {
	Columns float64
	Rows    float64
}

type ContextVars struct {
	Command    string
	Pid        int
	Hostname   string
	RemoteAddr string
}
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

type Daemon struct {
	options       *Options
	upgrader      *websocket.Upgrader
	titleTemplate *template.Template
	server        *manners.GracefulServer
	session       map[ConnKey]*session
	waitingConn   *Slist
	chuser        *user.User
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
	RecId      string
	Cmd        string
	Share      bool
	Time       string
}

type Session_infos []Session_info

func (slice Session_infos) Len() int {
	return len(slice)
}

func (slice Session_infos) Less(i, j int) bool {
	return slice[i].Key.Name < slice[j].Key.Name
}

func (slice Session_infos) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
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
	nets       *[]*net.IPNet
	recorder   *rec.Recorder
	player     *rec.Player
}

type Options struct {
	Address             string                 `hcl:"address"`
	Port                string                 `hcl:"port"`
	EnableBasicAuth     bool                   `hcl:"enable_basic_auth"`
	Credential          string                 `hcl:"credential"`
	DemoDir             string                 `hcl:"demo_dir"`
	DemoEnable          bool                   `hcl:"demo_enable"`
	DemoAddr            string                 `hcl:"demo_addr"`
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
	RecFileDir          string                 `hcl:"rec_file_dir"`
	SkipTlsVerify       bool                   `hcl:"skip_tls_verify"`
	UnixSocket          string                 `hcl:"unix_socket"`
	Debug               bool                   `hcl:"debug"`
	Resourses           string                 `hcl:"resources"`
	Chuser              string                 `hcl:"chuser"`
}

type CallOptions struct {
	Opt  CmdOptions
	Args []string
}

type CmdOptions struct {
	All              bool    `json:"all"`
	PermitWrite      bool    `json:"write"`
	PermitShare      bool    `json:"share"`
	PermitShareWrite bool    `json:"sharew"`
	Rec              bool    `json:"rec"`
	Repeat           bool    `json:"repeat"`
	MaxWait          int64   `json:"maxwait"`
	Speed            float64 `json:"speed"`
	Name             string  `json:"name"`
	Addr             string  `json:"addr"`
	Cmd              string  `json:"cmd"`
	SName            string  `json:"sname"`
	SAddr            string  `json:"saddr"`
	RecId            string  `json:"recid"`
	Action           string  `json:"action"`
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
	CONN_M_PLAY      = "play"
	NULL_FILE        = "/dev/null"
)

var (
	DefaultOptions = Options{
		Address:         "",
		Port:            "8080",
		EnableBasicAuth: false,
		Credential:      "",
		//EnableRandomUrl:     false,
		//RandomUrlLength:     8,
		//IndexFile:           "",
		DemoDir:             "/var/lib/gotty/static",
		DemoEnable:          true,
		DemoAddr:            "127.0.0.0/8",
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
		RecFileDir:          "/var/lib/gotty",
		SkipTlsVerify:       false,
		UnixSocket:          "/tmp/gotty.sock",
		Debug:               false,
		Resourses:           "./resources",
		Chuser:              "",
	}
	DefaultCmdOptions = CmdOptions{
		All:              false,
		PermitWrite:      false,
		PermitShare:      false,
		PermitShareWrite: false,
		Rec:              false,
		Repeat:           true,
		Speed:            1.0,
		Addr:             "127.0.0.0/8",
		MaxWait:          0,
	}
)
