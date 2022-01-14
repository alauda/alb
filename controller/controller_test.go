package controller

import (
	"context"
	"os"
	"runtime"
	"sort"
	"testing"

	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils"
	"alauda.io/alb2/utils/test_utils"
	"github.com/stretchr/testify/assert"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRule_GetPriority(t *testing.T) {
	type fields struct {
		Priority int
		DSLX     v1.DSLX
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		{
			name: "include priority",
			fields: fields{
				Priority: 100,
				DSLX: []v1.DSLXTerm{
					{
						Values: [][]string{{"START_WITH", "/"}},
						Type:   "URL",
					},
				},
			},
			want: 10000 + 100,
		},
		// notice that this rule has same priority with above rule, and that is not correct
		// we will compare the length of whole dlsx when compare policy
		{
			name: "include priority 1",
			fields: fields{
				Priority: 100,
				DSLX: []v1.DSLXTerm{
					{
						Values: [][]string{{"START_WITH", "/a"}},
						Type:   "URL",
					},
				},
			},
			want: 10000 + 100,
		},

		{
			name:   "no priority with dsl 1",
			fields: fields{},
			want:   0,
		},
		{
			name: "no priority with dslx",
			fields: fields{
				DSLX: []v1.DSLXTerm{
					{
						Values: [][]string{{"START_WITH", "/"}},
						Type:   "URL",
					},
				},
			},
			want: 10000 + 100,
		},
		{
			name: "no priority with dslx",
			fields: fields{
				DSLX: []v1.DSLXTerm{
					{
						Values: [][]string{{"START_WITH", "/"}},
						Type:   "URL",
					},
				},
			},
			want: 10000 + 100,
		},
		{
			name: "no priority with complex dslx",
			fields: fields{
				DSLX: []v1.DSLXTerm{
					{
						Values: [][]string{{"START_WITH", "/k8s"}, {"START_WITH", "/kubernetes"}},
						Type:   "URL",
					},
					{
						Values: [][]string{{"EQ", "lorem"}},
						Type:   "COOKIE",
						Key:    "test",
					},
				},
			},
			want: 10000 + 100 + 100 + 10000 + 100,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := Rule{
				Priority: tt.fields.Priority,
				DSLX:     tt.fields.DSLX,
			}
			if got := rl.GetPriority(); got != tt.want {
				t.Errorf("GetPriority() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortPolicy(t *testing.T) {

	tests := []struct {
		name     string
		policies Policies
		order    []string
	}{
		{
			name: "compare rawpriority first",
			policies: []*Policy{
				{
					Rule:        "a",
					RawPriority: 5,
					Priority:    10000 + 100,
					InternalDSL: []interface{}{[]string{"STARTS_WITH", "URL", "/"}},
				},
				{
					Rule:        "b",
					RawPriority: 1,
					Priority:    10000 + 100,
					InternalDSL: []interface{}{[]string{"STARTS_WITH", "URL", "/b"}},
				},
			},
			order: []string{"b", "a"},
		},
		{
			name: "same rawpriority compare priority",
			policies: []*Policy{
				{
					Rule:        "a",
					RawPriority: 5,
					Priority:    10000 + 100,
					InternalDSL: []interface{}{[]string{"STARTS_WITH", "URL", "/"}},
				},
				{
					Rule:        "b",
					RawPriority: 5,
					Priority:    10000 + 100 + 1,
					InternalDSL: []interface{}{[]string{"STARTS_WITH", "URL", "/b"}},
				},
			},
			order: []string{"b", "a"},
		},
		{
			name: "same priority and raw priority compare complex",
			policies: []*Policy{
				{
					Rule:        "a",
					RawPriority: 5,
					Priority:    10000 + 100,
					InternalDSL: []interface{}{[]string{"STARTS_WITH", "URL", "/"}},
				},
				{
					Rule:        "b",
					RawPriority: 5,
					Priority:    10000 + 100,
					InternalDSL: []interface{}{[]string{"STARTS_WITH", "URL", "/b"}},
				},
			},
			order: []string{"b", "a"},
		},
		// we have nothing to do,but must stablelize the order when compare policy,so we compare the name
		{
			name: "same priority and raw priority and same complex",
			policies: []*Policy{
				{
					Rule:        "b",
					RawPriority: 5,
					Priority:    10000 + 100,
					InternalDSL: []interface{}{[]string{"STARTS_WITH", "URL", "/"}},
				},
				{
					Rule:        "a",
					RawPriority: 5,
					Priority:    10000 + 100,
					InternalDSL: []interface{}{[]string{"STARTS_WITH", "URL", "/"}},
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
		cfg           map[string]string
		fakeResource  test_utils.FakeResource
		expectActions []GCAction
	}

	defaultConfig := map[string]string{
		"DOMAIN":          "alauda.io",
		"TWEAK_DIRECTORY": "../driver/texture", // set TWEAK_DIRECTORY to a exist path: make calculate hash happy
		"NAME":            "alb-1",
		"NAMESPACE":       "ns-1",
	}

	defaultAlbs := []v1.ALB2{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alb-1",
				Namespace: "ns-1",
			},
			Spec: v1.ALB2Spec{
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
			cfg:     defaultConfig,
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
			cfg:     defaultConfig,
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
			cfg:     defaultConfig,
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
				{Kind: DeleteRule, Name: "rule-1", Namespace: "ns-1", Reason: RuleAllSerivceNonExist},
			},
			options: defaultGCOptions,
			cfg:     defaultConfig,
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
				{Kind: DeleteRule, Name: "rule-1", Namespace: "ns-1", Reason: RuleSerivceBindkeyEmpty},
			},
			options: defaultGCOptions,
			cfg:     defaultConfig,
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
			cfg:           defaultConfig,
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
		drv, _ := test_utils.InitFakeAlb(t, ctx, testCase.fakeResource, test_utils.DEFAULT_CONFIG_FOR_TEST)

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
