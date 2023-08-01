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

// IngressClassConfiguration defines the various aspects of IngressClass parsing
// and how the controller should behave in each case
type IngressClassConfiguration struct {
	// Controller defines the controller value this daemon watch to.
	// Defaults to "alauda.io/alb2"
	Controller string
	// AnnotationValue defines the annotation value this Controller watch to, in case of the
	// ingressClass is not found but the annotation is.
	// The Annotation is deprecated and should not be used in future releases
	AnnotationValue string
	// WatchWithoutClass defines if Controller should watch to Ingress Objects that does
	// not contain an IngressClass configuration
	WatchWithoutClass bool
	// IgnoreIngressClass defines if Controller should ignore the IngressClass Object if no permissions are
	// granted on IngressClass
	IgnoreIngressClass bool
	//IngressClassByName defines if the Controller should watch for Ingress Classes by
	// .metadata.name together with .spec.Controller
	IngressClassByName bool
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

func CheckIngressClass(ingclass *networkingv1.IngressClass, icConfig *IngressClassConfiguration) bool {
	foundClassByName := false
	if icConfig.IngressClassByName && ingclass.Name == icConfig.AnnotationValue {
		foundClassByName = true
	}
	if !foundClassByName && ingclass.Spec.Controller != icConfig.Controller {
		return false
	}
	return true
}

func (c *Controller) GetIngressClass(ing *networkingv1.Ingress, icConfig *IngressClassConfiguration) (string, error) {
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
		if ingressClass != "" && ingressClass != config.GetConfig().GetAlbName() {
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
