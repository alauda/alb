package ingress

import (
	"fmt"

	"alauda.io/alb2/config"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/tools/cache"
)

type ingressClassLister struct {
	cache.Store
}

// ByKey returns the Ingress matching key in the local Ingress Store.
func (il ingressClassLister) ByKey(key string) (*networkingv1.IngressClass, error) {
	i, exists, err := il.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, NotExistsError(key)
	}
	return i.(*networkingv1.IngressClass), nil
}

func CheckIngressClass(ingclass *networkingv1.IngressClass, icConfig *config.IngressClassConfiguration) bool {
	foundClassByName := false
	if icConfig.IngressClassByName && ingclass.Name == icConfig.AnnotationValue {
		foundClassByName = true
	}
	if !foundClassByName && ingclass.Spec.Controller != icConfig.Controller {
		return false
	}
	return true
}

func (c *Controller) GetIngressClass(ing *networkingv1.Ingress, icConfig *config.IngressClassConfiguration) (string, error) {
	// First we try ingressClassName
	if !icConfig.IgnoreIngressClass && ing.Spec.IngressClassName != nil {
		name := *ing.Spec.IngressClassName
		iclass, err := c.ingressClassLister.ByKey(name)
		if err != nil {
			return "", fmt.Errorf("get ingressclass %v fail %v", name, err)
		}
		return iclass.Name, nil
	}

	// Then we try annotation
	if ingressClass, ok := ing.GetAnnotations()[config.IngressKey]; ok {
		if ingressClass != "" && ingressClass != config.Get("NAME") {
			return "", fmt.Errorf("invalid ingress class annotation: %s", ingressClass)
		}
		return ingressClass, nil
	}

	// Then we accept if the WithoutClass is enabled
	if icConfig.WatchWithoutClass {
		// Reserving "_" as a "wildcard" name
		return "_", nil
	}
	return "", fmt.Errorf("ingress does not contain a valid IngressClass")
}
