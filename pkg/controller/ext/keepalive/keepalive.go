package keepalive

import (
	"alauda.io/alb2/config"
	m "alauda.io/alb2/controller/modules"
	ct "alauda.io/alb2/controller/types"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	et "alauda.io/alb2/pkg/controller/extctl/types"
	ngt "alauda.io/alb2/pkg/controller/ngxconf/types"
	"github.com/go-logr/logr"
)

type KeepAliveCtl struct {
	log    logr.Logger
	domain string
}

// 端口级别的downstream(client->alb)  l4 tcp keepalive和l7 keepalive.通过nginx template 配置
// TODO 1. upstream的keepalive配置 2. 规则级别的downstream和upstream的keepalive配置
func NewKeepAliveCtl(log logr.Logger, domain string) et.ExtensionInterface {
	x := &KeepAliveCtl{
		log:    log,
		domain: domain,
	}
	return et.ExtensionInterface{
		InitL4Ft:      x.InitL4Ft,
		InitL7Ft:      x.InitL7Ft,
		UpdateNgxTmpl: x.UpdateNgxTmpl,
	}
}

// InitL4Ft initializes L4 Frontend
func (k *KeepAliveCtl) InitL4Ft(mft *m.Frontend, cft *ct.Frontend) {
	// 只能配置在tcp上
	if cft.Protocol != albv1.FtProtocolTCP {
		return
	}
	defaultInitFt(mft, cft)
}

func (k *KeepAliveCtl) InitL7Ft(mft *m.Frontend, cft *ct.Frontend) {
	defaultInitFt(mft, cft)
}

func defaultInitFt(mft *m.Frontend, cft *ct.Frontend) {
	cfg := mft.GetFtConfig()
	if cfg == nil || cfg.KeepAlive == nil {
		return
	}
	cft.Config.KeepAlive = cfg.KeepAlive
}

func (k *KeepAliveCtl) UpdateNgxTmpl(tmpl_cfg *ngt.NginxTemplateConfig, alb *ct.LoadBalancer, _ *config.Config) {
	for _, ft := range alb.Frontends {
		if ft.Config.KeepAlive == nil {
			continue
		}
		ft_tmpl := tmpl_cfg.Frontends[ft.String()]
		if ft.Config.KeepAlive.TCP != nil {
			tcp := ft.Config.KeepAlive.TCP
			listen := ft_tmpl.Listen + " " + tcp.ToNginxConf()
			ft_tmpl.Listen = listen
		}
		if ft.Config.KeepAlive.HTTP != nil {
			http := ft.Config.KeepAlive.HTTP
			original_location := ft_tmpl.Location
			location := http.ToNginxConf()
			ft_tmpl.Location = original_location + "\n" + location
		}
		tmpl_cfg.Frontends[ft.String()] = ft_tmpl
	}
}
