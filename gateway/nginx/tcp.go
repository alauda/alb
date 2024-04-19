package nginx

import (
	"fmt"

	"alauda.io/alb2/controller/modules"
	ctltype "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	"alauda.io/alb2/gateway"
	gatewayPolicyType "alauda.io/alb2/gateway/nginx/policyattachment/types"
	ngxtype "alauda.io/alb2/gateway/nginx/types"
	"alauda.io/alb2/gateway/nginx/utils"
	"github.com/go-logr/logr"

	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type TcpProtocolTranslate struct {
	drv    *driver.KubernetesDriver
	log    logr.Logger
	handle gatewayPolicyType.PolicyAttachmentHandle
}

func NewTcpProtocolTranslate(drv *driver.KubernetesDriver, log logr.Logger) TcpProtocolTranslate {
	return TcpProtocolTranslate{drv: drv, log: log}
}

func (t *TcpProtocolTranslate) SetPolicyAttachmentHandle(handle gatewayPolicyType.PolicyAttachmentHandle) {
	t.handle = handle
}

// TODO add test case: 当 listener 没有 route 时，不应该影响到其他的 listener
func (t *TcpProtocolTranslate) TransLate(ls []*ngxtype.Listener, ftMap ngxtype.FtMap) error {
	// tcp listener could never be overlapped. each listener is a rule.
	for _, l := range ls {
		port := l.Port
		log := t.log.WithValues("listener", l.Name, "gateway", l.Gateway, "port", l.Port)
		var route gateway.CommonRoute
		var tcpRoute *gateway.TCPRoute
		// filter invalid listener
		{
			if l.Protocol != gv1.TCPProtocolType {
				continue
			}
			if len(l.Routes) == 0 {
				log.Info("could not found valid route", "error", true)
				continue
			}
			if len(l.Routes) > 1 {
				log.Info("tcp has more than one route", "port", port)
				continue
			}
			route = l.Routes[0]
			tcpRouteNew, ok := route.(*gateway.TCPRoute)
			if !ok {
				log.Info("only tcp route could attach to tcp listener")
				continue
			}
			tcpRoute = tcpRouteNew
			if len(tcpRoute.Spec.Rules) != 1 {
				log.Info("route rule more than 1")
				continue
			}
		}

		ft := &ctltype.Frontend{
			Port:     albv1.PortNumber(port),
			Protocol: albv1.FtProtocolTCP,
		}
		// TODO we do not support multiple tcp rules
		svcs, err := utils.BackendRefsToService(tcpRoute.Spec.Rules[0].BackendRefs)
		if err != nil {
			return nil
		}
		name := fmt.Sprintf("%v-%v-%v", port, tcpRoute.Namespace, tcpRoute.Name)
		backendGroup := &ctltype.BackendGroup{
			Name: name,
		}
		rule := ctltype.Rule{}
		rule.Type = ctltype.RuleTypeGateway
		rule.Source = &albv1.Source{
			Type:      modules.TypeTCPRoute,
			Namespace: tcpRoute.Namespace,
			Name:      tcpRoute.Name,
		}
		rule.Services = svcs
		rule.RuleID = name
		rule.BackendGroup = backendGroup
		ft.Rules = append(ft.Rules, &rule)
		if t.handle != nil {
			ref := gatewayPolicyType.Ref{
				Listener: &gatewayPolicyType.Listener{
					Listener:   l.Listener,
					Gateway:    l.Gateway,
					Generation: l.Generation,
					CreateTime: l.CreateTime,
				},
				Route:      route,
				RuleIndex:  0,
				MatchIndex: 0,
			}
			t.log.V(3).Info("onrule", "ref", ref.Describe(), "ft", ft.Port, "rule", rule.RuleID)
			err = t.handle.OnRule(ft, &rule, ref)
			if err != nil {
				t.log.Error(err, "onrule fail", "ref", ref.Describe())
			}
		}
		ftMap.SetFt(string(ft.Protocol), ft.Port, ft)
	}
	return nil
}
