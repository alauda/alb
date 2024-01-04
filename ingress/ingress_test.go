package ingress

import (
	"context"
	"testing"
	"time"

	"alauda.io/alb2/config"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/utils"
	"alauda.io/alb2/utils/log"
	"alauda.io/alb2/utils/test_utils"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	k8sv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetDSLX(t *testing.T) {

	tests := []struct {
		description string
		domain      string
		url         string
		pathType    networkingv1.PathType
		want        albv1.DSLX
	}{
		{
			description: "path is regex && type is impl spec, op should be regex",
			domain:      "alauda.io",
			url:         "^/v1/*",
			pathType:    networkingv1.PathTypeImplementationSpecific,
			want: []albv1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_REGEX, "^/v1/*"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "alauda.io"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			description: "path is regex && type is impl spec , op should be regex and add ^ prefix if it does not have",
			domain:      "alauda.io",
			url:         "/v1/*",
			pathType:    networkingv1.PathTypeImplementationSpecific,
			want: []albv1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_REGEX, "^/v1/*"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "alauda.io"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			description: "path is regex && type is exact, op should be eq",
			domain:      "alauda.io",
			url:         "/v1/*",
			pathType:    networkingv1.PathTypeExact,
			want: []albv1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_EQ, "/v1/*"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "alauda.io"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			description: "path is regex && type is prefix, op should be starts_with",
			domain:      "alauda.io",
			url:         "/v1/*",
			pathType:    networkingv1.PathTypePrefix,
			want: []albv1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/v1/*"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "alauda.io"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			description: "path is not regex and type is impl spec,op should be starts_with",
			domain:      "alauda.io",
			url:         "/v1",
			pathType:    networkingv1.PathTypeImplementationSpecific,
			want: []albv1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/v1"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "alauda.io"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			description: "path is not regex and type is prefix,op should be starts_with",
			domain:      "alauda.io",
			url:         "/v1",
			pathType:    networkingv1.PathTypePrefix,
			want: []albv1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/v1"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "alauda.io"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			description: "path is not regex and type is exact,op should be eq",
			domain:      "alauda.io",
			url:         "/v1",
			pathType:    networkingv1.PathTypeExact,
			want: []albv1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_EQ, "/v1"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "alauda.io"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
	}
	for _, test := range tests {
		dslx := GetDSLX(test.domain, test.url, test.pathType)
		assert.Equal(t, dslx, test.want, test.description)
	}
}

func TestNeedEnqueueObject(t *testing.T) {
	albName := "alb-1"
	nokIngressClass := string("test")
	okIngressClass := albName

	type TestCase struct {
		description   string
		only          bool
		skip          bool
		shouldEnqueue bool
		fakeResource  test_utils.FakeResource
		ingress       networkingv1.Ingress
	}

	defaultAlbs := []albv2.ALB2{
		{
			ObjectMeta: k8smetav1.ObjectMeta{
				Namespace: test_utils.DEFAULT_NS,
				Name:      albName,
			},
			Spec: albv2.ALB2Spec{
				Config: &albv2.ExternalAlbConfig{
					Projects: []string{"ALL_ALL", "project-1"},
				},
			},
		},
	}

	defaultNamespaces := []k8sv1.Namespace{
		{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name:   test_utils.DEFAULT_NS,
				Labels: map[string]string{"alauda.io/project": "random-project"},
			},
		},
	}

	defaultFakeResource := test_utils.FakeResource{
		Alb: test_utils.FakeALBResource{
			Albs: defaultAlbs,
		},
		K8s: test_utils.FakeK8sResource{
			IngressesClass: []networkingv1.IngressClass{
				{
					ObjectMeta: k8smetav1.ObjectMeta{
						Name:            "alb2",
						ResourceVersion: "1",
					},
					Spec: networkingv1.IngressClassSpec{
						Controller: "alauda.io/alb2",
					},
				},
			},
			Namespaces: defaultNamespaces,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	testCases := []TestCase{
		{
			description:   "do not enqueue when annotations['kubernetes.io/ingress.class'] is not empty and not alb name",
			shouldEnqueue: false,
			fakeResource:  defaultFakeResource,
			ingress: networkingv1.Ingress{
				ObjectMeta: k8smetav1.ObjectMeta{
					Namespace:       "ns-1",
					Name:            "ing-1",
					ResourceVersion: "1",
					Annotations:     map[string]string{"kubernetes.io/ingress.class": "x"},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "ingress-rule-1",
						},
					},
				},
			},
		},
		{
			description:   "enqueue when alb be assign to all project",
			shouldEnqueue: true,

			fakeResource: defaultFakeResource,
			ingress: networkingv1.Ingress{
				ObjectMeta: k8smetav1.ObjectMeta{
					Namespace:       "ns-1",
					Name:            "ing-2",
					ResourceVersion: "1",
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "ingress-rule-2",
						},
					},
				},
			},
		},
		{
			description:   "do not enqueue when spec.ingressClassName is not empty and irrelevant to alb2",
			shouldEnqueue: false,
			fakeResource:  defaultFakeResource,
			ingress: networkingv1.Ingress{
				ObjectMeta: k8smetav1.ObjectMeta{
					Namespace:       "ns-1",
					Name:            "ing-3",
					ResourceVersion: "1",
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: &nokIngressClass,
					Rules: []networkingv1.IngressRule{
						{
							Host: "ingress-rule-3",
						},
					},
				},
			},
		},
		{
			description:   "enqueue when spec.ingressClassName is not empty and relevant to alb2",
			shouldEnqueue: true,
			fakeResource:  defaultFakeResource,
			ingress: networkingv1.Ingress{
				ObjectMeta: k8smetav1.ObjectMeta{
					Namespace:       "ns-1",
					Name:            "ing-4",
					ResourceVersion: "1",
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: &okIngressClass,
					Rules: []networkingv1.IngressRule{
						{
							Host: "ingress-rule-4",
						},
					},
				},
			},
		},

		//TODO add more case here
		//1. enqueue when alb role is instance and ingress's project belongs to ft's project
		//2. enqueue when alb role is port and ingress's project belongs to alb's project
		//3. do not enqueue when alb role is port, without project and ingress without project neither
		//4. do not enqueue when ingress has spec.ingressClassName or annotation referenced to ingress controller,
		//   but irrelevant to alb2
	}

	// filter the testcase we what to run
	runCases := func(testCases []TestCase) []TestCase {
		runCases := make([]TestCase, 0)
		hasOnly := false
		// if there is only we just pick this only
		for _, testCase := range testCases {
			if testCase.only {
				hasOnly = true
				break
			}
		}
		for _, testCase := range testCases {
			if !hasOnly {
				if !testCase.skip {
					runCases = append(runCases, testCase)
				}
			} else {
				if testCase.only && !testCase.skip {
					runCases = append(runCases, testCase)
				}
			}
		}

		return runCases
	}(testCases)

	for index, testCase := range runCases {
		t.Logf("case %d: %s\n", index, testCase.description)
		a := assert.New(t)
		defer cancel()
		cfg := config.DefaultMock()
		cfg.Ns = "ns-1"
		cfg.Name = "alb-1"
		cfg.Domain = "alauda.io"
		config.UseMock(cfg)
		drv := test_utils.InitFakeAlb(t, ctx, testCase.fakeResource)
		informers := drv.Informers
		ingressController := NewController(drv, informers, cfg, log.L())
		// start to make sure ingress class cache synced.
		go func(ctx context.Context) {
			err := ingressController.StartIngressLoop(ctx)
			a.NoError(err)
		}(ctx)
		time.Sleep(10 * time.Millisecond)
		c, err := informers.K8s.IngressClass.Lister().Get("alb2")
		t.Logf("class err %v %v", c, err)
		alb, err := drv.LoadALBbyName(test_utils.DEFAULT_NS, test_utils.DEFAULT_ALB)
		a.NoError(err)
		need, reason := ingressController.shouldHandleIngress(alb, &testCase.ingress)
		t.Logf("class reason %v", reason)

		a.Equal(testCase.shouldEnqueue, need, testCase.description)

	}
}

func TestFindUnSyncedIngress(t *testing.T) {
	expect := []string{"ing-1", "ing-2"}
	var fakeResource = test_utils.FakeResource{
		Alb: test_utils.FakeALBResource{
			Rules: []albv1.Rule{
				{
					ObjectMeta: k8smetav1.ObjectMeta{
						Namespace: "ns-1",
						Name:      "alb-1-00443-1",
						Labels: map[string]string{
							"alb2.alauda.io/source-type": "ingress",
							"alb2.alauda.io/name":        "test1",
						},
					},
					Spec: albv1.RuleSpec{
						Source: &albv1.Source{
							Type:      "ingress",
							Namespace: "ns-2",
							Name:      "ing-1",
						},
					},
				},
				{
					ObjectMeta: k8smetav1.ObjectMeta{
						Namespace: "ns-1",
						Name:      "alb-1-00080-2",
						Annotations: map[string]string{
							"alb2.alauda.io/source-ingress-version": "1",
						},
						Labels: map[string]string{
							"alb2.alauda.io/source-type": "ingress",
							"alb2.alauda.io/name":        "alb-1",
						},
					},
					Spec: albv1.RuleSpec{
						Source: &albv1.Source{
							Type:      "ingress",
							Namespace: "ns-2",
							Name:      "ing-2",
						},
					},
				},
				{
					ObjectMeta: k8smetav1.ObjectMeta{
						Namespace: "ns-1",
						Name:      "alb-1-00080-3",
						Annotations: map[string]string{
							"alb2.alauda.io/source-ingress-version": "2",
						},
						Labels: map[string]string{
							"alb2.alauda.io/source-type": "ingress",
							"alb2.alauda.io/name":        "alb-1",
						},
					},
					Spec: albv1.RuleSpec{
						Source: &albv1.Source{
							Type:      "ingress",
							Namespace: "ns-2",
							Name:      "ing-3",
						},
					},
				},
			},
		},
		K8s: test_utils.FakeK8sResource{
			Ingresses: []networkingv1.Ingress{
				{
					ObjectMeta: k8smetav1.ObjectMeta{
						Namespace:       "ns-2",
						Name:            "ing-1",
						ResourceVersion: "1",
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "ingress-rule-1",
							},
						},
					},
				},
				{
					ObjectMeta: k8smetav1.ObjectMeta{
						Namespace:       "ns-2",
						Name:            "ing-2",
						ResourceVersion: "2",
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "ingress-rule-1",
							},
						},
					},
				},
				{
					ObjectMeta: k8smetav1.ObjectMeta{
						Namespace:       "ns-2",
						Name:            "ing-3",
						ResourceVersion: "2",
					},
					Spec: networkingv1.IngressSpec{
						Rules: []networkingv1.IngressRule{
							{
								Host: "ingress-rule-1",
							},
						},
					},
				},
			},
		},
	}
	_ = fakeResource
	_ = expect
}

func TestGenSyncAction(t *testing.T) {
	type testCase struct {
		ing    networkingv1.Ingress
		kind   string
		expect []*albv1.Rule
		exist  []*albv1.Rule
		cmd    SyncRule
	}
	backend := networkingv1.IngressBackend{
		Service: &networkingv1.IngressServiceBackend{
			Name: "svc-1",
			Port: networkingv1.ServiceBackendPort{
				Number: 80,
			},
		},
	}
	ing := networkingv1.Ingress{
		ObjectMeta: k8smetav1.ObjectMeta{
			ResourceVersion: "2",
			Name:            "ing-1",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:    "/test",
									Backend: backend,
								},
							},
						},
					},
				},
			},
		},
	}
	labels := map[string]string{
		"alb2.cpaas.io/frontend":    "alb-dev-00080",
		"alb2.cpaas.io/name":        "alb-dev",
		"alb2.cpaas.io/source-type": "ingress",
	}

	rulespec := albv1.RuleSpec{
		Source: &albv1.Source{
			Name: "ing-1",
			Type: "ingress",
		},
	}
	testCases := []testCase{
		{
			ing:  ing,
			kind: "http",
			exist: []*albv1.Rule{
				{
					ObjectMeta: k8smetav1.ObjectMeta{
						Annotations: map[string]string{
							"alb2.cpaas.io/source-ingress-path-index": "0",
							"alb2.cpaas.io/source-ingress-rule-index": "0",
							"alb2.cpaas.io/source-ingress-version":    "1",
						},
						Name:   "rule-1",
						Labels: labels,
					},
					Spec: rulespec,
				},
			},
			expect: []*albv1.Rule{
				{
					ObjectMeta: k8smetav1.ObjectMeta{
						Annotations: map[string]string{
							"alb2.cpaas.io/source-ingress-path-index": "0",
							"alb2.cpaas.io/source-ingress-rule-index": "0",
							"alb2.cpaas.io/source-ingress-version":    "2",
						},
						Labels: labels,
						Name:   "rule-1-x",
					},
					Spec: rulespec,
				},
			},
			cmd: SyncRule{
				Create: []*albv1.Rule{},
				Delete: []*albv1.Rule{},
				Update: []*albv1.Rule{
					{
						ObjectMeta: k8smetav1.ObjectMeta{
							Annotations: map[string]string{
								"alb2.cpaas.io/source-ingress-path-index": "0",
								"alb2.cpaas.io/source-ingress-rule-index": "0",
								"alb2.cpaas.io/source-ingress-version":    "2",
							},
							Labels: labels,
							Name:   "rule-1",
						},
						Spec: rulespec,
					},
				},
			},
		},
	}
	for _, tcase := range testCases {
		_ = tcase
		c := Controller{
			log:    log.L(),
			Config: config.DefaultMock(),
		}
		act, err := c.genSyncRuleAction(tcase.kind, &tcase.ing, tcase.exist, tcase.expect, c.log)
		assert.NoError(t, err)
		t.Logf("cur %v", utils.PrettyJson(act))
		t.Logf("expect %v", utils.PrettyJson(tcase.cmd))
		t.Logf("diff %v", cmp.Diff(utils.PrettyJson(tcase.cmd), utils.PrettyJson(act)))
		assert.Equal(t, utils.PrettyJson(tcase.cmd), utils.PrettyJson(act))
	}
}
