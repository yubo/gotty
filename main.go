package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	"github.com/yubo/gotool/flags"
	"github.com/yubo/gotty/app"
)

var (
	configFile string
	options    app.Options
	version    bool
	daemon     *flag.FlagSet
)

func init() {
	flags.CommandLine.Usage = "Share your terminal as a web application"
	flags.CommandLine.Name = "gotty"

	flag.BoolVar(&version, "version", false, "Print version")

	//daemon
	daemon = flags.NewCommand("daemon", "Enable daemon mode", flag.ExitOnError)
	daemon.StringVar(&configFile, "config", "/etc/gotty/gotty.conf", "Config file path")
	daemon.StringVar(&options.Address, "address",
		app.DefaultOptions.Address, "IP address to listen")
	daemon.StringVar(&options.Port, "port",
		app.DefaultOptions.Port, "Port number to listen")
	daemon.BoolVar(&options.PermitWrite, "permit-write",
		app.DefaultOptions.PermitWrite, "Permit clients to write to the TTY (BE CAREFUL)")
	daemon.StringVar(&options.Credential, "credential",
		app.DefaultOptions.Credential, "Credential for Basic Authentication (ex: user:pass, default disabled)")
	daemon.BoolVar(&options.EnableBasicAuth, "base-auth",
		app.DefaultOptions.EnableBasicAuth, "Enable base auth")
	daemon.BoolVar(&options.EnableRandomUrl, "random-url",
		app.DefaultOptions.EnableRandomUrl, "Add a random string to the URL")
	daemon.IntVar(&options.RandomUrlLength, "random-url-length",
		app.DefaultOptions.RandomUrlLength, "Random URL length")
	daemon.BoolVar(&options.EnableTLS, "tls",
		app.DefaultOptions.EnableTLS, "Enable TLS/SSL")
	daemon.BoolVar(&options.EnableTLSClientAuth, "tls-client",
		app.DefaultOptions.EnableTLSClientAuth, "Enable client TLS/SSL")
	daemon.StringVar(&options.TLSCrtFile, "tls-crt",
		app.DefaultOptions.TLSCrtFile, "TLS/SSL certificate file path")
	daemon.StringVar(&options.TLSKeyFile, "tls-key",
		app.DefaultOptions.TLSKeyFile, "TLS/SSL key file path")
	daemon.StringVar(&options.TLSCACrtFile, "tls-ca-crt",
		app.DefaultOptions.TLSCACrtFile, "TLS/SSL CA certificate file for client certifications")
	daemon.StringVar(&options.IndexFile, "index",
		app.DefaultOptions.IndexFile, "Custom index.html file")
	daemon.StringVar(&options.TitleFormat, "title-format",
		app.DefaultOptions.TitleFormat, "Title format of browser window")
	daemon.BoolVar(&options.EnableReconnect, "reconnect",
		app.DefaultOptions.EnableReconnect, "Enable reconnection")
	daemon.IntVar(&options.ReconnectTime, "reconnect-time",
		app.DefaultOptions.ReconnectTime, "Time to reconnect")
	daemon.BoolVar(&options.Once, "once",
		app.DefaultOptions.Once, "Accept only one client and exit on disconnection")
	daemon.BoolVar(&options.PermitArguments, "permit-arguments",
		app.DefaultOptions.PermitArguments, "Permit clients to send command line arguments in URL (e.g. http://example.com:8080/?arg=AAA&arg=BBB)")
	daemon.IntVar(&options.CloseSignal, "close-signal",
		app.DefaultOptions.CloseSignal, "Signal sent to the command process when gotty close it (default: SIGHUP)")

}

func main() {
	flags.Parse() //for glog

	if version {
		fmt.Fprintf(os.Stderr, "%s\n", app.Version)
		os.Exit(0)
	}

	_, err := os.Stat(app.ExpandHomeDir(configFile))
	if !os.IsNotExist(err) {
		if err := app.ApplyConfigFile(&options, configFile); err != nil {
			exit(err, 2)
		}
	}

	// overlay configFile
	flags.Parse()

	if err := app.CheckConfig(&options); err != nil {
		exit(err, 6)
	}

	app, err := app.New(daemon.Args(), &options)
	if err != nil {
		exit(err, 3)
	}

	registerSignals(app)
	if err = app.Run(); err != nil {
		exit(err, 4)
	}
}

func exit(err error, code int) {
	if err != nil {
		glog.Errorln(err)
	}
	os.Exit(code)
}

func registerSignals(app *app.App) {
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
				if app.Exit() {
					glog.Infoln("Send ^C to force exit.")
				} else {
					os.Exit(5)
				}
			}
		}
	}()
}
