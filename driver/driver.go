package driver

import (
	"context"
	"errors"
	"fmt"

	"alauda.io/alb2/config"
	albinformers "alauda.io/alb2/pkg/client/informers/externalversions"
	albv1 "alauda.io/alb2/pkg/client/informers/externalversions/alauda/v1"
	kubeinformers "k8s.io/client-go/informers"
	v1 "k8s.io/client-go/informers/core/v1"
	networkingV1 "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	OwnerUnknown = "Unknown"
)

type Backend struct {
	InstanceID string `json:"instance_id"`
	IP         string `json:"ip"`
	Port       int    `json:"port"`
}

func (b *Backend) String() string {
	return fmt.Sprintf("%s:%d", b.IP, b.Port)
}

type ByBackend []*Backend

func (b ByBackend) Len() int {
	return len(b)
}

func (b ByBackend) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b ByBackend) Less(i, j int) bool {
	return fmt.Sprintf("%s-%d", b[i].IP, b[i].Port) < fmt.Sprintf("%s-%d", b[j].IP, b[j].Port)
}

type Service struct {
	Name        string     `json:"service_name"`
	Namespace   string     `json:"namespace"`
	NetworkMode string     `json:"network_mode"`
	ServicePort int        `json:"service_port"`
	Backends    []*Backend `json:"backends"`
}

type NodePort struct {
	Name      string
	Labels    map[string]string
	Ports     []int
	Selector  map[string]string
	Namespace string
}

func GetDriver() (*KubernetesDriver, error) {
	// TEST != "" means we are debugging or testing
	return GetKubernetesDriver(config.Get("TEST") != "")
}

// SetDebug enable debug mode. GetDriver() will return a driver with fake client
func SetDebug() {
	klog.Info("Set Debug")
	config.Set("TEST", "true")
}

func InitDriver(driver *KubernetesDriver, ctx context.Context) {
	informers, _ := InitInformers(driver, ctx, InitInformersOptions{ErrorIfWaitSyncFail: false})
	driver.FillUpListers(
		informers.K8s.Service.Lister(),
		informers.K8s.Endpoint.Lister(),
		informers.Alb.Alb.Lister(),
		informers.Alb.Ft.Lister(),
		informers.Alb.Rule.Lister())
}

// Informers will be used by alb
type Informers struct {
	K8s K8sInformers
	Alb AlbInformers
}

type K8sInformers struct {
	Ingress      networkingV1.IngressInformer
	IngressClass networkingV1.IngressClassInformer
	Service      v1.ServiceInformer
	Endpoint     v1.EndpointsInformer
	Namespace    v1.NamespaceInformer
}

type AlbInformers struct {
	Alb  albv1.ALB2Informer
	Ft   albv1.FrontendInformer
	Rule albv1.RuleInformer
}

type InitInformersOptions struct {
	ErrorIfWaitSyncFail bool // if errorIfWaitSyncFail set to false, and some error happens, it will ignore this error(just log) and still fill-up Informers
}

func InitInformers(driver *KubernetesDriver, ctx context.Context, options InitInformersOptions) (*Informers, error) {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(driver.Client, 0)

	namespaceInformer := kubeInformerFactory.Core().V1().Namespaces()
	namespaceSynced := namespaceInformer.Informer().HasSynced

	ingressInformer := kubeInformerFactory.Networking().V1().Ingresses()
	ingressSynced := ingressInformer.Informer().HasSynced

	ingressClassInformer := kubeInformerFactory.Networking().V1().IngressClasses()
	ingressClassSynced := ingressClassInformer.Informer().HasSynced

	serviceInformer := kubeInformerFactory.Core().V1().Services()
	serviceSynced := serviceInformer.Informer().HasSynced

	endpointInformer := kubeInformerFactory.Core().V1().Endpoints()
	endpointSynced := endpointInformer.Informer().HasSynced

	kubeInformerFactory.Start(ctx.Done())

	albInformerFactory := albinformers.NewSharedInformerFactoryWithOptions(driver.ALBClient, 0,
		albinformers.WithNamespace(config.Get("NAMESPACE")))

	alb2Informer := albInformerFactory.Crd().V1().ALB2s()
	alb2Synced := alb2Informer.Informer().HasSynced

	frontendInformer := albInformerFactory.Crd().V1().Frontends()
	frontendSynced := frontendInformer.Informer().HasSynced

	ruleInformer := albInformerFactory.Crd().V1().Rules()
	ruleSynced := ruleInformer.Informer().HasSynced

	albInformerFactory.Start(ctx.Done())

	if ok := cache.WaitForNamedCacheSync("alb2", ctx.Done(), namespaceSynced, ingressSynced, ingressClassSynced, serviceSynced, endpointSynced, alb2Synced, frontendSynced, ruleSynced); !ok {
		if options.ErrorIfWaitSyncFail {
			return nil, errors.New("wait alb2 informers sync fail")
		}
	}

	return &Informers{
		K8s: K8sInformers{
			Ingress:      ingressInformer,
			IngressClass: ingressClassInformer,
			Service:      serviceInformer,
			Endpoint:     endpointInformer,
			Namespace:    namespaceInformer,
		},
		Alb: AlbInformers{
			Alb:  alb2Informer,
			Ft:   frontendInformer,
			Rule: ruleInformer,
		},
	}, nil
}
