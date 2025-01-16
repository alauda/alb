package types

import (
	"strings"
)

func init() {
	// make go happy
	_ = strings.Clone("")
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
	rt.Method = lt.Method

	rt.UpstreamHeaders = lt.UpstreamHeaders

	for _, m := range ReAssignForwardAuthInCrToForwardAuthPolicyTrans {
		err := m(lt, rt, opt)
		if err != nil {
			return err
		}
	}
	return nil
}
