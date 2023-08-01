package controller

import (
	"os"
	"strconv"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/controller/types"
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
	ngx := config.GetConfig().GetNginxCfg()
	return NginxParam{
		EnablePrometheus: ngx.EnablePrometheus,
		EnableIPV6:       checkIPV6(ngx.EnableIpv6),
		EnableHTTP2:      ngx.EnableHttp2,
		CPUNum:           strconv.Itoa(min(cpu_preset(), workerLimit())),
		MetricsPort:      config.GetConfig().GetMetricsPort(),
		Backlog:          ngx.BackLog,
		EnableGzip:       ngx.EnableGzip,
		GzipLevel:        5,
		GzipMinLength:    256,
		GzipTypes:        gzipTypes,
	}
}

func checkIPV6(enable bool) bool {
	if !enable {
		return false
	}
	if _, err := os.Stat("/proc/net/if_inet6"); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
