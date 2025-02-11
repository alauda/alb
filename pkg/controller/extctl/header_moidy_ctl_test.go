package extctl

import (
	"fmt"
	"testing"

	"alauda.io/alb2/config"
	"alauda.io/alb2/controller/modules"
	. "alauda.io/alb2/controller/types"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils/log"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRuleConfig(t *testing.T) {
	cfg := config.DefaultMock()
	config.UseMock(cfg)
	n := cfg.Names
	ALBIngressRewriteRequestAnnotation := n.GetAlbIngressRewriteRequestAnnotation()
	RuleRewriteRequestAnnotation := n.GetAlbRuleRewriteRequestAnnotation()
	ALBIngressRewriteResponseAnnotation := n.GetAlbIngressRewriteResponseAnnotation()
	RuleRewriteResponseAnnotation := n.GetAlbRuleRewriteResponseAnnotation()
	type TestCase struct {
		ingressAnnotation    map[string]string
		expectRuleAnnotation map[string]string
		expectRuleConfig     *RuleExt
	}
	empty := &RuleExt{}
	case1 := TestCase{
		map[string]string{},
		map[string]string{},
		empty,
	}
	// should add annotation to rule when ingress annotation is correctly and should get the rule config correctly
	case2 := TestCase{
		map[string]string{ALBIngressRewriteResponseAnnotation: `{"headers":{"aa":"bb"}}`},
		map[string]string{RuleRewriteResponseAnnotation: `{"headers":{"aa":"bb"}}`},
		&RuleExt{
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
		empty,
	}
	// if ingress annotation is valid but in fact is empty cfg,rule annotation should be nil,rule config should be nil.
	case4 := TestCase{
		map[string]string{ALBIngressRewriteResponseAnnotation: `{}`},
		map[string]string{},
		empty,
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
		{
			map[string]string{ALBIngressRewriteRequestAnnotation: `{"headers_var":{"a":"cookie_b"},"headers_add_var":{"aa":["cookie_b"]}}`},
			map[string]string{RuleRewriteRequestAnnotation: `{"headers_var":{"a":"cookie_b"},"headers_add_var":{"aa":["cookie_b"]}}`},
			&RuleExt{
				RewriteRequest: &RewriteRequestConfig{
					HeadersVar: map[string]string{
						"a": "cookie_b",
					},
					HeadersAddVar: map[string][]string{
						"aa": {"cookie_b"},
					},
				},
			},
		},
	}
	for i, c := range cases {
		ctl := HeaderModifyCtl{
			domain: cfg.Domain,
			log:    log.L(),
		}
		ruleAnnotation := ctl.GenRewriteResponseOrRequestRuleAnnotation("xx", c.ingressAnnotation, cfg.Domain)
		assert.Equal(t, ruleAnnotation, c.expectRuleAnnotation, fmt.Sprintf("case %v fail", i+1))
		rule := &modules.Rule{
			Rule: &albv1.Rule{
				ObjectMeta: v1.ObjectMeta{
					Name:        "xx",
					Annotations: ruleAnnotation,
				},
			},
		}
		ir := &InternalRule{
			Config: RuleExt{},
		}
		ctl.ToInternalRule(rule, ir)
		assert.Equal(t, ir.Config, *c.expectRuleConfig, fmt.Sprintf("case %v fail", i+1))
	}

	type RuleTestCase struct {
		ruleAnnotation   map[string]string
		expectRuleConfig *RuleExt
	}
	// if rule annotation is invalid, rule config should be nil.
	ruleCase1 := RuleTestCase{
		map[string]string{RuleRewriteResponseAnnotation: "a invalid rule annotation"},
		empty,
	}
	// if rule annotation is empty, rule config should be nil.
	ruleCase2 := RuleTestCase{
		map[string]string{RuleRewriteResponseAnnotation: `{"sth":"unrelated"}`},
		empty,
	}
	_ = ruleCase1
	_ = ruleCase2
	ruleCases := []RuleTestCase{
		ruleCase1,
		ruleCase2,
	}
	for _, c := range ruleCases {
		rule := &modules.Rule{
			Rule: &albv1.Rule{
				ObjectMeta: v1.ObjectMeta{
					Name:        "xx",
					Annotations: c.ruleAnnotation,
				},
			},
		}
		ir := &InternalRule{Config: RuleExt{}}

		ctl := HeaderModifyCtl{
			domain: cfg.Domain,
			log:    log.L(),
		}
		ctl.ToInternalRule(rule, ir)
		assert.Equal(t, ir.Config, *c.expectRuleConfig)
	}
}
