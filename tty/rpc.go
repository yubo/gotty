package tty

import (
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"time"

	"github.com/docker/docker/pkg/namesgenerator"
)

var (
	CmdOpt     CmdOptions
	configFile string
)

type Cmd int

func (c *Cmd) Ps(arg *CallOptions, reply *[]Session_info) error {
	for _, session := range tty.session {
		info := Session_info{
			Key:      session.key,
			Method:   session.method,
			Status:   session.status,
			Command:  session.command,
			ConnTime: session.connTime,
			LinkNb:   session.linkNb,
		}
		if session.linkTo != nil {
			info.PKey = session.linkTo.key
			info.Command = session.linkTo.command
		}
		if session.context != nil {
			info.RemoteAddr = session.context.request.RemoteAddr
		}
		*reply = append(*reply, info)
	}
	return nil
}

func (c *Cmd) Exec(arg *CallOptions, key *ConnKey) error {
	key.Addr = arg.Opt.Addr
	key.Name = arg.Opt.Name

	if key.Name == "" {
		if err := keyGenerator(key); err != nil {
			return err
		}
	}
	sess := &session{
		key:        *key,
		linkNb:     1,
		status:     CONN_S_WAITING,
		method:     CONN_M_EXEC,
		createTime: time.Now().Unix(),
		options:    &arg.Opt,
		command:    arg.Args,
	}
	return tty.newWaitingConn(sess)
}

func (c *Cmd) Attach(arg CallOptions, key *ConnKey) error {
	key.Name = arg.Opt.Name
	key.Addr = arg.Opt.Addr
	skey := ConnKey{
		Name: arg.Opt.SName,
		Addr: arg.Opt.SAddr,
	}

	if s, ok := tty.session[skey]; ok {
		if key.Name == "" {
			if err := keyGenerator(key); err != nil {
				return err
			}
		}

		s.Lock()
		defer s.Unlock()

		if s.status != CONN_S_CONNECTED {
			return fmt.Errorf("session{name:\"%s\", addr:\"%s\"} is not connected",
				s.key.Name, s.key.Addr)
		}

		s.linkNb += 1
		sess := &session{
			key:        *key,
			linkTo:     s,
			linkNb:     1,
			status:     CONN_S_WAITING,
			method:     CONN_M_ATTACH,
			createTime: time.Now().Unix(),
			options:    &arg.Opt,
			command:    arg.Args,
		}
		return tty.newWaitingConn(sess)
	} else {
		return fmt.Errorf("session{name:\"%s\", addr:\"%s\"} is not exist",
			skey.Name, skey.Addr)
	}
}
func (c *Cmd) Close(arg *CallOptions, keys *[]ConnKey) error {
	key := ConnKey{Name: arg.Opt.Name, Addr: arg.Opt.Addr}

	s, ok := tty.session[key]
	if !ok {
		return fmt.Errorf("session{name:\"%s\", addr:\"%s\"} is not exist",
			key.Name, key.Addr)
	}

	if !arg.Opt.All {
		*keys = append(*keys, key)
		s.context.close(key)
	} else {
		for k, _ := range *s.context.connections {
			*keys = append(*keys, k)
			s.context.close(k)
		}
	}
	return nil
}

func keyGenerator(key *ConnKey) error {
	for i := 0; i < 10; i++ {
		key.Name = namesgenerator.GetRandomName(i)
		if _, exsit := tty.session[*key]; exsit {
			continue
		}
		return nil
	}
	return errors.New("key generator fail")
}

func rpc_init() error {
	cmd := new(Cmd)

	rpc.Register(cmd)

	l, err := net.Listen("unix", UNIX_SOCKET)
	if err != nil {
		return err
	}
	go func() {
		var tempDelay time.Duration
		for {
			conn, err := l.Accept()
			if err != nil {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			tempDelay = 0
			go func() {
				rpc.ServeConn(conn)
			}()
		}
	}()
	return nil
}

func rpc_done() {
	os.Remove(UNIX_SOCKET)
}

func Call(serviceMethod string, args interface{}, reply interface{}) error {
	client, err := rpc.Dial("unix", UNIX_SOCKET)
	if err != nil {
		return err
	}
	return client.Call(serviceMethod, args, reply)
}
