package test_utils

import (
	"context"

	"github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	albclient "alauda.io/alb2/pkg/client/clientset/versioned"
	cliu "alauda.io/alb2/utils/client"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayVersioned "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// TODO 直接用controller-runtime的client
type K8sClient struct {
	k8sClient       kubernetes.Interface
	albClient       albclient.Interface
	gatewayClient   gatewayVersioned.Interface
	ctlClient       client.Client
	ctlDirectClient client.Client
	ctx             context.Context
}

func NewK8sClient(ctx context.Context, cfg *rest.Config) *K8sClient {
	k8sClient := kubernetes.NewForConfigOrDie(cfg)
	albClient := albclient.NewForConfigOrDie(cfg)
	gatewayClient := gatewayVersioned.NewForConfigOrDie(cfg)
	scheme := cliu.InitScheme(runtime.NewScheme())
	cli, err := cliu.GetClient(ctx, cfg, scheme)
	if err != nil {
		panic(err)
	}
	dcli, err := cliu.GetDirectlyClient(ctx, cfg, scheme)
	if err != nil {
		panic(err)
	}
	return &K8sClient{
		k8sClient:       k8sClient,
		albClient:       albClient,
		gatewayClient:   gatewayClient,
		ctx:             ctx,
		ctlClient:       cli,
		ctlDirectClient: dcli,
	}
}

func (f *K8sClient) GetK8sClient() kubernetes.Interface {
	return f.k8sClient
}

func (f *K8sClient) GetAlbClient() albclient.Interface {
	return f.albClient
}

func (f *K8sClient) GetGatewayClient() gatewayVersioned.Interface {
	return f.gatewayClient
}

func (f *K8sClient) GetClient() client.Client {
	return f.ctlClient
}

func (f *K8sClient) GetDirectClient() client.Client {
	return f.ctlDirectClient
}

func (f *K8sClient) SetSvcLBIp(ns, name, ip string) (*corev1.Service, error) {
	svc, err := f.k8sClient.CoreV1().Services(ns).Get(f.ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
		{
			IP: ip,
		},
	}
	svc, err = f.k8sClient.CoreV1().Services(ns).UpdateStatus(f.ctx, svc, metav1.UpdateOptions{})
	return svc, err
}

type ServiceAssertPort struct {
	Port     int
	Protocol string
	NodePort *int
}
type ServiceAssertCfg struct {
	Ports map[string]ServiceAssertPort
}

func (f *K8sClient) AssertService(ns string, name string, sassert *ServiceAssertCfg, exts ...func(*corev1.Service) (bool, error)) {
	svc, err := f.GetK8sClient().CoreV1().Services(ns).Get(f.ctx, name, metav1.GetOptions{})
	assert.NoError(ginkgo.GinkgoT(), err, "get svc fail")
	if sassert != nil {
		svcPortMap := map[string]corev1.ServicePort{}
		for _, p := range svc.Spec.Ports {
			svcPortMap[p.Name] = p
		}
		for name, p := range sassert.Ports {
			svcp, find := svcPortMap[name]
			assert.True(ginkgo.GinkgoT(), find, "could not find port %v", name)
			assert.Equal(ginkgo.GinkgoT(), svcp.Port, int32(p.Port), "")
			assert.Equal(ginkgo.GinkgoT(), string(svcp.Protocol), p.Protocol, "")
			if p.NodePort != nil {
				assert.Equal(ginkgo.GinkgoT(), svcp.NodePort, int32(*p.NodePort), "")
			}
		}
	}
	for i, as := range exts {
		ret, err := as(svc)
		assert.NoError(ginkgo.GinkgoT(), err, "%v fail", i)
		assert.True(ginkgo.GinkgoT(), ret, "%v fail", i)
	}
}

func (f *K8sClient) GetDeploymentEnv(ns, name, container string) (map[string]string, error) {
	dep, err := f.k8sClient.AppsV1().Deployments(ns).Get(f.ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	ret := map[string]string{}
	for _, c := range dep.Spec.Template.Spec.Containers {
		if container != c.Name {
			continue
		}
		for _, e := range c.Env {
			ret[e.Name] = e.Value
		}
	}
	return ret, nil
}

func (f *K8sClient) CreateGatewayClass(name string) error {
	_, err := f.gatewayClient.GatewayV1().GatewayClasses().Create(f.ctx, &gv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gv1.GatewayClassSpec{
			ControllerName: "alb.gateway.operator/test",
		},
	}, metav1.CreateOptions{})
	return err
}

func (f *K8sClient) CreateNsIfNotExist(name string) error {
	_, err := f.k8sClient.CoreV1().Namespaces().Get(f.ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = f.k8sClient.CoreV1().Namespaces().Create(f.ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	return nil
}
