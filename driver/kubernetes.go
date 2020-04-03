package driver

import (
	v1 "alauda.io/alb2/pkg/client/listers/alauda/v1"
	"errors"
	"fmt"
	corev1 "k8s.io/client-go/listers/core/v1"
	"sort"
	"strings"

	v1types "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/klog"

	"alauda.io/alb2/config"
	albclient "alauda.io/alb2/pkg/client/clientset/versioned"
	albfakeclient "alauda.io/alb2/pkg/client/clientset/versioned/fake"
)

type KubernetesDriver struct {
	Client         kubernetes.Interface
	ALBClient      albclient.Interface
	ALB2Lister     v1.ALB2Lister
	FrontendLister v1.FrontendLister
	RuleLister     v1.RuleLister
	ServiceLister  corev1.ServiceLister
	EndpointLister corev1.EndpointsLister
}

func GetKubernetesDriver(isFake bool) (*KubernetesDriver, error) {
	var client kubernetes.Interface
	var albClient albclient.Interface
	if isFake {
		// placeholder will reset in test
		client = fake.NewSimpleClientset()
		albClient = albfakeclient.NewSimpleClientset()
	} else {
		var cf *rest.Config
		var err error
		cf, err = rest.InClusterConfig()
		if err != nil {
			if config.Get("KUBERNETES_SERVER") != "" && config.Get("KUBERNETES_BEARERTOKEN") != "" {
				// maybe run by docker directly, such as migrate
				tlsClientConfig := rest.TLSClientConfig{Insecure: true}
				cf = &rest.Config{
					Host:            config.Get("KUBERNETES_SERVER"),
					BearerToken:     config.Get("KUBERNETES_BEARERTOKEN"),
					TLSClientConfig: tlsClientConfig,
				}
			} else {
				return nil, err
			}
		}
		client, err = kubernetes.NewForConfig(cf)
		if err != nil {
			return nil, err
		}
		albClient, err = albclient.NewForConfig(cf)
		if err != nil {
			return nil, err
		}

	}
	return &KubernetesDriver{Client: client, ALBClient: albClient}, nil
}

func (kd *KubernetesDriver) GetType() string {
	return config.Kubernetes
}

func (kd *KubernetesDriver) FillUpListers(serviceLister corev1.ServiceLister, endpointLister corev1.EndpointsLister,
	alb2Lister v1.ALB2Lister, frontendLister v1.FrontendLister, ruleLister v1.RuleLister) {
	kd.ServiceLister = serviceLister
	kd.EndpointLister = endpointLister
	kd.ALB2Lister = alb2Lister
	kd.FrontendLister = frontendLister
	kd.RuleLister = ruleLister
}

func selectorToLabelSelector(selector map[string]string) string {
	if len(selector) == 0 {
		return ""
	}

	labels := make([]string, 0, len(selector))
	for label, value := range selector {
		if value != "" {
			labels = append(labels, fmt.Sprintf("%s=%s", label, value))
		} else {
			labels = append(labels, label)
		}
	}
	return strings.Join(labels, ",")
}

func (kd *KubernetesDriver) ListService() ([]*Service, error) {
	alb, err := kd.LoadALBbyName(config.Get("NAMESPACE"), config.Get("NAME"))
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	services, err := kd.LoadServices(alb)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return services, nil
}

