package http

import (
	"fmt"
	"strconv"
	"strings"

	"alauda.io/alb2/config"
	"alauda.io/alb2/controller/modules"
	ctltype "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	"alauda.io/alb2/gateway"
	ngxtype "alauda.io/alb2/gateway/nginx/types"
	nu "alauda.io/alb2/gateway/nginx/utils"
	u "alauda.io/alb2/gateway/utils"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils"
	"github.com/go-logr/logr"
	"github.com/samber/lo"

	gatewayPolicyType "alauda.io/alb2/gateway/nginx/policyattachment/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type HttpProtocolTranslate struct {
	drv    *driver.KubernetesDriver
	handle gatewayPolicyType.PolicyAttachmentHandle
	log    logr.Logger
	cfg    *config.Config
}

func NewHttpProtocolTranslate(drv *driver.KubernetesDriver, log logr.Logger, cfg *config.Config) HttpProtocolTranslate {
	return HttpProtocolTranslate{drv: drv, log: log, cfg: cfg}
}

func (h *HttpProtocolTranslate) TransLate(ls []*ngxtype.Listener, ftMap ngxtype.FtMap) error {
	PatchHttpRouteDefaultMatch(ls) // if http route has empty matches, add a prefix / as default match
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

func (h *HttpProtocolTranslate) SetPolicyAttachmentHandle(handle gatewayPolicyType.PolicyAttachmentHandle) {
	h.handle = handle
}

func (h *HttpProtocolTranslate) applyPolicyAttachmentOnRule(ft *ctltype.Frontend, ref gatewayPolicyType.Ref, rule *ctltype.InternalRule) error {
	if h.handle == nil {
		return nil
	}
	return h.handle.OnRule(ft, rule, ref)
}

// ctx is sth that contain enough information to identify a rule.
// now, each 'match' correspond to a alb rule.
type HttpCtx struct {
	listener   *ngxtype.Listener
	httpRoute  *gateway.HTTPRoute
	ruleIndex  uint
	rule       *gv1.HTTPRouteRule
	matchIndex uint
}

func (c *HttpCtx) ToString() string {
	return fmt.Sprintf("%s-%s-%s-%s-%s", c.listener.Gateway.Namespace, c.listener.Gateway.Name, c.listener.Listener.Name, c.httpRoute.Namespace, c.httpRoute.Name)
}

func (c *HttpCtx) GetMatcher() gv1.HTTPRouteMatch {
	return c.httpRoute.Spec.Rules[c.ruleIndex].Matches[c.matchIndex]
}

// http route attach to http listener
func (h *HttpProtocolTranslate) translateHttp(lss []*ngxtype.Listener, ftMap ngxtype.FtMap) error {
	log := h.log.WithName("http")
	_ = log // make go happy

	portMap := make(map[int][]HttpCtx)
	{
		plsMap := GroupListenerByProtocol(lss, gv1.HTTPProtocolType)
		for port, lss := range plsMap {
			ctxList := IterHttpListener(lss, func(ctx HttpCtx) *HttpCtx {
				return &ctx
			})
			portMap[port] = ctxList
		}
	}

	// now, we could generate alb rule
	for port, ctxList := range portMap {
		ft := &ctltype.Frontend{}
		ft.Port = albv1.PortNumber(port)
		ft.Protocol = albv1.FtProtocolHTTP
		rules := []*ctltype.InternalRule{}
		for _, ctx := range ctxList {
			rule, err := h.generateHttpRule(ctx)
			if err != nil {
				log.Error(err, "generate http rule failed")
				continue
			}
			err = h.applyPolicyAttachmentOnRule(ft, ctx.ToAttachRef(), rule)
			if err != nil {
				log.Error(err, "attach policy fail")
			}
			err = h.applyHttpFilterOnRule(ctx, rule, ctx.rule.Filters)
			if err != nil {
				log.Error(err, "apply filter fail")
			}
			rules = append(rules, rule)
		}
		ft.Rules = append(ft.Rules, rules...)

		if len(ft.Rules) == 0 {
			log.Info("could not find rule. ignore this port", "port", port)
			continue
		}
		ftMap.SetFt(string(ft.Protocol), ft.Port, ft)
	}
	return nil
}

func (h *HttpProtocolTranslate) generateHttpRule(ctx HttpCtx) (*ctltype.InternalRule, error) {
	route := ctx.httpRoute
	gRule := route.Spec.Rules[ctx.ruleIndex]
	match := gRule.Matches[ctx.matchIndex]

	rule := &ctltype.InternalRule{}
	rule.Source = &albv1.Source{
		Type:      modules.TypeHttpRoute,
		Namespace: ctx.httpRoute.Namespace,
		Name:      ctx.httpRoute.Name,
	}
	rule.Type = ctltype.RuleTypeGateway
	rule.RuleID = genRuleIdViaCtx(ctx)
	rule.Priority = h.getRulePriority(ctx)
	hostnames := JoinHostnames((*string)(ctx.listener.Hostname), lo.Map(route.Spec.Hostnames, func(h gv1.Hostname, _ int) string {
		return string(h)
	}))
	// gen rule dsl
	dslx, err := HttpRuleMatchToDSLX(hostnames, match)
	if err != nil {
		return nil, err
	}
	rule.DSLX = dslx

	rule.BackendProtocol = "$http_backend_protocol"
	svcs, err := nu.BackendRefsToService(pickHttpBackendRefs(gRule.BackendRefs))
	if err != nil {
		return nil, err
	}
	rule.Services = svcs
	return rule, nil
}

// httproute attach to https listener
type HttpsCtx struct {
	HttpCtx
	cert       client.ObjectKey
	certDomain string
}

// http route attach to https listener
// https listener hostname is the hostname of it certref, a route who attach to https listener, it's hostname must contains listener's hostname.
func (h *HttpProtocolTranslate) translateHttps(lss []*ngxtype.Listener, ftMap ngxtype.FtMap) error {
	log := h.log.WithName("https")

	portMap := make(map[int][]HttpsCtx)
	{
		plsMap := GroupListenerByProtocol(lss, gv1.HTTPSProtocolType)
		for port, lss := range plsMap {
			ctxList := IterHttpListener(lss, func(ctx HttpCtx) *HttpsCtx {
				cert, domain, err := getCert(ctx.listener)
				if err != nil {
					log.Error(err, "get cert from https listener fail")
					return nil
				}
				return &HttpsCtx{
					ctx,
					*cert,
					*domain,
				}
			})
			portMap[port] = ctxList
		}
	}

	for port, ctxList := range portMap {
		ft := &ctltype.Frontend{}
		ft.Port = albv1.PortNumber(port)
		ft.Protocol = albv1.FtProtocolHTTPS
		rules := []*ctltype.InternalRule{}
		for _, ctx := range ctxList {
			rule, err := h.generateHttpsRule(ctx)
			if err != nil {
				log.Error(err, "generate http rule failed")
				continue
			}
			err = h.applyPolicyAttachmentOnRule(ft, ctx.ToAttachRef(), rule)
			// NOTE: if attach is failed, we just log and ignore.
			if err != nil {
				log.Error(err, "attach policy fail")
			}
			err = h.applyHttpFilterOnRule(ctx.HttpCtx, rule, ctx.rule.Filters)
			if err != nil {
				log.Error(err, "apply filter fail")
			}

			rules = append(rules, rule)
		}
		ft.Rules = append(ft.Rules, rules...)

		if len(ft.Rules) == 0 {
			log.V(2).Info("could not find rule. ignore this port", "port", port)
			continue
		}
		ftMap.SetFt(string(ft.Protocol), ft.Port, ft)
	}
	return nil
}

func (h *HttpProtocolTranslate) generateHttpsRule(ctx HttpsCtx) (*ctltype.InternalRule, error) {
	route := ctx.httpRoute
	gRule := route.Spec.Rules[ctx.ruleIndex]
	match := gRule.Matches[ctx.matchIndex]

	rule := &ctltype.InternalRule{}
	rule.Source = &albv1.Source{
		Type:      modules.TypeHttpRoute,
		Namespace: ctx.httpRoute.Namespace,
		Name:      ctx.httpRoute.Name,
	}
	rule.Type = ctltype.RuleTypeGateway
	// gen rule id
	rule.RuleID = genRuleIdViaCtx(ctx.HttpCtx)
	rule.Domain = ctx.certDomain
	rule.CertificateName = fmt.Sprintf("%s/%s", ctx.cert.Namespace, ctx.cert.Name)

	hostnames := JoinHostnames((*string)(ctx.listener.Hostname), lo.Map(route.Spec.Hostnames, func(h gv1.Hostname, _ int) string { return string(h) }))
	// gen rule dsl
	dslx, err := HttpRuleMatchToDSLX(hostnames, match)
	if err != nil {
		return nil, err
	}
	rule.DSLX = dslx

	rule.BackendProtocol = "$http_backend_protocol"
	svcs, err := nu.BackendRefsToService(pickHttpBackendRefs(gRule.BackendRefs))
	if err != nil {
		return nil, err
	}
	rule.Services = svcs
	return rule, nil
}

func HttpRuleMatchToDSLX(hostnameStrs []string, m gv1.HTTPRouteMatch) (albv1.DSLX, error) {
	dslx := albv1.DSLX{}

	// match hostname
	// generic domain name
	if len(hostnameStrs) == 1 && strings.HasPrefix(hostnameStrs[0], "*.") {
		exp := albv1.DSLXTerm{Type: utils.KEY_HOST, Values: [][]string{
			{utils.OP_ENDS_WITH, hostnameStrs[0]},
		}}
		dslx = append(dslx, exp)
	} else if len(hostnameStrs) != 0 {
		vals := []string{utils.OP_IN}
		vals = append(vals, hostnameStrs...)
		exp := albv1.DSLXTerm{Type: utils.KEY_HOST, Values: [][]string{
			vals,
		}}
		dslx = append(dslx, exp)
	}

	// match query
	if m.QueryParams != nil {
		for _, q := range m.QueryParams {
			op, err := nu.ToOP((*string)(q.Type))
			if err != nil {
				return nil, fmt.Errorf("invalid query params match err %v", err)
			}
			exp := albv1.DSLXTerm{Type: utils.KEY_PARAM, Values: [][]string{{op, q.Value}}, Key: string(q.Name)}
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
		op, err := nu.ToOP((*string)(path.Type))
		if err != nil {
			return nil, fmt.Errorf("invalid path match err %v", err)
		}
		exp := albv1.DSLXTerm{Type: utils.KEY_URL, Values: [][]string{{op, matchValue}}}
		dslx = append(dslx, exp)
	}

	// match headers
	if m.Headers != nil {
		for _, h := range m.Headers {
			op, err := nu.ToOP((*string)(h.Type))
			if err != nil {
				return nil, fmt.Errorf("invalid header match err %v", err)
			}
			exp := albv1.DSLXTerm{Type: utils.KEY_HEADER, Values: [][]string{{op, h.Value}}, Key: string(h.Name)}
			dslx = append(dslx, exp)
		}
	}

	if m.Method != nil {
		exp := albv1.DSLXTerm{Type: utils.KEY_METHOD, Values: [][]string{{utils.OP_EQ, string(*m.Method)}}}
		dslx = append(dslx, exp)
	}

	return dslx, nil
}

func genRuleIdViaCtx(ctx HttpCtx) string {
	gateway := ctx.listener.Gateway
	route := ctx.httpRoute
	return fmt.Sprintf("%d-%s-%s-%s-%s-%s-%d-%d",
		ctx.listener.Port,
		gateway.Namespace,
		gateway.Name,
		ctx.listener.Name,
		route.Namespace,
		route.Name,
		ctx.ruleIndex,
		ctx.matchIndex,
	)
}

func (h *HttpProtocolTranslate) getRulePriority(ctx HttpCtx) int {
	FMT_ALB_GATEWAY_HTTP_ROUTER_RULE_PRIORITY := "alb.%s/gateway-http-router-rule-priority-%d-%d"
	key := fmt.Sprintf(FMT_ALB_GATEWAY_HTTP_ROUTER_RULE_PRIORITY, h.cfg.GetDomain(), ctx.ruleIndex, ctx.matchIndex)
	if ctx.httpRoute.Annotations != nil && ctx.httpRoute.Annotations[key] != "" {
		priority, err := strconv.Atoi(ctx.httpRoute.Annotations[key])
		if err != nil {
			return priority
		}
	}
	return 0
}

func JoinHostnames(listenerHostname *string, routeHostnames []string) []string {
	if listenerHostname == nil {
		return routeHostnames
	}
	if len(routeHostnames) == 0 {
		return []string{*listenerHostname}
	}
	host := u.FindIntersection(*listenerHostname, routeHostnames)
	if len(host) == 0 {
		return routeHostnames
	}
	return host
}
