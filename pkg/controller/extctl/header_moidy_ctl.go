package extctl

import (
	"encoding/json"
	"fmt"

	m "alauda.io/alb2/controller/modules"
	ct "alauda.io/alb2/controller/types"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	u "alauda.io/alb2/pkg/utils"
	"github.com/go-logr/logr"
	nv1 "k8s.io/api/networking/v1"

	"alauda.io/alb2/config"
	"alauda.io/alb2/controller/types"
	ngt "alauda.io/alb2/pkg/controller/ngxconf/types"
)

// rewrite request/response header
type HeaderModifyCtl struct {
	domain string
	log    logr.Logger
}

func NewHeaderModifyCtl(log logr.Logger, domain string) HeaderModifyCtl {
	return HeaderModifyCtl{
		domain: domain,
		log:    log,
	}
}

func (c HeaderModifyCtl) IngressAnnotationToRule(ing *nv1.Ingress, ruleIndex int, pathIndex int, rule *albv1.Rule) {
	annotations := c.GenRewriteResponseOrRequestRuleAnnotation(ing.Name, ing.Annotations, c.domain)
	rule.Annotations = u.MergeMap(rule.Annotations, annotations)
}

func rewriteResponseConfigFromJson(jsonStr string) (*types.RewriteResponseConfig, error) {
	cfg := types.RewriteResponseConfig{}
	err := json.Unmarshal([]byte(jsonStr), &cfg)
	if err != nil {
		return nil, err
	}
	if cfg.IsEmpty() {
		return nil, fmt.Errorf("empty config")
	}
	return &cfg, err
}

func rewriteRequestConfigFromJson(jsonStr string) (*types.RewriteRequestConfig, error) {
	cfg := types.RewriteRequestConfig{}
	err := json.Unmarshal([]byte(jsonStr), &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, err
}

func (c HeaderModifyCtl) GenRewriteResponseOrRequestRuleAnnotation(ingressName string, annotation map[string]string, domain string) map[string]string {
	ruleAnnotation := make(map[string]string)
	n := config.NewNames(domain)

	if val, ok := annotation[n.GetAlbIngressRewriteResponseAnnotation()]; ok {
		_, err := rewriteResponseConfigFromJson(val)
		if err != nil {
			c.log.Error(err, "ext ingress rewrite_response: invalid annotation", "ing", ingressName, "val", err)
		} else {
			ruleAnnotation[n.GetAlbRuleRewriteResponseAnnotation()] = val
		}
	}
	if val, ok := annotation[n.GetAlbIngressRewriteRequestAnnotation()]; ok {
		_, err := rewriteRequestConfigFromJson(val)
		if err != nil {
			c.log.Error(err, "ext ingress rewrite_request: invalid annotation", "ing", ingressName, "val", err)
		} else {
			ruleAnnotation[n.GetAlbRuleRewriteRequestAnnotation()] = val
		}
	}
	return ruleAnnotation
}

func (c HeaderModifyCtl) ToInternalRule(rule *m.Rule, r *ct.InternalRule) {
	n := config.NewNames(c.domain)
	annotation := rule.Annotations
	ruleName := rule.Name
	if val, ok := annotation[n.GetAlbRuleRewriteResponseAnnotation()]; ok {
		rewriteCfg, err := rewriteResponseConfigFromJson(val)
		if err != nil {
			c.log.Error(err, "ext ingress rewrite_response: invalid annotation", "rule", ruleName, "val", err)
		} else {
			r.Config.RewriteResponse = rewriteCfg
		}
	}
	if val, ok := annotation[n.GetAlbRuleRewriteRequestAnnotation()]; ok {
		rewriteCfg, err := rewriteRequestConfigFromJson(val)
		if err != nil {
			c.log.Error(err, "ext ingress rewrite_request: invalid annotation", "rule", ruleName, "val", err)
		} else {
			r.Config.RewriteRequest = rewriteCfg
		}
	}
}

func (c HeaderModifyCtl) ToPolicy(r *ct.InternalRule, p *ct.Policy, refs ct.RefMap) {
	if r.Config.RewriteRequest != nil {
		p.Config.RewriteRequest = r.Config.RewriteRequest
	}
	if r.Config.RewriteResponse != nil {
		p.Config.RewriteResponse = r.Config.RewriteResponse
	}
}

func (c HeaderModifyCtl) CollectRefs(ir *ct.InternalRule, refs ct.RefMap) {
}

func (c HeaderModifyCtl) UpdateNgxTmpl(_ *ngt.NginxTemplateConfig, _ *ct.LoadBalancer, _ *config.Config) {
}

func (c HeaderModifyCtl) UpdatePolicyAfterUniq(_ *ct.PolicyExt) {
}
