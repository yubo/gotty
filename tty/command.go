package tty

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/yubo/gotool/flags"
	"github.com/yubo/gotty/rec"
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
	cmd.BoolVar(&CmdOpt.PermitWrite, "w",
		DefaultCmdOptions.PermitWrite,
		"Permit clients to write to the TTY (BE CAREFUL)")
	cmd.BoolVar(&CmdOpt.PermitShare, "share",
		DefaultCmdOptions.PermitShare,
		"Allow muilt-clients to join the TTY (addr should be network or muilt-addr)")
	cmd.BoolVar(&CmdOpt.PermitShareWrite, "share-write",
		DefaultCmdOptions.PermitShareWrite,
		"Permit joined clients to wirte to the TTY if this TTY writable(BE CAREFULL)")
	cmd.StringVar(&CmdOpt.Name, "name", "", "set tty session name")
	cmd.StringVar(&CmdOpt.Addr, "addr", DefaultCmdOptions.Addr,
		"allow access nets, e.g. 127.0.0.1,192.168.0.0/24")
	cmd.BoolVar(&CmdOpt.Rec, "rec",
		DefaultCmdOptions.Rec, "record tty and save")

	// ps
	cmd = flags.NewCommand("ps", "List session",
		ps_handle, flag.ExitOnError)
	cmd.BoolVar(&CmdOpt.All, "a", DefaultCmdOptions.All,
		"Show all session(default show just "+
			CONN_S_CONNECTED+"/"+CONN_S_WAITING+")")

	// attach
	cmd = flags.NewCommand("attach", "Attach to a seesion",
		attach_handle, flag.ExitOnError)
	cmd.BoolVar(&CmdOpt.PermitWrite, "w",
		DefaultCmdOptions.PermitWrite,
		"Permit clients to write to the TTY (BE CAREFUL)")
	cmd.StringVar(&CmdOpt.Name, "name", "", "set the new session name")
	cmd.StringVar(&CmdOpt.Addr, "addr",
		DefaultCmdOptions.Addr, "allow ipv4 address")
	cmd.StringVar(&CmdOpt.SName, "sname", "", "attach to the session name")
	cmd.StringVar(&CmdOpt.SAddr, "saddr",
		DefaultCmdOptions.Addr, "attach to the session addr")

	// close
	cmd = flags.NewCommand("close", "Close a pty/session",
		close_handle, flag.ExitOnError)
	cmd.StringVar(&CmdOpt.Name, "name", "", "set tty session name")
	cmd.StringVar(&CmdOpt.Addr, "addr", DefaultCmdOptions.Addr,
		"allow access nets, e.g. 127.0.0.1,192.168.0.0/24")
	cmd.BoolVar(&CmdOpt.All, "a", false,
		"Close all session use the same pty(default close just a seesion)")

	// play
	cmd = flags.NewCommand("play",
		"replay recorded file in a webtty",
		play_handle, flag.ExitOnError)
	cmd.StringVar(&CmdOpt.Name, "name", "", "set tty session name")
	cmd.StringVar(&CmdOpt.Addr, "addr",
		DefaultCmdOptions.Addr,
		"allow access nets, e.g. 127.0.0.1,192.168.0.0/24")
	cmd.StringVar(&CmdOpt.RecId, "id", "", "replay tty id")
	cmd.Float64Var(&CmdOpt.Speed, "speed",
		DefaultCmdOptions.Speed, "replay speed")
	cmd.BoolVar(&CmdOpt.Repeat, "repeat",
		DefaultCmdOptions.Repeat, "replay repeat")
	cmd.BoolVar(&CmdOpt.PermitShare, "share",
		DefaultCmdOptions.PermitShare,
		"Allow muilt-clients to join the TTY (addr should be network or muilt-addr)")
	cmd.Int64Var(&CmdOpt.MaxWait, "max-wait",
		DefaultCmdOptions.MaxWait,
		"Reduce recorded terminal inactivity to max <sec> second")

	cmd = flags.NewCommand("convert",
		"convert seesion id to asciicast format(json)", convert_handle, flag.ExitOnError)
	cmd.StringVar(&CmdOpt.SName, "i", "", "convert tty, input filename or seesion id")
	cmd.StringVar(&CmdOpt.Name, "o", "out.json", "convert tty, output filename")
	cmd.Int64Var(&CmdOpt.MaxWait, "max-wait",
		DefaultCmdOptions.MaxWait,
		"Reduce recorded terminal inactivity to max <sec> second")

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

func play_handle(arg interface{}) {
	var info Session_info
	opt := arg.(*CallOptions)
	if err := Call("Cmd.Play", opt, &info); err != nil {
		fmt.Fprintf(os.Stderr, "play failed: %v \n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "play successful, name:\"%s\" addr:\"%s\" recid:\"%s\"\n",
		info.Key.Name, info.Key.Addr, info.RecId)
}

func exec_handle(arg interface{}) {
	var info Session_info
	opt := arg.(*CallOptions)
	if err := Call("Cmd.Exec", opt, &info); err != nil {
		fmt.Fprintf(os.Stderr, "exec %v \n", err)
		os.Exit(1)
	}
	if opt.Opt.Rec {
		fmt.Fprintf(os.Stdout, "exec successful, name:\"%s\" addr:\"%s\" recid:\"%s\"\n",
			info.Key.Name, info.Key.Addr, info.RecId)
	} else {
		fmt.Fprintf(os.Stdout, "exec successful, name:\"%s\" addr:\"%s\"\n",
			info.Key.Name, info.Key.Addr)
	}
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
		if !CmdOpt.All && s.Status == CONN_S_CLOSED {
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
	if err := Call("Cmd.Close", opt, nil); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	} else {
		fmt.Fprintf(os.Stdout, "close successful\n")
	}
}

func convert_handle(arg interface{}) {
	opt := arg.(*CallOptions)
	filename := expandHomeDir(opt.Opt.SName)

	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		filename = expandHomeDir(GlobalOpt.RecFileDir) +
			"/" + opt.Opt.SName
		_, err := os.Stat(filename)
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "file/RecID(%s) is not exsit\n",
				opt.Opt.SName)
			os.Exit(1)
		}
	}
	rec.Convert(filename, opt.Opt.Name, opt.Opt.MaxWait)

	fmt.Fprintf(os.Stdout, "%s\n", Version)
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
