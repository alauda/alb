package framework

import (
	"context"

	"github.com/onsi/ginkgo"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	albclient "alauda.io/alb2/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	gatewayVersioned "sigs.k8s.io/gateway-api/pkg/client/clientset/gateway/versioned"

	gt "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type K8sClient struct {
	k8sClient     kubernetes.Interface
	albClient     albclient.Interface
	gatewayClient gatewayVersioned.Interface
	ctx           context.Context
}

func NewK8sClient(cfg *rest.Config) *K8sClient {
	k8sClient := kubernetes.NewForConfigOrDie(cfg)
	albClient := albclient.NewForConfigOrDie(cfg)
	gatewayClient := gatewayVersioned.NewForConfigOrDie(cfg)
	return &K8sClient{
		k8sClient:     k8sClient,
		albClient:     albClient,
		gatewayClient: gatewayClient,
		ctx:           context.Background(),
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
			assert.True(ginkgo.GinkgoT(), find, "could not find %v", name)
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

type SvcOptPort struct {
	port        int
	Protocol    string
	AppProtocol *string
}

type SvcOpt struct {
	Ns    string
	Name  string
	Ep    []string
	Ports []corev1.ServicePort
}

func (f *K8sClient) initSvcWithOpt(opt SvcOpt) error {
	ns := opt.Ns
	name := opt.Name
	ep := opt.Ep

	Logf("init svc %+v", opt)
	service_spec := corev1.ServiceSpec{
		Ports: opt.Ports,
	}
	epPorts := lo.Map(opt.Ports, func(p corev1.ServicePort, _ int) corev1.EndpointPort {
		return corev1.EndpointPort{
			Name:        p.Name,
			Protocol:    p.Protocol,
			Port:        p.Port,
			AppProtocol: p.AppProtocol,
		}
	})
	_, err := f.k8sClient.CoreV1().Services(ns).Create(f.ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: service_spec,
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	address := []corev1.EndpointAddress{}
	for _, ip := range ep {
		address = append(address, corev1.EndpointAddress{IP: ip})
	}
	_, err = f.k8sClient.CoreV1().Endpoints(ns).Create(f.ctx, &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			Labels:    map[string]string{"kube-app": name},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: address,
				Ports:     epPorts,
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (f *K8sClient) InitSvcWithOpt(opt SvcOpt) {
	err := f.initSvcWithOpt(opt)
	assert.NoError(ginkgo.GinkgoT(), err)
}

func (f *K8sClient) InitSvc(ns, name string, ep []string) {
	opt := SvcOpt{
		Ns:   ns,
		Name: name,
		Ep:   ep,
		Ports: []corev1.ServicePort{
			{
				Port: 80,
			},
		},
	}
	f.initSvcWithOpt(opt)
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
	_, err := f.gatewayClient.GatewayV1alpha2().GatewayClasses().Create(f.ctx, &gt.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gt.GatewayClassSpec{
			ControllerName: "alb.gateway.operator/test",
		},
	}, metav1.CreateOptions{})
	return err
}
