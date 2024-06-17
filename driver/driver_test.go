package driver

import (
	"context"
	"testing"

	"alauda.io/alb2/config"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/utils/test_utils"
	"github.com/stretchr/testify/assert"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
)

func TestLoadALByName(t *testing.T) {
	defaultNs := "ns-1"
	testCase := test_utils.FakeResource{
		K8s: test_utils.FakeK8sResource{
			Namespaces: []corev1.Namespace{
				{
					ObjectMeta: k8smetav1.ObjectMeta{
						Name: defaultNs,
					},
				},
			},
		},
		Alb: test_utils.FakeALBResource{
			Albs: []albv2.ALB2{
				{
					ObjectMeta: k8smetav1.ObjectMeta{
						Namespace: defaultNs,
						Name:      "alb-1",
					},
					Spec: albv2.ALB2Spec{
						Type: "nginx",
					},
				},
			},
			Frontends: []albv1.Frontend{
				{
					ObjectMeta: k8smetav1.ObjectMeta{
						Name:      "ft-1",
						Namespace: defaultNs,
						Labels: map[string]string{
							"alb2.alauda.io/name": "alb-1",
						},
					},
					Spec: albv1.FrontendSpec{
						Port:     12345,
						Protocol: "http",
						Source: &albv1.Source{
							Name:      "ft-source-1",
							Namespace: defaultNs,
							Type:      "bind",
						},
						ServiceGroup: &albv1.ServiceGroup{
							Services: []albv1.Service{
								{
									Name:      "ft-service-1",
									Namespace: defaultNs,
								},
							},
						},
					},
				},
			},
			Rules: []albv1.Rule{
				{
					ObjectMeta: k8smetav1.ObjectMeta{
						Name:      "rule-1",
						Namespace: defaultNs,
						Labels: map[string]string{
							"alb2.alauda.io/name":     "alb-1",
							"alb2.alauda.io/frontend": "ft-1",
							"app.alauda.io/name":      "appname.nsname",
						},
					},
					Spec: albv1.RuleSpec{
						Source: &albv1.Source{
							Name:      "rule-source-1",
							Namespace: defaultNs,
							Type:      "bind",
						},
					},
				},
			},
		},
	}
	cfg := config.DefaultMock()
	cfg.SetDomain("alauda.io")
	cfg.Name = "alb-1"
	cfg.Ns = defaultNs
	config.UseMock(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	a := assert.New(t)
	defer cancel()
	env := test_utils.NewFakeEnv()
	env.AssertStart()
	err := env.ApplyFakes(testCase)
	a.NoError(err)
	driver, err := GetAndInitKubernetesDriverFromCfg(ctx, env.GetCfg())
	a.NoError(err)

	alb, err := driver.LoadALBbyName(defaultNs, "alb-1")
	a.NoError(err)
	a.Equal(alb.Name, "alb-1")
	a.Equal(alb.Namespace, defaultNs)
	a.Equal(len(alb.Frontends), 1)
	a.Equal(alb.Frontends[0].Name, "ft-1")
	env.Stop()
}
