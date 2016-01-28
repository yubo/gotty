package tty

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yubo/gotool/flags"
)

func init() {
	// exec
	cmd := flags.NewCommand("exec", "Run a command in a new pty",
		exec_handle, flag.ExitOnError)
	cmd.BoolVar(&CmdOpt.PermitWrite, "w", DefaultCmdOptions.PermitWrite,
		"Permit clients to write to the TTY (BE CAREFUL)")
	cmd.BoolVar(&CmdOpt.PermitShare, "share", DefaultCmdOptions.PermitShare,
		"Permit clients to join the TTY (BE CAREFULL with -w)")
	cmd.StringVar(&CmdOpt.Name, "name", "", "set tty session name")
	cmd.StringVar(&CmdOpt.Addr, "addr", "0.0.0.0", "allow ipv4 address")

	// ps
	cmd = flags.NewCommand("ps", "List session", ps_handle, flag.ExitOnError)

	// attach
	cmd = flags.NewCommand("attach", "Attach to a seesion",
		attach_handle, flag.ExitOnError)
	cmd.BoolVar(&CmdOpt.PermitWrite, "w", DefaultCmdOptions.PermitWrite,
		"Permit clients to write to the TTY (BE CAREFUL)")
	cmd.StringVar(&CmdOpt.Name, "name", "", "set tty session name")
	cmd.StringVar(&CmdOpt.Addr, "addr", "0.0.0.0", "allow ipv4 address")

	// close
	cmd = flags.NewCommand("close", "Close a pty", close_handle, flag.ExitOnError)
	cmd.StringVar(&CmdOpt.Name, "name", "", "set tty session name")
	cmd.StringVar(&CmdOpt.Addr, "addr", "0.0.0.0", "allow ipv4 address")

	// version
	cmd = flags.NewCommand("version",
		"Show the gotty version information", version_handle, flag.ExitOnError)

}

func exec_handle(arg interface{}) {
	var name string
	opt := arg.(CallOptions)
	if err := Call("Cmd.Exec", opt, &name); err != nil {
		fmt.Fprintf(os.Stderr, "exec %v \n", err)
	}
	fmt.Fprintf(os.Stdout, "exec successful, name:%s\n", name)
}

func ps_handle(arg interface{}) {
	opt := arg.(CallOptions)
	ret := []Session_info{}
	now := time.Now().Unix()

	if err := Call("Cmd.Ps", opt, &ret); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}

	fmt.Fprintf(os.Stdout, "%-20s %10s %-20s %20s %10s\n",
		"Name", "Addr", "Command", "RemoteAddr", "ConnTime")
	for _, s := range ret {
		if s.ConnTime > 0 {
			s.ConnTime = now - s.ConnTime
		}
		fmt.Fprintf(os.Stdout, "%-20s %10s %-20s %20s %10d\n",
			s.Name, s.Addr, strings.Join(s.Command, " "),
			s.RemoteAddr, s.ConnTime)
	}
}

func attach_handle(arg interface{}) {
	opt := arg.(CallOptions)
	err := Call("Cmd.Attach", opt, nil)
	fmt.Fprintf(os.Stdout, "attach %v\n", opt, err)
}

func close_handle(arg interface{}) {
	opt := arg.(CallOptions)
	err := Call("Cmd.Close", opt, nil)
	fmt.Fprintf(os.Stdout, "close %v\n", opt, err)
}

func version_handle(arg interface{}) {
	fmt.Fprintf(os.Stdout, "%s\n", Version)
}
