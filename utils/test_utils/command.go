package test_utils

import (
	"fmt"
	"os/exec"
	"strings"
)

type Cmd struct {
	logcmd bool
	logout bool
}

func NewCmd() *Cmd {
	return &Cmd{logcmd: true, logout: true}
}

func (c *Cmd) Logout(logout bool) *Cmd {
	c.logout = logout
	return c
}

func (c *Cmd) Call(name string, cmds ...string) (string, error) {
	cmdStr := name + " " + strings.Join(cmds, " ")
	if c.logcmd {
		fmt.Printf("call: %s\n", cmdStr)
	}
	cmd := exec.Command(name, cmds...)
	stdout, err := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	if err != nil {
		return "", err
	}
	if err = cmd.Start(); err != nil {
		return "", err
	}
	out := ""
	for {
		tmp := make([]byte, 1024)
		n, err := stdout.Read(tmp)
		if c.logout {
			fmt.Print(string(tmp))
		}
		out = out + string(tmp[0:n])
		if err != nil {
			break
		}
	}
	err = cmd.Wait()
	return string(out), err
}

func Command(name string, cmds ...string) (string, error) {
	return NewCmd().Call(name, cmds...)
}
