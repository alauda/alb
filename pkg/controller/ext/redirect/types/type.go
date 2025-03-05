package types

type RedirectIngress struct {
	PermanentRedirect     string `annotation:"permanent-redirect" key:"url"`
	PermanentRedirectCode string `annotation:"permanent-redirect-code" key:"code"`
	TemporalRedirect      string `annotation:"temporal-redirect" key:"url"`
	TemporalRedirectCode  string `annotation:"temporal-redirect-code" key:"code"`
	SSLRedirect           string `annotation:"ssl-redirect"`       // handle_scheme
	ForceSSLRedirect      string `annotation:"force-ssl-redirect"` // handle_scheme
	Code                  string `annotation:"redirect-code" key:"code"`
	Host                  string `annotation:"redirect-host" key:"host"`
	Port                  string `annotation:"redirect-port" key:"port"`
	PrefixMatch           string `annotation:"redirect-prefix-match" key:"prefix_match"`
	ReplacePrefix         string `annotation:"redirect-replace-prefix" key:"replace_prefix"`
}

// +k8s:deepcopy-gen=true
type RedirectCr struct {
	Code          *int   `json:"code,omitempty" key:"code" trans:"string_to_int"`
	URL           string `json:"url,omitempty"`
	Scheme        string `json:"scheme,omitempty"`
	Host          string `json:"host,omitempty" key:"host"`
	Port          *int   `json:"port,omitempty" key:"port" trans:"string_to_int"`
	PrefixMatch   string `json:"prefix_match,omitempty" key:"prefix_match"`
	ReplacePrefix string `json:"replace_prefix,omitempty" key:"replace_prefix"`
}

// 完整的约束是
// code 可以为空 默认301
// url scheme host port prefix_match replace_prefix 不能同时为空
// prefix_match 和 replace_prefix 不能同时为空
// scheme 只能是http 或者https?
// port 只能是合法端口

func (t *RedirectCr) OnlySslRedirect(port int) bool {
	same_port := t.Port == nil || *t.Port == port
	return t.URL == "" && t.Scheme == "https" && same_port && t.Host == "" && t.PrefixMatch == "" && t.ReplacePrefix == ""
}

func (t *RedirectCr) Empty() bool {
	return t.URL == "" && t.Scheme == "" && t.Host == "" && t.Port == nil && t.PrefixMatch == "" && t.ReplacePrefix == ""
}
