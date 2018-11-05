package driver

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	v1types "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"alb2/config"
)

type KubernetesDriver struct {
	Endpoint    string
	BearerToken string
	Timeout     int
	Client      kubernetes.Interface
}

var FAKE_ENDPOINT = "fake-endpoint"

func GetKubernetesDriver(endpoint string, bearerToken string, timeout int) (*KubernetesDriver, error) {
	cf := &rest.Config{
		Host:        endpoint,
		BearerToken: bearerToken,
		Timeout:     time.Duration(timeout) * time.Second,
	}
	cf.ContentType = "application/vnd.kubernetes.protobuf"
	cf.Insecure = true
	if cf.APIPath == "" {
		cf.APIPath = "/api"
	}
	if cf.GroupVersion == nil {
		cf.GroupVersion = &schema.GroupVersion{}
	}

	var client kubernetes.Interface
	var err error
	if endpoint != FAKE_ENDPOINT {
		client, err = kubernetes.NewForConfig(cf)
	} else {
		client = fake.NewSimpleClientset()
	}

	if err == nil {
		return &KubernetesDriver{
			Endpoint:    endpoint,
			BearerToken: bearerToken,
			Timeout:     timeout,
			Client:      client}, nil
	}
	return nil, err
}

func (kd *KubernetesDriver) GetType() string {
	return config.Kubernetes
}

func (kd *KubernetesDriver) IsHealthy() bool {
	_, err := kd.Client.CoreV1().Nodes().List(metav1.ListOptions{})
	return err == nil
}

func nodePortToService(namespace string, np *NodePort) *v1types.Service {
	ports := make([]v1types.ServicePort, 0, len(np.Ports))
	for _, port := range np.Ports {
		ports = append(ports, v1types.ServicePort{Port: int32(port), Name: "port-" + strconv.Itoa(port)})
	}
	service := &v1types.Service{
		TypeMeta:   metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: np.Name, Labels: np.Labels, Namespace: namespace},
		Spec: v1types.ServiceSpec{
			Type:     v1types.ServiceTypeNodePort,
			Ports:    ports,
			Selector: np.Selector,
		},
	}
	return service
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

func (kd *KubernetesDriver) CreateNodePort(np *NodePort) error {
	ls := selectorToLabelSelector(np.Selector)
	if ls == "" {
		return fmt.Errorf("no selector")
	}
	podList, err := kd.Client.CoreV1().Pods("").List(metav1.ListOptions{
		LabelSelector: ls,
	})
	if len(podList.Items) == 0 {
		return fmt.Errorf("no pod found for nodePoint %v", np)
	}
	namespace := podList.Items[0].Namespace
	_, err = kd.Client.CoreV1().Services(namespace).Create(nodePortToService(namespace, np))
	if err != nil {
		glog.Errorf("Failed to create nodeport for kubernetes, error: %s", err.Error())
	} else {
		glog.Infof("create nodeport %v", *np)
	}
	return err
}

func (kd *KubernetesDriver) DeleteNodePort(name string, namespace string) error {
	err := kd.Client.CoreV1().Services(namespace).Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		glog.Errorf("Failed to delete nodeport %s %s for kubernetes, error: %s", name, namespace, err)
		return err
	}
	glog.Infof("delete nodeport %s %s", namespace, name)
	return nil
}

