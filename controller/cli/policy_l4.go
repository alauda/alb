package cli

import (
	. "alauda.io/alb2/controller/types"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"k8s.io/klog/v2"
)

func (p *PolicyCli) initStreamModeFt(ft *Frontend, ngxPolicy *NgxPolicy) {
	// create a default rule for stream mode ft.
	if len(ft.Rules) == 0 {
		if ft.BackendGroup == nil || ft.BackendGroup.Backends == nil {
			klog.Warningf("ft %s,stream mode ft must have backend group", ft.FtName)
		}
		if ft.Protocol == albv1.FtProtocolTCP {
			policy := Policy{}
			policy.Subsystem = SubsystemStream
			policy.Upstream = ft.BackendGroup.Name
			policy.Rule = ft.BackendGroup.Name
			ngxPolicy.Stream.Tcp[ft.Port] = append(ngxPolicy.Stream.Tcp[ft.Port], &policy)
		}
		if ft.Protocol == albv1.FtProtocolUDP {
			policy := Policy{}
			policy.Subsystem = SubsystemStream
			policy.Upstream = ft.BackendGroup.Name
			policy.Rule = ft.BackendGroup.Name
			ngxPolicy.Stream.Udp[ft.Port] = append(ngxPolicy.Stream.Udp[ft.Port], &policy)
		}
		return
	}

	if len(ft.Rules) != 1 {
		klog.Warningf("stream mode ft could only have one rule", ft.FtName)
	}
	rule := ft.Rules[0]
	policy := Policy{}
	policy.Subsystem = SubsystemStream
	policy.Upstream = rule.BackendGroup.Name
	policy.Rule = rule.RuleID
	if ft.Protocol == albv1.FtProtocolTCP {
		ngxPolicy.Stream.Tcp[ft.Port] = append(ngxPolicy.Stream.Tcp[ft.Port], &policy)
	}
	if ft.Protocol == albv1.FtProtocolUDP {
		ngxPolicy.Stream.Udp[ft.Port] = append(ngxPolicy.Stream.Udp[ft.Port], &policy)
	}
}
