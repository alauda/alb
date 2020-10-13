package driver

import (
	v1 "alauda.io/alb2/pkg/client/listers/alauda/v1"
	"errors"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	v1types "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"net"
	"sort"

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
	ServiceLister  corev1lister.ServiceLister
	EndpointLister corev1lister.EndpointsLister
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

func (kd *KubernetesDriver) FillUpListers(serviceLister corev1lister.ServiceLister, endpointLister corev1lister.EndpointsLister,
	alb2Lister v1.ALB2Lister, frontendLister v1.FrontendLister, ruleLister v1.RuleLister) {
	kd.ServiceLister = serviceLister
	kd.EndpointLister = endpointLister
	kd.ALB2Lister = alb2Lister
	kd.FrontendLister = frontendLister
	kd.RuleLister = ruleLister
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

// GetClusterIPAddress return addresses of a clusterip service by using cluster ip and service port
func (kd *KubernetesDriver) GetClusterIPAddress(svc *v1types.Service, port int) (*Service, error) {
	service := &Service{
		ServiceID:     fmt.Sprintf("%s.%s", svc.Name, svc.Namespace),
		ServiceName:   svc.Name,
		ContainerPort: port,
		Namespace:     svc.Namespace,
		Backends: []*Backend{
			{
				IP:   svc.Spec.ClusterIP,
				Port: port,
			},
		},
	}
	return service, nil
}

// GetNodePortAddr return addresses of a NodePort service by using host ip and node port
// TODO: remove or use lister
func (kd *KubernetesDriver) GetNodePortAddress(svc *v1types.Service, port int) (*Service, error) {
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

	podList, err := kd.Client.CoreV1().Pods(svc.Namespace).List(metav1.ListOptions{
		LabelSelector: labels.Set(svc.Spec.Selector).String(),
	})
	if err != nil {
		klog.Errorf("Get pods of service %s.%s failed: %s", svc.Name, svc.Namespace, err.Error())
		return service, nil //return a service with empty backend list
	}
	nodeSet := make(map[string]bool)
	for _, pod := range podList.Items {
		if !IsPodReady(&pod) {
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
	klog.V(3).Infof("backends of svc %s: %+v", svc.Name, service.Backends)
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

// GetExternalNameAddress return a fqdn address for service
func (kd *KubernetesDriver) GetExternalNameAddress(svc *v1types.Service, port int) (*Service, error) {
	service := &Service{
		ServiceID:     fmt.Sprintf("%s.%s", svc.Name, svc.Namespace),
		ServiceName:   svc.Name,
		ContainerPort: port,
		Namespace:     svc.Namespace,
		Backends:      make([]*Backend, 0),
	}
	if net.ParseIP(svc.Spec.ExternalName) != nil {
		service.Backends = append(service.Backends, &Backend{
			IP:   svc.Spec.ExternalName,
			Port: port,
		})
	} else {
		// for simplify we do dns resolve in golang
		hosts, err := net.LookupHost(svc.Spec.ExternalName)
		if err != nil {
			return nil, err
		}
		for _, host := range hosts {
			service.Backends = append(service.Backends, &Backend{
				IP:   host,
				Port: port,
			})
		}
	}
	sort.Sort(ByBackend(service.Backends))
	klog.V(3).Infof("backends of svc %s: %+v", svc.Name, service.Backends)
	return service, nil
}

// GetServiceAddress return ip list of a service base on the type of service
func (kd *KubernetesDriver) GetServiceAddress(name, namespace string, servicePort int) (*Service, error) {
	svc, err := kd.ServiceLister.Services(namespace).Get(name)
	if err != nil || svc == nil {
		klog.Errorf("Get service %s.%s failed: %s", name, namespace, err)
		return nil, err
	}
	switch svc.Spec.Type {
	case v1types.ServiceTypeClusterIP:
		if svc.Spec.ClusterIP == "None" {
			return kd.GetEndPointAddress(name, namespace, servicePort)
		}
		if config.GetBool("USE_ENDPOINT") {
			return kd.GetEndPointAddress(name, namespace, servicePort)
		}
		return kd.GetClusterIPAddress(svc, servicePort)
	case v1types.ServiceTypeNodePort:
		if config.GetBool("USE_ENDPOINT") {
			return kd.GetEndPointAddress(name, namespace, servicePort)
		}
		return kd.GetNodePortAddress(svc, servicePort)
	case v1types.ServiceTypeExternalName:
		return kd.GetExternalNameAddress(svc, servicePort)
	default:
		// ServiceTypeLoadBalancer
		klog.Errorf("Unsupported type %s of service %s.%s.", svc.Spec.Type, name, namespace)
		return nil, errors.New("Unknown Service Type")
	}
}

func (kd *KubernetesDriver) GetServiceByName(namespace, name string, servicePort int) (*Service, error) {
	return kd.GetServiceAddress(name, namespace, servicePort)
}

// IsPodReady returns true if a pod is ready; false otherwise.
func IsPodReady(pod *corev1.Pod) bool {
	return isPodReadyConditionTrue(pod.Status)
}

// IsPodReadyConditionTrue returns true if a pod is ready; false otherwise.
func isPodReadyConditionTrue(status corev1.PodStatus) bool {
	condition := getPodReadyCondition(status)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

// GetPodReadyCondition extracts the pod ready condition from the given status and returns that.
// Returns nil if the condition is not present.
func getPodReadyCondition(status corev1.PodStatus) *corev1.PodCondition {
	_, condition := getPodCondition(&status, corev1.PodReady)
	return condition
}

// GetPodCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func getPodCondition(status *corev1.PodStatus, conditionType corev1.PodConditionType) (int, *corev1.PodCondition) {
	if status == nil {
		return -1, nil
	}
	return getPodConditionFromList(status.Conditions, conditionType)
}

// GetPodConditionFromList extracts the provided condition from the given list of condition and
// returns the index of the condition and the condition. Returns -1 and nil if the condition is not present.
func getPodConditionFromList(conditions []corev1.PodCondition, conditionType corev1.PodConditionType) (int, *corev1.PodCondition) {
	if conditions == nil {
		return -1, nil
	}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return i, &conditions[i]
		}
	}
	return -1, nil
}
