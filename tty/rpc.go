package tty

import (
	"errors"
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

func (c *Cmd) Ps(arg CallOptions, reply *[]Session_info) error {
	for _, session := range tty.session {
		*reply = append(*reply, Session_info{
			Name:       session.key.name,
			Addr:       session.key.addr,
			Command:    session.command,
			RemoteAddr: session.remoteAddr,
			ConnTime:   session.connTime,
		})
	}
	return nil
}

func (c *Cmd) Exec(arg CallOptions, name *string) error {
	key := connKey{
		name: arg.Opt.Name,
		addr: arg.Opt.Addr,
	}
	if key.name == "" {
		if err := keyGenerator(&key); err != nil {
			return err
		}
	}
	*name = key.name
	sess := &session{
		key:        key,
		status:     CONN_S_WAITING,
		createTime: time.Now().Unix(),
		options:    &arg.Opt,
		command:    arg.Args,
	}

	return tty.newWaitingConn(sess)
}

func keyGenerator(key *connKey) error {
	for i := 0; i < 10; i++ {
		key.name = namesgenerator.GetRandomName(i)
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