//ListServiceEndpoints List service's backend address by using Pod ip
func (kd *KubernetesDriver) ListServiceEndpoints() ([]*Service, error) {
	endpoints, err := kd.Client.CoreV1().Endpoints("").List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s", config.Get("LABEL_SERVICE_ID")),
	})
	if err != nil {
		glog.Errorf("Failed to get pods, error is %s.", err)
		return nil, err
	}
	servicesMap := make(map[string]*Service)
	for _, ep := range endpoints.Items {
		serviceID, ok := ep.Labels[config.Get("LABEL_SERVICE_ID")]
		if !ok || serviceID == "" {
			continue
		}
		service, ok := servicesMap[serviceID]
		if ok {
			continue
		}
		owner, ok := ep.Labels[config.Get("LABEL_CREATOR")]
		if !ok {
			owner = OwnerUnknown
		}
		service = &Service{
			ServiceID:     serviceID,
			ServiceName:   ep.Name,
			ContainerPort: 0,
			Namespace:     ep.Namespace,
			Backends:      make([]*Backend, 0),
			Owner:         owner,
		}
		servicesMap[serviceID] = service
		for _, subset := range ep.Subsets {
			for _, addr := range subset.Addresses {
				service.Backends = append(service.Backends, &Backend{
					InstanceID: addr.Hostname,
					IP:         addr.IP,
					Port:       0,
				})
			}
		}
		sort.Sort(ByBackend(service.Backends))
	}
	services := make([]*Service, 0, len(servicesMap))
	for _, service := range servicesMap {
		services = append(services, service)
	}
	return services, nil
}

func (kd *KubernetesDriver) ListService() ([]*Service, error) {
	alb, err := LoadALBbyName(config.Get("NAMESPACE"), config.Get("NAME"))
	if err != nil {
		glog.Error(err)
		return nil, err
	}
	services, err := LoadServices(alb)
	if err != nil {
		glog.Error(err)
		return nil, err
	}

	return services, nil
}

func (kd *KubernetesDriver) fetchKubernetesBackends() ([]*Backend, error) {
	backends := []*Backend{}
	nodeList, err := kd.Client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		glog.Errorf("Failed to get nodes list for kubernetes, error: %s", err)
		return nil, err
	}
nodeLoop:
	for _, node := range nodeList.Items {
		for _, condition := range node.Status.Conditions {
			if condition.Type == v1types.NodeReady &&
				condition.Status != v1types.ConditionTrue {
				glog.Infof("node %s is not ready", node.Name)
				continue nodeLoop
			}
		}
		internalIP := ""
		for _, ip := range node.Status.Addresses {
			if ip.Type == v1types.NodeInternalIP {
				internalIP = ip.Address
				break
			}
		}
		if internalIP == "" {
			glog.Errorf("Failed to get external ip for node %s", node.UID)
			continue
		}
		backends = append(backends, &Backend{InstanceID: string(node.UID), IP: internalIP})
	}
	sort.Sort(ByBackend(backends))
	return backends, nil
}

func (kd *KubernetesDriver) parseService(service *v1types.Service, backends []*Backend) ([]*Service, error) {
	serviceEndpoints := []*Service{}
	serviceID, ok := service.Labels[config.Get("LABEL_SERVICE_ID")]
	if !ok {
		return nil, fmt.Errorf("Service %s has no %s label", service.Name, config.Get("LABEL_SERVICE_ID"))
	}
	owner, ok := service.Labels[config.Get("LABEL_CREATOR")]
	if !ok {
		owner = OwnerUnknown
	}

	usePodHostIP := strings.EqualFold(config.Get("USE_POD_HOST_IP"), "true")
	var podList *v1types.PodList
	var err error
	if usePodHostIP {
		podList, err = kd.Client.CoreV1().Pods("").List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", config.Get("LABEL_SERVICE_ID"), serviceID),
		})
		if err != nil {
			glog.Error(err)
			return nil, err
		}
		if len(podList.Items) == 0 {
			return nil, fmt.Errorf("no pod found for service %s", serviceID)
		}
	}

	for _, port := range service.Spec.Ports {
		var serviceBackends []*Backend
		if usePodHostIP {
			nodeSet := make(map[string]bool)
			serviceBackends = make([]*Backend, 0, len(podList.Items))
			for _, pod := range podList.Items {
				if pod.Status.HostIP == "" || pod.Status.Phase != v1types.PodRunning {
					glog.Info("pod %s is not ready.", pod.Name)
					continue
				}
				if _, ok := nodeSet[pod.Status.HostIP]; ok {
					// host has already been added
					continue
				}
				serviceBackends = append(serviceBackends, &Backend{
					IP: pod.Status.HostIP, Port: int(port.NodePort)})
				nodeSet[pod.Status.HostIP] = true
			}
		} else {
			serviceBackends = make([]*Backend, 0, len(backends))
			for _, backend := range backends {
				serviceBackends = append(serviceBackends, &Backend{
					InstanceID: backend.InstanceID, IP: backend.IP, Port: int(port.NodePort)})
			}
		}

		serviceEndpoint := &Service{
			ServiceID:     serviceID,
			ServiceName:   service.Name,
			ContainerPort: int(port.Port),
			Backends:      serviceBackends,
			Namespace:     service.Namespace,
			Owner:         owner,
		}
		serviceEndpoints = append(serviceEndpoints, serviceEndpoint)
	}

	return serviceEndpoints, nil
}

