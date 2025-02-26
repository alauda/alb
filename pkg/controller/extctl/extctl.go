package extctl

import (
	"encoding/json"
	"sort"
	"time"

	"alauda.io/alb2/config"
	m "alauda.io/alb2/controller/modules"
	ct "alauda.io/alb2/controller/types"

	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/pkg/controller/ext/auth"
	"alauda.io/alb2/pkg/controller/ext/keepalive"
	"alauda.io/alb2/pkg/controller/ext/otel"
	"alauda.io/alb2/pkg/controller/ext/redirect"
	"alauda.io/alb2/pkg/controller/ext/timeout"
	"alauda.io/alb2/pkg/controller/ext/waf"
	. "alauda.io/alb2/pkg/controller/extctl/types"
	ngt "alauda.io/alb2/pkg/controller/ngxconf/types"
	pu "alauda.io/alb2/pkg/utils"
	pm "alauda.io/alb2/pkg/utils/metrics"
	"github.com/go-logr/logr"
	"golang.org/x/exp/maps"
	nv1 "k8s.io/api/networking/v1"
)

// extension controller
type ExtCtl struct {
	log                  logr.Logger
	domain               string
	extensions           []Extension
	extensions_interface []ExtensionInterface // go 的interface必须实现所有method,用函数指针来模拟。
	skip_merge           map[ct.PolicyExtKind]bool
}

type ExtCtlCfgOpt struct {
	Log    logr.Logger
	Domain string
}

// Deprecated: use ExtensionInterface
type Extension interface {
	IngressAnnotationToRule(ing *nv1.Ingress, rindex int, pindex int, rule *albv1.Rule)
	ToInternalRule(rule *m.Rule, r *ct.InternalRule)
	CollectRefs(*ct.InternalRule, ct.RefMap)
	ToPolicy(*ct.InternalRule, *ct.Policy, ct.RefMap)
	UpdateNgxTmpl(tmpl_cfg *ngt.NginxTemplateConfig, alb *ct.LoadBalancer, cfg *config.Config)
	UpdatePolicyAfterUniq(*ct.PolicyExt)
}

func NewExtensionCtl(opt ExtCtlCfgOpt) ExtCtl {
	e := ExtCtl{
		log:    opt.Log,
		domain: opt.Domain,
		extensions: []Extension{
			NewLegacyExtCtl(),
			NewHeaderModifyCtl(opt.Log, opt.Domain),
			otel.NewOtel(opt.Log, opt.Domain),
			waf.NewWaf(opt.Log, opt.Domain),
			auth.NewAuthCtl(opt.Log, opt.Domain),
		},
		extensions_interface: []ExtensionInterface{
			redirect.NewRedirectCtl(opt.Log, opt.Domain),
			timeout.NewTimeoutCtl(opt.Log, opt.Domain),
			keepalive.NewKeepAliveCtl(opt.Log, opt.Domain),
		},
	}
	// TODO 当有更多的插件需要配置时，暴露到interface上
	e.skip_merge = map[ct.PolicyExtKind]bool{
		ct.Redirect: true,
	}
	return e
}

// ingress sync
func (c ExtCtl) IngressWithFtAnnotationToRule(ft *albv1.Frontend, ing *nv1.Ingress, rindex int, pindex int, rule *albv1.Rule) {
	for _, ext := range c.extensions_interface {
		if ext.IngressWithFtAnnotationToRule != nil {
			ext.IngressWithFtAnnotationToRule(ft, ing, rindex, pindex, rule)
		}
		if ext.IngressAnnotationToRule != nil {
			ext.IngressAnnotationToRule(ing, rindex, pindex, rule)
		}
	}
	for _, ext := range c.extensions {
		ext.IngressAnnotationToRule(ing, rindex, pindex, rule)
	}
}

// cr:rule -> m:rule
func (c ExtCtl) ToInternalRule(mr *m.Rule, ir *ct.InternalRule) {
	for _, ext := range c.extensions {
		ext.ToInternalRule(mr, ir)
	}
	for _, ext := range c.extensions_interface {
		if ext.ToInternalRule != nil {
			ext.ToInternalRule(mr, ir)
		}
	}
}

func (c ExtCtl) ToPolicy(ir *ct.InternalRule, p *ct.Policy, refs ct.RefMap) {
	for _, ext := range c.extensions {
		ext.ToPolicy(ir, p, refs)
	}
	for _, ext := range c.extensions_interface {
		if ext.ToPolicy != nil {
			ext.ToPolicy(ir, p, refs)
		}
	}
}

func (ctl ExtCtl) init_http_share(share ct.SharedExtPolicyConfig, p *ct.Policy) {
	used_plugin_map := map[string]bool{}
	for k, c := range p.Config.ToMaps() {
		// TODO migrate those to new framework
		if k == "rewrite" || k == "cors" || k == "rewrite_request" || k == "rewrite_response" {
			continue
		}
		k_str := string(k)
		used_plugin_map[k_str] = true
		if ctl.skip_merge[k] {
			continue
		}
		id := ""
		if c.Source == "" {
			id = hash(c)
		} else {
			id = c.Source + "/" + k_str
		}
		if _, exist := share[id]; !exist {
			share[id] = ct.RefBox{
				Type:      k,
				PolicyExt: *c,
			}
		}
		p.Config.Refs[k] = id
		p.Config.Clean(k)
	}
	used_plugin_list := maps.Keys(used_plugin_map)
	sort.Strings(used_plugin_list)
	p.Plugins = used_plugin_list
}

