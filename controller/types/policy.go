package types

import (
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
)

// keep it as same as rule
type Policy struct {
	Rule                  string        `json:"rule"` // the name of rule, corresponding with k8s rule cr
	Config                *RuleConfig   `json:"config,omitempty"`
	DSL                   albv1.DSLX    `json:"-"`
	InternalDSL           []interface{} `json:"internal_dsl"` // dsl determine whether a request match this rule, same as rule.spec.dlsx
	InternalDSLLen        int           `json:"-"`            // the len of jsonstringify internal dsl, used to sort policy
	Upstream              string        `json:"upstream"`     // name in backend group
	URL                   string        `json:"url"`
	RewriteBase           string        `json:"rewrite_base"`
	RewriteTarget         string        `json:"rewrite_target"`
	RewritePrefixMatch    *string       `json:"rewrite_prefix_match,omitempty"`
	RewriteReplacePrefix  *string       `json:"rewrite_replace_prefix,omitempty"`
	Priority              int           `json:"complexity_priority"` // priority calculated by the complex of dslx, used to sort policy after user_priority
	RawPriority           int           `json:"user_priority"`       // priority set by user, used to sort policy which is rule's priority
	Subsystem             string        `json:"subsystem"`
	EnableCORS            bool          `json:"enable_cors"`
	CORSAllowHeaders      string        `json:"cors_allow_headers"`
	CORSAllowOrigin       string        `json:"cors_allow_origin"`
	BackendProtocol       string        `json:"backend_protocol"`
	RedirectScheme        *string       `json:"redirect_scheme,omitempty"`
	RedirectHost          *string       `json:"redirect_host,omitempty"`
	RedirectPort          *int          `json:"redirect_port,omitempty"`
	RedirectURL           string        `json:"redirect_url"`
	RedirectCode          int           `json:"redirect_code"`
	RedirectPrefixMatch   *string       `json:"redirect_prefix_match,omitempty"`
	RedirectReplacePrefix *string       `json:"redirect_replace_prefix,omitempty"`
	VHost                 string        `json:"vhost"`
	Source                *albv1.Source `json:"source,omitempty"`
}

type NgxPolicy struct {
	CertificateMap map[string]Certificate `json:"certificate_map"`
	Http           HttpPolicy             `json:"http"`
	Stream         StreamPolicy           `json:"stream"`
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

type StreamPolicy struct {
	Tcp map[albv1.PortNumber]Policies `json:"tcp"`
	Udp map[albv1.PortNumber]Policies `json:"udp"`
}

type Policies []*Policy

func (p Policies) Len() int { return len(p) }

func (p Policies) Less(i, j int) bool {
	// raw priority is set by user it should be [1,10]. the smaller the number, the higher the ranking
	if p[i].RawPriority != p[j].RawPriority {
		return p[i].RawPriority < p[j].RawPriority
	}
	// priority is calculated by the "complex" of this policy. the bigger the number, the higher the ranking
	if p[i].Priority != p[j].Priority {
		return p[i].Priority > p[j].Priority
	}
	if p[i].InternalDSLLen != p[j].InternalDSLLen {
		return p[i].InternalDSLLen > p[j].InternalDSLLen
	}
	return p[i].Rule < p[j].Rule
}

func (p Policies) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
