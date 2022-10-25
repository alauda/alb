package controller

import (
	"errors"
	"io/ioutil"

	"k8s.io/klog/v2"
)

func GetProcessId() (string, error) {
	process := "/etc/alb2/nginx/nginx.pid"
	out, err := ioutil.ReadFile(process)
	if err != nil {
		klog.Errorf("nginx process is not started: %s", err.Error())
		return "", err
	}
	return string(out), err
}

var (
	//ErrStandAlone will be returned when do something that is not allowed in stand-alone mode
	ErrStandAlone = errors.New("operation is not allowed in stand-alone mode")
)
