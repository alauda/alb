package test_utils

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
)

func PickStreamServerListen(cfgRaw string) ([]string, error) {
	return jqNgxConf(cfgRaw, `jq -r ".config[0].parsed|.[]|select(.directive==\"stream\").block|.[]|select(.directive==\"server\").block|.[]|select(.directive==\"listen\").args| join(\" \")"`)
}

func PickHttpServerListen(cfgRaw string) ([]string, error) {
	return jqNgxConf(cfgRaw, `jq -r ".config[0].parsed|.[]|select(.directive==\"http\").block|.[]|select(.directive==\"server\").block|.[]|select(.directive==\"listen\").args| join(\" \")"`)
}

// jqNgxConf use crossplane convert nginx.conf to json and use jq to query it.
// you need to install crossplane,jq,bash first.
// TODO a better way to parse nginx.conf.
func jqNgxConf(cfgRaw string, jq string) ([]string, error) {
	f, err := ioutil.TempFile("", "ngx-conf.")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	f.WriteString(cfgRaw)
	p := f.Name()
	shell := fmt.Sprintf(`crossplane parse %s | %s`, p, jq)
	out, err := exec.Command("bash", "-c", shell).CombinedOutput()
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n"), nil
}
