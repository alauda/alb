package controller

import (
	"encoding/json"
	"fmt"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/controller/types"
	"k8s.io/klog/v2"
)

func GetAlbIngressRewriteResponseAnnotation() string {
	return fmt.Sprintf("alb.ingress.%s/rewrite-response", config.GetConfig().GetDomain())
}
func GetAlbRuleRewriteResponseAnnotation() string {
	return fmt.Sprintf("alb.rule.%s/rewrite-response", config.GetConfig().GetDomain())
}
func GetAlbIngressRewriteRequestAnnotation() string {
	return fmt.Sprintf("alb.ingress.%s/rewrite-request", config.GetConfig().GetDomain())
}
func GetAlbRuleRewriteRequestAnnotation() string {
	return fmt.Sprintf("alb.rule.%s/rewrite-request", config.GetConfig().GetDomain())
}

func rewriteResponseConfigFromJson(jsonStr string) (*RewriteResponseConfig, error) {
	cfg := RewriteResponseConfig{}
	err := json.Unmarshal([]byte(jsonStr), &cfg)
	if err != nil {
		return nil, err
	}
	if cfg.IsEmpty() {
		return nil, fmt.Errorf("empty config")
	}
	return &cfg, err
}

func rewriteRequestConfigFromJson(jsonStr string) (*RewriteRequestConfig, error) {
	cfg := RewriteRequestConfig{}
	err := json.Unmarshal([]byte(jsonStr), &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, err
}

func GenerateRuleAnnotationFromIngressAnnotation(ingressName string, annotation map[string]string) map[string]string {

	ruleAnnotation := make(map[string]string)

	if val, ok := annotation[GetAlbIngressRewriteResponseAnnotation()]; ok {
		_, err := rewriteResponseConfigFromJson(val)
		if err != nil {
			klog.Errorf("ext ingress rewrite_response: invalid annotation in ingress '%v' annotation is '%v' err %v", ingressName, val, err)
		} else {
			ruleAnnotation[GetAlbRuleRewriteResponseAnnotation()] = val
		}
	}
	if val, ok := annotation[GetAlbIngressRewriteRequestAnnotation()]; ok {
		_, err := rewriteRequestConfigFromJson(val)
		if err != nil {
			klog.Errorf("ext ingress rewrite_request: invalid annotation in ingress '%v' annotation is '%v' err %v", ingressName, val, err)
		} else {
			ruleAnnotation[GetAlbRuleRewriteRequestAnnotation()] = val
		}
	}
	return ruleAnnotation
}

func RuleConfigFromRuleAnnotation(ruleName string, annotation map[string]string) *RuleConfig {
	cfg := RuleConfig{}

	if val, ok := annotation[GetAlbRuleRewriteResponseAnnotation()]; ok {
		rewriteCfg, err := rewriteResponseConfigFromJson(val)
		if err != nil {
			klog.Errorf("ext rule rewrite_response: invalid annotation in rule '%v' annotation is '%v' err %v", ruleName, val, err)
		} else {
			cfg.RewriteResponse = rewriteCfg
		}
	}
	if val, ok := annotation[GetAlbRuleRewriteRequestAnnotation()]; ok {
		rewriteCfg, err := rewriteRequestConfigFromJson(val)
		if err != nil {
			klog.Errorf("ext rule rewrite_request: invalid annotation in rule '%v' annotation is '%v' err %v", ruleName, val, err)
		} else {
			cfg.RewriteRequest = rewriteCfg
		}
	}
	if cfg.IsEmpty() {
		return nil
	}
	return &cfg
}
