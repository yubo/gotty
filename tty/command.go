package tty

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/yubo/gotool/flags"
)

func init() {
	flags.CommandLine.Usage = "Share your terminal as a web application"
	flags.CommandLine.Name = "gotty"

	// daemon
	cmd := flags.NewCommand("daemon", "Enable daemon mode",
		daemon_handle, flag.ExitOnError)
	cmd.StringVar(&configFile, "c",
		"/etc/gotty/gotty.conf", "Config file path")

	// exec
	cmd = flags.NewCommand("exec", "Run a command in a new pty",
		exec_handle, flag.ExitOnError)
	cmd.BoolVar(&CmdOpt.PermitWrite, "w", DefaultCmdOptions.PermitWrite,
		"Permit clients to write to the TTY (BE CAREFUL)")
	cmd.BoolVar(&CmdOpt.PermitShare, "share", DefaultCmdOptions.PermitShare,
		"Permit clients to join the TTY (BE CAREFULL with -w)")
	cmd.StringVar(&CmdOpt.Name, "name", "", "set tty session name")
	cmd.StringVar(&CmdOpt.Addr, "addr", "0.0.0.0", "allow ipv4 address")

	// ps
	cmd = flags.NewCommand("ps", "List session", ps_handle, flag.ExitOnError)
	cmd.BoolVar(&CmdOpt.ShowAll, "a", false,
		"Show all session(default show just "+
			CONN_S_CONNECTED+"/"+CONN_S_WAITING+")")

	// attach
	cmd = flags.NewCommand("attach", "Attach to a seesion",
		attach_handle, flag.ExitOnError)
	cmd.BoolVar(&CmdOpt.PermitWrite, "w", DefaultCmdOptions.PermitWrite,
		"Permit clients to write to the TTY (BE CAREFUL)")
	cmd.StringVar(&CmdOpt.Name, "name", "", "set the new session name")
	cmd.StringVar(&CmdOpt.Addr, "addr", "0.0.0.0", "allow ipv4 address")
	cmd.StringVar(&CmdOpt.SName, "sname", "", "attach to the session name")
	cmd.StringVar(&CmdOpt.SAddr, "saddr", "0.0.0.0", "attach to the session addr")

	// close
	cmd = flags.NewCommand("close", "Close a pty", close_handle, flag.ExitOnError)
	cmd.StringVar(&CmdOpt.Name, "name", "", "set tty session name")
	cmd.StringVar(&CmdOpt.Addr, "addr", "0.0.0.0", "allow ipv4 address")

	// version
	cmd = flags.NewCommand("version",
		"Show the gotty version information", version_handle, flag.ExitOnError)

}

func daemon_handle(arg interface{}) {
	args := arg.(*CallOptions).Args

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

func exec_handle(arg interface{}) {
	var key ConnKey
	opt := arg.(*CallOptions)
	if err := Call("Cmd.Exec", opt, &key); err != nil {
		fmt.Fprintf(os.Stderr, "exec %v \n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "exec successful, name:\"%s\" addr:\"%s\"\n",
		key.Name, key.Addr)
}

func ps_handle(arg interface{}) {
	opt := arg.(*CallOptions)
	ret := []Session_info{}
	now := time.Now().Unix()

	if err := Call("Cmd.Ps", opt, &ret); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "%-15s %15s %3s %6s %10s %20s %20s %8s\n",
		"Name", "PName", "", "Method", "Status",
		"Command", "RemoteAddr", "ConnTime")
	for _, s := range ret {
		if !CmdOpt.ShowAll && s.Status == CONN_S_CLOSED {
			continue
		}
		if s.ConnTime > 0 {
			s.ConnTime = now - s.ConnTime
		}
		fmt.Fprintf(os.Stdout, "%-15s %15s %3d %6s %10s %20s %20s %8d\n",
			s.Key, s.PKey, s.LinkNb, s.Method, s.Status,
			strings.Join(s.Command, " "), s.RemoteAddr, s.ConnTime)
	}
}

func attach_handle(arg interface{}) {
	var key ConnKey
	opt := arg.(*CallOptions)
	if err := Call("Cmd.Attach", opt, &key); err != nil {
		fmt.Fprintf(os.Stderr, "attach %v \n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "attach successful, name:\"%s\" addr:\"%s\"\n",
		key.Name, key.Addr)
}

func close_handle(arg interface{}) {
	opt := arg.(*CallOptions)
	err := Call("Cmd.Close", opt, nil)
	fmt.Fprintf(os.Stdout, "close %v\n", opt, err)
}

func version_handle(arg interface{}) {
	fmt.Fprintf(os.Stdout, "%s\n", Version)
}

func exit(err error, code int) {
	if err != nil {
		glog.Errorln(err)
	}
	os.Exit(code)
}
