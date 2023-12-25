package controller

import (
	"context"
	"os"
	"runtime"
	"sort"
	"testing"

	"alauda.io/alb2/config"
	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/utils"
	"alauda.io/alb2/utils/test_utils"
	"github.com/stretchr/testify/assert"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDslString(t *testing.T) {

	dslx := v1.DSLX{
		{
			Values: [][]string{{utils.OP_STARTS_WITH, "/k8s"}, {utils.OP_REGEX, "^/v1/*"}},
			Type:   utils.KEY_URL,
		},
	}
	assert.Equal(t, dslx.ToSearchbleString(), "[{[[STARTS_WITH /k8s] [REGEX ^/v1/*]] URL }]")
}

func TestRuleOrder(t *testing.T) {
	type fields struct {
		Name string
		DSLX v1.DSLX
	}
	rules := []fields{
		{
			Name: "start with /abc or regex ^/v1/*",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/k8s"}, {utils.OP_REGEX, "^/v1/*"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "start with /abcde",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/abcde"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "start with /abc",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/abc"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "wildcard host and regex /abcd.*",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_ENDS_WITH, ".c.com"}},
					Type:   utils.KEY_HOST,
				},
				{
					Values: [][]string{{utils.OP_REGEX, "/abcd.*"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "wildcard host and regex /a.*",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_ENDS_WITH, ".c.com"}},
					Type:   utils.KEY_HOST,
				},
				{
					Values: [][]string{{utils.OP_REGEX, "/a.*"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "wildcard host and start with /abc",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_ENDS_WITH, ".c.com"}},
					Type:   utils.KEY_HOST,
				},
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/abc"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "wildcard host and start with /ab",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_ENDS_WITH, ".c.com"}},
					Type:   utils.KEY_HOST,
				},
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/ab"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "wildcard host and start with /",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_ENDS_WITH, ".c.com"}},
					Type:   utils.KEY_HOST,
				},
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "wildcard host",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_ENDS_WITH, ".c.com"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			Name: "start with /",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "start with /abc with host",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/abc"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "a.c"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			Name: "start with / with host",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "a.c"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			Name: "regex /a.* wiht host",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_REGEX, "/a.*"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "a.c"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			Name: "exact /a  with host",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_EQ, "/a"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "a.com"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			Name: "regex /abcd.*",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_REGEX, "/abcd.*"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "regex /a.*",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_REGEX, "/a.*"}},
					Type:   utils.KEY_URL,
				},
			},
		},
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].DSLX.Priority() > rules[j].DSLX.Priority()
	})
	expectOrder := []string{
		"exact /a  with host",
		"start with /abc with host",
		"regex /a.* wiht host",
		"start with / with host",
		"wildcard host and regex /abcd.*",
		"wildcard host and start with /abc",
		"wildcard host and start with /ab",
		"wildcard host and regex /a.*",
		"wildcard host and start with /",
		"wildcard host",
		"start with /abc or regex ^/v1/*",
		"start with /abcde",
		"regex /abcd.*",
		"start with /abc",
		"regex /a.*",
		"start with /",
	}
	order := []string{}
	for _, r := range rules {
		order = append(order, r.Name)
		t.Logf(r.Name)
	}
	assert.Equal(t, expectOrder, order)
}

func TestSortPolicy(t *testing.T) {

	tests := []struct {
		name     string
		policies Policies
		order    []string
	}{
		{
			name: "compare policy RawPriority first",
			policies: []*Policy{
				{
					Rule:        "a",
					RawPriority: 5,
					Priority:    50000 + 1000,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/"}},
				},
				{
					Rule:        "b",
					RawPriority: 1,
					Priority:    10000 + 500,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/b"}},
				},
			},
			order: []string{"b", "a"},
		},
		{
			name: "compare policy RawPriority first",
			policies: []*Policy{
				{
					Rule:        "a",
					RawPriority: 5,
					Priority:    50000 + 1000,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/"}},
				},
				{
					Rule:        "b",
					RawPriority: 1,
					Priority:    10000 + 500,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/b"}},
				},
				{
					Rule:        "c",
					RawPriority: -1,
					Priority:    10000 + 500,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/b"}},
				},
			},
			order: []string{"c", "b", "a"},
		},
		{
			name: "same RawPriority compare priority",
			policies: []*Policy{
				{
					Rule:        "a",
					RawPriority: 5,
					Priority:    10000 + 1000 + 500 + 100,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/"}},
				},
				{
					Rule:        "b",
					RawPriority: 5,
					Priority:    50000 + 1000,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/b"}},
				},
			},
			order: []string{"b", "a"},
		},
		{
			name: "same RawPriority and priority, compare complex",
			policies: []*Policy{
				{
					Rule:        "a",
					RawPriority: 5,
					Priority:    10000 + 1000,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/"}},
				},
				{
					Rule:        "b",
					RawPriority: 5,
					Priority:    10000 + 1000,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/b"}},
				},
			},
			order: []string{"b", "a"},
		},
		// totally same priority and DSL, compare name to stabilize the order when compare policy
		{
			name: "same RawPriority/priority and complex, compare name to stabilize order",
			policies: []*Policy{
				{
					Rule:        "b",
					RawPriority: 5,
					Priority:    10000 + 1000,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/"}},
				},
				{
					Rule:        "a",
					RawPriority: 5,
					Priority:    10000 + 1000,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/"}},
				},
			},
			order: []string{"a", "b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := range tt.policies {
				tt.policies[i].InternalDSLLen = utils.InternalDSLLen((tt.policies[i].InternalDSL))
			}
			sort.Sort(tt.policies)
			realOrder := []string{}
			for _, p := range tt.policies {
				realOrder = append(realOrder, p.Rule)
			}
			assert.Equal(t, realOrder, tt.order, tt.name+"fail")
		})
	}
}

func TestGCRule(t *testing.T) {

	type TestCase struct {
		description   string
		options       GCOptions
		fakeResource  test_utils.FakeResource
		expectActions []GCAction
	}

	defaultAlbs := []albv2.ALB2{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alb-1",
				Namespace: "ns-1",
			},
			Spec: albv2.ALB2Spec{
				Address: "1.2.3.4",
				Type:    "nginx",
			},
		},
	}

	defaultGCOptions := GCOptions{
		GCAppRule:     true,
		GCServiceRule: true,
	}

	testCases := []TestCase{
		{
			description: "frontend's default backend service is none-exist",
			expectActions: []GCAction{
				{Kind: UpdateFT, Name: "ft-1", Namespace: "ns-1", Reason: FTServiceNonExist},
			},
			options: defaultGCOptions,
			fakeResource: test_utils.FakeResource{
				Alb: test_utils.FakeALBResource{
					Albs: defaultAlbs,
					Frontends: []v1.Frontend{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "ft-1",
								Namespace: "ns-1",
								Labels: map[string]string{
									"alb2.alauda.io/name": "alb-1",
								},
							},
							Spec: v1.FrontendSpec{
								Port:     12345,
								Protocol: "tcp",
								Source: &v1.Source{
									Name:      "ft-source-1",
									Namespace: "ns-1",
									Type:      "bind",
								},
								ServiceGroup: &v1.ServiceGroup{
									Services: []v1.Service{
										{
											Name: "ft-default-backend-service-whcih-should-not-exist",
										},
									},
								},
							},
						},
					},
				},
			},
		},

		{
			description: "frontend default backend service bindkey is empty",
			expectActions: []GCAction{
				{Kind: UpdateFT, Name: "ft-1", Namespace: "ns-1", Reason: FTServiceBindkeyEmpty},
			},
			options: defaultGCOptions,
			fakeResource: test_utils.FakeResource{
				Alb: test_utils.FakeALBResource{
					Albs: defaultAlbs,
					Frontends: []v1.Frontend{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "ft-1",
								Namespace: "ns-1",
								Labels: map[string]string{
									"alb2.alauda.io/name": "alb-1",
								},
							},
							Spec: v1.FrontendSpec{
								Port:     12345,
								Protocol: "tcp",
								Source: &v1.Source{
									Name:      "ft-source-1",
									Namespace: "ns-1",
									Type:      "bind",
								},
								ServiceGroup: &v1.ServiceGroup{
									Services: []v1.Service{
										{
											Name:      "ft-service-1",
											Namespace: "ns-1",
										},
									},
								},
							},
						},
					},
				},
				K8s: test_utils.FakeK8sResource{
					Services: []k8sv1.Service{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "ft-service-1",
								Namespace: "ns-1",
								Annotations: map[string]string{
									"alb2.alauda.io/bindkey": "[]",
								},
							},
						},
					},
				},
			},
		},
		{
			description: "rule belongs to a orphaned app",
			expectActions: []GCAction{
				{Kind: DeleteRule, Name: "rule-1", Namespace: "ns-1", Reason: RuleOrphaned},
			},
			options: defaultGCOptions,
			fakeResource: test_utils.FakeResource{
				Alb: test_utils.FakeALBResource{
					Albs: defaultAlbs,
					Frontends: []v1.Frontend{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "ft-1",
								Namespace: "ns-1",
								Labels: map[string]string{
									"alb2.alauda.io/name": "alb-1",
								},
							},
							Spec: v1.FrontendSpec{
								Port:     12345,
								Protocol: "http",
								Source: &v1.Source{
									Name:      "ft-source-1",
									Namespace: "ns-1",
									Type:      "bind",
								},
								ServiceGroup: &v1.ServiceGroup{
									Services: []v1.Service{
										{
											Name:      "ft-service-1",
											Namespace: "ns-1",
										},
									},
								},
							},
						},
					},
					Rules: []v1.Rule{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "rule-1",
								Namespace: "ns-1",
								Labels: map[string]string{
									"alb2.alauda.io/name":     "alb-1",
									"alb2.alauda.io/frontend": "ft-1",
									"app.alauda.io/name":      "appname.nsname",
								},
							},
							Spec: v1.RuleSpec{
								Source: &v1.Source{
									Name:      "rule-source-1",
									Namespace: "ns-1",
									Type:      "bind",
								},
							},
						},
					},
				},
			},
		},

		{
			description: "all backend service are none-exist",
			expectActions: []GCAction{
				{Kind: DeleteRule, Name: "rule-1", Namespace: "ns-1", Reason: RuleAllServiceNonExist},
			},
			options: defaultGCOptions,
			fakeResource: test_utils.FakeResource{
				Alb: test_utils.FakeALBResource{
					Albs: defaultAlbs,
					Frontends: []v1.Frontend{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "ft-1",
								Namespace: "ns-1",
								Labels: map[string]string{
									"alb2.alauda.io/name": "alb-1",
								},
							},
							Spec: v1.FrontendSpec{
								Port:     12345,
								Protocol: "http",
								Source: &v1.Source{
									Name:      "ft-source-1",
									Namespace: "ns-1",
									Type:      "bind",
								},
								ServiceGroup: &v1.ServiceGroup{
									Services: []v1.Service{
										{
											Name:      "ft-service-1",
											Namespace: "ns-1",
										},
									},
								},
							},
						},
					},
					Rules: []v1.Rule{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "rule-1",
								Namespace: "ns-1",
								Labels: map[string]string{
									"alb2.alauda.io/name":     "alb-1",
									"alb2.alauda.io/frontend": "ft-1",
								},
							},
							Spec: v1.RuleSpec{
								ServiceGroup: &v1.ServiceGroup{
									Services: []v1.Service{
										{
											Name:      "rule-svc-1",
											Namespace: "ns-1",
										},
										{
											Name:      "rule-svc-2",
											Namespace: "ns-1",
										},
									},
								},
								Source: &v1.Source{
									Name:      "rule-source-1",
									Namespace: "ns-1",
									Type:      "bind",
								},
							},
						},
					},
				},
			},
		},
		{
			description: "rule service bindkey is empty",
			expectActions: []GCAction{
				{Kind: DeleteRule, Name: "rule-1", Namespace: "ns-1", Reason: RuleServiceBindkeyEmpty},
			},
			options: defaultGCOptions,
			fakeResource: test_utils.FakeResource{
				Alb: test_utils.FakeALBResource{
					Albs: defaultAlbs,
					Frontends: []v1.Frontend{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "ft-1",
								Namespace: "ns-1",
								Labels: map[string]string{
									"alb2.alauda.io/name": "alb-1",
								},
							},
							Spec: v1.FrontendSpec{
								Port:     12345,
								Protocol: "http",
								Source: &v1.Source{
									Name:      "ft-source-1",
									Namespace: "ns-1",
									Type:      "bind",
								},
								ServiceGroup: &v1.ServiceGroup{
									Services: []v1.Service{
										{
											Name:      "ft-service-1",
											Namespace: "ns-1",
										},
									},
								},
							},
						},
					},
					Rules: []v1.Rule{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "rule-1",
								Namespace: "ns-1",
								Labels: map[string]string{
									"alb2.alauda.io/name":     "alb-1",
									"alb2.alauda.io/frontend": "ft-1",
								},
							},
							Spec: v1.RuleSpec{
								ServiceGroup: &v1.ServiceGroup{
									Services: []v1.Service{
										{
											Name:      "rule-svc-1",
											Namespace: "ns-1",
										},
										{
											Name:      "rule-svc-2",
											Namespace: "ns-1",
										},
									},
								},
								Source: &v1.Source{
									Name:      "rule-source-1",
									Namespace: "ns-1",
									Type:      "bind",
								},
							},
						},
					},
				},
				K8s: test_utils.FakeK8sResource{
					Services: []k8sv1.Service{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "rule-svc-1",
								Namespace: "ns-1",
								Annotations: map[string]string{
									"alb2.alauda.io/bindkey": "[]",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "rule-svc-2",
								Namespace: "ns-1",
							},
						},
					},
				},
			},
		},
		{
			description:   "do not do gc, if rules's type is ingress",
			expectActions: []GCAction{},
			options:       defaultGCOptions,
			fakeResource: test_utils.FakeResource{
				Alb: test_utils.FakeALBResource{
					Albs: defaultAlbs,
					Frontends: []v1.Frontend{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "ft-1",
								Namespace: "ns-1",
								Labels: map[string]string{
									"alb2.alauda.io/name": "alb-1",
								},
							},
							Spec: v1.FrontendSpec{
								Port:     12345,
								Protocol: "http",
								Source: &v1.Source{
									Name:      "ft-source-1",
									Namespace: "ns-1",
									Type:      "ingress",
								},
								ServiceGroup: &v1.ServiceGroup{
									Services: []v1.Service{
										{
											Name:      "ft-service-1",
											Namespace: "ns-1",
										},
									},
								},
							},
						},
					},
					Rules: []v1.Rule{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "rule-1",
								Namespace: "ns-1",
								Labels: map[string]string{
									"alb2.alauda.io/name":     "alb-1",
									"alb2.alauda.io/frontend": "ft-1",
								},
							},
							Spec: v1.RuleSpec{
								ServiceGroup: &v1.ServiceGroup{
									Services: []v1.Service{
										{
											Name:      "rule-svc-1",
											Namespace: "ns-1",
										},
										{
											Name:      "rule-svc-2",
											Namespace: "ns-1",
										},
									},
								},
								Source: &v1.Source{
									Name:      "rule-source-1",
									Namespace: "ns-1",
									Type:      "ingress",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {

		ctx, cancel := context.WithCancel(context.Background())
		a := assert.New(t)
		defer cancel()
		cfg := config.DefaultMock()
		cfg.Domain = "alauda.io"
		cfg.Ns = "ns-1"
		cfg.Name = "alb-1"
		config.UseMock(cfg)
		drv := test_utils.InitFakeAlb(t, ctx, testCase.fakeResource)

		actions, err := calculateGCActions(drv, GCOptions{
			GCAppRule:     true,
			GCServiceRule: true,
		})
		a.NoError(err)
		a.ElementsMatch(actions, testCase.expectActions, testCase.description)

	}
}

func TestIsSameFile(t *testing.T) {
	_, current_go_path, _, _ := runtime.Caller(1)
	exe_path, err := os.Executable()
	assert.NoError(t, err)
	assert.True(t, sameFiles(current_go_path, current_go_path))
	assert.False(t, sameFiles(current_go_path, exe_path))
}
