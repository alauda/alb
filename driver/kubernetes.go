package driver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/clientcmd"
	gatewayVersioned "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gv1l "sigs.k8s.io/gateway-api/pkg/client/listers/apis/v1"

	"alauda.io/alb2/config"
	"alauda.io/alb2/controller/modules"
	albclient "alauda.io/alb2/pkg/client/clientset/versioned"
	v1 "alauda.io/alb2/pkg/client/listers/alauda/v1"
	albv2 "alauda.io/alb2/pkg/client/listers/alauda/v2beta1"
	"alauda.io/alb2/utils/log"
	v1types "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"

	"k8s.io/client-go/rest"
)

type Backend struct {
	InstanceID        string  `json:"instance_id"`
	Pod               string  `json:"pod"`
	Ns                string  `json:"name"`
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

// TODO 这个和测试中使用的k8sclient很类似，本质上都是封装了各种client
type KubernetesDriver struct {
	DynamicClient  dynamic.Interface
	Client         kubernetes.Interface
	GatewayClient  gatewayVersioned.Interface
	Informers      Informers
	ALBClient      albclient.Interface
	ALBv2Client    albclient.Interface
	ALB2Lister     albv2.ALB2Lister
	FrontendLister v1.FrontendLister
	RuleLister     v1.RuleLister
	ServiceLister  corev1lister.ServiceLister
	EndpointLister corev1lister.EndpointsLister
	GatewayLister  gv1l.GatewayLister
	Ctx            context.Context
	Opt            Opt
	Log            logr.Logger
	n              config.Names
}

// we do not want to reply on the golbal config
// define what we need here
type Opt struct {
	Domain              string
	Ns                  string // which alb cr exist
	EnableCrossClusters bool   // TODO seems odd the add those flag here
}

type DrvOpt struct {
	Ctx context.Context
	Cf  *rest.Config
	Opt Opt
}

func NewDriver(opt DrvOpt) (*KubernetesDriver, error) {
	drv, err := getKubernetesDriverFromCfg(opt.Ctx, opt.Cf)
	if err != nil {
		return nil, err
	}
	if err := initDriver(drv, opt.Ctx); err != nil {
		return nil, err
	}
	drv.Opt = opt.Opt
	drv.n = config.NewNames(drv.Opt.Domain)
	drv.Log = log.L()
	return drv, nil
}

func cfg2opt(cfg *config.Config) Opt {
	return Opt{
		Domain:              cfg.GetDomain(),
		Ns:                  cfg.GetNs(),
		EnableCrossClusters: cfg.GetFlags().EnableCrossClusters,
	}
}

// Deprecated: use NewDriver instead
func GetDriver(ctx context.Context) (*KubernetesDriver, error) {
	return GetAndInitDriver(ctx)
}

// Deprecated: use NewDriver instead
func GetAndInitDriver(ctx context.Context) (*KubernetesDriver, error) {
	cfg := config.GetConfig()
	cf, err := GetKubeCfg(cfg.K8s)
	if err != nil {
		return nil, err
	}
	return NewDriver(DrvOpt{
		Ctx: ctx,
		Cf:  cf,
		Opt: cfg2opt(cfg),
	})
}

// Deprecated: use NewDriver instead
func GetAndInitKubernetesDriverFromCfg(ctx context.Context, cf *rest.Config) (*KubernetesDriver, error) {
	cfg := config.GetConfig()
	opt := cfg2opt(cfg)
	return NewDriver(DrvOpt{Ctx: ctx, Cf: cf, Opt: opt})
}

func GetKubeCfg(k8s config.K8sConfig) (*rest.Config, error) {
	// respect KUBECONFIG env
	if k8s.Mode == "kubecfg" {
		kubecfg := k8s.KubeCfg
		cf, err := clientcmd.BuildConfigFromFlags("", kubecfg)
		return cf, err
	}
	// respect KUBERNETES_XXX env. only used for test
	if k8s.Mode == "kube_xx" {
		host := k8s.K8sServer
		if host == "" {
			return nil, fmt.Errorf("invalid host from KUBERNETES_SERVER env")
		}
		tlsClientConfig := rest.TLSClientConfig{Insecure: true}
		cf := &rest.Config{
			Host:            host,
			BearerToken:     k8s.K8sToken,
			TLSClientConfig: tlsClientConfig,
		}
		return cf, nil
	}
	cf, err := rest.InClusterConfig()
	return cf, err
}

func getKubernetesDriverFromCfg(ctx context.Context, cf *rest.Config) (*KubernetesDriver, error) {
	client, err := kubernetes.NewForConfig(cf)
	if err != nil {
		return nil, err
	}
	albClient, err := albclient.NewForConfig(cf)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(cf)
	if err != nil {
		return nil, err
	}
	gatewayClient, err := gatewayVersioned.NewForConfig(cf)
	if err != nil {
		return nil, err
	}
	return &KubernetesDriver{Client: client, ALBClient: albClient, DynamicClient: dynamicClient, GatewayClient: gatewayClient, Ctx: ctx}, nil
}

func initDriver(driver *KubernetesDriver, ctx context.Context) error {
	informers, err := InitInformers(driver, ctx, InitInformersOptions{ErrorIfWaitSyncFail: false})
	if err != nil {
		return err
	}
	driver.Informers = *informers
	driver.ALB2Lister = informers.Alb.Alb.Lister()
	driver.FrontendLister = informers.Alb.Ft.Lister()
	driver.RuleLister = informers.Alb.Rule.Lister()
	driver.ServiceLister = informers.K8s.Service.Lister()
	driver.EndpointLister = informers.K8s.Endpoint.Lister()
	driver.GatewayLister = informers.Gateway.Gateway.Lister()
	return nil
}

// GetClusterIPAddress return addresses of a cluster ip service by using cluster ip and service port
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
	appNameLabelKey := fmt.Sprintf("app.%s/name", kd.Opt.Domain)
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
	}).Namespace(appNamespace).Get(kd.Ctx, appName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		// The owner application is not found, the rule is orphaned.
		return true, nil
	}

	return false, err
}

