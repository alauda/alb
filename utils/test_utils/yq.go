package test_utils

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Yq struct {
	Base string
}

func YqDo(raw string, cmd string) (string, error) {
	// only use for test
	raw = strings.TrimSpace(raw)
	base, err := os.MkdirTemp("", "yq")
	if err != nil {
		return "", err
	}
	p := base + "/" + "x.yaml"
	err = os.WriteFile(p, []byte(raw), 0o666)
	if err != nil {
		return "", err
	}
	sh := fmt.Sprintf(`#!/bin/bash
cat %s | %s
`, p, cmd)
	os.WriteFile(base+"/x.sh", []byte(sh), 0o666)
	sh_p := base + "/x.sh"
	out, err := exec.Command("bash", sh_p).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("eval %s fail %v", sh_p, err)
	}
	return strings.TrimSpace(string(out)), nil
}
