package types

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

type WafInternal struct {
	Key     string  // 要跳转到的 nginx location
	Raw     WafConf // 其他的waf的配置
	Snippet string  // nginx location 的配置
}
