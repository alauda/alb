package driver

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	gatewayVersioned "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gv1l "sigs.k8s.io/gateway-api/pkg/client/listers/apis/v1"

	"alauda.io/alb2/config"
	albclient "alauda.io/alb2/pkg/client/clientset/versioned"
	v1 "alauda.io/alb2/pkg/client/listers/alauda/v1"
	albv2 "alauda.io/alb2/pkg/client/listers/alauda/v2beta1"
	"alauda.io/alb2/utils/log"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cliu "alauda.io/alb2/utils/client"
)

type KubernetesDriver struct {
	Cli            client.Client // use this
	DynamicClient  dynamic.Interface
	Client         kubernetes.Interface
	GatewayClient  gatewayVersioned.Interface
	Informers      Informers // deprecated
	ALBClient      albclient.Interface
	ALBv2Client    albclient.Interface
	ALB2Lister     albv2.ALB2Lister             // deprecated
	FrontendLister v1.FrontendLister            // deprecated
	RuleLister     v1.RuleLister                // deprecated
	ServiceLister  corev1lister.ServiceLister   // deprecated
	EndpointLister corev1lister.EndpointsLister // deprecated
	GatewayLister  gv1l.GatewayLister           // deprecated
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
	if err := initInformerAndLister(drv, opt.Ctx); err != nil {
		return nil, err
	}
	drv.Opt = opt.Opt
	drv.n = config.NewNames(drv.Opt.Domain)
	drv.Log = log.L()
	return drv, nil
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
	scheme := cliu.InitScheme(runtime.NewScheme())
	cli, err := cliu.GetClient(ctx, cf, scheme)
	if err != nil {
		return nil, err
	}
	return &KubernetesDriver{Client: client, Cli: cli, ALBClient: albClient, DynamicClient: dynamicClient, GatewayClient: gatewayClient, Ctx: ctx}, nil
}

func Cfg2opt(cfg *config.Config) Opt {
	return Opt{
		Domain:              cfg.GetDomain(),
		Ns:                  cfg.GetNs(),
		EnableCrossClusters: cfg.GetFlags().EnableCrossClusters,
	}
}

// Deprecated: use NewDriver instead
func GetDriver(ctx context.Context) (*KubernetesDriver, error) {
	return GetAndInitKubernetesDriverFromCfg(ctx, nil)
}

// Deprecated: use NewDriver instead
func GetAndInitKubernetesDriverFromCfg(ctx context.Context, cf *rest.Config) (*KubernetesDriver, error) {
	cfg := config.GetConfig()
	if cf == nil {
		lcf, err := GetKubeCfg(cfg.K8s)
		if err != nil {
			return nil, err
		}
		cf = lcf
	}
	opt := Cfg2opt(cfg)
	return NewDriver(DrvOpt{Ctx: ctx, Cf: cf, Opt: opt})
}

func GetKubeCfgFromFile(f string) (*rest.Config, error) {
	cf, err := clientcmd.BuildConfigFromFlags("", f)
	return cf, err
}

func GetKubeCfg(k8s config.K8sConfig) (*rest.Config, error) {
	// respect KUBECONFIG env
	if k8s.Mode == "kubecfg" {
		kubecfg := k8s.KubeCfg
		return GetKubeCfgFromFile(kubecfg)
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
