package types

import (
	"strings"
)

func init() {
	// make go happy
	_ = strings.Clone("")
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
