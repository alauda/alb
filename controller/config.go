package controller

import (
	"alauda.io/alb2/config"
	. "alauda.io/alb2/controller/types"
	"strconv"
)

const (
	gzipTypes = "application/atom+xml application/javascript application/x-javascript application/json application/rss+xml application/vnd.ms-fontobject application/x-font-ttf application/x-web-app-manifest+json application/xhtml+xml application/xml font/opentype image/svg+xml image/x-icon text/css text/javascript text/plain text/x-component"
)

type Config struct {
	Name           string
	Address        string
	BindAddress    string
	LoadBalancerID string
	Frontends      map[int]*Frontend
	BackendGroup   BackendGroups
	CertificateMap map[string]Certificate
	TweakHash      string
	Phase          string
	NginxParam
}

type NginxParam struct {
	EnablePrometheus bool
	EnableIPV6       bool
	EnableHTTP2      bool
	CPUNum           string
	MetricsPort      int
	Backlog          int
	EnableGzip       bool
	GzipLevel        int
	GzipMinLength    int
	GzipTypes        string
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func newNginxParam() NginxParam {
	return NginxParam{
		EnablePrometheus: config.Get("ENABLE_PROMETHEUS") == "true",
		EnableIPV6:       checkIPV6(),
		EnableHTTP2:      config.Get("ENABLE_HTTP2") == "true",
		CPUNum:           strconv.Itoa(min(cpu_preset(), workerLimit())),
		MetricsPort:      config.GetInt("METRICS_PORT"),
		Backlog:          config.GetInt("BACKLOG"),
		EnableGzip:       config.Get("ENABLE_GZIP") == "true",
		GzipLevel:        5,
		GzipMinLength:    256,
		GzipTypes:        gzipTypes,
	}
}
