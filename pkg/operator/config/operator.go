package config

import (
	"fmt"
	"os"
)

type OperatorCfg struct {
	AlbImage   string
	NginxImage string
	BaseDomain string
	Version    string
}

var DEFAULT_OPERATOR_CFG = OperatorCfg{
	AlbImage:   "alb.img",
	NginxImage: "nginx.img",
	BaseDomain: "cpaas.io",
	Version:    "v0.0.1",
}

func OperatorCfgFromEnv() (OperatorCfg, error) {
	alb := os.Getenv("ALB_IMAGE")
	nginx := os.Getenv("NGINX_IMAGE")
	version := os.Getenv("VERSION")
	base := os.Getenv("LABEL_BASE_DOMAIN")
	if alb == "" || nginx == "" || version == "" || base == "" {
		return OperatorCfg{}, fmt.Errorf("env not set %v %v %v %v", alb, nginx, version, base)
	}
	return OperatorCfg{
		AlbImage:   alb,
		NginxImage: nginx,
		Version:    version,
		BaseDomain: base,
	}, nil
}

func (o OperatorCfg) GetAlbImage() string {
	return o.AlbImage
}

// get nginx image
func (o OperatorCfg) GetNginxImage() string {
	return o.NginxImage
}

// get version
func (o OperatorCfg) GetVersion() string {
	return o.Version
}

// get base domain
func (o OperatorCfg) GetLabelBaseDomain() string {
	return o.BaseDomain
}
