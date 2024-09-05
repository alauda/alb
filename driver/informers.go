package driver

import (
	"context"
	"errors"

	albinformers "alauda.io/alb2/pkg/client/informers/externalversions"
	albv1 "alauda.io/alb2/pkg/client/informers/externalversions/alauda/v1"
	albv2 "alauda.io/alb2/pkg/client/informers/externalversions/alauda/v2beta1"
	albGateway "alauda.io/alb2/pkg/client/informers/externalversions/gateway/v1alpha1"
	kubeinformers "k8s.io/client-go/informers"
	v1 "k8s.io/client-go/informers/core/v1"
	discoveryv1 "k8s.io/client-go/informers/discovery/v1"
	networkingV1 "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/tools/cache"
	gatewayExternal "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"
	gv1b1i "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions/apis/v1"
	gv1a2i "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions/apis/v1alpha2"
)

// Informers will be used by alb
type Informers struct {
	K8s     K8sInformers
	Alb     AlbInformers
	Gateway GatewayInformers
}

type K8sInformers struct {
	Ingress       networkingV1.IngressInformer
	IngressClass  networkingV1.IngressClassInformer
	Service       v1.ServiceInformer
	Endpoint      v1.EndpointsInformer
	EndpointSlice discoveryv1.EndpointSliceInformer
	Namespace     v1.NamespaceInformer
}

type GatewayInformers struct {
	Gateway      gv1b1i.GatewayInformer
	GatewayClass gv1b1i.GatewayClassInformer
	HttpRoute    gv1b1i.HTTPRouteInformer
	TcpRoute     gv1a2i.TCPRouteInformer
	UdpRoute     gv1a2i.UDPRouteInformer
	TlsRoute     gv1a2i.TLSRouteInformer
}

type AlbInformers struct {
	Alb           albv2.ALB2Informer
	Ft            albv1.FrontendInformer
	Rule          albv1.RuleInformer
	TimeoutPolicy albGateway.TimeoutPolicyInformer
}

type InitInformersOptions struct {
	ErrorIfWaitSyncFail bool // if errorIfWaitSyncFail set to false, and some error happens, it will ignore this error(just log) and still fill-up Informers
}

func initInformerAndLister(driver *KubernetesDriver, ctx context.Context) error {
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

	endpointSliceInformer := kubeInformerFactory.Discovery().V1().EndpointSlices()
	endpointSliceSynced := endpointSliceInformer.Informer().HasSynced

	kubeInformerFactory.Start(ctx.Done())

	albInformerFactory := albinformers.NewSharedInformerFactoryWithOptions(driver.ALBClient, 0,
		albinformers.WithNamespace(driver.Opt.Ns))

	alb2Informer := albInformerFactory.Crd().V2beta1().ALB2s()
	alb2Synced := alb2Informer.Informer().HasSynced

	frontendInformer := albInformerFactory.Crd().V1().Frontends()
	frontendSynced := frontendInformer.Informer().HasSynced

	ruleInformer := albInformerFactory.Crd().V1().Rules()
	ruleSynced := ruleInformer.Informer().HasSynced

	albInformerFactory.Start(ctx.Done())

	gatewayInformerFactory := gatewayExternal.NewSharedInformerFactory(driver.GatewayClient, 0)

	gatewayClassInformer := gatewayInformerFactory.Gateway().V1().GatewayClasses()
	gatewayClassSynced := gatewayClassInformer.Informer().HasSynced

	gatewayInformer := gatewayInformerFactory.Gateway().V1().Gateways()
	gatewaySynced := gatewayInformer.Informer().HasSynced

	httpRouteInformer := gatewayInformerFactory.Gateway().V1().HTTPRoutes()
	httpRouteSynced := httpRouteInformer.Informer().HasSynced

	tcpRouteInformer := gatewayInformerFactory.Gateway().V1alpha2().TCPRoutes()
	tcpRouteSynced := tcpRouteInformer.Informer().HasSynced

	udpRouteInformer := gatewayInformerFactory.Gateway().V1alpha2().UDPRoutes()
	udpRouteSynced := udpRouteInformer.Informer().HasSynced

	tlsRouteInformer := gatewayInformerFactory.Gateway().V1alpha2().TLSRoutes()
	tlsRouteSynced := tlsRouteInformer.Informer().HasSynced

	gatewayInformerFactory.Start(ctx.Done())

	// gateway policyattachment could used in any ns.
	albGatewayInformerFactory := albinformers.NewSharedInformerFactoryWithOptions(driver.ALBClient, 0)

	timeoutPolicyInformer := albGatewayInformerFactory.Gateway().V1alpha1().TimeoutPolicies()
	timeoutPolicySynced := timeoutPolicyInformer.Informer().HasSynced

	albGatewayInformerFactory.Start(ctx.Done())

	if ok := cache.WaitForNamedCacheSync("alb2", ctx.Done(),
		namespaceSynced,
		ingressSynced,
		ingressClassSynced,
		serviceSynced,
		endpointSynced,
		endpointSliceSynced,
		alb2Synced,
		frontendSynced,
		ruleSynced,
		gatewayClassSynced,
		gatewaySynced,
		httpRouteSynced,
		tcpRouteSynced,
		udpRouteSynced,
		tlsRouteSynced,
		timeoutPolicySynced,
	); !ok {
		if options.ErrorIfWaitSyncFail {
			return nil, errors.New("wait alb2 informers sync fail")
		}
	}

	return &Informers{
		K8s: K8sInformers{
			Ingress:       ingressInformer,
			IngressClass:  ingressClassInformer,
			Service:       serviceInformer,
			Endpoint:      endpointInformer,
			EndpointSlice: endpointSliceInformer,
			Namespace:     namespaceInformer,
		},
		Alb: AlbInformers{
			Alb:           alb2Informer,
			Ft:            frontendInformer,
			Rule:          ruleInformer,
			TimeoutPolicy: timeoutPolicyInformer,
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
