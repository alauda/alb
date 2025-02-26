package types

import (
	"fmt"
	"strings"
)

func init() {
	// make go happy
	_ = strings.Clone
	_ = fmt.Sprintf
}

type ReAssignAuthIngressForwardToForwardAuthInCrOpt struct {
	From_bool func(string) (bool, error)

	Resolve_response_headers func(string) ([]string, error)
}

var ReAssignAuthIngressForwardToForwardAuthInCrTrans = map[string]func(lt *AuthIngressForward, rt *ForwardAuthInCr, opt *ReAssignAuthIngressForwardToForwardAuthInCrOpt) error{
	"always_set_cookie": func(lt *AuthIngressForward, rt *ForwardAuthInCr, opt *ReAssignAuthIngressForwardToForwardAuthInCrOpt) error {
		ret := strings.ToLower(lt.AlwaysSetCookie) == "true"
		rt.AlwaysSetCookie = ret
		return nil
	},

	"response_headers": func(lt *AuthIngressForward, rt *ForwardAuthInCr, opt *ReAssignAuthIngressForwardToForwardAuthInCrOpt) error {
		ret, err := opt.Resolve_response_headers(lt.ResponseHeaders)
		if err != nil {
			return err
		}
		rt.UpstreamHeaders = ret
		return nil
	},
}

func ReAssignAuthIngressForwardToForwardAuthInCr(lt *AuthIngressForward, rt *ForwardAuthInCr, opt *ReAssignAuthIngressForwardToForwardAuthInCrOpt) error {
	if lt.Method != "" {
		rt.Method = lt.Method
	}

	if lt.ProxySetHeaders != "" {
		rt.AuthHeadersCmRef = lt.ProxySetHeaders
	}

	if lt.RequestRedirect != "" {
		rt.AuthRequestRedirect = lt.RequestRedirect
	}

	if lt.Signin != "" {
		rt.Signin = lt.Signin
	}

	if lt.SigninRedirectParam != "" {
		rt.SigninRedirectParam = lt.SigninRedirectParam
	}

	if lt.Url != "" {
		rt.Url = lt.Url
	}

	for _, m := range ReAssignAuthIngressForwardToForwardAuthInCrTrans {
		err := m(lt, rt, opt)
		if err != nil {
			return err
		}
	}
	return nil
}

type ReAssignForwardAuthInCrToForwardAuthPolicyOpt struct {
	Resolve_proxy_set_headers func(string) (map[string]VarString, error)

	Resolve_varstring func(string) (VarString, error)
}

var ReAssignForwardAuthInCrToForwardAuthPolicyTrans = map[string]func(lt *ForwardAuthInCr, rt *ForwardAuthPolicy, opt *ReAssignForwardAuthInCrToForwardAuthPolicyOpt) error{
	"proxy_set_headers": func(lt *ForwardAuthInCr, rt *ForwardAuthPolicy, opt *ReAssignForwardAuthInCrToForwardAuthPolicyOpt) error {
		ret, err := opt.Resolve_proxy_set_headers(lt.AuthHeadersCmRef)
		if err != nil {
			return err
		}
		rt.AuthHeaders = ret
		return nil
	},

	"request_redirect": func(lt *ForwardAuthInCr, rt *ForwardAuthPolicy, opt *ReAssignForwardAuthInCrToForwardAuthPolicyOpt) error {
		ret, err := opt.Resolve_varstring(lt.AuthRequestRedirect)
		if err != nil {
			return err
		}
		rt.AuthRequestRedirect = ret
		return nil
	},

	"url": func(lt *ForwardAuthInCr, rt *ForwardAuthPolicy, opt *ReAssignForwardAuthInCrToForwardAuthPolicyOpt) error {
		ret, err := opt.Resolve_varstring(lt.Url)
		if err != nil {
			return err
		}
		rt.Url = ret
		return nil
	},
}

func ReAssignForwardAuthInCrToForwardAuthPolicy(lt *ForwardAuthInCr, rt *ForwardAuthPolicy, opt *ReAssignForwardAuthInCrToForwardAuthPolicyOpt) error {
	rt.AlwaysSetCookie = lt.AlwaysSetCookie

	if lt.Method != "" {
		rt.Method = lt.Method
	}

	if lt.UpstreamHeaders != nil {
		rt.UpstreamHeaders = lt.UpstreamHeaders
	}

	for _, m := range ReAssignForwardAuthInCrToForwardAuthPolicyTrans {
		err := m(lt, rt, opt)
		if err != nil {
			return err
		}
	}
	return nil
}

type ReAssignAuthIngressBasicToBasicAuthInCrOpt struct{}

var ReAssignAuthIngressBasicToBasicAuthInCrTrans = map[string]func(lt *AuthIngressBasic, rt *BasicAuthInCr, opt *ReAssignAuthIngressBasicToBasicAuthInCrOpt) error{}

func ReAssignAuthIngressBasicToBasicAuthInCr(lt *AuthIngressBasic, rt *BasicAuthInCr, opt *ReAssignAuthIngressBasicToBasicAuthInCrOpt) error {
	if lt.AuthType != "" {
		rt.AuthType = lt.AuthType
	}

	if lt.Realm != "" {
		rt.Realm = lt.Realm
	}

	if lt.Secret != "" {
		rt.Secret = lt.Secret
	}

	if lt.SecretType != "" {
		rt.SecretType = lt.SecretType
	}

	for _, m := range ReAssignAuthIngressBasicToBasicAuthInCrTrans {
		err := m(lt, rt, opt)
		if err != nil {
			return err
		}
	}
	return nil
}

type ReAssignBasicAuthInCrToBasicAuthPolicyOpt struct{}

var ReAssignBasicAuthInCrToBasicAuthPolicyTrans = map[string]func(lt *BasicAuthInCr, rt *BasicAuthPolicy, opt *ReAssignBasicAuthInCrToBasicAuthPolicyOpt) error{}

func ReAssignBasicAuthInCrToBasicAuthPolicy(lt *BasicAuthInCr, rt *BasicAuthPolicy, opt *ReAssignBasicAuthInCrToBasicAuthPolicyOpt) error {
	if lt.AuthType != "" {
		rt.AuthType = lt.AuthType
	}

	if lt.Realm != "" {
		rt.Realm = lt.Realm
	}

	for _, m := range ReAssignBasicAuthInCrToBasicAuthPolicyTrans {
		err := m(lt, rt, opt)
		if err != nil {
			return err
		}
	}
	return nil
}
