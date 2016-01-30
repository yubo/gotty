package tty

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/fatih/structs"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
)

type clientContext struct {
	session     *session
	request     *http.Request
	connection  *webConn
	connections *map[ConnKey]*webConn
	command     *exec.Cmd
	pty         *os.File
	writeMutex  *sync.Mutex
	connRx      chan *connRx
}

const (
	Input          = '0'
	Ping           = '1'
	ResizeTerminal = '2'
)

const (
	Output         = '0'
	Pong           = '1'
	SetWindowTitle = '2'
	SetPreferences = '3'
	SetReconnect   = '4'
)

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

func (context *clientContext) goHandleClientJoin() error {
	if err := context.sendInitialize(); err != nil {
		glog.Errorln(err.Error())
		return err
	}
	(*context.connections)[context.session.key] = context.connection
	go func() {
		rx := &connRx{key: context.session.key}

		for {
			rx.messageType, rx.p, rx.err = context.connection.conn.ReadMessage()
			context.connRx <- rx
			if rx.err != nil {
				context.close(rx.key)
				return
			}
		}
	}()
	return nil
}

func (context *clientContext) goHandleClient() {
	exit := make(chan bool, 2)

	(*context.connections)[context.session.key] = context.connection
	go func() {
		defer func() { exit <- true }()

		context.processSend()
	}()

	go func() {
		defer func() { exit <- true }()

		context.processReceive()
	}()

	go func() {

		<-exit
		context.session.status = CONN_S_CLOSED
		context.pty.Close()

		// Even if the PTY has been closed,
		// Read(0 in processSend() keeps blocking and the process doen't exit
		context.command.Process.Signal(syscall.Signal(tty.options.CloseSignal))
		tty.server.FinishRoutine()

		context.command.Wait()
		for key, _ := range *context.connections {
			context.close(key)
		}

		if context.session.linkNb != 0 {
			glog.Errorf("connection closed: %s:%s, but linkNb(%d) is not zero",
				context.session.key.Name, context.session.key.Addr,
				context.session.linkNb)
		}

		glog.Infof("Connection closed: %s", context.request.RemoteAddr)
	}()

	go func() {
		rx := &connRx{key: context.session.key}

		for {
			rx.messageType, rx.p, rx.err = context.connection.conn.ReadMessage()
			context.connRx <- rx
			if rx.err != nil {
				context.close(rx.key)
				return
			}
		}
	}()

}

func (context *clientContext) close(key ConnKey) {
	if conn, ok := (*context.connections)[key]; ok {
		conn.conn.Close()
		tty.session[key].status = CONN_S_CLOSED
		delete(*context.connections, key)

		if tty.session[key].linkTo != nil {
			n := atomic.AddInt32(&tty.session[key].linkTo.linkNb, -1)
			glog.Infof("linkNb:%d should be:%d", n,
				len(*tty.session[key].linkTo.context.connections))
			if n == 0 {
				delete(tty.session, tty.session[key].linkTo.key)
			}
		}

		n := atomic.AddInt32(&tty.session[key].linkNb, -1)
		if tty.session[key].linkNb == 0 {
			delete(tty.session, key)
		}
		glog.Infof("connection closed:%s, linkNb:%d", key, n)
	}
}
func (context *clientContext) processSend() {
	if err := context.sendInitialize(); err != nil {
		glog.Errorln(err.Error())
		return
	}

	buf := make([]byte, 1024)

	for {
		size, err := context.pty.Read(buf)
		if err != nil {
			glog.Errorf("Command exited for: %s", context.request.RemoteAddr)
			return
		}
		safeMessage := base64.StdEncoding.EncodeToString([]byte(buf[:size]))
		if errs := context.write(append([]byte{Output},
			[]byte(safeMessage)...)); len(errs) > 0 {
			for _, e := range errs {
				glog.Errorln(e.err.Error())
				context.close(e.key)
			}
			if len(*context.connections) == 0 {
				return
			}
		}
	}
}

func (wc *webConn) write(data []byte) error {
	wc.Lock()
	defer wc.Unlock()
	return wc.conn.WriteMessage(websocket.TextMessage, data)
}

func (context *clientContext) write(data []byte) []connErr {
	var errs []connErr
	for key, wc := range *context.connections {
		if err := wc.write(data); err != nil {
			errs = append(errs, connErr{key: key, err: err})
		}
	}
	return errs
}

func (context *clientContext) sendInitialize() error {
	hostname, _ := os.Hostname()
	titleVars := ContextVars{
		Command:    strings.Join(context.session.command, " "),
		Pid:        context.command.Process.Pid,
		Hostname:   hostname,
		RemoteAddr: context.request.RemoteAddr,
	}

	titleBuffer := new(bytes.Buffer)
	if err := tty.titleTemplate.Execute(titleBuffer, titleVars); err != nil {
		return err
	}
	if err := context.connection.write(append([]byte{SetWindowTitle},
		titleBuffer.Bytes()...)); err != nil {
		return err
	}

	prefStruct := structs.New(tty.options.Preferences)
	prefMap := prefStruct.Map()
	htermPrefs := make(map[string]interface{})
	for key, value := range prefMap {
		rawKey := prefStruct.Field(key).Tag("hcl")
		if _, ok := tty.options.RawPreferences[rawKey]; ok {
			htermPrefs[strings.Replace(rawKey, "_", "-", -1)] = value
		}
	}
	prefs, err := json.Marshal(htermPrefs)
	if err != nil {
		return err
	}

	if err := context.connection.write(append([]byte{SetPreferences},
		prefs...)); err != nil {
		return err
	}
	if tty.options.EnableReconnect {
		reconnect, _ := json.Marshal(tty.options.ReconnectTime)
		if err := context.connection.write(append([]byte{SetReconnect},
			reconnect...)); err != nil {
			return err
		}
	}
	return nil
}

func (context *clientContext) processReceive() {
	var rx *connRx
	var ok bool
	var err error
	for {
		if rx, ok = <-context.connRx; !ok {
			return
		}
		if rx.err != nil {
			glog.Errorln(rx.err.Error())
			context.close(rx.key)
			if len(*context.connections) == 0 {
				return
			} else {
				continue
			}
		}

		if len(rx.p) == 0 {
			glog.Errorln("An error has occured")
			return
		}

		switch rx.p[0] {
		case Input:
			if !context.session.options.PermitWrite {
				break
			}

			_, err = context.pty.Write(rx.p[1:])
			if err != nil {
				return
			}

		case Ping:
			if errs := context.write([]byte{Pong}); len(errs) > 0 {
				for _, e := range errs {
					glog.Errorln(e.err.Error())
					context.close(e.key)
				}
				if len(*context.connections) == 0 {
					return
				}
			}
		case ResizeTerminal:
			var args argResizeTerminal
			err = json.Unmarshal(rx.p[1:], &args)
			if err != nil {
				glog.Errorln("Malformed remote command")
				return
			}

			window := struct {
				row uint16
				col uint16
				x   uint16
				y   uint16
			}{
				uint16(args.Rows),
				uint16(args.Columns),
				0,
				0,
			}
			syscall.Syscall(
				syscall.SYS_IOCTL,
				context.pty.Fd(),
				syscall.TIOCSWINSZ,
				uintptr(unsafe.Pointer(&window)),
			)

		default:
			glog.Errorln("Unknown message type")
			return
		}
	}
}
