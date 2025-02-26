package types

import "fmt"

// +k8s:deepcopy-gen=true
type KeepAliveCr struct {
	TCP *TCPKeepAlive `json:"tcp,omitempty"`
	// downstream l7 keepalive
	HTTP *HTTPKeepAlive `json:"http,omitempty"`
}

// TCPKeepAlive defines TCP keepalive parameters (so_keepalive)
type TCPKeepAlive struct {
	// the TCP_KEEPIDLE socket option
	Idle string `json:"idle,omitempty"` // seconds
	// the TCP_KEEPINTVL socket option
	Interval string `json:"interval,omitempty"` // seconds
	// the TCP_KEEPCNT socket option
	Count int `json:"count,omitempty"` // count
}

func (k *TCPKeepAlive) ToNginxConf() string {
	if k.Idle == "" && k.Interval == "" && k.Count == 0 {
		return "so_keepalive=on"
	}
	idle := str_empty_or(k.Idle, "30m")
	interval := str_empty_or(k.Interval, "")
	count := int_empty_or(k.Count, 10)
	return fmt.Sprintf("so_keepalive=%s:%s:%d", idle, interval, count)
}

// HTTPKeepAlive defines HTTP keepalive parameters
type HTTPKeepAlive struct {
	// keepalive_timeout. default is 75s
	Timeout string `json:"timeout,omitempty"`
	// keepalive_header_timeout. default not set
	HeaderTimeout string `json:"header_timeout,omitempty"`
	// keepalive_requests. default is 1000
	Requests int `json:"requests,omitempty"`
}

func (h *HTTPKeepAlive) ToNginxConf() string {
	timeout := str_empty_or(h.Timeout, "75s") // 不允许设置空字符串
	header_timeout := str_empty_or(h.HeaderTimeout, "")
	requests := int_empty_or(h.Requests, 1000) // 不允许设置0
	// nginx 自己会处理header-timeout为空的场景.
	return fmt.Sprintf("\nkeepalive_timeout %s %s;\n keepalive_requests %d;", timeout, header_timeout, requests)
}

func str_empty_or(s string, default_value string) string {
	if s == "" {
		return default_value
	}
	return s
}

func int_empty_or(i int, default_value int) int {
	if i == 0 {
		return default_value
	}
	return i
}