// GetEndPointAddress return a list of pod ip in the endpoint
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
			backend := &Backend{
				InstanceID:        addr.Hostname,
				IP:                addr.IP,
				Protocol:          string(protocol),
				AppProtocol:       appProtocol,
				Port:              containerPort,
				FromOtherClusters: false,
			}
			if addr.TargetRef != nil {
				backend.Pod = addr.TargetRef.Name
				backend.Ns = addr.TargetRef.Namespace
			}
			service.Backends = append(service.Backends, backend)
		}
	}
	kd.Log.V(3).Info("backends of svc", "name", name, "protocol", protocol, "backends", service.Backends)

	if kd.Opt.EnableCrossClusters {
		kd.Log.Info("begin serve cross-cluster endpointslice")
		service.Backends = kd.mergeSubmarinerCrossClusterBackends(namespace, name, svcPortName, service.Backends, protocol)
		kd.Log.V(3).Info("added cross-cluster backends of svc", "name", name, "protocol", protocol, "backends", service.Backends)
	}

	if len(service.Backends) == 0 {
		kd.Log.Info("service has 0 backends means has no health pods", "svc", name, "warn", true)
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
	kd.Log.V(3).Info("backends of svc", "name", svc.Name, "backends", service.Backends)
	return service, nil
}

// GetServiceAddress return ip list of a service base on the type of service
func (kd *KubernetesDriver) GetServiceAddress(name, namespace string, servicePort int, protocol v1types.Protocol) (*Service, error) {
	svc, err := kd.ServiceLister.Services(namespace).Get(name)
	if err != nil || svc == nil {
		if !kd.Opt.EnableCrossClusters {
			kd.Log.Error(err, "Get service failed", "name", name, "ns", namespace)
			return nil, err
		} else {
			kd.Log.Info("begin serve cross-cluster endpointslice")
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
	case v1types.ServiceTypeLoadBalancer:
		return kd.GetEndPointAddress(name, namespace, svc, servicePort, protocol)
	default:
		err := errors.New("unknown service type")
		kd.Log.Error(err, "Unsupported type of service", "type", svc.Spec.Type, "name", name, "ns", namespace)
		return nil, err
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

func (kd *KubernetesDriver) mergeSubmarinerCrossClusterBackends(namespace, name, svcPortName string, backends []*Backend, protocol v1types.Protocol) []*Backend {
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
		kd.Log.Error(err, "label selector:%s, for endpointslice is wrong:", sel)
		return backends
	}
	endpointSlices, err := kd.Informers.K8s.EndpointSlice.Lister().EndpointSlices(namespace).List(selector)
	if err != nil {
		kd.Log.Error(err, "Get cross-cluster endpointSlices from ns failed", "ns", namespace)
		return backends
	} else {
		kd.Log.Info("Get cross-cluster endpointSlices from ns", "ns", namespace, endpointSlices)
	}
	for _, endpointSlice := range endpointSlices {
		if strings.HasPrefix(endpointSlice.Name, name) {
			for _, port := range endpointSlice.Ports {
				if svcPortName == "" || svcPortName == *port.Name {
					for _, endpoint := range endpointSlice.Endpoints {
						if *endpoint.Conditions.Ready {
							for _, ip := range endpoint.Addresses {
								containerPort := int(*port.Port)
								backends = append(backends, &Backend{
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
	return backends
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
