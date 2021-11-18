package controller

import (
	"encoding/json"
	"fmt"

	"k8s.io/klog"
)

const ALBIngressRewriteResponseAnnotation = "alb.ingress.kubernetes.io/rewrite-response"
const RuleRewriteResponseAnnotation = "alb.rule.kubernetes.io/rewrite-response"

type RuleConfig struct {
	RewriteResponse *RewriteResponseConfig `json:"rewrite_response,omitempty"`
}
type RewriteResponseConfig struct {
	Headers map[string]string `json:"headers,omitempty"`
}

func (r RewriteResponseConfig) isEmpty() bool {
	return len(r.Headers) == 0
}

func (r RuleConfig) ToJsonString() (string, error) {
	ret, err := json.Marshal(&r)
	return string(ret), err
}

func (r RuleConfig) isEmpty() bool {
	if r.RewriteResponse != nil && !r.RewriteResponse.isEmpty() {
		return false
	}
	return true
}

func rewriteResponseConfigFromJson(jsonStr string) (*RewriteResponseConfig, error) {
	cfg := RewriteResponseConfig{}
	err := json.Unmarshal([]byte(jsonStr), &cfg)
	if err != nil {
		return nil, err
	}
	if cfg.isEmpty() {
		return nil, fmt.Errorf("empty config")
	}
	return &cfg, err
}

func GenerateRuleAnnotationFromIngressAnnotation(ingressName string, annotation map[string]string) map[string]string {

	ruleAnnotation := make(map[string]string)

	if val, ok := annotation[ALBIngressRewriteResponseAnnotation]; ok {
		_, err := rewriteResponseConfigFromJson(val)
		if err != nil {
			klog.Errorf("ext ingress rewrite_response: invalid annotation in ingress %v annotation %v err %v", ingressName, val, err)
		} else {
			ruleAnnotation[RuleRewriteResponseAnnotation] = val
		}
	}
	return ruleAnnotation
}

func RuleConfigFromRuleAnnotation(ruleName string, annotation map[string]string) *RuleConfig {
	cfg := RuleConfig{}

	if val, ok := annotation[RuleRewriteResponseAnnotation]; ok {
		rewriteCfg, err := rewriteResponseConfigFromJson(val)
		if err != nil {
			klog.Errorf("ext rule rewrite_response: invalid annotation in rule %v annotation %v err %v", ruleName, val, err)
		} else {
			cfg.RewriteResponse = rewriteCfg
		}
	}
	if cfg.isEmpty() {
		return nil
	}
	return &cfg
}
