package custom_config

import (
	"encoding/json"
	"fmt"

	"alauda.io/alb2/config"
	mt "alauda.io/alb2/controller/modules"
	"alauda.io/alb2/controller/types"
	"k8s.io/klog/v2"
)

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

func legacyGenerateRuleAnnotationFromIngressAnnotation(ingressName string, annotation map[string]string, domain string) map[string]string {
	ruleAnnotation := make(map[string]string)
	n := config.NewNames(domain)

	if val, ok := annotation[n.GetAlbIngressRewriteResponseAnnotation()]; ok {
		_, err := rewriteResponseConfigFromJson(val)
		if err != nil {
			klog.Errorf("ext ingress rewrite_response: invalid annotation in ingress '%v' annotation is '%v' err %v", ingressName, val, err)
		} else {
			ruleAnnotation[n.GetAlbRuleRewriteResponseAnnotation()] = val
		}
	}
	if val, ok := annotation[n.GetAlbIngressRewriteRequestAnnotation()]; ok {
		_, err := rewriteRequestConfigFromJson(val)
		if err != nil {
			klog.Errorf("ext ingress rewrite_request: invalid annotation in ingress '%v' annotation is '%v' err %v", ingressName, val, err)
		} else {
			ruleAnnotation[n.GetAlbRuleRewriteRequestAnnotation()] = val
		}
	}
	return ruleAnnotation
}

func ruleInPolicyFromRuleAnnotation(rule *mt.Rule, domain string, cfg *types.RuleConfigInPolicy) {
	n := config.NewNames(domain)
	annotation := rule.Annotations
	ruleName := rule.Name
	if val, ok := annotation[n.GetAlbRuleRewriteResponseAnnotation()]; ok {
		rewriteCfg, err := rewriteResponseConfigFromJson(val)
		if err != nil {
			klog.Errorf("ext rule rewrite_response: invalid annotation in rule '%v' annotation is '%v' err %v", ruleName, val, err)
		} else {
			cfg.RewriteResponse = rewriteCfg
		}
	}
	if val, ok := annotation[n.GetAlbRuleRewriteRequestAnnotation()]; ok {
		rewriteCfg, err := rewriteRequestConfigFromJson(val)
		if err != nil {
			klog.Errorf("ext rule rewrite_request: invalid annotation in rule '%v' annotation is '%v' err %v", ruleName, val, err)
		} else {
			cfg.RewriteRequest = rewriteCfg
		}
	}
}
