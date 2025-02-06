package auth

import (
	"fmt"
	"net/url"
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
	forward := &ForwardAuthInCr{
		Url:                 "",
		Method:              "GET",
		AuthHeadersCmRef:    "",
		AuthRequestRedirect: "",
		UpstreamHeaders:     []string{},
		Signin:              "",
		AlwaysSetCookie:     false,
		SigninRedirectParam: "",
	}
	err := ReAssignAuthIngressForwardToForwardAuthInCr(&auth_ingress.AuthIngressForward, auth_cr.Forward, &ReAssignAuthIngressForwardToForwardAuthInCrOpt{
		Resolve_response_headers: func(ls string) ([]string, error) {
			ls = strings.TrimSpace(ls)
			if ls == "" {
				return []string{}, nil
			}
			return strings.Split(ls, ","), nil
		},
	})
	if err != nil {
		f.l.Error(err, "failed to reassign auth ingress forward to forward auth in cr")
		return
	}
	auth_cr.Forward = forward
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
	err := ReAssignForwardAuthInCrToForwardAuthPolicy(forward, fp, &ReAssignForwardAuthInCrToForwardAuthPolicyOpt{
		Resolve_proxy_set_headers: func(cm_ref string) (map[string]VarString, error) {
			rv := map[string]VarString{}
			if cm_ref == "" {
				return rv, nil
			}
			cm_key, err := ParseStringToObjectKey(cm_ref)
			if err != nil {
				return nil, err
			}
			cm, ok := refs.ConfigMap[cm_key]
			if !ok {
				fp.InvalidAuthReqCmRef = true
				log.Info("cm not found", "key", cm_key)
				return nil, nil
			}
			for k, v := range cm.Data {
				var_str, err := ParseVarString(v)
				if err != nil {
					return nil, err
				}
				rv[k] = var_str
			}
			return rv, nil
		},
		Resolve_varstring: ParseVarString,
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
func buildAuthSignURL(authSignURL, authRedirectParam string) (string, error) {
	u, err := url.Parse(authSignURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if authRedirectParam == "" {
		authRedirectParam = "rd"
	}
	if len(q) == 0 {
		return fmt.Sprintf("%s?%s=$pass_access_scheme://$http_host$escaped_request_uri", authSignURL, authRedirectParam), nil
	}

	if q.Get(authRedirectParam) != "" {
		return authSignURL, nil
	}
	return fmt.Sprintf("%s&%s=$pass_access_scheme://$http_host$escaped_request_uri", authSignURL, authRedirectParam), nil
}

func resolve_signin_url(signin_url string, redirect_param string) (VarString, error) {
	full, err := buildAuthSignURL(signin_url, redirect_param)
	if err != nil {
		return []string{}, err
	}
	return ParseVarString(full)
}
