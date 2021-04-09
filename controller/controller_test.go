package controller

import (
	"context"
	"testing"

	"alauda.io/alb2/config"
	drv "alauda.io/alb2/driver"
	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albfakeclient "alauda.io/alb2/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRule_GetPriority(t *testing.T) {
	type fields struct {
		Priority int
		DSL      string
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
				DSL:      "(START_WITH URL /)",
				DSLX: []v1.DSLXTerm{
					{
						Values: [][]string{{"START_WITH", "/"}},
						Type:   "URL",
					},
				},
			},
			want: 10000 + 100 + len("(START_WITH URL /)"),
		},
		{
			name: "no priority with dsl 1",
			fields: fields{
				DSL: "(START_WITH URL /)",
			},
			want: 10000 + 100 + len("(START_WITH URL /)"),
		},
		{
			name: "no priority with dsl 2",
			fields: fields{
				DSL: "(START_WITH URL /lorem)",
			},
			want: 10000 + 100 + len("(START_WITH URL /lorem)"),
		},
		{
			name: "no priority with dslx",
			fields: fields{
				DSL: "(START_WITH URL /)",
				DSLX: []v1.DSLXTerm{
					{
						Values: [][]string{{"START_WITH", "/"}},
						Type:   "URL",
					},
				},
			},
			want: 10000 + 100 + len("(START_WITH URL /)"),
		},
		{
			name: "no priority with dslx",
			fields: fields{
				DSL: "(START_WITH URL /)",
				DSLX: []v1.DSLXTerm{
					{
						Values: [][]string{{"START_WITH", "/"}},
						Type:   "URL",
					},
				},
			},
			want: 10000 + 100 + len("(START_WITH URL /)"),
		},
		{
			name: "no priority with complex dslx",
			fields: fields{
				DSL: "(AND (OR (START_WITH URL /k8s) (START_WITH URL /kubernetes)) (EQ COOKIE test lorem))",
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
			want: 10000 + 100 + 100 + 10000 + 100 + len("(AND (OR (START_WITH URL /k8s) (START_WITH URL /kubernetes)) (EQ COOKIE test lorem))"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := Rule{
				Priority: tt.fields.Priority,
				DSL:      tt.fields.DSL,
				DSLX:     tt.fields.DSLX,
			}
			if got := rl.GetPriority(); got != tt.want {
				t.Errorf("GetPriority() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGCRule(t *testing.T) {

	type TestCase struct {
		description   string
		options       GCOptions
		cfg           map[string]string
		albs          []v1.ALB2
		frontends     []v1.Frontend
		rules         []v1.Rule
		services      []k8sv1.Service
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
			albs:    defaultAlbs,
			frontends: []v1.Frontend{
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

		{
			description: "frontend default backend service bindkey is empty",
			expectActions: []GCAction{
				{Kind: UpdateFT, Name: "ft-1", Namespace: "ns-1", Reason: FTServiceBindkeyEmpty},
			},
			options: defaultGCOptions,
			cfg:     defaultConfig,
			albs:    defaultAlbs,
			frontends: []v1.Frontend{
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
			services: []k8sv1.Service{
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
		{
			description: "rule belongs to a orphaned app",
			expectActions: []GCAction{
				{Kind: DeleteRule, Name: "rule-1", Namespace: "ns-1", Reason: RuleOrphaned},
			},
			options: defaultGCOptions,
			cfg:     defaultConfig,
			albs:    defaultAlbs,
			frontends: []v1.Frontend{
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
			rules: []v1.Rule{
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

		{
			description: "all backend service are none-exist",
			expectActions: []GCAction{
				{Kind: DeleteRule, Name: "rule-1", Namespace: "ns-1", Reason: RuleAllSerivceNonExist},
			},
			options: defaultGCOptions,
			cfg:     defaultConfig,
			albs:    defaultAlbs,
			frontends: []v1.Frontend{
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
			rules: []v1.Rule{
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
		{
			description: "rule service bindkey is empty",
			expectActions: []GCAction{
				{Kind: DeleteRule, Name: "rule-1", Namespace: "ns-1", Reason: RuleSerivceBindkeyEmpty},
			},
			options: defaultGCOptions,
			cfg:     defaultConfig,
			albs:    defaultAlbs,
			frontends: []v1.Frontend{
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
			rules: []v1.Rule{
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
			services: []k8sv1.Service{
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
		{
			description:   "do not do gc, if rules's type is ingress",
			expectActions: []GCAction{},
			options:       defaultGCOptions,
			cfg:           defaultConfig,
			albs:          defaultAlbs,
			frontends: []v1.Frontend{
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
			rules: []v1.Rule{
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
	}

	for _, testCase := range testCases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		for key, val := range testCase.cfg {
			config.Set(key, val)
		}

		a := assert.New(t)
		driver, err := drv.GetKubernetesDriver(true)
		a.NoError(err)
		a.NoError(err)

		var albList v1.ALB2List = v1.ALB2List{
			Items: testCase.albs,
		}

		var ftList v1.FrontendList = v1.FrontendList{
			Items: testCase.frontends,
		}

		var ruleList v1.RuleList = v1.RuleList{
			Items: testCase.rules,
		}

		serviceList := k8sv1.ServiceList{
			Items: testCase.services,
		}

		crdDataset := []runtime.Object{&albList, &ftList, &ruleList}
		nativeDataset := []runtime.Object{&serviceList}
		driver.ALBClient = albfakeclient.NewSimpleClientset(crdDataset...)
		driver.Client = fake.NewSimpleClientset(nativeDataset...)
		drv.InitDriver(driver, ctx)

		actions, err := calculateGCActions(driver, GCOptions{
			GCAppRule:     true,
			GCServiceRule: true,
		})
		a.NoError(err)
		a.ElementsMatch(actions, testCase.expectActions, testCase.description)

	}
}
