package waf_type

// +k8s:deepcopy-gen=true
type WafCrConf struct {
	Enable bool `json:"enable"`
	// +optional
	WafConf `json:",inline"`
}

// +k8s:deepcopy-gen=true
type WafConf struct {
	// +optional
	TransactionId string `json:"transactionId"` // could be ""
	// +optional
	UseCoreRules bool `json:"useCoreRules"`
	// +optional
	UseRecommend bool `json:"useRecommend"`
	// +optional
	CmRef string `json:"cmRef"` // ns/name#section
}

type WafInRule struct {
	Key     string
	Raw     WafConf
	Snippet string
}
