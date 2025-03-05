package util

import "strings"

func ParseSSLAnnotation(sslAnno string) map[string]string {
	// alb.networking.{domain}/tls: qq.com=cpaas-system/dex.tls,qq1.com=cpaas-system/dex1.tls
	if sslAnno == "" {
		return nil
	}
	rv := make(map[string]string)
	parts := strings.Split(sslAnno, ",")
	for _, p := range parts {
		kv := strings.Split(strings.TrimSpace(p), "=")
		if len(kv) != 2 {
			return nil
		}
		k, v := kv[0], kv[1]
		if rv[k] != "" && rv[k] != v {
			return nil
		}
		// ["qq.com"]="cpaas-system/dex.tls"
		rv[k] = v
	}
	return rv
}
