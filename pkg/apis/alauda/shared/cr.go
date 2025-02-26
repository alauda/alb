package shared

import (
	auth_t "alauda.io/alb2/pkg/controller/ext/auth/types"
	otelt "alauda.io/alb2/pkg/controller/ext/otel/types"
	timeout_t "alauda.io/alb2/pkg/controller/ext/timeout/types"
	waft "alauda.io/alb2/pkg/controller/ext/waf/types"
)

// struct shared in alb/ft/rule
// +k8s:deepcopy-gen=true
type SharedCr struct {
	Otel         *otelt.OtelCrConf    `json:"otel,omitempty"`
	ModeSecurity *waft.WafCrConf      `json:"modsecurity,omitempty"`
	Auth         *auth_t.AuthCr       `json:"auth,omitempty"`
	Timeout      *timeout_t.TimeoutCr `json:"timeout,omitempty"`
}
