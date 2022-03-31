package nginx

import (
	"fmt"

	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	. "alauda.io/alb2/gateway"
	albType "alauda.io/alb2/pkg/apis/alauda/v1"
	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils"
	. "alauda.io/alb2/utils/log"
	"github.com/go-logr/logr"

	gatewayPolicyType "alauda.io/alb2/gateway/nginx/policyattachment/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type HttpProtocolTranslate struct {
	drv    *driver.KubernetesDriver
	handle gatewayPolicyType.PolicyAttachmentHandle
	log    logr.Logger
}

func NewHttpProtocolTranslate(drv *driver.KubernetesDriver, log logr.Logger) HttpProtocolTranslate {
	return HttpProtocolTranslate{drv: drv, log: log}
}

func (h *HttpProtocolTranslate) SetPolicyAttachmentHandle(handle gatewayPolicyType.PolicyAttachmentHandle) {
	h.handle = handle
}

func (h *HttpProtocolTranslate) TransLate(ls []*Listener, ftMap FtMap) error {
	err := h.translateHttp(ls, ftMap)
	if err != nil {
		return err
	}
	err = h.translateHttps(ls, ftMap)
	if err != nil {
		return err
	}
	return nil
}

func (h *HttpProtocolTranslate) translateHttp(ls []*Listener, ftMap FtMap) error {
	log := h.log.WithName("http")
	portMap := make(map[gatewayType.PortNumber][]*Listener)
	for _, l := range ls {
		if !SameProtocol(l.Protocol, gatewayType.HTTPProtocolType) {
			continue
		}
		if _, ok := portMap[l.Port]; !ok {
			portMap[l.Port] = []*Listener{}
		}
		portMap[l.Port] = append(portMap[l.Port], l)
	}

	for port, lss := range portMap {
		if len(lss) == 0 {
			continue
		}
		ft := &Frontend{}
		ft.Port = int(port)
		ft.Protocol = v1.FtProtocolHTTP

		for _, ls := range lss {
			for _, route := range ls.routes {
				httpRoute, ok := route.(*HTTPRoute)
				if !ok {
					continue
				}
				rules, err := httpRouteToRule(httpRoute)
				if err != nil {
					log.Error(err, "translate to rule fail ingore this rule", "port", port, "route", GetObjectKey(route))
					continue
				}
				for i, rule := range rules {
					rule.RuleID = genRuleId(client.ObjectKey{Namespace: httpRoute.Namespace, Name: httpRoute.Name}, i, 0, ft.Port)
					if h.handle != nil {
						ref := gatewayPolicyType.Ref{
							Listener: &gatewayPolicyType.Listener{
								Listener:   ls.Listener,
								Gateway:    ls.gateway,
								Generation: ls.generation,
								CreateTime: ls.createTime,
							},
							Route:      httpRoute,
							RuleIndex:  i,
							MatchIndex: 0,
						}
						log.V(3).Info("onrule", "ref", ref.Describe(), "ft", ft.Port, "rule", rule.RuleID)
						err := h.handle.OnRule(ft, rule, ref)
						if err != nil {
							log.Error(err, "onrule fail", "ref", ref.Describe())
						}
					}
					ft.Rules = append(ft.Rules, rule)
				}
			}
		}
		if len(ft.Rules) == 0 {
			log.V(2).Info("could not find rule. ignore this port", "port", port)
			continue
		}
		ftMap.SetFt(string(ft.Protocol), ft.Port, ft)
	}
	return nil
}

// httproute attach to https listener
type Route struct {
	route         HTTPRoute
	certSecretKey client.ObjectKey
	certDomain    string
	listener      *Listener
}

// TODO fetch cert and parse hostname from it.
// https listener hostname is the hostname of it certref, a route who attach to https listener, it's hostname must contains listener's hostname.
func (h *HttpProtocolTranslate) translateHttps(ls []*Listener, ftMap FtMap) error {
	log := h.log.WithName("https")
	portMap := make(map[gatewayType.PortNumber][]*Route)
	for _, l := range ls {
		// TODO we beed webhook?
		if !SameProtocol(l.Protocol, gatewayType.HTTPSProtocolType) {
			continue
		}
		if l.TLS == nil {
			log.Info("invalid https listener tls is nill", "gateway", l.gateway, "listener", l.Name, "port", l.Port)
			continue
		}
		if len(l.TLS.CertificateRefs) != 1 {
			log.Info("invalid https listener more then one certificate", "certs", l.TLS.CertificateRefs, "gateway", l.gateway, "listener", l.Name, "port", l.Port)
			continue
		}

		cert := l.TLS.CertificateRefs[0]
		if cert.Kind != nil && *cert.Kind != "Secret" {
			log.Info("invalid cert kind", "kind", cert.Kind)
			continue
		}
		if cert.Namespace == nil {
			log.Info("invalid cert ns could not be nill")
			continue
		}
		if _, ok := portMap[l.Port]; !ok {
			portMap[l.Port] = []*Route{}
		}

		if l.Hostname == nil {
			log.Info("invalid https listener must have hostname")
			continue
		}
		domain := string(*l.Hostname)
		log.V(2).Info("cert for route", "cert-name", cert.Name, "cert-ns", *cert.Namespace, "domain", domain, "gateway", l.gateway, "listener", l.Name, "port", l.Port)
		for _, r := range l.routes {
			route, ok := r.(*HTTPRoute)
			if !ok {
				continue
			}
			portMap[l.Port] = append(portMap[l.Port], &Route{listener: l, route: *route, certSecretKey: client.ObjectKey{Namespace: string(*cert.Namespace), Name: string(cert.Name)}, certDomain: domain})
		}
	}

	for port, routes := range portMap {
		ft := &Frontend{}
		ft.Port = int(port)
		ft.Protocol = v1.FtProtocolHTTPS
		for _, route := range routes {
			rules, err := httpRouteToRule(&route.route)
			if err != nil {
				log.Error(err, "translate to rule fail ingore this rule", "port", port, "route", GetObjectKey(&route.route))
				continue
			}
			// every rule in same route use same cert
			for _, rule := range rules {
				rule.BackendProtocol = "https"
				rule.Domain = route.certDomain
				rule.CertificateName = fmt.Sprintf("%s/%s", route.certSecretKey.Namespace, route.certSecretKey.Name)
			}
			for i, rule := range rules {
				rule.RuleID = genRuleId(client.ObjectKey{Namespace: route.route.Namespace, Name: route.route.Name}, i, 0, ft.Port)
				if h.handle != nil {
					ref := gatewayPolicyType.Ref{
						Listener: &gatewayPolicyType.Listener{
							Listener:   route.listener.Listener,
							Gateway:    route.listener.gateway,
							Generation: route.listener.generation,
							CreateTime: route.listener.createTime,
						},
						Route:      &route.route,
						RuleIndex:  i,
						MatchIndex: 0,
					}
					log.V(3).Info("onrule", "ref", ref.Describe(), "ft", ft.Port, "rule", rule.RuleID)
					err := h.handle.OnRule(ft, rule, ref)
					if err != nil {
						log.Error(err, "onrule fail", "ref", ref.Describe())
					}
				}
				log.V(5).Info("https rule cert", "rule", rule.RuleID, "domain", rule.Domain, "cert", rule.CertificateName)
				ft.Rules = append(ft.Rules, rule)
			}
		}
		if len(ft.Rules) == 0 {
			log.V(2).Info("could not find any rule. ignore this port", "port", port)
			continue
		}
		ftMap.SetFt(string(ft.Protocol), ft.Port, ft)
	}
	return nil
}

