package types

// +k8s:deepcopy-gen=true
type OtelCrConf struct {
	Enable   bool `json:"enable"`
	OtelConf `json:",inline"`
}

// +k8s:deepcopy-gen=true
type OtelConf struct {
	Exporter *Exporter         `json:"exporter,omitempty"`
	Sampler  *Sampler          `json:"sampler,omitempty"`
	Flags    *Flags            `json:"flags,omitempty"`
	Resource map[string]string `json:"resource,omitempty"`
}

func (o *OtelCrConf) Need() bool {
	if o == nil {
		return false
	}
	return o.Enable
}

func (o *OtelConf) HasCollector() bool {
	return !(o.Exporter == nil || o.Exporter.Collector == nil)
}

// +k8s:deepcopy-gen=true
type Flags struct {
	HideUpstreamAttrs        bool `json:"hide_upstream_attrs"`
	ReportHttpRequestHeader  bool `json:"report_http_request_header"`
	ReportHttpResponseHeader bool `json:"report_http_response_header"`
	NoTrustIncomingSpan      bool `json:"notrust_incoming_span"`
}

// +k8s:deepcopy-gen=true
type Exporter struct {
	// +optional
	Collector *Collector `json:"collector,omitempty"`
	// +optional
	BatchSpanProcessor *BatchSpanProcessor `json:"batch_span_processor,omitempty"`
}

// +k8s:deepcopy-gen=true
type Collector struct {
	Address        string `json:"address"`
	RequestTimeout int    `json:"request_timeout"`
}

// -- opts.drop_on_queue_full: if true, drop span when queue is full, otherwise force process batches, default true
// -- opts.max_queue_size: maximum queue size to buffer spans for delayed processing, default 2048
// -- opts.batch_timeout: maximum duration for constructing a batch, default 5s
// -- opts.inactive_timeout: timer interval for processing batches, default 2s
// -- opts.max_export_batch_size: maximum number of spans to process in a single batch, default 256

// +k8s:deepcopy-gen=true
type BatchSpanProcessor struct {
	MaxQueueSize    int `json:"max_queue_size"`
	InactiveTimeout int `json:"inactive_timeout"`
}

// +k8s:deepcopy-gen=true
type Sampler struct {
	Name string `json:"name"` // always_on always_off parent_base trace_id_ratio
	// +optional
	Options *SamplerOptions `json:"options"`
}

// +k8s:deepcopy-gen=true
type SamplerOptions struct {
	// +optional
	ParentName *string `json:"parent_name"` // name of parent if parent_base sampler -- do not omitempty, it should not be overwrite
	// +optional
	Fraction *string `json:"fraction"` // k8s does not like float, so use string -- do not omitempty, it should not be overwrite
}
