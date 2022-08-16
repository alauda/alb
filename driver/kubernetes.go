package driver

import (
	"context"
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/clientcmd"
	"net"
	"os"
	gatewayVersioned "sigs.k8s.io/gateway-api/pkg/client/clientset/gateway/versioned"
	gatewayFakeClient "sigs.k8s.io/gateway-api/pkg/client/clientset/gateway/versioned/fake"
	gatewayLister "sigs.k8s.io/gateway-api/pkg/client/listers/gateway/apis/v1alpha2"
	"strings"

	"alauda.io/alb2/config"
	"alauda.io/alb2/modules"
	albclient "alauda.io/alb2/pkg/client/clientset/versioned"
	albfakeclient "alauda.io/alb2/pkg/client/clientset/versioned/fake"
	v1 "alauda.io/alb2/pkg/client/listers/alauda/v1"
	v1types "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfakeclient "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	corev1lister "k8s.io/client-go/listers/core/v1"

	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type Backend struct {
	InstanceID        string  `json:"instance_id"`
	IP                string  `json:"ip"`
	Protocol          string  `json:"-"`
	AppProtocol       *string `json:"-"`
	Port              int     `json:"port"`
	FromOtherClusters bool    `json:"otherclusters"`
}

func (b *Backend) String() string {
	return fmt.Sprintf("%s:%d", b.IP, b.Port)
}

type Service struct {
	Name        string     `json:"service_name"`
	Namespace   string     `json:"namespace"`
	NetworkMode string     `json:"network_mode"`
	ServicePort int        `json:"service_port"`
	Protocol    string     `json:"protocol"`
	AppProtocol *string    `json:"app_protocol"`
	Backends    []*Backend `json:"backends"`
}

type NodePort struct {
	Name      string
	Labels    map[string]string
	Ports     []int
	Selector  map[string]string
	Namespace string
}

type KubernetesDriver struct {
	DynamicClient  dynamic.Interface
	Client         kubernetes.Interface
	GatewayClient  gatewayVersioned.Interface
	Informers      Informers
	ALBClient      albclient.Interface
	ALB2Lister     v1.ALB2Lister
	FrontendLister v1.FrontendLister
	RuleLister     v1.RuleLister
	ServiceLister  corev1lister.ServiceLister
	EndpointLister corev1lister.EndpointsLister
	GatewayLister  gatewayLister.GatewayLister
}

func GetDriver() (*KubernetesDriver, error) {
	// TEST != "" means we are debugging or testing
	testMode := config.Get("TEST") == "true"
	return GetKubernetesDriver(testMode)
}

func (kd *KubernetesDriver) GetType() string {
	return config.Kubernetes
}

func GetKubeCfg() (*rest.Config, error) {
	klog.Infof("driver: out cluster")
	// respect KUBECONFIG env
	if os.Getenv("USE_KUBECONFIG") == "true" {
		klog.Infof("driver: use kube")
		kubecfg := os.Getenv("KUBECONFIG")
		cf, err := clientcmd.BuildConfigFromFlags("", kubecfg)
		return cf, err
	}
	cf, err := rest.InClusterConfig()
	if err == nil {
		return cf, nil
	}

	// respect KUBERNETES_XXX env. only used for test
	host := config.Get("KUBERNETES_SERVER")
	if host == "" {
		return nil, fmt.Errorf("invalid host from KUBERNETES_SERVER env")
	}
	klog.Infof("driver: k8s host is %v", host)
	tlsClientConfig := rest.TLSClientConfig{Insecure: true}
	cf = &rest.Config{
		Host:            host,
		BearerToken:     config.Get("KUBERNETES_BEARERTOKEN"),
		TLSClientConfig: tlsClientConfig,
	}
	return cf, nil
}

func GetKubernetesDriver(isFake bool) (*KubernetesDriver, error) {
	klog.Infof("fake mode %v", isFake)
	var client kubernetes.Interface
	var albClient albclient.Interface
	var dynamicClient dynamic.Interface
	var gatewayClient gatewayVersioned.Interface
	if isFake {
		client = fake.NewSimpleClientset()
		albClient = albfakeclient.NewSimpleClientset()
		dynamicClient = dynamicfakeclient.NewSimpleDynamicClient(runtime.NewScheme())
		gatewayClient = gatewayFakeClient.NewSimpleClientset()
	} else {
		cf, err := GetKubeCfg()
		if err != nil {
			return nil, err
		}
		client, err = kubernetes.NewForConfig(cf)
		if err != nil {
			return nil, err
		}
		albClient, err = albclient.NewForConfig(cf)
		if err != nil {
			return nil, err
		}
		dynamicClient, err = dynamic.NewForConfig(cf)
		if err != nil {
			return nil, err
		}
		gatewayClient, err = gatewayVersioned.NewForConfig(cf)
		if err != nil {
			return nil, err
		}
	}
	return &KubernetesDriver{Client: client, ALBClient: albClient, DynamicClient: dynamicClient, GatewayClient: gatewayClient}, nil
}

func InitDriver(driver *KubernetesDriver, ctx context.Context) error {
	informers, err := InitInformers(driver, ctx, InitInformersOptions{ErrorIfWaitSyncFail: false})
	if err != nil {
		return err
	}
	driver.Informers = *informers
	driver.ServiceLister = informers.K8s.Service.Lister()
	driver.EndpointLister = informers.K8s.Endpoint.Lister()
	driver.ALB2Lister = driver.Informers.Alb.Alb.Lister()
	driver.FrontendLister = driver.Informers.Alb.Ft.Lister()
	driver.RuleLister = driver.Informers.Alb.Rule.Lister()
	return nil
}

// GetClusterIPAddress return addresses of a clusterip service by using cluster ip and service port
func (kd *KubernetesDriver) GetClusterIPAddress(svc *v1types.Service, port int) (*Service, error) {
	service := &Service{
		Name:        svc.Name,
		Namespace:   svc.Namespace,
		ServicePort: port,
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
func (kd *KubernetesDriver) RuleIsOrphanedByApplication(rule *modules.Rule) (bool, error) {
	var appName, appNamespace string
	appNameLabelKey := fmt.Sprintf("app.%s/name", config.Get("DOMAIN"))
	for k, v := range rule.Labels {
		if strings.HasPrefix(k, appNameLabelKey) {
			vv := strings.Split(v, ".")
			if len(vv) != 2 {
				// Invalid application label, assume it's not an application component.
				return false, nil
			}
			appName, appNamespace = vv[0], vv[1]
			break
		}
	}
	if appName == "" {
		// No application label found, the rule doesn't belong to an application.
		return false, nil
	}
	_, err := kd.DynamicClient.Resource(schema.GroupVersionResource{
		Group:    "app.k8s.io",
		Version:  "v1beta1",
		Resource: "applications",
	}).Namespace(appNamespace).Get(context.TODO(), appName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		// The owner application is not found, the rule is orphaned.
		return true, nil
	}

	return false, err
}

// GetEndPointAddress return a list of pod ip in the endpoint, we assume all protocol are tcp now.
func (kd *KubernetesDriver) GetEndPointAddress(name, namespace string, svc *v1types.Service, svcPortNum int, protocol v1types.Protocol) (*Service, error) {

	ep, err := kd.EndpointLister.Endpoints(namespace).Get(name)

	if err != nil {
		return nil, err
	}
	containerPort, appProtocol, svcPortName, err := findContainerPort(svc, ep, svcPortNum, protocol)
	if err != nil {
		return nil, err
	}

	service := &Service{
		Name:        name,
		Namespace:   namespace,
		ServicePort: svcPortNum,
		Protocol:    string(protocol),
		AppProtocol: appProtocol,
		Backends:    make([]*Backend, 0),
	}

	for _, subset := range ep.Subsets {
		for _, addr := range subset.Addresses {
			service.Backends = append(service.Backends, &Backend{
				InstanceID:        addr.Hostname,
				IP:                addr.IP,
				Protocol:          string(protocol),
				AppProtocol:       appProtocol,
				Port:              containerPort,
				FromOtherClusters: false,
			})
		}
	}
	klog.V(3).Infof("backends of svc %s/%s : %+v", name, protocol, service.Backends)

	if config.GetBool("SERVE_CROSSCLUSTERS") {
		klog.Infof("begin serve cross-cluster endpointslice")
		service.Backends = kd.mergeSubmarinerCrossClusterBackends(namespace, name, svcPortName, service.Backends, protocol)
		klog.V(3).Infof("added cross-cluster backends of svc %s/%s : %+v", name, protocol, service.Backends)
	}

	if len(service.Backends) == 0 {
		klog.Warningf("service %s has 0 backends, means has no health pods", name)
	}
	return service, nil
}

// findContainerPort via given svc and endpoints, we assume protocol are tcp now.
//
// endpoint-controller https://github.com/kubernetes/kubernetes/blob/e8cf412e5ec70081477ad6f126c7f7ef7449109c/pkg/controller/endpoint/endpoints_controller.go#L442-L442
func findContainerPort(svc *v1types.Service, ep *v1types.Endpoints, svcPortNum int, protocol v1types.Protocol) (int, *string, string, error) {
	containerPort := 0
	var appProtocol *string
	var svcPortName string
	for _, svcPort := range svc.Spec.Ports {
		if svcPort.Protocol == "" {
			svcPort.Protocol = "tcp"
		}
		if svcPort.Protocol != protocol {
			continue
		}
		if svcPortNum == int(svcPort.Port) {
			svcPortName = svcPort.Name
			appProtocol = svcPort.AppProtocol
			if svcPort.TargetPort.Type == intstr.Int {
				containerPort = int(svcPort.TargetPort.IntVal)
				break
			}

			for _, subset := range ep.Subsets {
				for _, epp := range subset.Ports {
					if epp.Name == svcPortName {
						containerPort = int(epp.Port)
					}
				}
			}
			break
		}
	}

	if containerPort == 0 {
		return 0, nil, svcPortName, fmt.Errorf("could not find container port in svc %s/%s port %v svc.ports %v ep %v", svc.Namespace, svc.Name, svcPortNum, svc.Spec.Ports, ep)
	}
	return containerPort, appProtocol, svcPortName, nil
}

// GetExternalNameAddress return a fqdn address for service
func (kd *KubernetesDriver) GetExternalNameAddress(svc *v1types.Service, port int, protocol v1types.Protocol) (*Service, error) {
	service := &Service{
		Namespace:   svc.Namespace,
		Name:        svc.Name,
		ServicePort: port,
		Backends:    make([]*Backend, 0),
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
	klog.V(3).Infof("backends of svc %s: %+v", svc.Name, service.Backends)
	return service, nil
}

// GetServiceAddress return ip list of a service base on the type of service
func (kd *KubernetesDriver) GetServiceAddress(name, namespace string, servicePort int, protocol v1types.Protocol) (*Service, error) {
	svc, err := kd.ServiceLister.Services(namespace).Get(name)
	if err != nil || svc == nil {
		if !config.GetBool("SERVE_CROSSCLUSTERS") {
			klog.Errorf("Get service %s.%s failed: %s", name, namespace, err)
			return nil, err
		} else {
			klog.Infof("begin serve cross-cluster endpointslice")
			service := &Service{
				Name:        name,
				Namespace:   namespace,
				ServicePort: servicePort,
				Protocol:    string(protocol),
				Backends:    make([]*Backend, 0),
			}
			service.Backends = kd.mergeSubmarinerCrossClusterBackends(namespace, name, "", service.Backends, protocol)
			return service, nil
		}
	}
	switch svc.Spec.Type {
	case v1types.ServiceTypeClusterIP:
		return kd.GetEndPointAddress(name, namespace, svc, servicePort, protocol)
	case v1types.ServiceTypeNodePort:
		return kd.GetEndPointAddress(name, namespace, svc, servicePort, protocol)
	case v1types.ServiceTypeExternalName:
		return kd.GetExternalNameAddress(svc, servicePort, protocol)
	default:
		// ServiceTypeLoadBalancer
		klog.Errorf("Unsupported type %s of service %s.%s.", svc.Spec.Type, name, namespace)
		return nil, errors.New("Unknown Service Type")
	}
}

func (kd *KubernetesDriver) GetServicePortNumber(namespace, name string, port intstr.IntOrString, protocol v1types.Protocol) (int, error) {
	if port.Type == intstr.Int {
		return int(port.IntVal), nil
	}
	svc, err := kd.ServiceLister.Services(namespace).Get(name)
	if err != nil {
		return 0, err
	}
	portInService := 0
	for _, p := range svc.Spec.Ports {
		if p.Protocol != protocol {
			continue
		}
		if port.StrVal != "" && port.StrVal == p.Name {
			portInService = int(p.Port)
			break
		}
		if port.IntVal != 0 && port.IntVal == p.Port {
			portInService = int(p.Port)
			break
		}
	}
	if portInService == 0 {
		return 0, fmt.Errorf("could not find port %v in svc %s/%s %v", port, namespace, name, svc.Spec.Ports)
	}
	return portInService, nil
}

func (kd *KubernetesDriver) GetServiceByName(namespace, name string, servicePort int, protocol v1types.Protocol) (*Service, error) {
	return kd.GetServiceAddress(name, namespace, servicePort, protocol)
}

func (kd *KubernetesDriver) mergeSubmarinerCrossClusterBackends(namespace, name, svcPortName string, Backends []*Backend, protocol v1types.Protocol) []*Backend {

	sel := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "submariner-io/clusterID",
				Operator: metav1.LabelSelectorOpExists,
			},
		},
	}
	selector, err := metav1.LabelSelectorAsSelector(&sel)
	if err != nil {
		klog.Errorf("label selector:%s, for endpointslice is wrong: %s", selector, err.Error())
		return Backends
	}
	endpointSlices, err := kd.Informers.K8s.EndpointSlice.Lister().EndpointSlices(namespace).List(selector)
	if err != nil {
		klog.Errorf("Get cross-cluster endpointSlices from ns: %s, failed: %s", namespace, err.Error())
		return Backends
	} else {
		klog.Infof("Got endpointslice cross-clusters from ns:%s, %s", namespace, endpointSlices)
	}
	for _, endpointSlice := range endpointSlices {
		if strings.HasPrefix(endpointSlice.Name, name) {
			for _, port := range endpointSlice.Ports {
				if svcPortName == "" || svcPortName == *port.Name {
					for _, endpoint := range endpointSlice.Endpoints {
						if *endpoint.Conditions.Ready == true {
							for _, ip := range endpoint.Addresses {
								containerPort := int(*port.Port)
								Backends = append(Backends, &Backend{
									InstanceID:        *endpoint.Hostname,
									IP:                ip,
									Protocol:          string(protocol),
									Port:              containerPort,
									FromOtherClusters: true,
								})
							}
						}
					}
				}
			}
		}
	}
	return Backends
}

// IsPodReady returns true if a pod is ready; false otherwise.
func IsPodReady(pod *v1types.Pod) bool {
	return isPodReadyConditionTrue(pod.Status)
}

// IsPodReadyConditionTrue returns true if a pod is ready; false otherwise.
func isPodReadyConditionTrue(status v1types.PodStatus) bool {
	condition := getPodReadyCondition(status)
	return condition != nil && condition.Status == v1types.ConditionTrue
}

// GetPodReadyCondition extracts the pod ready condition from the given status and returns that.
// Returns nil if the condition is not present.
func getPodReadyCondition(status v1types.PodStatus) *v1types.PodCondition {
	_, condition := getPodCondition(&status, v1types.PodReady)
	return condition
}

// GetPodCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func getPodCondition(status *v1types.PodStatus, conditionType v1types.PodConditionType) (int, *v1types.PodCondition) {
	if status == nil {
		return -1, nil
	}
	return getPodConditionFromList(status.Conditions, conditionType)
}

// GetPodConditionFromList extracts the provided condition from the given list of condition and
// returns the index of the condition and the condition. Returns -1 and nil if the condition is not present.
func getPodConditionFromList(conditions []v1types.PodCondition, conditionType v1types.PodConditionType) (int, *v1types.PodCondition) {
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