func httpRouteToRule(r *HTTPRoute) ([]*Rule, error) {
	log := L().WithName(ALB_GATEWAY_NGINX).WithValues("route", client.ObjectKeyFromObject(r.GetObject()))
	rules := []*Rule{}
	for i, rr := range r.Spec.Rules {
		rule := Rule{}
		rule.Type = RuleTypeGateway
		dslx, err := HttpRuleMatchToDSLX(r.Spec.Hostnames, rr)
		if err != nil {
			log.Error(err, "translate http rule to dslx fail", "rule_index", i)
			continue
		}
		rule.DSLX = dslx
		// TODO backend protocol should be determined by the app-protocol in backend svc
		rule.BackendProtocol = "http"
		svcs, err := backendRefsToService(pickBackendRefs(rr.BackendRefs))
		if err != nil {
			continue
		}
		rule.Services = svcs
		rules = append(rules, &rule)
	}
	return rules, nil
}

// TODO use internaldsl instead of dslx
func HttpRuleMatchToDSLX(hostnames []gatewayType.Hostname, r gatewayType.HTTPRouteRule) (albType.DSLX, error) {
	ms := r.Matches
	dslx := albType.DSLX{}

	hostnameStrs := []string{}
	for _, h := range hostnames {
		hostnameStrs = append(hostnameStrs, string(h))
	}
	if len(ms) == 0 {
		return dslx, fmt.Errorf("empty matches is invalid")
	}
	if len(ms) != 1 {
		return dslx, fmt.Errorf("multiple matches not support yet")
	}
	m := ms[0]

	// match method
	if m.Method != nil {
		return nil, fmt.Errorf("method match not support yet")
	}
	// match hostname
	if len(hostnameStrs) != 0 {
		vals := []string{utils.OP_IN}
		vals = append(vals, hostnameStrs...)
		exp := albType.DSLXTerm{Type: utils.KEY_HOST, Values: [][]string{
			vals,
		}}
		dslx = append(dslx, exp)
	}
	// match query
	if m.QueryParams != nil {
		for _, q := range m.QueryParams {
			op, err := toOP((*string)(q.Type))
			if err != nil {
				return nil, fmt.Errorf("invalid query params match err %v", err)
			}
			exp := albType.DSLXTerm{Type: utils.KEY_PARAM, Values: [][]string{{op, q.Value}}, Key: q.Name}
			dslx = append(dslx, exp)
		}
	}

	// match url
	if m.Path != nil {
		path := m.Path
		matchValue := "/"
		if path.Value != nil {
			matchValue = *path.Value
		}
		op, err := toOP((*string)(path.Type))
		if err != nil {
			return nil, fmt.Errorf("invalid path match err %v", err)
		}
		exp := albType.DSLXTerm{Type: utils.KEY_URL, Values: [][]string{{op, matchValue}}}
		dslx = append(dslx, exp)
	}

	// match headers
	if m.Headers != nil {
		for _, h := range m.Headers {
			op, err := toOP((*string)(h.Type))
			if err != nil {
				return nil, fmt.Errorf("invalid header match err %v", err)
			}
			exp := albType.DSLXTerm{Type: utils.KEY_HEADER, Values: [][]string{{op, h.Value}}, Key: string(h.Name)}
			dslx = append(dslx, exp)
		}
	}
	return dslx, nil
}

func pickBackendRefs(refs []gatewayType.HTTPBackendRef) []gatewayType.BackendRef {
	ret := []gatewayType.BackendRef{}
	for _, r := range refs {
		ret = append(ret, r.BackendRef)
	}
	return ret
}

// each rule must have a unique ID
// each route can have multiple rules, and each rule can have multiple matches.
func genRuleId(route client.ObjectKey, ruleIndex int, matchIndex int, port int) string {
	return fmt.Sprintf("%d-%s-%s-%d-%d", port, route.Namespace, route.Name, ruleIndex, matchIndex)
}
