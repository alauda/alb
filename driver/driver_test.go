package driver

import (
	"context"
	"flag"
	"os"
	"testing"

	"alauda.io/alb2/config"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albFake "alauda.io/alb2/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog"
)

func TestMain(m *testing.M) {
	klog.InitFlags(nil)
	flag.Set("logtostderr", "true")
	flag.Parse()
	code := m.Run()
	os.Exit(code)
}

func TestLoadALByName(t *testing.T) {

	type FakeALBResource struct {
		albs      []albv1.ALB2
		frontends []albv1.Frontend
		rules     []albv1.Rule
	}

	defaultNs := "ns-1"
	testCase := FakeALBResource{
		albs: []albv1.ALB2{
			{
				ObjectMeta: k8smetav1.ObjectMeta{
					Namespace: defaultNs,
					Name:      "alb-1",
				},
			},
		},
		frontends: []albv1.Frontend{
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
		rules: []albv1.Rule{
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
	}

	defaultCfg := map[string]string{
		"DOMAIN":          "alauda.io",
		"TWEAK_DIRECTORY": "../driver/texture", // set TWEAK_DIRECTORY to a exist path: make calculate hash happy
		"NAME":            "alb-1",
		"NAMESPACE":       defaultNs,
		"bindkey":         "loadbalancer.%s/bind",
		"name":            "alb2.%s/name",
		"frontend":        "alb2.%s/frontend",
		"lock":            "alb2.%s/locker",
		"source_type":     "alb2.%s/source-type",
		"source_name":     "alb2.%s/source-name",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for key, val := range defaultCfg {
		config.Set(key, val)
	}

	driver, err := GetKubernetesDriver(true)

	a := assert.New(t)
	a.NoError(err)
	albs := []albv1.ALB2{
		{
			ObjectMeta: k8smetav1.ObjectMeta{
				Namespace: defaultNs,
				Name:      "alb-1",
			},
		},
	}
	albDataset := []runtime.Object{
		&albv1.ALB2List{Items: albs},
		&albv1.FrontendList{Items: testCase.frontends},
		&albv1.RuleList{Items: testCase.rules},
	}
	k8sDataset := []runtime.Object{}
	driver.ALBClient = albFake.NewSimpleClientset(albDataset...)
	driver.Client = fake.NewSimpleClientset(k8sDataset...)

	informers, err := InitInformers(driver, ctx, InitInformersOptions{ErrorIfWaitSyncFail: false})
	a.NoError(err)
	driver.FillUpListers(
		informers.K8s.Service.Lister(),
		informers.K8s.Endpoint.Lister(),
		informers.Alb.Alb.Lister(),
		informers.Alb.Ft.Lister(),
		informers.Alb.Rule.Lister())

	alb, err := driver.LoadALBbyName(defaultNs, "alb-1")
	a.NoError(err)
	a.Equal(alb.Name, "alb-1")
	a.Equal(alb.Namespace, defaultNs)
	a.Equal(len(alb.Frontends), 1)
	a.Equal(alb.Frontends[0].Name, "ft-1")
}
