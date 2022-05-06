package nginx

import (
	"fmt"

	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	. "alauda.io/alb2/gateway"
	gatewayPolicyType "alauda.io/alb2/gateway/nginx/policyattachment/types"
	. "alauda.io/alb2/gateway/nginx/types"
	. "alauda.io/alb2/gateway/nginx/utils"
	"github.com/go-logr/logr"

	albType "alauda.io/alb2/pkg/apis/alauda/v1"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
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

func (t *TcpProtocolTranslate) TransLate(ls []*Listener, ftMap FtMap) error {
	// tcp listener could never be overlaped. each listener is a rule.
	for _, l := range ls {
		port := l.Port
		var route CommonRoute
		var tcproute *TCPRoute
		// filter invalid listener
		{
			if l.Protocol != gatewayType.TCPProtocolType {
				continue
			}
			if len(l.Routes) == 0 {
				t.log.Info("could not found vaild route", "error", true)
				return nil
			}
			if len(l.Routes) > 1 {
				t.log.Info("tcp has more than one route", "port", port)
				return nil
			}
			route = l.Routes[0]
			tcprouteNew, ok := route.(*TCPRoute)
			if !ok {
				t.log.Info("only tcp route could attach to tcp listener")
				return nil
			}
			tcproute = tcprouteNew
			if len(tcproute.Spec.Rules) != 1 {
				t.log.Error(fmt.Errorf("we do not support multiple tcp rules"), "port", port, "route", GetObjectKey(tcproute))
				return nil
			}
		}

		ft := &Frontend{
			Port:     int(port),
			Protocol: albType.FtProtocolTCP,
		}
		// TODO we donot support multiple tcp rules
		svcs, err := BackendRefsToService(tcproute.Spec.Rules[0].BackendRefs)
		if err != nil {
			return nil
		}
		name := fmt.Sprintf("%v-%v-%v", port, tcproute.Namespace, tcproute.Name)
		backendGroup := &BackendGroup{
			Name: name,
		}
		rule := Rule{}
		rule.Type = RuleTypeGateway
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