// GetNodePortAddr return addresses of a NodePort service by using host ip and nodeport.
// TODO: remove or use lister
func (kd *KubernetesDriver) GetNodePortAddr(svc *v1types.Service, port int) (*Service, error) {
	service := &Service{
		ServiceID:     fmt.Sprintf("%s.%s", svc.Name, svc.Namespace),
		ServiceName:   svc.Name,
		ContainerPort: port,
		Namespace:     svc.Namespace,
		Backends:      make([]*Backend, 0),
	}
	var nodeport int32
	for _, p := range svc.Spec.Ports {
		if p.Port == int32(port) {
			nodeport = p.NodePort
			break
		}
	}
	if nodeport == 0 {
		klog.Errorf("Service %s.%s NOT have port %d", svc.Name, svc.Namespace, port)
		return nil, errors.New("Port NOT Found")
	}

	podList, err := kd.Client.CoreV1().Pods("").List(metav1.ListOptions{
		LabelSelector: selectorToLabelSelector(svc.Spec.Selector),
	})
	if err != nil {
		klog.Errorf("Get pods of service %s.%s failed: %s", svc.Name, svc.Namespace, err.Error())
		return service, nil //return a service with empty backend list
	}
	nodeSet := make(map[string]bool)
	for _, pod := range podList.Items {
		if pod.Status.HostIP == "" || pod.Status.Phase != v1types.PodRunning {
			// pod not ready
			continue
		}
		if _, ok := nodeSet[pod.Status.HostIP]; ok {
			// host has already been added
			continue
		}
		service.Backends = append(
			service.Backends,
			&Backend{
				IP:   pod.Status.HostIP,
				Port: int(nodeport),
			},
		)
		nodeSet[pod.Status.HostIP] = true
	}
	sort.Sort(ByBackend(service.Backends))
	return service, nil
}

// GetEndPointAddress return a list of pod ip in the endpoint
func (kd *KubernetesDriver) GetEndPointAddress(name, namespace string, servicePort int) (*Service, error) {
	svc, err := kd.ServiceLister.Services(namespace).Get(name)
	if err != nil {
		klog.Errorf("Failed to get svc %s.%s, error is %s.", name, namespace, err.Error())
		return nil, err
	}
	var port int
	for _, svcPort := range svc.Spec.Ports {
		if servicePort == int(svcPort.Port) {
			p := svcPort.TargetPort.IntValue()
			if p == 0 {
				p = servicePort
			}
			// set service port to target port
			port = p
			break
		}
	}

	ep, err := kd.EndpointLister.Endpoints(namespace).Get(name)
	if err != nil {
		klog.Errorf("Failed to get ep %s.%s, error is %s.", name, namespace, err.Error())
		return nil, err
	}

	service := &Service{
		ServiceID:     fmt.Sprintf("%s.%s", name, namespace),
		ServiceName:   name,
		ServicePort:   servicePort,
		ContainerPort: port,
		Namespace:     namespace,
		Backends:      make([]*Backend, 0),
	}

	for _, subset := range ep.Subsets {
		for _, addr := range subset.Addresses {
			service.Backends = append(service.Backends, &Backend{
				InstanceID: addr.Hostname,
				IP:         addr.IP,
				Port:       port,
			})
		}
	}
	sort.Sort(ByBackend(service.Backends))
	klog.V(3).Infof("backends of svc %s: %+v", name, service.Backends)
	if len(service.Backends) == 0 {
		klog.Warningf("service %s has 0 backends, means has no health pods", name)
	}
	return service, nil
}

// GetServiceAddress return ip list of a service base on the type of service
func (kd *KubernetesDriver) GetServiceAddress(name, namespace string, port int) (*Service, error) {
	svc, err := kd.ServiceLister.Services(namespace).Get(name)
	if err != nil || svc == nil {
		klog.Errorf("Get service %s.%s failed: %s", name, namespace, err)
		return nil, err
	}
	switch svc.Spec.Type {
	case v1types.ServiceTypeClusterIP:
		service := &Service{
			ServiceID:     fmt.Sprintf("%s.%s", name, namespace),
			ServiceName:   name,
			ContainerPort: port,
			Namespace:     namespace,
			Backends: []*Backend{
				&Backend{
					IP:   svc.Spec.ClusterIP,
					Port: port,
				},
			},
		}
		return service, nil
	case v1types.ServiceTypeNodePort:
		return kd.GetNodePortAddr(svc, port)
	case "None": //headless service
		return kd.GetEndPointAddress(name, namespace, port)
	default:
		klog.Errorf("Unsupported type %s of service %s.%s.", svc.Spec.Type, name, namespace)
		return nil, errors.New("Unknown Service Type")
	}
}

func (kd *KubernetesDriver) GetServiceByName(namespace, name string, port int) (*Service, error) {
	if config.GetBool("USE_ENDPOINT") {
		return kd.GetEndPointAddress(name, namespace, port)
	}
	return kd.GetServiceAddress(name, namespace, port)
}
