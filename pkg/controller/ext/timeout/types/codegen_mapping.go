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

type ReAssignTimeoutIngressToTimeoutCrOpt struct {
	Time_from_string func(string) (*uint, error)
}

var ReAssignTimeoutIngressToTimeoutCrTrans = map[string]func(lt *TimeoutIngress, rt *TimeoutCr, opt *ReAssignTimeoutIngressToTimeoutCrOpt) error{
	"proxy_connect_timeout_ms": func(lt *TimeoutIngress, rt *TimeoutCr, opt *ReAssignTimeoutIngressToTimeoutCrOpt) error {
		ret, err := opt.Time_from_string(lt.ProxyConnectTimeoutMs)
		if err != nil {
			return err
		}
		rt.ProxyConnectTimeoutMs = ret
		return nil
	},

	"proxy_read_timeout_ms": func(lt *TimeoutIngress, rt *TimeoutCr, opt *ReAssignTimeoutIngressToTimeoutCrOpt) error {
		ret, err := opt.Time_from_string(lt.ProxyReadTimeoutMs)
		if err != nil {
			return err
		}
		rt.ProxyReadTimeoutMs = ret
		return nil
	},

	"proxy_send_timeout_ms": func(lt *TimeoutIngress, rt *TimeoutCr, opt *ReAssignTimeoutIngressToTimeoutCrOpt) error {
		ret, err := opt.Time_from_string(lt.ProxySendTimeoutMs)
		if err != nil {
			return err
		}
		rt.ProxySendTimeoutMs = ret
		return nil
	},
}

func ReAssignTimeoutIngressToTimeoutCr(lt *TimeoutIngress, rt *TimeoutCr, opt *ReAssignTimeoutIngressToTimeoutCrOpt) error {
	for _, m := range ReAssignTimeoutIngressToTimeoutCrTrans {
		err := m(lt, rt, opt)
		if err != nil {
			return err
		}
	}
	return nil
}

var TimeoutIngressAnnotationList = []string{
	"proxy-connect-timeout",

	"proxy-read-timeout",

	"proxy-send-timeout",
}

func ResolverTimeoutIngressFromAnnotation(ing *TimeoutIngress, annotation map[string]string, prefix []string) (bool, error) {
	find := false
	for _, annotation_key := range TimeoutIngressAnnotationList {
		for _, prefix := range prefix {
			annotation_full_key := fmt.Sprintf("%s/%s", prefix, annotation_key)
			if val, ok := annotation[annotation_full_key]; ok {
				find = true
				switch annotation_key {

				case "proxy-connect-timeout":
					ing.ProxyConnectTimeoutMs = val

				case "proxy-read-timeout":
					ing.ProxyReadTimeoutMs = val

				case "proxy-send-timeout":
					ing.ProxySendTimeoutMs = val

				}
				break
			}
		}
	}
	return find, nil
}
