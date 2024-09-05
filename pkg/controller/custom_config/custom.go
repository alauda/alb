package custom_config

import (
	m "alauda.io/alb2/controller/modules"
	ct "alauda.io/alb2/controller/types"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/pkg/controller/ext/otel"
	"alauda.io/alb2/pkg/controller/ext/waf"
	u "alauda.io/alb2/pkg/utils"
	"github.com/go-logr/logr"
	nv1 "k8s.io/api/networking/v1"
)

//  phase
//           +--------------+
//           | (gen rule cr)|
//           | ingress sync |-----------------+
//           +--------------+                 |
//                                            |
//                                            |                +-----------+
//                                            v         +----> | otel/other|
//           +-----------------+         +--------------+      +-----------+
//           |cr:rule -> m:rule|-------->| custom config|
//           +-------+---------+         +--------------+     +-----------------+
//                   |                         ^         +--->| (legacy)rewrite req/res |
//                   |                         |              -------------------+
//                   |                         |
//             +-----v------+                  |
//             | policy gen +------------------+
//             +------------+

// TODO a better name?
type CustomCfgCtl struct {
	log    logr.Logger
	domain string
	otel   *otel.Otel
	waf    *waf.Waf
}

type CustomCfgOpt struct {
	Log    logr.Logger
	Domain string
}

func NewCustomCfgCtl(opt CustomCfgOpt) CustomCfgCtl {
	return CustomCfgCtl{
		log:    opt.Log,
		domain: opt.Domain,
		otel:   otel.NewOtel(opt.Log),
		waf:    waf.NewWaf(opt.Log),
	}
}

// ingress sync
func (c CustomCfgCtl) IngressToRule(ing *nv1.Ingress, rindex int, pindex int, rule *albv1.Rule) {
	annotations := legacyGenerateRuleAnnotationFromIngressAnnotation(ing.Name, ing.Annotations, c.domain)
	rule.Annotations = u.MergeMap(rule.Annotations, annotations)
	c.otel.UpdateRuleViaIngress(ing, rindex, pindex, rule, c.domain)
	c.waf.UpdateRuleViaIngress(ing, rindex, pindex, rule, c.domain)
}

// cr:rule -> m:rule
func (c CustomCfgCtl) FromRuleCr(rule *m.Rule, r *ct.Rule) {
	ruleInPolicyFromRuleAnnotation(rule, c.domain, r.Config)
	c.otel.FromRuleCr(rule, r)
	c.waf.FromRuleCr(rule, r)
}

// policy gen
func (c CustomCfgCtl) ResolvePolicies(alb *ct.LoadBalancer, ngx *ct.NgxPolicy) {
	_ = c.otel.ResolvePolicy(alb, ngx)
}
