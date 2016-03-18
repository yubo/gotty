package tty

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/golang/glog"
	"github.com/kr/pty"
)

// Start assigns a pseudo-terminal tty os.File to c.Stdin, c.Stdout,
// and c.Stderr, calls c.Start, and returns the File of the tty's
// corresponding pty.
func ptyStart(c *exec.Cmd) (p *os.File, err error) {
	p, tty, err := pty.Open()
	if err != nil {
		return nil, err
	}
	defer tty.Close()
	c.Stdout = tty
	c.Stdin = tty
	c.Stderr = tty
	c.SysProcAttr = &syscall.SysProcAttr{Setctty: true, Setsid: true}

	if daemon.user != nil {
		uid, e1 := strconv.Atoi(daemon.user.Uid)
		gid, e2 := strconv.Atoi(daemon.user.Gid)
		if e1 == nil && e2 == nil {
			c.SysProcAttr.Credential = &syscall.Credential{
				Uid: uint32(uid),
				Gid: uint32(gid),
			}
			c.Dir = daemon.user.HomeDir

			c.Env = make([]string, len(GlobalOpt.Env)+1)
			i := 0
			for k, v := range GlobalOpt.Env {
				c.Env[i] = fmt.Sprintf("%s=%s", k, v)
				i += 1
			}
			c.Env[i] = fmt.Sprintf("HOME=%s", c.Dir)

			glog.V(3).Infof("uid:%d gid:%d env:\n%s\n", uid, gid, strings.Join(c.Env, "\n"))
		}
	}

	err = c.Start()
	if err != nil {
		p.Close()
		return nil, err
	}
	return p, err
}
