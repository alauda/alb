package extctl

import (
	"alauda.io/alb2/config"
	m "alauda.io/alb2/controller/modules"
	ct "alauda.io/alb2/controller/types"

	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	ngt "alauda.io/alb2/pkg/controller/ngxconf/types"
	nv1 "k8s.io/api/networking/v1"
)

// rewrite_url redirect cors vhost... those ext born with alb.. just donot touch them, for now.
type LegacyExtCtl struct{}

func NewLegacyExtCtl() LegacyExtCtl {
	return LegacyExtCtl{}
}

func (o LegacyExtCtl) IngressAnnotationToRule(ing *nv1.Ingress, rindex int, pindex int, rule *albv1.Rule) {
}

func (o LegacyExtCtl) ToInternalRule(mr *m.Rule, ir *ct.InternalRule) {
	mrs := mr.Spec

	ru := ct.RewriteConf{}
	ru.URL = mrs.URL
	ru.RewriteBase = mrs.RewriteBase
	ru.RewriteTarget = mrs.RewriteTarget
	ir.Config.Rewrite = &ru

	cors := ct.Cors{}
	cors.EnableCORS = mrs.EnableCORS
	cors.CORSAllowHeaders = mrs.CORSAllowHeaders
	cors.CORSAllowOrigin = mrs.CORSAllowOrigin
	ir.Config.Cors = &cors

	vhost := ct.Vhost{}
	vhost.VHost = mrs.VHost
	ir.Config.Vhost = &vhost
}

func (o LegacyExtCtl) CollectRefs(_ *ct.InternalRule, _ ct.RefMap) {
}

func (o LegacyExtCtl) ToPolicy(ir *ct.InternalRule, p *ct.Policy, refs ct.RefMap) {
	// rewrite
	rule_ext := ir.Config
	if rule_ext.Rewrite != nil {
		p.RewriteConf = *rule_ext.Rewrite
	}
	// cors
	if rule_ext.Cors != nil {
		p.Cors = *rule_ext.Cors
	}
	// vhost
	if rule_ext.Vhost != nil {
		p.Vhost = *rule_ext.Vhost
	}
}

func (o LegacyExtCtl) UpdateNgxTmpl(tmpl_cfg *ngt.NginxTemplateConfig, alb *ct.LoadBalancer, cfg *config.Config) {
}

func (o LegacyExtCtl) UpdatePolicyAfterUniq(*ct.PolicyExt) {
}
