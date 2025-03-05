package ingress

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1 "k8s.io/api/networking/v1"
)

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
