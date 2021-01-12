package ingress

import (
	networkingv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/klog"
	"strings"
)

func parseSSLAnnotation(sslAnno string) map[string]string {
	// alb.networking.{domain}/tls: qq.com=cpaas-system/dex.tls,qq1.com=cpaas-system/dex1.tls
	if sslAnno == "" {
		return nil
	}
	rv := make(map[string]string)
	parts := strings.Split(sslAnno, ",")
	for _, p := range parts {
		kv := strings.Split(strings.TrimSpace(p), "=")
		if len(kv) != 2 {
			klog.Warningf("invalid ssl annotation format, %s", p)
			return nil
		}
		k, v := kv[0], kv[1]
		if rv[k] != "" {
			klog.Warningf("invalid ssl annotation duplicate host, %s", p)
			return nil
		}
		// ["qq.com"]="cpaas-system/dex.tls"
		rv[k] = v
	}
	return rv
}

func isDefaultBackend(ing *networkingv1beta1.Ingress) bool {
	return len(ing.Spec.Rules) == 0 && ing.Spec.Backend != nil
}
