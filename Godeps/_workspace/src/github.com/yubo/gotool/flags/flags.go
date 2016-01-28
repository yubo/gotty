/*
 * yubo@yubo.org
 * 2016-01-26
 */
package flags

import (
	"flag"
	"fmt"
	"os"
)

type flag_t struct {
	Name   string
	usage  string
	h      bool
	Flag   *flag.FlagSet
	Action func(args interface{})
}

type flags_t struct {
	flags []*flag_t
	Usage string
	Name  string
	Cmd   *flag_t
	h     bool
}

var CommandLine flags_t

func NewCommand(name, usage string, action func(args interface{}),
	errorHandling flag.ErrorHandling) *flag.FlagSet {

	f := &flag_t{
		Name:   name,
		usage:  usage,
		Flag:   flag.NewFlagSet(name, errorHandling),
		Action: action,
	}
	f.Flag.Usage = f.Usage
	CommandLine.flags = append(CommandLine.flags, f)

	f.Flag.BoolVar(&f.h, "h", false, "Print usage")
	return f.Flag
}

func Parse() {
	CommandLine.Parse(os.Args[1:])
}

func (f *flags_t) Parse(args []string) (err error) {
	for i, arg := range args {
		for _, f := range CommandLine.flags {
			if arg == f.Name {
				CommandLine.Cmd = f
				if err = flag.CommandLine.Parse(args[0:i]); err != nil {
					return
				}
				if err = f.Flag.Parse(args[i+1:]); err != nil {
					return
				}
				if f.h {
					f.Usage()
					os.Exit(0)
				}
			}
		}
	}
	err = flag.CommandLine.Parse(args)
	if CommandLine.h {
		flag.Usage()
		os.Exit(0)
	}
	return
}

func (f *flag_t) Usage() {
	fmt.Fprintf(os.Stderr,
		"Usage: %s [OPTIONS] %s [ARG...]\n", os.Args[0], f.Name)
	fmt.Fprintf(os.Stderr, "\n%s\n\n", f.usage)
	f.Flag.PrintDefaults()
}

func Usage() {
	fmt.Fprintf(os.Stderr,
		"Usage: %s [OPTIONS] COMMAND [arg...]\n", os.Args[0])
	if len(CommandLine.Usage) > 0 {
		fmt.Fprintf(os.Stderr, "\n%s\n", CommandLine.Usage)
	}
	fmt.Fprintf(os.Stderr, "\nOptions:\n\n")

	flag.PrintDefaults()

	fmt.Fprintf(os.Stderr, "\nCommands:\n")
	for _, f := range CommandLine.flags {
		fmt.Fprintf(os.Stderr, "    %-9s %s\n", f.Name, f.usage)
	}

	fmt.Fprintf(os.Stderr,
		"\nRun '%s COMMAND --help' for more information on a command.\n",
		os.Args[0])
}

func init() {
	flag.Usage = Usage
	flag.BoolVar(&CommandLine.h, "h", false, "Print usage")
}
