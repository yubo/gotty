package main

import (
	"os"

	"github.com/yubo/gotool/flags"
	"github.com/yubo/gotty/tty"
)

func main() {

	tty.Parse()

	cmd := flags.CommandLine.Cmd
	if cmd != nil && cmd.Action != nil {
		cmd.Action(tty.CallOptions{Opt: tty.CmdOpt, Args: cmd.Flag.Args()})
		return
	} else {
		flags.Usage()
		os.Exit(1)
	}
}
