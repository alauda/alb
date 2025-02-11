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
	"alauda.io/alb2/pkg/controller/ext/otel"
	"alauda.io/alb2/pkg/controller/ext/waf"
	ngt "alauda.io/alb2/pkg/controller/ngxconf/types"
	pu "alauda.io/alb2/pkg/utils"
	pm "alauda.io/alb2/pkg/utils/metrics"
	"github.com/go-logr/logr"
	"golang.org/x/exp/maps"
	nv1 "k8s.io/api/networking/v1"
)

// extension controller
type ExtCtl struct {
	log        logr.Logger
	domain     string
	extensions []Extension
}

type ExtCtlCfgOpt struct {
	Log    logr.Logger
	Domain string
}

type Extension interface {
	IngressAnnotationToRule(ing *nv1.Ingress, rindex int, pindex int, rule *albv1.Rule)
	ToInternalRule(rule *m.Rule, r *ct.InternalRule)
	CollectRefs(*ct.InternalRule, ct.RefMap)
	ToPolicy(*ct.InternalRule, *ct.Policy, ct.RefMap)
	UpdateNgxTmpl(tmpl_cfg *ngt.NginxTemplateConfig, alb *ct.LoadBalancer, cfg *config.Config)
	UpdatePolicyAfterUniq(*ct.PolicyExt)
}

func NewExtensionCtl(opt ExtCtlCfgOpt) ExtCtl {
	return ExtCtl{
		log:    opt.Log,
		domain: opt.Domain,
		extensions: []Extension{
			NewLegacyExtCtl(),
			NewHeaderModifyCtl(opt.Log, opt.Domain),
			otel.NewOtel(opt.Log, opt.Domain),
			waf.NewWaf(opt.Log, opt.Domain),
			auth.NewAuthCtl(opt.Log, opt.Domain),
		},
	}
}

// ingress sync
func (c ExtCtl) IngressAnnotationToRule(ing *nv1.Ingress, rindex int, pindex int, rule *albv1.Rule) {
	for _, ext := range c.extensions {
		ext.IngressAnnotationToRule(ing, rindex, pindex, rule)
	}
}

// cr:rule -> m:rule
func (c ExtCtl) ToInternalRule(mr *m.Rule, ir *ct.InternalRule) {
	for _, ext := range c.extensions {
		ext.ToInternalRule(mr, ir)
	}
}

func (c ExtCtl) ToPolicy(ir *ct.InternalRule, p *ct.Policy, refs ct.RefMap) {
	for _, ext := range c.extensions {
		ext.ToPolicy(ir, p, refs)
	}
}

// TODO performance when huge rule? 应该使用某种hint 不要每次都重新计算
// NOTE 每个插件负责根据对每个rule生成配置，这些配置可能是从其他地方继承而来的，但是在policy.new中对于相同的配置，我们把他merge到一起.
func (c ExtCtl) MergeSamePolicyConfig(ngx *ct.NgxPolicy) {
	share := ct.SharedExtPolicyConfig{}
	for _, ps := range ngx.Http.Tcp {
		for _, p := range ps {
			if p.Config.Refs == nil {
				p.Config.Refs = map[ct.PolicyExtKind]string{}
			}
			used_plugin_map := map[string]bool{}
			for k, c := range p.Config.PolicyExt.ToMaps() {
				// TODO migrate those to new framework
				if k == "rewrite" || k == "cors" || k == "rewrite_request" || k == "rewrite_response" || k == "timeout" {
					continue
				}
				used_plugin_map[string(k)] = true
				hash := hash(c)
				if _, exist := share[hash]; !exist {
					share[hash] = ct.RefBox{
						Hash:      hash,
						Type:      k,
						PolicyExt: *c,
					}
				}
				p.Config.Refs[k] = hash
				p.Config.Clean(k)
			}
			used_plugin_list := maps.Keys(used_plugin_map)
			sort.Strings(used_plugin_list)
			p.Plugins = used_plugin_list
		}
	}
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
	}
}

func (c ExtCtl) CollectRefs(ir *ct.InternalRule, refs ct.RefMap) {
	for _, ext := range c.extensions {
		ext.CollectRefs(ir, refs)
	}
}

func (c ExtCtl) UpdateNgxTmpl(tmpl_cfg *ngt.NginxTemplateConfig, alb *ct.LoadBalancer, cfg *config.Config) error {
	for _, ext := range c.extensions {
		ext.UpdateNgxTmpl(tmpl_cfg, alb, cfg)
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
