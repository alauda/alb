package gateway

import (
	"fmt"
	"strings"

	"alauda.io/alb2/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gv1a2t "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// TODO refactor with generic
type HTTPRoute gv1b1t.HTTPRoute

func (r *HTTPRoute) GetSpec() gv1b1t.CommonRouteSpec {
	return r.Spec.CommonRouteSpec
}

func (r *HTTPRoute) GetObject() client.Object {
	return (*gv1b1t.HTTPRoute)(r)
}

type TCPRoute gv1a2t.TCPRoute

func (r *TCPRoute) GetSpec() gv1b1t.CommonRouteSpec {
	return r.Spec.CommonRouteSpec
}

func (r *TCPRoute) GetObject() client.Object {
	return (*gv1a2t.TCPRoute)(r)
}

type UDPRoute gv1a2t.UDPRoute

func (r *UDPRoute) GetSpec() gv1b1t.CommonRouteSpec {
	return r.Spec.CommonRouteSpec
}

func (r *UDPRoute) GetObject() client.Object {
	return (*gv1a2t.UDPRoute)(r)
}

type TLSRoute gv1a2t.TLSRoute

func (r *TLSRoute) GetSpec() gv1b1t.CommonRouteSpec {
	return r.Spec.CommonRouteSpec
}
func (r *TLSRoute) GetObject() client.Object {
	return (*gv1a2t.TLSRoute)(r)
}

type CommonRoute interface {
	GetSpec() gv1b1t.CommonRouteSpec
	GetObject() client.Object
}

func IsRefsToGateway(refs []gv1b1t.ParentReference, gateway client.ObjectKey) bool {
	for _, ref := range refs {
		if IsRefToGateway(ref, gateway) {
			return true
		}
	}
	return false
}

func IsRefToGateway(ref gv1b1t.ParentReference, gateway client.ObjectKey) bool {
	return ref.Namespace != nil &&
		string(ref.Name) == gateway.Name &&
		string(*ref.Namespace) == gateway.Namespace
}

func IsRefToListener(ref gv1b1t.ParentReference, gateway client.ObjectKey, name string) bool {
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

// TODO fixme
func GetClassName() string {
	return config.GetConfig().GetAlbName()
}

func GetControllerName() string {
	return fmt.Sprintf("alb2.gateway.%s/%s", config.GetConfig().GetDomain(), config.GetConfig().GetAlbName())
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

func SameProtocol(p1, p2 gv1b1t.ProtocolType) bool {
	p1Str := string(p1)
	p2Str := string(p2)
	return strings.EqualFold(p1Str, p2Str)
}
