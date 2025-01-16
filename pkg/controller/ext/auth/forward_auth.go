package auth

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"

	ct "alauda.io/alb2/controller/types"
	. "alauda.io/alb2/pkg/controller/ext/auth/types"
	"github.com/go-logr/logr"

	. "alauda.io/alb2/pkg/utils"
)

type ForwardAuthCtl struct {
	l logr.Logger
}

func (f ForwardAuthCtl) AuthIngressToAuthCr(auth_ingress *AuthIngress, auth_cr *AuthCr) {
	auth_cr.Forward = &ForwardAuthInCr{
		Url:                 "",
		Method:              "",
		AuthHeadersCmRef:    "",
		AuthRequestRedirect: "",
		Signin:              "",
		AlwaysSetCookie:     false,
		SigninRedirectParam: "",
	}
	_ = ReassignStructViaMapping(auth_ingress, auth_cr.Forward, ReassignStructOpt{
		Resolver: map[string]func(l, r reflect.Value, _ reflect.Value) error{
			"resolve_response_headers": func(l, r reflect.Value, _ reflect.Value) error {
				ls := l.Interface().(string)
				if ls == "" {
					return nil
				}
				r.Set(reflect.ValueOf(strings.Split(ls, ",")))
				return nil
			},
		},
	})
}

func (f ForwardAuthCtl) ToPolicy(forward *ForwardAuthInCr, p *AuthPolicy, refs ct.RefMap, rule string) {
	log := f.l.WithValues("rule", rule)
	fp := &ForwardAuthPolicy{
		Url:                 []string{},
		Method:              "GET",
		AuthHeaders:         map[string]VarString{},
		AuthRequestRedirect: []string{},
		UpstreamHeaders:     []string{},
		AlwaysSetCookie:     false,
		SigninUrl:           []string{},
	}
	err := ReassignStructViaMapping(forward, fp, ReassignStructOpt{
		Resolver: map[string]func(l, r reflect.Value, root reflect.Value) error{
			"resolve_varstring": func(l, r reflect.Value, _ reflect.Value) error {
				ls := l.Interface().(string)
				var_str, err := ParseVarString(ls)
				if err != nil {
					return err
				}
				r.Set(reflect.ValueOf(var_str))
				return nil
			},
			"resolve_proxy_set_headers": func(l, r reflect.Value, root reflect.Value) error {
				cm_ref := l.Interface().(string)
				if cm_ref == "" {
					return nil
				}
				root_real := root.Interface().(*ForwardAuthPolicy)
				root_real.AuthHeaders = map[string]VarString{}
				cm_key, err := ParseStringToObjectKey(cm_ref)
				if err != nil {
					return err
				}
				rv := r.Interface().(map[string]VarString)
				cm, ok := refs.ConfigMap[cm_key]
				if !ok {
					root_real.InvalidAuthReqCmRef = true
					log.Info("cm not found", "key", cm_key)
					return nil
				}
				for k, v := range cm.Data {
					var_str, err := ParseVarString(v)
					if err != nil {
						return err
					}
					rv[k] = var_str
				}
				return nil
			},
		},
	})
	if err != nil {
		log.Error(err, "gen policy fail")
		return
	}

	if forward.Signin != "" {
		redirect_param := forward.SigninRedirectParam
		if redirect_param == "" {
			redirect_param = "rd"
		}
		full, err := resolve_signin_url(forward.Signin, redirect_param)
		if err != nil {
			log.Error(err, "resolve signin url fail")
			return
		}
		fp.SigninUrl = full
	}
	p.Forward = fp
}

// https://github.com/kubernetes/ingress-nginx/blob/d1dc3e827f818ee23a08af09e9a7be0b12af1736/internal/ingress/controller/template/template.go#L1156
func buildAuthSignURL(authSignURL, authRedirectParam string) string {
	u, _ := url.Parse(authSignURL)
	q := u.Query()
	if authRedirectParam == "" {
		authRedirectParam = "rd"
	}
	if len(q) == 0 {
		return fmt.Sprintf("%s?%s=$pass_access_scheme://$http_host$escaped_request_uri", authSignURL, authRedirectParam)
	}

	if q.Get(authRedirectParam) != "" {
		return authSignURL
	}
	return fmt.Sprintf("%s&%s=$pass_access_scheme://$http_host$escaped_request_uri", authSignURL, authRedirectParam)
}

func resolve_signin_url(signin_url string, redirect_param string) (VarString, error) {
	full := buildAuthSignURL(signin_url, redirect_param)
	return ParseVarString(full)
}
