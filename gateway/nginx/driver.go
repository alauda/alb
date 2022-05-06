package nginx

import (
	"alauda.io/alb2/driver"
	. "alauda.io/alb2/gateway"
	. "alauda.io/alb2/gateway/nginx/types"
	"alauda.io/alb2/utils"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = gatewayType.AddToScheme(scheme)
}

func ListListenerByClass(kd *driver.KubernetesDriver, classname string) ([]*Listener, error) {
	// TODO use labels?
	gatewayList, err := listGatewayByClassName(kd, classname)
	if err != nil {
		return nil, err
	}
	routes, err := listRoutes(kd)
	if err != nil {
		return nil, err
	}
	// merge gateway and route
	lsList := []*Listener{}
	for _, g := range gatewayList {
		key := client.ObjectKeyFromObject(g)
		for _, l := range g.Spec.Listeners {
			if IsListenerReady(g.Status.Listeners, key, string(l.Name), g.Generation) {
				ls := &Listener{
					Listener:   l,
					Gateway:    client.ObjectKeyFromObject(g),
					Generation: g.Generation,
					Routes:     []CommonRoute{},
				}
				lsList = append(lsList, ls)
			}
		}
	}

	for _, r := range routes {
		// route refs to ls and this route is ready
		for _, ref := range r.GetSpec().ParentRefs {
			for _, ls := range lsList {
				isRef := IsRefToListener(ref, ls.Gateway, string(ls.Name))
				isReady := IsRouteReady(GetStatus(r), ls.Gateway, string(ls.Name), r.GetObject().GetGeneration())
				if isRef && isReady {
					ls.Routes = append(ls.Routes, r)
				}
			}
		}
	}
	return lsList, nil
}

// ListGatewayByClassName list gateway in all ns
func listGatewayByClassName(kd *driver.KubernetesDriver, classname string) ([]*gatewayType.Gateway, error) {
	var ret []*gatewayType.Gateway
	gateways, err := kd.Informers.Gateway.Gateway.Lister().List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, gateway := range gateways {
		_ = utils.AddTypeInformationToObject(scheme, gateway)
		if string(gateway.Spec.GatewayClassName) == classname {
			ret = append(ret, gateway)
		}
	}
	return ret, nil
}

func listRoutes(kd *driver.KubernetesDriver) ([]CommonRoute, error) {
	ret := []CommonRoute{}
	httpList, err := kd.Informers.Gateway.HttpRoute.Lister().List(labels.Everything())
	if err != nil {
		return nil, err
	}
	tcpList, err := kd.Informers.Gateway.TcpRoute.Lister().List(labels.Everything())
	if err != nil {
		return nil, err
	}
	udpList, err := kd.Informers.Gateway.UdpRoute.Lister().List(labels.Everything())
	if err != nil {
		return nil, err
	}
	tlsList, err := kd.Informers.Gateway.TlsRoute.Lister().List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, http := range httpList {
		_ = utils.AddTypeInformationToObject(scheme, http)
		r := HTTPRoute(*http)
		ret = append(ret, &r)
	}
	for _, tcp := range tcpList {
		_ = utils.AddTypeInformationToObject(scheme, tcp)
		r := TCPRoute(*tcp)
		ret = append(ret, &r)
	}
	for _, tls := range tlsList {
		_ = utils.AddTypeInformationToObject(scheme, tls)
		r := TLSRoute(*tls)
		ret = append(ret, &r)
	}
	for _, udp := range udpList {
		_ = utils.AddTypeInformationToObject(scheme, udp)
		r := UDPRoute(*udp)
		ret = append(ret, &r)
	}
	return ret, nil
}

func IsRouteReady(status []gatewayType.RouteParentStatus, key client.ObjectKey, name string, generation int64) bool {
	for _, s := range status {
		if IsRefToListener(s.ParentRef, key, name) {
			for _, c := range s.Conditions {
				if c.ObservedGeneration == generation && c.Type == "Ready" && c.Status == "True" {
					return true
				}
			}
			return true
		}
	}
	return false
}

func IsListenerReady(status []gatewayType.ListenerStatus, key client.ObjectKey, name string, generation int64) bool {
	for _, s := range status {
		if string(s.Name) == string(name) {
			for _, c := range s.Conditions {
				if c.ObservedGeneration == generation && c.Type == "Ready" && c.Status == "True" {
					return true
				}
			}
		}
	}
	return false
}

func GetStatus(r CommonRoute) []gatewayType.RouteParentStatus {
	switch route := r.(type) {
	case *HTTPRoute:
		return route.Status.Parents
	case *TCPRoute:
		return route.Status.Parents
	case *TLSRoute:
		return route.Status.Parents
	case *UDPRoute:
		return route.Status.Parents
	}
	return nil
}
