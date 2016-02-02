package tty

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/fatih/structs"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/yubo/gotty/rec"
)

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

	daemon.server.StartRoutine()
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
		if context.session.recorder != nil {
			context.session.recorder.Close()
		}

		// Even if the PTY has been closed,
		// Read(0 in processSend() keeps blocking and the process doen't exit
		if context.session.method != CONN_M_PLAY {
			context.command.Process.Signal(syscall.Signal(daemon.options.CloseSignal))
		}
		daemon.server.FinishRoutine()

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
		daemon.session[key].status = CONN_S_CLOSED
		delete(*context.connections, key)

		if daemon.session[key].linkTo != nil {
			n := atomic.AddInt32(&daemon.session[key].linkTo.linkNb, -1)
			glog.V(2).Infof("linkNb:%d should be:%d", n,
				len(*daemon.session[key].linkTo.context.connections))
			if n == 0 {
				delete(daemon.session, daemon.session[key].linkTo.key)
			}
		}

		n := atomic.AddInt32(&daemon.session[key].linkNb, -1)
		if daemon.session[key].linkNb == 0 {
			delete(daemon.session, key)
		}
		glog.V(2).Infof("connection closed:%s, linkNb:%d", key, n)
	}
}

func (context *clientContext) record(data []byte) {
	if r := context.session.recorder; r != nil {
		if _, err := r.Write(data); err != nil {
			glog.Errorf(err.Error())
			r.Close()
			r = nil
		}
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
		context.record(append([]byte{rec.Output}, buf[:size]...))
		safeMessage := base64.StdEncoding.EncodeToString([]byte(buf[:size]))
		if errs := context.write(append([]byte{rec.Output},
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
	if err := daemon.titleTemplate.Execute(titleBuffer, titleVars); err != nil {
		return err
	}
	if err := context.connection.write(append([]byte{rec.SetWindowTitle},
		titleBuffer.Bytes()...)); err != nil {
		return err
	}

	prefStruct := structs.New(daemon.options.Preferences)
	prefMap := prefStruct.Map()
	htermPrefs := make(map[string]interface{})
	for key, value := range prefMap {
		rawKey := prefStruct.Field(key).Tag("hcl")
		if _, ok := daemon.options.RawPreferences[rawKey]; ok {
			htermPrefs[strings.Replace(rawKey, "_", "-", -1)] = value
		}
	}
	prefs, err := json.Marshal(htermPrefs)
	if err != nil {
		return err
	}

	if err := context.connection.write(append([]byte{rec.SetPreferences},
		prefs...)); err != nil {
		return err
	}
	if daemon.options.EnableReconnect {
		reconnect, _ := json.Marshal(daemon.options.ReconnectTime)
		if err := context.connection.write(append([]byte{rec.SetReconnect},
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
		case rec.Input:
			if !daemon.session[rx.key].options.PermitWrite {
				if len(rx.p) == 2 && (rx.p[1] == 3 || rx.p[1] == 4) {
					//close conn by ctrl-c/ctrl-d
					daemon.session[rx.key].context.close(rx.key)
				}
				break
			}

			_, err = context.pty.Write(rx.p[1:])
			if err != nil {
				return
			}

		case rec.Ping:
			if errs := context.write([]byte{rec.Pong}); len(errs) > 0 {
				for _, e := range errs {
					glog.Errorln(e.err.Error())
					context.close(e.key)
				}
				if len(*context.connections) == 0 {
					return
				}
			}
		case rec.ResizeTerminal:
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
				context.fd,
				syscall.TIOCSWINSZ,
				uintptr(unsafe.Pointer(&window)),
			)
			context.record(rx.p)

		default:
			glog.Errorln("Unknown message type")
			return
		}
	}
}
