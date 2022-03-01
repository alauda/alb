package gateway

import (
	"fmt"
	"strings"

	"alauda.io/alb2/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type HTTPRoute gatewayType.HTTPRoute

func (r *HTTPRoute) GetSpec() gatewayType.CommonRouteSpec {
	return r.Spec.CommonRouteSpec
}

func (r *HTTPRoute) GetObject() client.Object {
	return (*gatewayType.HTTPRoute)(r)
}

type TCPRoute gatewayType.TCPRoute

func (r *TCPRoute) GetSpec() gatewayType.CommonRouteSpec {
	return r.Spec.CommonRouteSpec
}

func (r *TCPRoute) GetObject() client.Object {
	return (*gatewayType.TCPRoute)(r)
}

type UDPRoute gatewayType.UDPRoute

func (r *UDPRoute) GetSpec() gatewayType.CommonRouteSpec {
	return r.Spec.CommonRouteSpec
}

func (r *UDPRoute) GetObject() client.Object {
	return (*gatewayType.UDPRoute)(r)
}

type TLSRoute gatewayType.TLSRoute

func (r *TLSRoute) GetSpec() gatewayType.CommonRouteSpec {
	return r.Spec.CommonRouteSpec
}
func (r *TLSRoute) GetObject() client.Object {
	return (*gatewayType.TLSRoute)(r)
}

type CommonRoute interface {
	GetSpec() gatewayType.CommonRouteSpec
	GetObject() client.Object
}

func IsRefsToGateway(refs []gatewayType.ParentRef, gateway client.ObjectKey) bool {
	for _, ref := range refs {
		if IsRefToGateway(ref, gateway) {
			return true
		}
	}
	return false
}

func IsRefToGateway(ref gatewayType.ParentRef, gateway client.ObjectKey) bool {
	return ref.Namespace != nil &&
		string(ref.Name) == gateway.Name &&
		string(*ref.Namespace) == gateway.Namespace
}

func IsRefToListener(ref gatewayType.ParentRef, gateway client.ObjectKey, name string) bool {
	return ref.Namespace != nil &&
		ref.SectionName != nil &&
		string(ref.Name) == gateway.Name &&
		string(*ref.Namespace) == gateway.Namespace &&
		string(*ref.SectionName) == name
}

// TODO use this to get key from route
func GetObjectKey(c CommonRoute) string {
	return client.ObjectKeyFromObject(c.GetObject()).String()
}

func GetClassName() string {
	return config.GetAlbName()
}

func GetControllerName() string {
	return fmt.Sprintf("alb2.gateway.%s/%s", config.Get("DOMAIN"), config.GetAlbName())
}

func IsSupported(protocol string, routekind string) bool {
	protocol = strings.ToUpper(protocol)
	routekind = strings.ToUpper(routekind)

	kinds := SUPPORT_KIND_MAP[protocol]
	find := false
	for _, k := range kinds {
		if routekind == strings.ToUpper(k) {
			find = true
			break
		}
	}
	return find
}

func SameProtocol(p1, p2 gatewayType.ProtocolType) bool {
	p1Str := string(p1)
	p2Str := string(p2)
	return strings.EqualFold(p1Str, p2Str)
}
