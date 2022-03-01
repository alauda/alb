package driver

import (
	"context"
	"errors"

	"alauda.io/alb2/config"
	albinformers "alauda.io/alb2/pkg/client/informers/externalversions"
	albv1 "alauda.io/alb2/pkg/client/informers/externalversions/alauda/v1"
	kubeinformers "k8s.io/client-go/informers"
	v1 "k8s.io/client-go/informers/core/v1"
	networkingV1 "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/tools/cache"
	gatewayExternal "sigs.k8s.io/gateway-api/pkg/client/informers/gateway/externalversions"
	gatewayV1alpha2 "sigs.k8s.io/gateway-api/pkg/client/informers/gateway/externalversions/apis/v1alpha2"
)

// Informers will be used by alb
type Informers struct {
	K8s     K8sInformers
	Alb     AlbInformers
	Gateway GatewayInformers
}

type K8sInformers struct {
	Ingress      networkingV1.IngressInformer
	IngressClass networkingV1.IngressClassInformer
	Service      v1.ServiceInformer
	Endpoint     v1.EndpointsInformer
	Namespace    v1.NamespaceInformer
}

type GatewayInformers struct {
	Gateway      gatewayV1alpha2.GatewayInformer
	GatewayClass gatewayV1alpha2.GatewayClassInformer
	HttpRoute    gatewayV1alpha2.HTTPRouteInformer
	TcpRoute     gatewayV1alpha2.TCPRouteInformer
	UdpRoute     gatewayV1alpha2.UDPRouteInformer
	TlsRoute     gatewayV1alpha2.TLSRouteInformer
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
	gatewayInformerFactory := gatewayExternal.NewSharedInformerFactory(driver.GatewayClient, 0)

	gatewayClassInformer := gatewayInformerFactory.Gateway().V1alpha2().GatewayClasses()
	gatewayClassSynced := gatewayClassInformer.Informer().HasSynced

	gatewayInformer := gatewayInformerFactory.Gateway().V1alpha2().Gateways()
	gatewaySynced := gatewayInformer.Informer().HasSynced

	httpRouteInformer := gatewayInformerFactory.Gateway().V1alpha2().HTTPRoutes()
	httpRouteSynced := httpRouteInformer.Informer().HasSynced

	tcpRouteInformer := gatewayInformerFactory.Gateway().V1alpha2().TCPRoutes()
	tcpRouteSynced := tcpRouteInformer.Informer().HasSynced

	udpRouteInformer := gatewayInformerFactory.Gateway().V1alpha2().UDPRoutes()
	udpRouteSynced := udpRouteInformer.Informer().HasSynced
	tlsRouteInformer := gatewayInformerFactory.Gateway().V1alpha2().TLSRoutes()
	tlsRouteSynced := tlsRouteInformer.Informer().HasSynced

	gatewayInformerFactory.Start(ctx.Done())

	if ok := cache.WaitForNamedCacheSync("alb2", ctx.Done(),
		namespaceSynced,
		ingressSynced,
		ingressClassSynced,
		serviceSynced,
		endpointSynced,
		alb2Synced,
		frontendSynced,
		ruleSynced,
		gatewayClassSynced,
		gatewaySynced,
		httpRouteSynced,
		tcpRouteSynced,
		udpRouteSynced,
		tlsRouteSynced,
	); !ok {
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
		Gateway: GatewayInformers{
			GatewayClass: gatewayClassInformer,
			Gateway:      gatewayInformer,
			HttpRoute:    httpRouteInformer,
			TcpRoute:     tcpRouteInformer,
			UdpRoute:     udpRouteInformer,
			TlsRoute:     tlsRouteInformer,
		},
	}, nil
}