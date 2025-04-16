package types

const (
	USE_FORWARDED_HEADERS_ANNOTATION = "use-forwarded-headers"
)

// +k8s:deepcopy-gen=true
type ProxyHeader struct {
	UseForwardedHeaders bool `json:"use_forwarded_headers"`
}
