package controller

import (
	"fmt"
	"testing"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/controller/types"
	"github.com/stretchr/testify/assert"
)

func TestRuleConfig(t *testing.T) {
	config.UseMock(config.DefaultMock())
	ALBIngressRewriteResponseAnnotation := GetAlbIngressRewriteResponseAnnotation()
	RuleRewriteResponseAnnotation := GetAlbRuleRewriteResponseAnnotation()
	type TestCase struct {
		ingressAnnotation    map[string]string
		expectRuleAnnotation map[string]string
		expectRuleConfig     *RuleConfig
	}

	case1 := TestCase{
		map[string]string{},
		map[string]string{},
		nil,
	}
	// should add annotation to rule when ingress annotation is correctly and should get the rule config correctly
	case2 := TestCase{
		map[string]string{ALBIngressRewriteResponseAnnotation: `{"headers":{"aa":"bb"}}`},
		map[string]string{RuleRewriteResponseAnnotation: `{"headers":{"aa":"bb"}}`},
		&RuleConfig{
			RewriteResponse: &RewriteResponseConfig{
				Headers: map[string]string{
					"aa": "bb",
				},
			},
		},
	}
	// if ingress annotation is invalid,rule annotation should be nil,rule config should be nil.
	case3 := TestCase{
		map[string]string{ALBIngressRewriteResponseAnnotation: `a invalid ingress annotation`},
		map[string]string{},
		nil,
	}
	// if ingress annotation is valid but in fact is empty cfg,rule annotation should be nil,rule config should be nil.
	case4 := TestCase{
		map[string]string{ALBIngressRewriteResponseAnnotation: `{}`},
		map[string]string{},
		nil,
	}
	_ = case1
	_ = case2
	_ = case3
	_ = case4
	cases := []TestCase{
		case1,
		case2,
		case3,
		case4,
	}
	for i, c := range cases {
		ruleAnnotation := GenerateRuleAnnotationFromIngressAnnotation("xx", c.ingressAnnotation)
		assert.Equal(t, ruleAnnotation, c.expectRuleAnnotation, fmt.Sprintf("case %v fail", i+1))
		cfg := RuleConfigFromRuleAnnotation("", ruleAnnotation)
		assert.Equal(t, cfg, c.expectRuleConfig, fmt.Sprintf("case %v fail", i+1))
	}

	type RuleTestCase struct {
		ruleAnnotation   map[string]string
		expectRuleConfig *RuleConfig
	}
	// if rule annotation is invalid, rule config should be nil.
	ruleCase1 := RuleTestCase{
		map[string]string{RuleRewriteResponseAnnotation: "a invalid rule annotation"},
		nil,
	}
	// if rule annotation is empty, rule config should be nil.
	ruleCase2 := RuleTestCase{
		map[string]string{RuleRewriteResponseAnnotation: `{"sth":"unrelated"}`},
		nil,
	}
	_ = ruleCase1
	_ = ruleCase2
	ruleCases := []RuleTestCase{
		ruleCase1,
		ruleCase2,
	}
	for _, c := range ruleCases {
		cfg := RuleConfigFromRuleAnnotation("", c.ruleAnnotation)
		assert.Equal(t, cfg, c.expectRuleConfig)
	}
}
