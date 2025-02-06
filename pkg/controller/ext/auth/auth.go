package auth

import (
	"fmt"

	av1 "alauda.io/alb2/pkg/apis/alauda/v1"
	. "alauda.io/alb2/pkg/controller/ext/auth/types"
	. "alauda.io/alb2/pkg/utils"
	nv1 "k8s.io/api/networking/v1"

	m "alauda.io/alb2/controller/modules"
	ct "alauda.io/alb2/controller/types"
	"github.com/go-logr/logr"

	"alauda.io/alb2/config"
	ngt "alauda.io/alb2/pkg/controller/ngxconf/types"
)

type AuthCtl struct {
	L       logr.Logger
	domain  string
	forward ForwardAuthCtl
	basic   BasicAuthCtl
}

func NewAuthCtl(l logr.Logger, domain string) *AuthCtl {
	return &AuthCtl{
		L:      l,
		domain: domain,
		forward: ForwardAuthCtl{
			l: l.WithName("forward-auth"),
		},
		basic: BasicAuthCtl{
			l: l.WithName("basic-auth"),
		},
	}
}

func (a *AuthCtl) IngressAnnotationToRule(ingress *nv1.Ingress, ruleIndex int, pathIndex int, rule *av1.Rule) {
	auth_ingress := AuthIngress{}
	err := ResolverStructFromAnnotation(&auth_ingress, ingress.Annotations, ResolveAnnotationOpt{Prefix: []string{fmt.Sprintf("index.%d-%d.alb.ingress.%s", ruleIndex, pathIndex, a.domain), fmt.Sprintf("alb.ingress.%s", a.domain), "nginx.ingress.kubernetes.io"}})
	if err != nil {
		a.L.Error(err, "failed to resolve auth ingress", "ing", ingress.Name, "ing-ns", ingress.Namespace)
		return
	}
	if auth_ingress.Enable == "false" {
		return
	}
	auth_cr := AuthCr{}
	if auth_ingress.Url != "" {
		a.forward.AuthIngressToAuthCr(&auth_ingress, &auth_cr)
	}
	if auth_ingress.AuthType != "" {
		a.basic.AuthIngressToAuthCr(&auth_ingress, &auth_cr)
	}

	if auth_cr.Basic == nil && auth_cr.Forward == nil {
		return
	}
	// we have to take a choice
	if auth_cr.Basic != nil && auth_cr.Forward != nil {
		a.L.Info("both basic auth and forward auth ? use basic", "ing", ingress.Name, "ing-ns", ingress.Namespace)
		auth_cr.Forward = nil
	}
	rule.Spec.Config.Auth = &auth_cr
}

func (c *AuthCtl) ToInternalRule(mr *m.Rule, ir *ct.InternalRule) {
	if mr.Spec.Config.Auth != nil {
		ir.Config.Auth = mr.Spec.Config.Auth
		return
	}
	ft := mr.GetFtConfig()
	if ft != nil && ft.Auth != nil {
		ir.Config.Auth = ft.Auth
		return
	}

	lb := mr.GetAlbConfig()
	if lb != nil && lb.Auth != nil {
		ir.Config.Auth = lb.Auth
		return
	}
}

func (c *AuthCtl) CollectRefs(ir *ct.InternalRule, refs ct.RefMap) {
	if ir.Config.Auth == nil {
		return
	}
	cfg := ir.Config.Auth
	c.ForwardAuthCollectRefs(ir, cfg, refs)
	c.BasicAuthCollectRefs(ir, cfg, refs)
}

func (c *AuthCtl) ForwardAuthCollectRefs(ir *ct.InternalRule, cfg *AuthCr, refs ct.RefMap) {
	if cfg.Forward == nil || cfg.Forward.AuthHeadersCmRef == "" {
		return
	}
	raw_key := cfg.Forward.AuthHeadersCmRef
	key, err := ParseStringToObjectKey(raw_key)
	if err != nil {
		c.L.Error(err, "invalid cmref", "cmref", key, "rule", ir.RuleID)
		return
	}
	refs.ConfigMap[key] = nil
}

func (c *AuthCtl) BasicAuthCollectRefs(ir *ct.InternalRule, cfg *AuthCr, refs ct.RefMap) {
	if cfg.Basic == nil || cfg.Basic.Secret == "" {
		return
	}
	raw_key := cfg.Basic.Secret
	key, err := ParseStringToObjectKey(raw_key)
	if err != nil {
		c.L.Error(err, "invalid secret-ref", "ref", key, "rule", ir.RuleID)
		return
	}
	refs.Secret[key] = nil
}

func (c *AuthCtl) ToPolicy(ir *ct.InternalRule, p *ct.Policy, refs ct.RefMap) {
	if ir.Config.Auth == nil {
		return
	}
	ir_auth := ir.Config.Auth
	p.Config.Auth = &AuthPolicy{}
	if ir_auth.Forward != nil {
		c.forward.ToPolicy(ir_auth.Forward, p.Config.Auth, refs, ir.RuleID)
	}
	if ir_auth.Basic != nil {
		c.basic.ToPolicy(ir_auth.Basic, p.Config.Auth, refs, ir.RuleID)
	}
}

func (c *AuthCtl) UpdateNgxTmpl(_ *ngt.NginxTemplateConfig, _ *ct.LoadBalancer, _ *config.Config) {
}

func (c *AuthCtl) UpdatePolicyAfterUniq(_ *ct.PolicyExt) {
}
