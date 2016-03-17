#GoTTY - Share your terminal as a web application

[![wercker status](https://app.wercker.com/status/fff04a43b1cdb0ea190ab9578eceeb17/s/master "wercker status")](https://app.wercker.com/project/bykey/fff04a43b1cdb0ea190ab9578eceeb17)
[![MIT License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat-square)][license]

[release]: https://github.com/yubo/gotty/releases
[wercker]: https://app.wercker.com/project/bykey/fff04a43b1cdb0ea190ab9578eceeb17
[license]: https://github.com/yubo/gotty/blob/master/LICENSE

GoTTY Fork from https://github.com/yudai/gotty.git. A simple command line tool that turns your CLI tools into web applications. 

create a session
[![demo](https://asciinema.org/a/4itvykdx2a9m1l4khgqm8jner.png)](https://asciinema.org/a/4itvykdx2a9m1l4khgqm8jner?autoplay=1)

record/replay a session
[![demo](https://asciinema.org/a/cpkhygqp9xun8a7p7nuuqufou.png)](https://asciinema.org/a/cpkhygqp9xun8a7p7nuuqufou?autoplay=1)

# Installation


Download the latest binary file from the [Releases](https://github.com/yubo/gotty/releases) page.

(`darwin_amd64.tar.gz` is for Mac OS X users)

#### Quick Start with demo webpage

![](resources/gotty_demo.png?raw-true)

```shell
sudo mkdir -p /var/lib/gotty
sudo ./gotty deamon
#open the url with an browser
#http://127.0.0.1:8080/demo
```

#### build from source
```shell
#get source
$go get github.com/yubo/gotty
$cd $GOPATH/src/github.com/yubo/gotty

#build prepare
$make tools

#build gotty
$make

#run
$make run
```

# Usage
```
Usage: ./gotty [OPTIONS] COMMAND [arg...]

Usage: ./gotty [OPTIONS] COMMAND [arg...]
       ./gotty [OPTIONS] URL
```

## Options
```
Options:

  -D    debug
  -alsologtostderr
        log to standard error as well as files
  -c string
        Config file path (default "/etc/gotty/gotty.conf")
  -h    Print usage
  -log_backtrace_at value
        when logging hits line file:N, emit a stack trace (default :0)
  -log_dir string
        If non-empty, write log files in this directory
  -logtostderr
        log to standard error instead of files
  -skip-tls-verify
        Skip TLS verify
  -stderrthreshold value
        logs at or above this threshold go to stderr
  -v value
        log level for V logs
  -vmodule value
        comma-separated list of pattern=N settings for file-filtered logging

Commands:
    daemon    Enable daemon mode
    exec      Run a command in a new pty
    ps        List session
    attach    Attach to a seesion
    close     Close a pty/session
    play      replay recorded file in a webtty
    convert   convert seesion id to asciicast format(json)
    version   Show the gotty version information
```

#### server deamon
```shell
$gotty deamon
# or
$sudo service gotty start
```

#### config
```shell
### /etc/gotty/gotty.conf
// Listen at port 9000 by default
port = "9000"

// Enable TSL/SSL by default
//enable_tls = true

// hterm preferences
// Smaller font and a little bit bluer background color

preferences {
    font_size = 14
    font_family = "Consolas, 'Liberation Mono', Menlo, Courier, monospace"
    background_color = "rgb(16, 16, 32)"
}
```

#### create a session

Server side
```shell
#just allow 127.0.0.1 access
$gotty exec -name abc -w -addr 127.0.0.1 /bin/bash
#open access
$gotty exec -name abc -addr 0.0.0.0/0 top
#allow 127.0.0.0/8, 172.16.0.0/16 to access the tty, but can't write(allow with -a)
$gotty exec -w -share -name abc -addr=127.0.0.0/8,172.16.0.0/16 /bin/bash
```

Client side
```shell
$gotty "http://127.0.0.1:9000/?name=abc&addr=0.0.0.0/0"
```

#### attach a session

Server side
```shell
$gotty attach -name bbb -sname abc -w
```

Client side
```shell
#Open the URL on Web browser
http://127.0.0.1:9000/?name=bbb&addr=0.0.0.0/0
# or
$gotty "http://127.0.0.1:9000/?name=bbb&addr=0.0.0.0/0"
```

#### record/replay session
```shell
# add -rec to record session
$gotty exec -name abc -w -rec -addr 127.0.0.0/8 /bin/bash
exec successful, name:"abc" addr:"127.0.0.0/8" recid:"535086102"
# replay
$gotty play -name=abc -speed=2.0 -addr=127.0.0.0/8 -id=535086102
play successful, name:"abc" addr:"127.0.0.0/8" recid:"535086102"
# Open http://127.0.0.1:9000/?name=abc&addr=127.0.0.0/8
```

#### list session
```shell
$gotty ps -a
```

#### convert asciinema format
```shell
$gotty convert -i 535086102 -o out.json
```

#### close a pty/session
```shell
#Close all session use the same pty(name:abc,addr:0.0.0.0)
$gotty close -a -name abc
#Just close a session
$gotty close -name abc
```



### Security Options

By default, GoTTY doesn't allow clients to send any keystrokes or commands except terminal window resizing. When you want to permit clients to write input to the TTY, add the `-w` option. However, accepting input from remote clients is dangerous for most commands. When you need interaction with the TTY for some reasons, consider starting GoTTY with tmux or GNU Screen and run your command on it (see "Sharing with Multiple Clients" section for detail).

To restrict client access, you can use the `-c` option to enable the basic authentication. With this option, clients need to input the specified username and password to connect to the GoTTY server. Note that the credentical will be transmitted between the server and clients in plain text. For more strict authentication, consider the SSL/TLS client certificate authentication described below.

The `-r` option is a little bit casualer way to restrict access. With this option, GoTTY generates a random URL so that only people who know the URL can get access to the server.  

All traffic between the server and clients are NOT encrypted by default. When you send secret information through GoTTY, we strongly recommend you use the `-t` option which enables TLS/SSL on the session. By default, GoTTY loads the crt and key files placed at `~/.gotty.crt` and `~/.gotty.key`. You can overwrite these file paths with the `--tls-crt` and `--tls-key` options. When you need to generate a self-signed certification file, you can use the `openssl` command.

```sh
openssl req -x509 -nodes -days 9999 -newkey rsa:2048 -keyout ~/.gotty.key -out ~/.gotty.crt
```

(NOTE: For Safari uses, see [how to enable self-signed certificates for WebSockets](http://blog.marcon.me/post/24874118286/secure-websockets-safari) when use self-signed certificates)

For additional security, you can use the SSL/TLS client certificate authentication by providing a CA certificate file to the `--tls-ca-crt` option (this option requires the `-t` or `--tls` to be set). This option requires all clients to send valid client certificates that are signed by the specified certification authority.

## Sharing with Multiple Clients


### Quick Sharing on tmux

## Playing with Docker

```sh
$ gotty -w docker run -it --rm busybox
```

## Development

You can build a binary using the following commands. Windows is not supported now.

```sh
# Install tools
go get github.com/jteeuwen/go-bindata/...
go get github.com/tools/godep

# Restore libraries in Godeps
godep restore

# Build
make
```

### Command line client

* [gotty-client](https://github.com/moul/gotty-client): If you want to connect to GoTTY server from your terminal

### Terminal/SSH on Web Browsers

* [Secure Shell (Chrome App)](https://chrome.google.com/webstore/detail/secure-shell/pnhechapfaindjhompbnflcldabbghjo): If you are a chrome user and need a "real" SSH client on your web browser, perhaps the Secure Shell app is what you want
* [Wetty](https://github.com/krishnasrinivas/wetty): Node based web terminal (SSH/login)

### Terminal Sharing

* [tmate](http://tmate.io/): Forked-Tmux based Terminal-Terminal sharing
* [termshare](https://termsha.re): Terminal-Terminal sharing through a HTTP server
* [tmux](https://tmux.github.io/): Tmux itself also supports TTY sharing through SSH)

### Thanks

* https://github.com/lyuehh

# License

The MIT License