// GetNodePortAddr return addresses of a NodePort service by using host ip and nodeport.
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
		glog.Error("Service %s.%s NOT have port %d", svc.Name, svc.Namespace, port)
		return nil, errors.New("Port NOT Found")
	}

	podList, err := kd.Client.CoreV1().Pods("").List(metav1.ListOptions{
		LabelSelector: selectorToLabelSelector(svc.Spec.Selector),
	})
	if err != nil {
		glog.Error("Get pods of service %s.%s failed: %s", svc.Name, svc.Namespace, err.Error())
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
func (kd *KubernetesDriver) GetEndPointAddress(name, namespace string, port int) (*Service, error) {
	svc, err := kd.Client.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("Failed to get svc %s.%s, error is %s.", name, namespace, err.Error())
		return nil, err
	}
	for _, svcPort := range svc.Spec.Ports {
		if port == int(svcPort.Port) {
			p := svcPort.TargetPort.IntValue()
			if p == 0 {
				p = port
			}
			// set service port to target port
			port = p
			break
		}
	}

	ep, err := kd.Client.CoreV1().Endpoints(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("Failed to get ep %s.%s, error is %s.", name, namespace, err.Error())
		return nil, err
	}

	service := &Service{
		ServiceID:     fmt.Sprintf("%s.%s", name, namespace),
		ServiceName:   name,
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
	glog.Infof("backends of svc %s: %+v", name, service.Backends)
	return service, nil
}

// GetServiceAddress return ip list of a service base on the type of service
func (kd *KubernetesDriver) GetServiceAddress(name, namespace string, port int) (*Service, error) {
	svc, err := kd.Client.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
	if err != nil || svc == nil {
		glog.Error("Get service %s.%s failed: %s", name, namespace, err)
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
		glog.Error("Unsupported type %s of service %s.%s.", svc.Spec.Type, name, namespace)
		return nil, errors.New("Unknown Service Type")
	}
}

func (kd *KubernetesDriver) GetServiceByName(namespace, name string, port int) (*Service, error) {
	if config.GetBool("USE_ENDPOINT") {
		return kd.GetEndPointAddress(name, namespace, port)
	}
	return kd.GetServiceAddress(name, namespace, port)
}

func (kd *KubernetesDriver) GetServiceByID(serviceID string, port int) (*Service, error) {
	svcs, err := kd.Client.CoreV1().Services("").List(
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", config.Get("LABEL_SERVICE_ID"), serviceID),
		},
	)
	if err != nil || svcs == nil {
		glog.Errorf("List service with id %s failed: %s", serviceID, err)
		return nil, err
	}
	var kubeSvc *v1types.Service

svcLoop:
	for _, svc := range svcs.Items {
		for _, p := range svc.Spec.Ports {
			if p.Port == int32(port) {
				kubeSvc = &svc
				if kubeSvc.Labels[config.Get("LABEL_CREATOR")] == "" {
					break svcLoop
				}
			}
		}
	}
	if kubeSvc != nil {
		return kd.GetServiceAddress(kubeSvc.Name, kubeSvc.Namespace, port)
	}
	glog.Errorf("No service with id %s and port %d found", serviceID, port)
	return nil, errors.New("No Service Found")
}