// NOTE 每个插件负责根据对每个rule生成配置，这些配置可能是从其他地方继承而来的，在policy中对于相同的配置，我们可以把他merge到一起.
// 对于一些有可能会有继承且会merge的插件，比如oauth 可以使用hash做了ref key。
// 对其存在继承但是不会merge的配置。插件自己可以决定他的名字，一般就是cr的name。
// 对于不存在继承的插件。可以直接将merge policy关掉
func (c ExtCtl) MergeSamePolicyConfig(ngx *ct.NgxPolicy) {
	share := ct.SharedExtPolicyConfig{}
	for _, ps := range ngx.Http.Tcp {
		for _, p := range ps {
			c.init_http_share(share, p)
		}
	}
	// TODO  需要 stream mode下 config cache机制？
	for _, ps := range ngx.Stream.Tcp {
		for _, p := range ps {
			if p.Config.Refs == nil {
				p.Config.Refs = map[ct.PolicyExtKind]string{}
			}
			used_plugin_map := map[string]bool{}
			for k := range p.Config.ToMaps() {
				used_plugin_map[string(k)] = true
			}
			used_plugin_list := maps.Keys(used_plugin_map)
			sort.Strings(used_plugin_list)
			p.Plugins = used_plugin_list
		}
	}

	// 端口的配置不需要merge 默认就放在policy中.
	ngx.SharedConfig = share
}

// policy gen
func (c ExtCtl) ResolvePolicies(alb *ct.LoadBalancer, ngx *ct.NgxPolicy) {
	s := time.Now()
	defer func() {
		e := time.Now()
		pm.Write("ext-resolve-policy", float64(e.UnixMilli())-float64(s.UnixMilli()))
	}()
	c.MergeSamePolicyConfig(ngx)

	for _, v := range ngx.SharedConfig {
		for _, ext := range c.extensions {
			ext.UpdatePolicyAfterUniq(&v.PolicyExt)
		}
		for _, ext := range c.extensions_interface {
			if ext.UpdatePolicyAfterUniq != nil {
				ext.UpdatePolicyAfterUniq(&v.PolicyExt)
			}
		}
	}
}

func (c ExtCtl) CollectRefs(ir *ct.InternalRule, refs ct.RefMap) {
	for _, ext := range c.extensions {
		ext.CollectRefs(ir, refs)
	}
	for _, ext := range c.extensions_interface {
		if ext.CollectRefs != nil {
			ext.CollectRefs(ir, refs)
		}
	}
}

func (c ExtCtl) UpdateNgxTmpl(tmpl_cfg *ngt.NginxTemplateConfig, alb *ct.LoadBalancer, cfg *config.Config) error {
	for _, ext := range c.extensions {
		ext.UpdateNgxTmpl(tmpl_cfg, alb, cfg)
	}
	for _, ext := range c.extensions_interface {
		if ext.UpdateNgxTmpl != nil {
			ext.UpdateNgxTmpl(tmpl_cfg, alb, cfg)
		}
	}
	return nil
}

func hash(x interface{}) string {
	bytes, err := json.Marshal(x)
	if err != nil {
		return ""
	}
	return pu.HashBytes(bytes)
}

func (c ExtCtl) InitL4Ft(mft *m.Frontend, cft *ct.Frontend) {
	for _, ext := range c.extensions_interface {
		if ext.InitL4Ft != nil {
			ext.InitL4Ft(mft, cft)
		}
	}
}

func (c ExtCtl) InitL4DefaultPolicy(cft *ct.Frontend, policy *ct.Policy) {
	for _, ext := range c.extensions_interface {
		if ext.InitL4DefaultPolicy != nil {
			ext.InitL4DefaultPolicy(cft, policy)
		}
	}
}

func (c ExtCtl) InitL7Ft(mft *m.Frontend, cft *ct.Frontend) {
	for _, ext := range c.extensions_interface {
		if ext.InitL7Ft != nil {
			ext.InitL7Ft(mft, cft)
		}
	}
}

func (c ExtCtl) InitL7DefaultPolicy(cft *ct.Frontend, policy *ct.Policy) {
	for _, ext := range c.extensions_interface {
		if ext.InitL7DefaultPolicy != nil {
			ext.InitL7DefaultPolicy(cft, policy)
		}
	}
}

func (c ExtCtl) NeedL7DefaultPolicy(cft *ct.Frontend) (bool, string) {
	for _, ext := range c.extensions_interface {
		if ext.NeedL7DefaultPolicy != nil {
			need, name := ext.NeedL7DefaultPolicy(cft)
			if need {
				return true, name
			}
		}
	}
	return false, ""
}
