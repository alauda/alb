package proxy_header

import (
	"github.com/go-logr/logr"
	nv1 "k8s.io/api/networking/v1"

	m "alauda.io/alb2/controller/modules"
	ct "alauda.io/alb2/controller/types"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	. "alauda.io/alb2/pkg/controller/ext/proxy_header/types"
	et "alauda.io/alb2/pkg/controller/extctl/types"
	u "alauda.io/alb2/pkg/utils"
)

type ProxyHeaderCtl struct {
	log    logr.Logger
	domain string
}

func NewProxyHeaderCtl(log logr.Logger, domain string) et.ExtensionInterface {
	x := &ProxyHeaderCtl{
		log:    log,
		domain: domain,
	}
	return et.ExtensionInterface{
		IngressAnnotationToRule: x.IngressAnnotationToRule,
		ToInternalRule:          x.ToInternalRule,
		ToPolicy:                x.ToPolicy,
		IsBuiltIn: func() bool {
			return true
		},
		Name: func() ct.PolicyExtKind {
			return ct.ProxyHeader
		},
	}
}

// IngressAnnotationToRule processes ingress annotations and updates the rule configuration
func (x *ProxyHeaderCtl) IngressAnnotationToRule(ing *nv1.Ingress, rindex int, pindex int, rule *albv1.Rule) {
	if ing == nil || ing.Annotations == nil || rule == nil {
		return
	}
	use_forwarded_headers, _ := u.GetIngressAnnotation(ing.Annotations, pindex, rindex, x.domain, USE_FORWARDED_HEADERS_ANNOTATION)
	if use_forwarded_headers == "true" {
		rule.Spec.Config.ProxyHeader = &ProxyHeader{UseForwardedHeaders: true}
	}
}

func (x *ProxyHeaderCtl) ToInternalRule(rule *m.Rule, ir *ct.InternalRule) {
	if rule == nil || ir == nil {
		return
	}
	if rule.GetConfig() != nil && rule.GetConfig().ProxyHeader != nil {
		ir.Config.ProxyHeader = rule.GetConfig().ProxyHeader
		ir.Config.Source[ct.ProxyHeader] = rule.Name
		return
	}
	if rule.GetFtConfig() != nil && rule.GetFtConfig().ProxyHeader != nil {
		ir.Config.ProxyHeader = rule.GetFtConfig().ProxyHeader
		ir.Config.Source[ct.ProxyHeader] = rule.FT.Name
		return
	}
	if rule.GetAlbConfig() != nil && rule.GetAlbConfig().ProxyHeader != nil {
		ir.Config.ProxyHeader = rule.GetAlbConfig().ProxyHeader
		ir.Config.Source[ct.ProxyHeader] = rule.FT.LB.Alb.Name
		return
	}
}

func (x *ProxyHeaderCtl) ToPolicy(ir *ct.InternalRule, p *ct.Policy, refs ct.RefMap) {
	if ir == nil || p == nil {
		return
	}

	if ir.Config.ProxyHeader != nil {
		p.Config.ProxyHeader = ir.Config.ProxyHeader
	}
}
