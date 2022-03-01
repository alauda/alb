package gateway

// log tag
const ALB_GATEWAY_CONTROLLER = "agctl" // gateway/xxroute controller relevant. a(lb)g(ateway)c(on)t(rol)l(er)
const ALB_GATEWAY_NGINX = "agng"       // nginx config relevant. a(lb)g(ateway)ng(inxconfiggenerater)

const GATEWAY_GROUP = "gateway.networking.k8s.io"
const GATEWAY_KIND = "Gateway"

var SUPPORT_KIND_MAP map[string][]string = map[string][]string{
	"TCP":   {"TCPRoute"},
	"HTTP":  {"HTTPRoute"},
	"HTTPS": {"HTTPRoute"},
}
