package http

import (
	"fmt"

	gatewayPolicyType "alauda.io/alb2/gateway/nginx/policyattachment/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "alauda.io/alb2/gateway"
	. "alauda.io/alb2/gateway/nginx/types"

	gv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func getCert(l *Listener) (secret *client.ObjectKey, certDomain *string, err error) {
	if l.TLS == nil {
		return nil, nil, fmt.Errorf("must have tls config")
	}
	if len(l.TLS.CertificateRefs) != 1 {
		return nil, nil, fmt.Errorf("invalid https listener more then one certificate")
	}

	cert := l.TLS.CertificateRefs[0]
	if cert.Kind != nil && *cert.Kind != "Secret" {
		return nil, nil, fmt.Errorf("invalid cert kind, must be secret")
	}
	if cert.Namespace == nil {
		return nil, nil, fmt.Errorf("invalid cert ns could not be nill")
	}
	if l.Hostname == nil {
		return nil, nil, fmt.Errorf("invalid https listener must have hostname")
	}
	return &client.ObjectKey{Namespace: string(*cert.Namespace), Name: string(cert.Name)}, (*string)(l.Hostname), nil
}

func (c *HttpCtx) ToAttachRef() gatewayPolicyType.Ref {
	ctx := c
	return gatewayPolicyType.Ref{
		Listener: &gatewayPolicyType.Listener{
			Listener:   ctx.listener.Listener,
			Gateway:    ctx.listener.Gateway,
			Generation: ctx.listener.Generation,
			CreateTime: ctx.listener.CreateTime,
		},
		Route:      ctx.httpRoute,
		RuleIndex:  int(ctx.ruleIndex),
		MatchIndex: int(ctx.matchIndex), // TODO
	}
}

func PatchHttpRouteDefualtMatch(listenerList []*Listener) {
	prefix := gv1.PathMatchPathPrefix
	value := "/"
	defaultHttpMatch := gv1.HTTPRouteMatch{
		Path: &gv1.HTTPPathMatch{Type: &prefix, Value: &value},
	}
	for _, listener := range listenerList {
		for routeIndex, route := range listener.Routes {
			httpRoute, ok := route.(*HTTPRoute)
			if !ok {
				continue
			}
			for ruleIndex, rule := range httpRoute.Spec.Rules {
				if len(rule.Matches) == 0 {
					httpRoute.Spec.Rules[ruleIndex].Matches = []gv1.HTTPRouteMatch{
						defaultHttpMatch,
					}
					listener.Routes[routeIndex] = httpRoute
				}
			}
		}
	}
}

func IterHttpListener[T any, F func(HttpCtx) *T](listenerList []*Listener, f F) []T {
	// 4-level for loop T_T
	retList := []T{}
	for _, listener := range listenerList {
		for _, route := range listener.Routes {
			httpRoute, ok := route.(*HTTPRoute)
			if !ok {
				continue
			}
			for ruleIndex, r := range httpRoute.Spec.Rules {
				rule := r
				for matchIndex := range rule.Matches {
					// flatten nest tree
					ctx := HttpCtx{
						listener:   listener,
						httpRoute:  httpRoute,
						rule:       &rule,
						ruleIndex:  uint(ruleIndex),
						matchIndex: uint(matchIndex),
					}
					t := f(ctx)
					if t != nil {
						retList = append(retList, *t)
					}
				}
			}
		}
	}
	return retList
}

func pickHttpBackendRefs(refs []gv1.HTTPBackendRef) []gv1.BackendRef {
	ret := []gv1.BackendRef{}
	for _, r := range refs {
		ret = append(ret, r.BackendRef)
	}
	return ret
}

func GroupListener[K comparable, F func(ls *Listener) (k *K)](lss []*Listener, f F) map[K][]*Listener {
	portListenerMap := make(map[K][]*Listener)
	for _, l := range lss {
		key := f(l)
		if key == nil {
			continue
		}
		if portListenerMap[*key] == nil {
			portListenerMap[*key] = make([]*Listener, 0)
		}
		portListenerMap[*key] = append(portListenerMap[*key], l)
	}
	return portListenerMap
}

func GroupListenerByProtocol(lss []*Listener, protocol gv1.ProtocolType) map[int][]*Listener {
	plsMap := GroupListener(lss, func(ls *Listener) *int {
		if !SameProtocol(ls.Protocol, protocol) {
			return nil
		}
		port := int(ls.Port)
		return &port
	})
	return plsMap
}
