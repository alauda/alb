package helper

import (
	"fmt"
	"os/exec"
	"strings"
)

func Command(name string, cmds ...string) (string, error) {
	cmdStr := name + " " + strings.Join(cmds, " ")
	fmt.Printf("call: %s\n", cmdStr)
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
		fmt.Print(string(tmp))
		out = out + string(tmp[0:n])
		if err != nil {
			break
		}
	}
	err = cmd.Wait()
	return string(out), err
}
