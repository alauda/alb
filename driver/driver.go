package driver

import (
	"context"
	"fmt"
	"alauda.io/alb2/config"
	albinformers "alauda.io/alb2/pkg/client/informers/externalversions"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"k8s.io/klog"
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
	ServiceID     string     `json:"service_id"`
	ServiceName   string     `json:"service_name"`
	NetworkMode   string     `json:"network_mode"`
	ServicePort   int        `json:"service_port"`
	ContainerPort int        `json:"container_port"`
	Backends      []*Backend `json:"backends"`
	Namespace     string     `json:"namespace"`
	Owner         string     `json:"owner"`
}

func (s *Service) String() string {
	return fmt.Sprintf("%s-%s-%d", s.Namespace, s.ServiceName, s.ContainerPort)
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
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(driver.Client, 0)
	ingressInformer := kubeInformerFactory.Extensions().V1beta1().Ingresses()
	ingressSynced := ingressInformer.Informer().HasSynced
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	serviceLister := serviceInformer.Lister()
	serviceSynced := serviceInformer.Informer().HasSynced
	endpointInformer := kubeInformerFactory.Core().V1().Endpoints()
	endpointLister := endpointInformer.Lister()
	endpointSynced := endpointInformer.Informer().HasSynced
	kubeInformerFactory.Start(ctx.Done())

	albInformerFactory := albinformers.NewSharedInformerFactoryWithOptions(driver.ALBClient, 0,
		albinformers.WithNamespace(config.Get("NAMESPACE")))
	alb2Informer := albInformerFactory.Crd().V1().ALB2s()
	alb2Lister := alb2Informer.Lister()
	alb2Synced := alb2Informer.Informer().HasSynced
	frontendInformer := albInformerFactory.Crd().V1().Frontends()
	frontendLister := frontendInformer.Lister()
	frontendSynced := frontendInformer.Informer().HasSynced
	ruleInformer := albInformerFactory.Crd().V1().Rules()
	ruleLister := ruleInformer.Lister()
	ruleSynced := ruleInformer.Informer().HasSynced
	albInformerFactory.Start(ctx.Done())

	driver.FillUpListers(serviceLister, endpointLister, alb2Lister, frontendLister, ruleLister)

	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(ctx.Done(), ingressSynced, serviceSynced, endpointSynced, alb2Synced, frontendSynced, ruleSynced); !ok {
		klog.Fatalf("failed to wait for caches to sync")
	}
}