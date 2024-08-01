package types

import (
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	otelt "alauda.io/alb2/pkg/controller/ext/otel/types"
)

// keep it as same as rule
type Policy struct {
	InternalDSL     []interface{} `json:"internal_dsl"` // dsl determine whether a request match this rule, same as rule.spec.dlsx
	InternalDSLLen  int           `json:"-"`            // the len of jsonstringify internal dsl, used to sort policy
	Upstream        string        `json:"upstream"`     // name in backend group
	ComplexPriority int           `json:"-"`            // priority calculated by the complex of dslx, used to sort policy after user_priority
	Subsystem       string        `json:"-"`

	Rule   string              `json:"rule"` // the name of rule, corresponding with k8s rule cr
	Config *RuleConfigInPolicy `json:"config,omitempty"`

	SameInRuleCr
	SameInPolicy
	SourceType string `json:"source_type,omitempty"`
	SourceName string `json:"source_name,omitempty"`
	SourceNs   string `json:"source_ns,omitempty"`
}

func (p Policy) GetOtel() *otelt.OtelConf {
	if p.Config == nil || p.Config.Otel == nil || p.Config.Otel.Otel == nil {
		return nil
	}
	return p.Config.Otel.Otel
}

type SameInRuleCr struct {
	Priority         int           `json:"-"` // priority set by user, used to sort policy which is rule's priority
	DSLX             albv1.DSLX    `json:"-"`
	URL              string        `json:"url"`
	RewriteBase      string        `json:"rewrite_base"`
	RewriteTarget    string        `json:"rewrite_target"`
	EnableCORS       bool          `json:"enable_cors"`
	CORSAllowHeaders string        `json:"cors_allow_headers"`
	CORSAllowOrigin  string        `json:"cors_allow_origin"`
	BackendProtocol  string        `json:"backend_protocol"`
	RedirectURL      string        `json:"redirect_url"`
	VHost            string        `json:"vhost"`
	RedirectCode     int           `json:"redirect_code"`
	Source           *albv1.Source `json:"source,omitempty"`
}

type SameInPolicy struct {
	RewritePrefixMatch    *string `json:"rewrite_prefix_match,omitempty"`
	RewriteReplacePrefix  *string `json:"rewrite_replace_prefix,omitempty"`
	RedirectScheme        *string `json:"redirect_scheme,omitempty"`
	RedirectHost          *string `json:"redirect_host,omitempty"`
	RedirectPort          *int    `json:"redirect_port,omitempty"`
	RedirectPrefixMatch   *string `json:"redirect_prefix_match,omitempty"`
	RedirectReplacePrefix *string `json:"redirect_replace_prefix,omitempty"`
}

type NgxPolicy struct {
	CertificateMap map[string]Certificate `json:"certificate_map"`
	Http           HttpPolicy             `json:"http"`
	Stream         StreamPolicy           `json:"stream"`
	CommonConfig   CommonPolicyConfig     `json:"config"`
	BackendGroup   []*BackendGroup        `json:"backend_group"`
}

func (p *NgxPolicy) GetBackendGroup(name string) *BackendGroup {
	for _, be := range p.BackendGroup {
		if be.Name == name {
			return be
		}
	}
	return nil
}

type HttpPolicy struct {
	Tcp map[albv1.PortNumber]Policies `json:"tcp"`
}

func (p *HttpPolicy) GetPoliciesByPort(port int) Policies {
	return p.Tcp[albv1.PortNumber(port)]
}

type CommonPolicyConfig map[string]CommonPolicyConfigVal

type CommonPolicyConfigVal struct {
	Type string              `json:"type"`
	Otel *otelt.OtelInCommon `json:"otel,omitempty"`
}

type StreamPolicy struct {
	Tcp map[albv1.PortNumber]Policies `json:"tcp"`
	Udp map[albv1.PortNumber]Policies `json:"udp"`
}

type Policies []*Policy

func (p Policies) Len() int { return len(p) }

func (p Policies) Less(i, j int) bool {
	// raw priority is set by user it should be [1,10]. the smaller the number, the higher the ranking
	if p[i].Priority != p[j].Priority {
		return p[i].Priority < p[j].Priority
	}
	// priority is calculated by the "complex" of this policy. the bigger the number, the higher the ranking
	if p[i].ComplexPriority != p[j].ComplexPriority {
		return p[i].ComplexPriority > p[j].ComplexPriority
	}
	if p[i].InternalDSLLen != p[j].InternalDSLLen {
		return p[i].InternalDSLLen > p[j].InternalDSLLen
	}
	return p[i].Rule < p[j].Rule
}

func (p Policies) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
