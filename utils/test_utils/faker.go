// utils only used in test,this package could not be used in dirver package
package test_utils

import (
	"context"
	"testing"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	albFake "alauda.io/alb2/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	k8sv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

const DEFAULT_NS = "ns-1"
const DEFAULT_ALB = "alb-1"

var DEFAULT_CONFIG_FOR_TEST = map[string]string{
	"DOMAIN":               "alauda.io",
	"TWEAK_DIRECTORY":      "../driver/texture", // set TWEAK_DIRECTORY to an existing path: make calculate hash happy
	"NAME":                 DEFAULT_ALB,
	"NAMESPACE":            DEFAULT_NS,
	"bindkey":              "loadbalancer.%s/bind",
	"labels.name":          "alb2.%s/name",
	"labels.frontend":      "alb2.%s/frontend",
	"labels.source_type":   "alb2.%s/source-type",
	"DEFAULT-SSL-STRATEGY": "Request",
}

type FakeResource struct {
	Alb FakeALBResource
	K8s FakeK8sResource
}

type FakeALBResource struct {
	Albs      []albv2.ALB2
	Frontends []albv1.Frontend
	Rules     []albv1.Rule
}

type FakeK8sResource struct {
	Namespaces     []k8sv1.Namespace
	Services       []k8sv1.Service
	EndPoints      []k8sv1.Endpoints
	Ingresses      []networkingv1.Ingress
	IngressesClass []networkingv1.IngressClass
	Secrets        []k8sv1.Secret
}

func InitFakeAlb(t *testing.T, ctx context.Context, fakeResource FakeResource, configMap map[string]string) *driver.KubernetesDriver {

	for key, val := range configMap {
		config.Set(key, val)
	}

	drv, err := driver.GetKubernetesDriver(ctx, true)

	a := assert.New(t)
	a.NoError(err)

	albDataset := []runtime.Object{
		&albv2.ALB2List{Items: fakeResource.Alb.Albs},
		&albv1.FrontendList{Items: fakeResource.Alb.Frontends},
		&albv1.RuleList{Items: fakeResource.Alb.Rules},
	}

	k8sDataset := []runtime.Object{
		&k8sv1.NamespaceList{Items: fakeResource.K8s.Namespaces},
		&k8sv1.ServiceList{Items: fakeResource.K8s.Services},
		&k8sv1.EndpointsList{Items: fakeResource.K8s.EndPoints},
		&networkingv1.IngressList{Items: fakeResource.K8s.Ingresses},
		&networkingv1.IngressClassList{Items: fakeResource.K8s.IngressesClass},
		&k8sv1.SecretList{Items: fakeResource.K8s.Secrets},
	}
	drv.ALBClient = albFake.NewSimpleClientset(albDataset...)
	drv.Client = fake.NewSimpleClientset(k8sDataset...)
	driver.InitDriver(drv, ctx)
	return drv
}
