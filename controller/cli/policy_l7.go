package cli

import (
	"fmt"
	"sort"

	m "alauda.io/alb2/controller/modules"
	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/utils"
)

func (p *PolicyCli) InternalRuleToL7Policy(rule *InternalRule, refs RefMap) (*Policy, error) {
	if rule.DSLX == nil {
		return nil, fmt.Errorf("rule without matcher")
	}
	// translate our rule struct to policy (the json file)
	internalDSL, err := utils.DSLX2Internal(rule.DSLX)
	if err != nil {
		return nil, fmt.Errorf("dslx to interval fail %v", err)
	}

	policy := Policy{}
	policy.InternalDSL = internalDSL

	// sort-bean
	policy.InternalDSLLen = utils.InternalDSLLen(internalDSL)
	policy.Priority = rule.Priority
	policy.ComplexPriority = rule.DSLX.Priority()

	policy.Upstream = rule.RuleUpstream.BackendGroup.Name // IMPORTANT
	policy.BackendProtocol = rule.RuleUpstream.BackendProtocol
	policy.Config.Refs = map[PolicyExtKind]string{}
	policy.Rule = rule.RuleID
	initPolicySource(&policy, rule)
	p.initLegacyCfg(&policy, rule)
	p.initPolicyExt(&policy, rule, refs)
	return &policy, nil
}

func (c *PolicyCli) initLegacyCfg(p *Policy, ir *InternalRule) {
	if ir.Config.Rewrite != nil {
		p.RewriteConf = *ir.Config.Rewrite
	}
	if ir.Config.Redirect != nil {
		p.RedirectConf = *ir.Config.Redirect
	}
	if ir.Config.Cors != nil {
		p.Cors = *ir.Config.Cors
	}
	if ir.Config.Vhost != nil {
		p.Vhost = *ir.Config.Vhost
	}
}

func (c *PolicyCli) initPolicyExt(p *Policy, ir *InternalRule, refs RefMap) {
	c.cus.ToPolicy(ir, p, refs)
}

func (p *PolicyCli) initHttpModeFt(ft *Frontend, ngxPolicy *NgxPolicy, refs RefMap) {
	if _, ok := ngxPolicy.Http.Tcp[ft.Port]; !ok {
		ngxPolicy.Http.Tcp[ft.Port] = Policies{}
	}

	for _, rule := range ft.Rules {
		policy, err := p.InternalRuleToL7Policy(rule, refs)
		if err != nil {
			p.log.Error(err, "to policy fail, skip this rule", "rule", rule.RuleID)
			continue
		}
		ngxPolicy.Http.Tcp[ft.Port] = append(ngxPolicy.Http.Tcp[ft.Port], policy)
	}

	// set default rule if ft have default backend.
	if ft.BackendGroup != nil && ft.BackendGroup.Backends != nil {
		defaultPolicy := Policy{}
		defaultPolicy.Rule = ft.FtName
		defaultPolicy.ComplexPriority = -1 // default rule should have the minimum priority
		defaultPolicy.Priority = 999       // minimum number means higher priority
		defaultPolicy.Subsystem = SubsystemHTTP
		defaultPolicy.InternalDSL = []interface{}{[]string{"STARTS_WITH", "URL", "/"}} // [[START_WITH URL /]]
		defaultPolicy.BackendProtocol = ft.BackendProtocol
		defaultPolicy.Upstream = ft.BackendGroup.Name

		ngxPolicy.Http.Tcp[ft.Port] = append(ngxPolicy.Http.Tcp[ft.Port], &defaultPolicy)
	}
	sort.Sort(ngxPolicy.Http.Tcp[ft.Port]) // IMPORTANT sort to make sure priority work.
}

func initPolicySource(p *Policy, rule *InternalRule) {
	if rule.Source == nil || rule.Source.Type != m.TypeIngress {
		return
	}
	p.SourceType = m.TypeIngress
	p.SourceName = rule.Source.Name
	p.SourceNs = rule.Source.Namespace
}
