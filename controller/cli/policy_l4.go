package cli

import (
	. "alauda.io/alb2/controller/types"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
)

func (p *PolicyCli) initStreamModeFt(ft *Frontend, ngxPolicy *NgxPolicy) {
	// create a default rule for stream mode ft.
	policy := Policy{}
	policy.Subsystem = SubsystemStream
	// @ft_default_policy
	upstream, rule := getName(ft)
	policy.Upstream = upstream
	policy.Rule = rule
	p.cus.InitL4DefaultPolicy(ft, &policy)

	if ft.Protocol == albv1.FtProtocolTCP {
		ngxPolicy.Stream.Tcp[ft.Port] = append(ngxPolicy.Stream.Tcp[ft.Port], &policy)
	}
	if ft.Protocol == albv1.FtProtocolUDP {
		ngxPolicy.Stream.Udp[ft.Port] = append(ngxPolicy.Stream.Udp[ft.Port], &policy)
	}
}

// gateway-api中就是ft没有默认backend-group 但是有rule
func getName(ft *Frontend) (upstream string, rule string) {
	if len(ft.Rules) > 0 {
		rule := ft.Rules[0]
		if rule.BackendGroup != nil {
			return rule.BackendGroup.Name, rule.RuleID
		}
		// 可能是redirect规则。用rule id即可。在没有backend-group的情况下，upstream name 是什么都行
		return rule.RuleID, rule.RuleID
	}
	if ft.BackendGroup != nil {
		return ft.BackendGroup.Name, ft.BackendGroup.Name
	}
	return "", ""
}
