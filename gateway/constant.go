package gateway

import (
	sets "github.com/deckarep/golang-set/v2"
)

// log tag
const (
	ALB_GATEWAY_CONTROLLER = "agctl" // gateway/xxroute controller relevant. a(lb)g(ateway)c(on)t(rol)l(er)
	ALB_GATEWAY_NGINX      = "agng"  // nginx config relevant. a(lb)g(ateway)ng(inxconfiggenerater)
)

const (
	GATEWAY_GROUP = "gateway.networking.k8s.io"
	GATEWAY_KIND  = "Gateway"
)

const (
	GatewayClassKind = "GatewayClass"
	GatewayKind      = "Gateway"
	HttpRouteKind    = "HTTPRoute"
	TcpRouteKind     = "TCPRoute"
	UdpRouteKind     = "UDPRoute"
)

var SUPPORT_KIND_MAP map[string][]string = map[string][]string{
	"TCP":   {TcpRouteKind},
	"UDP":   {UdpRouteKind},
	"HTTP":  {HttpRouteKind},
	"HTTPS": {HttpRouteKind},
}

var SUPPORT_KIND_SET sets.Set[string] = sets.NewSet(TcpRouteKind, UdpRouteKind, HttpRouteKind)
