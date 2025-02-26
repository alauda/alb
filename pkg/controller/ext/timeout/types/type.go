package types

type TimeoutIngress struct {
	ProxyConnectTimeoutMs string `annotation:"proxy-connect-timeout" key:"proxy_connect_timeout_ms"`
	ProxySendTimeoutMs    string `annotation:"proxy-send-timeout" key:"proxy_send_timeout_ms"`
	ProxyReadTimeoutMs    string `annotation:"proxy-read-timeout" key:"proxy_read_timeout_ms"`
}

// k8s does not like float type, so we use uint instead

// +k8s:deepcopy-gen=true
type TimeoutCr struct {
	ProxyConnectTimeoutMs *uint `json:"proxy_connect_timeout_ms,omitempty" key:"proxy_connect_timeout_ms" trans:"time_from_string"`
	ProxySendTimeoutMs    *uint `json:"proxy_send_timeout_ms,omitempty" key:"proxy_send_timeout_ms" trans:"time_from_string"`
	ProxyReadTimeoutMs    *uint `json:"proxy_read_timeout_ms,omitempty" key:"proxy_read_timeout_ms" trans:"time_from_string"`
}
