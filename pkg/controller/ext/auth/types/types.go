package types

type VarString []string // a string "$host hello $uri$http_id $arg_id" => []string{"$host"," hello ", "$uri", "$http_id"," ","$arg_id"}

type AuthIngress struct {
	Enable string `annotation:"auth-enable" key:"enable" default:"true"` // for feature: no-auth-locations
	AuthIngressForward
	AuthIngressBasic
}

// ingress annotation 对应的结构
type AuthIngressForward struct {
	Url                 string `annotation:"auth-url" key:"url"`
	Method              string `annotation:"auth-method" default:"GET" key:"method"`
	ProxySetHeaders     string `annotation:"auth-proxy-set-headers" key:"proxy_set_headers"` //  the name of a ConfigMap that specifies headers to pass to the authentication service
	RequestRedirect     string `annotation:"auth-request-redirect" key:"request_redirect"`   //  to specify the X-Auth-Request-Redirect header value.
	ResponseHeaders     string `annotation:"auth-response-headers" key:"response_headers"`   //  <Response_Header_1, ..., Response_Header_n> to specify headers to pass to backend once authentication request completes.
	Signin              string `annotation:"auth-signin" key:"signin"`
	AlwaysSetCookie     string `annotation:"auth-always-set-cookie" default:"false" key:"always_set_cookie"`
	SigninRedirectParam string `annotation:"auth-signin-redirect-param" key:"signin_redirect_param"`
	// cacheDuration       string            `annotation:"auth-cache-duration"`
	// cacheKey            string            `annotation:"auth-cache-key"`
}

type AuthIngressBasic struct {
	Realm      string `annotation:"auth-realm" key:"realm"`
	Secret     string `annotation:"auth-secret" key:"secret"`
	SecretType string `annotation:"auth-secret-type" key:"secret_type"`
	AuthType   string `annotation:"auth-type" key:"auth_type"`
}

// auth via forward request
// +k8s:deepcopy-gen=true
type ForwardAuthInCr struct {
	// +optional
	Url string `json:"url,omitempty" key:"url"`

	// +optional
	// +kubebuilder:default="GET"
	Method string `json:"method,omitempty" key:"method"`
	// +optional
	AuthHeadersCmRef string `json:"auth_headers_cm_ref,omitempty" key:"proxy_set_headers"`
	// +optional
	AuthRequestRedirect string `json:"auth_request_redirect,omitempty" key:"request_redirect"`
	// +optional
	UpstreamHeaders []string `json:"upstream_headers,omitempty" key:"response_headers" trans:"resolve_response_headers"`
	// +optional
	Signin string `json:"signin,omitempty" key:"signin"`
	// +optional
	// +kubebuilder:default=false
	AlwaysSetCookie bool `json:"always_set_cookie,omitempty" key:"always_set_cookie" trans:"from_bool"`
	// +optional
	SigninRedirectParam string `json:"signin_redirect_param,omitempty" key:"signin_redirect_param"`
}

// +k8s:deepcopy-gen=true
type AuthCr struct {
	Forward *ForwardAuthInCr `json:"forward,omitempty"`
	Basic   *BasicAuthInCr   `json:"basic,omitempty"`
}

// +k8s:deepcopy-gen=true
type BasicAuthInCr struct {
	// +optional
	Realm string `json:"realm" key:"realm"`
	// +optional
	Secret string `json:"secret" key:"secret"`
	// auth-file|auth-map
	// +optional
	SecretType string `json:"secret_type" key:"secret_type"`

	// only support basic now
	// +optional
	AuthType string `json:"auth_type" key:"auth_type"`
}

type AuthPolicy struct {
	Forward *ForwardAuthPolicy `json:"forward_auth,omitempty"`
	Basic   *BasicAuthPolicy   `json:"basic_auth,omitempty"`
}

// 我们的lua能接受的auth配置
type ForwardAuthPolicy struct {
	Url                 VarString            `json:"url" key:"url" trans:"resolve_varstring"`
	Method              string               `json:"method" key:"method"`
	AuthHeaders         map[string]VarString `json:"auth_headers" key:"proxy_set_headers" trans:"resolve_proxy_set_headers"`
	InvalidAuthReqCmRef bool                 `json:"invalid_auth_req_cm_ref"`
	AuthRequestRedirect VarString            `json:"auth_request_redirect" key:"request_redirect" trans:"resolve_varstring"`
	UpstreamHeaders     []string             `json:"upstream_headers" key:"response_headers"`
	AlwaysSetCookie     bool                 `json:"always_set_cookie" key:"always_set_cookie"`
	SigninUrl           VarString            `json:"signin_url"` // resolved via ourself
}
type BasicAuthHash struct {
	Name      string `json:"name"`
	Algorithm string `json:"algorithm"`
	Salt      string `json:"salt"`
	Hash      string `json:"hash"`
}

type BasicAuthPolicy struct {
	Realm    string                   `json:"realm" key:"realm"`
	Secret   map[string]BasicAuthHash `json:"secret"`
	AuthType string                   `json:"auth_type" key:"auth_type"`
	Err      string                   `json:"err"` // if rule or ingress are invalid this route should report error to user
}
