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

type ReAssignRedirectIngressToRedirectCrOpt struct {
	String_to_int func(string) (*int, error)
}

var ReAssignRedirectIngressToRedirectCrTrans = map[string]func(lt *RedirectIngress, rt *RedirectCr, opt *ReAssignRedirectIngressToRedirectCrOpt) error{
	"code": func(lt *RedirectIngress, rt *RedirectCr, opt *ReAssignRedirectIngressToRedirectCrOpt) error {
		ret, err := opt.String_to_int(lt.PermanentRedirectCode)
		if err != nil {
			return err
		}
		rt.Code = ret
		return nil
	},

	"port": func(lt *RedirectIngress, rt *RedirectCr, opt *ReAssignRedirectIngressToRedirectCrOpt) error {
		ret, err := opt.String_to_int(lt.Port)
		if err != nil {
			return err
		}
		rt.Port = ret
		return nil
	},
}

func ReAssignRedirectIngressToRedirectCr(lt *RedirectIngress, rt *RedirectCr, opt *ReAssignRedirectIngressToRedirectCrOpt) error {
	if lt.Host != "" {
		rt.Host = lt.Host
	}

	if lt.PrefixMatch != "" {
		rt.PrefixMatch = lt.PrefixMatch
	}

	if lt.ReplacePrefix != "" {
		rt.ReplacePrefix = lt.ReplacePrefix
	}

	for _, m := range ReAssignRedirectIngressToRedirectCrTrans {
		err := m(lt, rt, opt)
		if err != nil {
			return err
		}
	}
	return nil
}

var RedirectIngressAnnotationList = []string{
	"force-ssl-redirect",

	"permanent-redirect",

	"permanent-redirect-code",

	"redirect-code",

	"redirect-host",

	"redirect-port",

	"redirect-prefix-match",

	"redirect-replace-prefix",

	"ssl-redirect",

	"temporal-redirect",

	"temporal-redirect-code",
}

func ResolverRedirectIngressFromAnnotation(ing *RedirectIngress, annotation map[string]string, prefix []string) (bool, error) {
	find := false
	for _, annotation_key := range RedirectIngressAnnotationList {
		for _, prefix := range prefix {
			annotation_full_key := fmt.Sprintf("%s/%s", prefix, annotation_key)
			if val, ok := annotation[annotation_full_key]; ok {
				find = true
				switch annotation_key {

				case "force-ssl-redirect":
					ing.ForceSSLRedirect = val

				case "permanent-redirect":
					ing.PermanentRedirect = val

				case "permanent-redirect-code":
					ing.PermanentRedirectCode = val

				case "redirect-code":
					ing.Code = val

				case "redirect-host":
					ing.Host = val

				case "redirect-port":
					ing.Port = val

				case "redirect-prefix-match":
					ing.PrefixMatch = val

				case "redirect-replace-prefix":
					ing.ReplacePrefix = val

				case "ssl-redirect":
					ing.SSLRedirect = val

				case "temporal-redirect":
					ing.TemporalRedirect = val

				case "temporal-redirect-code":
					ing.TemporalRedirectCode = val

				}
				break
			}
		}
	}
	return find, nil
}
