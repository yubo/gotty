package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	"github.com/yubo/gotool/flags"
	"github.com/yubo/gotty/tty"
)

var (
	configFile string
	options    tty.Options
	version    bool
)

func init() {
	flags.CommandLine.Usage = "Share your terminal as a web application"
	flags.CommandLine.Name = "gotty"

	flag.BoolVar(&version, "version", false, "Print version")

	// daemon
	cmd := flags.NewCommand("daemon", "Enable daemon mode", flag.ExitOnError)
	cmd.StringVar(&configFile, "c", "/etc/gotty/gotty.conf", "Config file path")

	// ps
	cmd = flags.NewCommand("ps", "List session", flag.ExitOnError)

	// exec
	cmd = flags.NewCommand("exec", "Run a command in a new pty", flag.ExitOnError)
	cmd.BoolVar(&options.PermitWrite, "permit-write",
		tty.DefaultOptions.PermitWrite, "Permit clients to write to the TTY (BE CAREFUL)")

	// close
	cmd = flags.NewCommand("close", "Close a pty", flag.ExitOnError)

	// version
	cmd = flags.NewCommand("version", "Show the gotty version information", flag.ExitOnError)

	/*
		cmd.StringVar(&options.Address, "address",
			tty.DefaultOptions.Address, "IP address to listen")
		cmd.StringVar(&options.Port, "port",
			tty.DefaultOptions.Port, "Port number to listen")
		cmd.BoolVar(&options.PermitWrite, "permit-write",
			tty.DefaultOptions.PermitWrite, "Permit clients to write to the TTY (BE CAREFUL)")
		cmd.StringVar(&options.Credential, "credential",
			tty.DefaultOptions.Credential, "Credential for Basic Authentication (ex: user:pass, default disabled)")
		cmd.BoolVar(&options.EnableBasicAuth, "base-auth",
			tty.DefaultOptions.EnableBasicAuth, "Enable base auth")
		cmd.BoolVar(&options.EnableRandomUrl, "random-url",
			tty.DefaultOptions.EnableRandomUrl, "Add a random string to the URL")
		cmd.IntVar(&options.RandomUrlLength, "random-url-length",
			tty.DefaultOptions.RandomUrlLength, "Random URL length")
		cmd.BoolVar(&options.EnableTLS, "tls",
			tty.DefaultOptions.EnableTLS, "Enable TLS/SSL")
		cmd.BoolVar(&options.EnableTLSClientAuth, "tls-client",
			tty.DefaultOptions.EnableTLSClientAuth, "Enable client TLS/SSL")
		cmd.StringVar(&options.TLSCrtFile, "tls-crt",
			tty.DefaultOptions.TLSCrtFile, "TLS/SSL certificate file path")
		cmd.StringVar(&options.TLSKeyFile, "tls-key",
			tty.DefaultOptions.TLSKeyFile, "TLS/SSL key file path")
		cmd.StringVar(&options.TLSCACrtFile, "tls-ca-crt",
			tty.DefaultOptions.TLSCACrtFile, "TLS/SSL CA certificate file for client certifications")
		cmd.StringVar(&options.IndexFile, "index",
			tty.DefaultOptions.IndexFile, "Custom index.html file")
		cmd.StringVar(&options.TitleFormat, "title-format",
			tty.DefaultOptions.TitleFormat, "Title format of browser window")
		cmd.BoolVar(&options.EnableReconnect, "reconnect",
			tty.DefaultOptions.EnableReconnect, "Enable reconnection")
		cmd.IntVar(&options.ReconnectTime, "reconnect-time",
			tty.DefaultOptions.ReconnectTime, "Time to reconnect")
		cmd.BoolVar(&options.Once, "once",
			tty.DefaultOptions.Once, "Accept only one client and exit on disconnection")
		cmd.BoolVar(&options.PermitArguments, "permit-arguments",
			tty.DefaultOptions.PermitArguments, "Permit clients to send command line arguments in URL (e.g. http://example.com:8080/?arg=AAA&arg=BBB)")
		cmd.IntVar(&options.CloseSignal, "close-signal",
			tty.DefaultOptions.CloseSignal, "Signal sent to the command process when gotty close it (default: SIGHUP)")
	*/

}

func main() {
	flags.Parse() //for glog

	if version {
		fmt.Fprintf(os.Stdout, "%s\n", tty.Version)
		os.Exit(0)
	}

	_, err := os.Stat(tty.ExpandHomeDir(configFile))
	if !os.IsNotExist(err) {
		if err := tty.ApplyConfigFile(&options, configFile); err != nil {
			exit(err, 2)
		}
	}

	cmd := flags.CommandLine.Cmd
	if cmd == nil {
		// miss command
		flags.Usage()
		os.Exit(1)
	}

	switch cmd.Name {
	case "daemon":
		if err := tty.CheckConfig(&options); err != nil {
			exit(err, 6)
		}

		err := tty.Init(cmd.Flag.Args(), &options)
		if err != nil {
			exit(err, 3)
		}

		registerSignals()
		if err = tty.Run(); err != nil {
			exit(err, 4)
		}
	case "ps":
		fmt.Fprintf(os.Stdout, "ps\n")
	case "exec":
		fmt.Fprintf(os.Stdout, "exec\n")
	case "close":
		fmt.Fprintf(os.Stdout, "close\n")
	case "version":
		fmt.Fprintf(os.Stdout, "%s\n", tty.Version)
	default:
		flags.Usage()
		os.Exit(1)
	}
	return
}

func exit(err error, code int) {
	if err != nil {
		glog.Errorln(err)
	}
	os.Exit(code)
}

func registerSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(
		sigChan,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	go func() {
		for {
			s := <-sigChan
			switch s {
			case syscall.SIGINT, syscall.SIGTERM:
				if tty.Exit() {
					glog.Infoln("Send ^C to force exit.")
				} else {
					os.Exit(5)
				}
			}
		}
	}()
}
