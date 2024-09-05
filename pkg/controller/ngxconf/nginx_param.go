package ngxconf

import (
	"os"
	"strconv"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/pkg/controller/ngxconf/types"
)

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

const (
	gzipTypes = "application/atom+xml application/javascript application/x-javascript application/json application/rss+xml application/vnd.ms-fontobject application/x-font-ttf application/x-web-app-manifest+json application/xhtml+xml application/xml font/opentype image/svg+xml image/x-icon text/css text/javascript text/plain text/x-component"
)

func newNginxParam(cfg *config.Config) NginxParam {
	ngx := cfg.GetNginxCfg()

	cpu_preset := cfg.GetCpuPreset()
	work_limit := cfg.GetWorkerLimit()
	if work_limit == 0 {
		work_limit = 4
	}
	return NginxParam{
		EnablePrometheus: ngx.EnablePrometheus,
		EnableIPV6:       checkIPV6(ngx.EnableIpv6),
		EnableHTTP2:      ngx.EnableHttp2,
		CPUNum:           strconv.Itoa(min(cpu_preset, work_limit)),
		MetricsPort:      cfg.GetMetricsPort(),
		Backlog:          ngx.BackLog,
		EnableGzip:       ngx.EnableGzip,
		GzipLevel:        5,
		GzipMinLength:    256,
		GzipTypes:        gzipTypes,
	}
}
