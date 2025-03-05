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
	policy.Source = Source{}
	policy.InternalDSL = internalDSL

	// sort-bean
	policy.InternalDSLLen = utils.InternalDSLLen(internalDSL)
	policy.Priority = rule.Priority
	policy.ComplexPriority = rule.DSLX.Priority()
	if rule.BackendGroup != nil {
		policy.Upstream = rule.BackendGroup.Name // IMPORTANT
	} else {
		policy.Upstream = rule.RuleID
	}
	policy.BackendProtocol = rule.BackendProtocol
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
		policy.Config.Refs = rule.Config.Source // source init in RuleToInternalRule
		ngxPolicy.Http.Tcp[ft.Port] = append(ngxPolicy.Http.Tcp[ft.Port], policy)
	}

	extension_need, name := p.cus.NeedL7DefaultPolicy(ft)
	ft_default_router := ft.BackendGroup != nil && ft.BackendGroup.Backends != nil

	// set default rule if ft have default backend.
	if extension_need || ft_default_router {
		defaultPolicy := Policy{}
		defaultPolicy.Source = Source{}
		defaultPolicy.Config.Refs = map[PolicyExtKind]string{}
		defaultPolicy.Rule = ft.FtName
		// 我们先处理有默认路由的情况,这样extension就可以对其进行感知
		if ft_default_router {
			defaultPolicy.MakeItMatchLast()
			defaultPolicy.Source = Source{
				SourceType: m.TypeFtDefaultRouter,
				SourceName: ft.FtName,
				SourceNs:   "",
			}
			defaultPolicy.BackendProtocol = ft.BackendProtocol
			defaultPolicy.Upstream = ft.BackendGroup.Name // 因为在pick backend的时候，我们会对检查ft的backend group 所以这里可以使用
		} else if extension_need {
			defaultPolicy.Source = Source{
				SourceType: m.TypeExtension,
				SourceName: ft.FtName + "-" + name, // 为了在policy中提示我们这个默认policy是那个extension生成的.
				SourceNs:   "",
			}
		}
		defaultPolicy.Subsystem = SubsystemHTTP
		defaultPolicy.InternalDSL = []interface{}{[]string{"STARTS_WITH", "URL", "/"}} // [[START_WITH URL /]]
		// 让extension进行自己的配置
		p.cus.InitL7DefaultPolicy(ft, &defaultPolicy)
		ngxPolicy.Http.Tcp[ft.Port] = append(ngxPolicy.Http.Tcp[ft.Port], &defaultPolicy)
	}
	sort.Sort(ngxPolicy.Http.Tcp[ft.Port]) // IMPORTANT sort to make sure priority work.
}

func initPolicySource(p *Policy, rule *InternalRule) {
	if rule.Source == nil {
		return
	}
	p.Source = Source{
		SourceType: rule.Source.Type,
		SourceName: rule.Source.Name,
		SourceNs:   rule.Source.Namespace,
	}
}
