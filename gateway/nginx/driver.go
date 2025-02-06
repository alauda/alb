package nginx

import (
	"fmt"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	"alauda.io/alb2/gateway"
	"alauda.io/alb2/gateway/nginx/types"
	"alauda.io/alb2/utils"
	"github.com/go-logr/logr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var scheme = runtime.NewScheme()

//nolint:errcheck
func init() {
	_ = gv1.AddToScheme(scheme)
}

type Driver struct {
	kd  *driver.KubernetesDriver
	log logr.Logger
}

func NewDriver(kd *driver.KubernetesDriver, log logr.Logger) *Driver {
	return &Driver{
		kd:  kd,
		log: log,
	}
}

func (d *Driver) ListListener(sel config.GatewaySelector) ([]*types.Listener, error) {
	log := d.log.WithValues("sel", sel.String())
	kd := d.kd
	gatewayList := []*gv1.Gateway{}
	{
		if sel.GatewayClass != nil {
			gl, err := listGatewayByClassName(kd, *sel.GatewayClass)
			if err != nil {
				return nil, err
			}
			gatewayList = gl
		}

		if sel.GatewayName != nil {
			ns := sel.GatewayName.Namespace
			name := sel.GatewayName.Name
			gateway, err := kd.GatewayLister.Gateways(ns).Get(name)
			// NOTE: gateway not exist is accept.
			if k8serrors.IsNotFound(err) {
				log.Info("gateway not found")
				gatewayList = []*gv1.Gateway{}
			} else if err != nil {
				return nil, err
			} else {
				gatewayList = []*gv1.Gateway{gateway}
			}
		}
	}

	routes, err := listRoutes(kd)
	if err != nil {
		return nil, err
	}
	log.V(5).Info("list listener ", "len-gateway", len(gatewayList), "len-route", len(routes))

	// merge gateway and route into listener

	lsMap := map[ListenerKey]*types.Listener{}
	{
		for _, g := range gatewayList {
			key := client.ObjectKeyFromObject(g)
			d.log.V(5).Info("list listener ", "gateway", key)
			for _, l := range g.Spec.Listeners {
				if IsListenerReady(g.Status.Listeners, key, string(l.Name), g.Generation) {
					ls := &types.Listener{
						Listener:   l,
						Gateway:    client.ObjectKeyFromObject(g),
						Generation: g.Generation,
						Routes:     []gateway.CommonRoute{},
					}
					key := ListenerToKey(ls)
					lsMap[key] = ls
					d.log.V(5).Info("find a valid listener ", "key", key)
				}
			}
		}
	}

	// for each route check each parentRefs's condition, find out is this route ready.
	for _, r := range routes {
		statusMap := map[ListenerKey]gv1.RouteParentStatus{}
		routeKey := client.ObjectKeyFromObject(r.GetObject())
		for _, status := range GetStatus(r) {
			statusMap[RefToKey(status.ParentRef)] = status
		}
		// route refs to ls and this route is ready
		for _, ref := range r.GetSpec().ParentRefs {
			key := RefToKey(ref)
			ls, find := lsMap[key]
			if !find {
				// not ref to use
				d.log.V(5).Info("route not ref to use", "route", routeKey, "ref", key)
				continue
			}
			status, find := statusMap[key]
			if !find {
				// missing status
				d.log.V(5).Info("could not find status of route ref", "route", routeKey, "ref", key)
				continue
			}

			ready := false
			reason := ""
			{
				conditionFind := false
				conditionReady := false

				for _, c := range status.Conditions {
					if c.ObservedGeneration != r.GetObject().GetGeneration() {
						continue
					}
					conditionFind = true
					if c.Type == "Ready" && c.Status == "True" {
						ready = true
						conditionReady = true
						break
					}
				}
				reason = fmt.Sprintf("condition find %v ready %v", conditionFind, conditionReady)
			}

			if ready {
				ls.Routes = append(ls.Routes, r)
			} else {
				// not ready
				d.log.V(5).Info("status not ready", "route", routeKey, "ref", key, "version", r.GetObject().GetGeneration(), "reason", reason)
			}
		}
	}

	lsList := []*types.Listener{}
	for _, ls := range lsMap {
		lsList = append(lsList, ls)
	}

	return lsList, nil
}

// ListGatewayByClassName list gateway in all ns
func listGatewayByClassName(kd *driver.KubernetesDriver, className string) ([]*gv1.Gateway, error) {
	var ret []*gv1.Gateway
	gateways, err := kd.GatewayLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, gateway := range gateways {
		err := utils.AddTypeInformationToObject(scheme, gateway)
		if err != nil {
			return nil, err
		}
		if string(gateway.Spec.GatewayClassName) == className {
			ret = append(ret, gateway)
		}
	}
	return ret, nil
}

//nolint:errcheck
func listRoutes(kd *driver.KubernetesDriver) ([]gateway.CommonRoute, error) {
	ret := []gateway.CommonRoute{}
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
		r := gateway.HTTPRoute(*http)
		ret = append(ret, &r)
	}
	for _, tcp := range tcpList {
		_ = utils.AddTypeInformationToObject(scheme, tcp)
		r := gateway.TCPRoute(*tcp)
		ret = append(ret, &r)
	}
	for _, tls := range tlsList {
		_ = utils.AddTypeInformationToObject(scheme, tls)
		r := gateway.TLSRoute(*tls)
		ret = append(ret, &r)
	}
	for _, udp := range udpList {
		_ = utils.AddTypeInformationToObject(scheme, udp)
		r := gateway.UDPRoute(*udp)
		ret = append(ret, &r)
	}
	return ret, nil
}

func IsRouteReady(status []gv1.RouteParentStatus, key client.ObjectKey, name string, generation int64) (bool, string) {
	for _, s := range status {
		if gateway.IsRefToListener(s.ParentRef, key, name) {
			for _, c := range s.Conditions {
				if c.ObservedGeneration == generation && c.Type == "Ready" && c.Status == "True" {
					return true, ""
				}
			}
			return true, ""
		}
	}
	return false, ""
}

type ListenerKey string

func RefToKey(ref gv1.ParentReference) ListenerKey {
	key := fmt.Sprintf("%s/%s/%s", *ref.Namespace, ref.Name, *ref.SectionName)
	return ListenerKey(key)
}

func ListenerToKey(ls *types.Listener) ListenerKey {
	key := fmt.Sprintf("%s/%s/%s", ls.Gateway.Namespace, ls.Gateway.Name, ls.Listener.Name)
	return ListenerKey(key)
}

func IsListenerReady(status []gv1.ListenerStatus, key client.ObjectKey, name string, generation int64) bool {
	for _, s := range status {
		if string(s.Name) == name {
			for _, c := range s.Conditions {
				if c.ObservedGeneration == generation && c.Type == "Ready" && c.Status == "True" {
					return true
				}
			}
		}
	}
	return false
}

func GetStatus(r gateway.CommonRoute) []gv1.RouteParentStatus {
	switch route := r.(type) {
	case *gateway.HTTPRoute:
		return route.Status.Parents
	case *gateway.TCPRoute:
		return route.Status.Parents
	case *gateway.TLSRoute:
		return route.Status.Parents
	case *gateway.UDPRoute:
		return route.Status.Parents
	}
	return nil
}
