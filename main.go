package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yubo/gotool/flags"
	"github.com/yubo/gotty/tty"
)

func main() {

	tty.Parse()

	cmd := flags.CommandLine.Cmd
	if cmd != nil && cmd.Action != nil {
		opts := &tty.CallOptions{Opt: tty.CmdOpt, Args: cmd.Flag.Args()}
		cmd.Action(opts)
	} else {
		// gotty-client
		if len(flag.Args()) == 0 {
			flags.Usage()
			os.Exit(1)
		}

		if err := tty.GottyClient(tty.GlobalOpt.SkipTlsVerify,
			flag.Args()[0]); err != nil {
			//flags.Usage()
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}
}
