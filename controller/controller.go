package controller

import (
	"errors"
	"fmt"
	"io/ioutil"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	"k8s.io/klog/v2"
)

type Controller interface {
	GetLoadBalancerType() string
	GenerateConf() error
	ReloadLoadBalancer() error
	GC() error
}

func GetProcessId() (string, error) {
	process := "/nginx/nginx-pid/nginx.pid"
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

func GetController(kd *driver.KubernetesDriver) (Controller, error) {
	switch config.Get("LB_TYPE") {
	case config.Nginx:
		return NewNginxController(kd), nil
	default:
		return nil, fmt.Errorf("unsupport lb type %s only support nginx. Will support elb, slb, clb in the future", config.Get("LB_TYPE"))
	}
}
