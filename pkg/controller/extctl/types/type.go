package extctl

import (
	"alauda.io/alb2/config"
	m "alauda.io/alb2/controller/modules"
	ct "alauda.io/alb2/controller/types"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	ngt "alauda.io/alb2/pkg/controller/ngxconf/types"
	nv1 "k8s.io/api/networking/v1"
)

// ingress/alb/ft/rule 系列的插件。
// TODO 应该能被用在gateway-api 上.
type ExtensionInterface struct {
	IngressAnnotationToRule       func(ing *nv1.Ingress, rindex int, pindex int, rule *albv1.Rule)
	IngressWithFtAnnotationToRule func(ft *albv1.Frontend, ing *nv1.Ingress, rindex int, pindex int, rule *albv1.Rule)
	ToInternalRule                func(rule *m.Rule, r *ct.InternalRule)
	CollectRefs                   func(*ct.InternalRule, ct.RefMap)
	ToPolicy                      func(*ct.InternalRule, *ct.Policy, ct.RefMap)
	UpdateNgxTmpl                 func(tmpl_cfg *ngt.NginxTemplateConfig, alb *ct.LoadBalancer, cfg *config.Config)
	UpdatePolicyAfterUniq         func(*ct.PolicyExt)
	InitL7Ft                      func(mft *m.Frontend, cft *ct.Frontend)
	NeedL7DefaultPolicy           func(cft *ct.Frontend) (bool, string)
	InitL7DefaultPolicy           func(cft *ct.Frontend, policy *ct.Policy)
	InitL4Ft                      func(mft *m.Frontend, cft *ct.Frontend)
	InitL4DefaultPolicy           func(cft *ct.Frontend, policy *ct.Policy)
	ShouldMergeConfig             func() bool
}
