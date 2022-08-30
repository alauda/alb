package ingress

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/klog/v2"
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

func HasDefaultBackend(ing *networkingv1.Ingress) bool {
	return len(ing.Spec.Rules) == 0 &&
		ing.Spec.DefaultBackend != nil &&
		ing.Spec.DefaultBackend.Resource == nil &&
		ing.Spec.DefaultBackend.Service != nil
}

func ToInStr(backendPort networkingv1.ServiceBackendPort) intstr.IntOrString {
	intStrType := intstr.Int
	if backendPort.Number == 0 {
		intStrType = intstr.String
	}
	return intstr.IntOrString{Type: intStrType, IntVal: backendPort.Number, StrVal: backendPort.Name}
}

type NotExistsError string

// Error implements the error interface.
func (e NotExistsError) Error() string {
	return fmt.Sprintf("no object matching key %q in local store", string(e))
}

func IngKey(ing *networkingv1.Ingress) client.ObjectKey {
	return client.ObjectKey{
		Namespace: ing.Namespace,
		Name:      ing.Name,
	}
}
