package gateway

import (
	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/pkg/operator/toolkit"
	"github.com/go-logr/logr"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type Template struct {
	namespace  string
	name       string
	baseDomain string
	alb2       *albv2.ALB2
	cur        *v1alpha2.GatewayClass
	log        logr.Logger
}

func NewTemplate(namespace, name, baseDomain string, alb2 *albv2.ALB2, gc *v1alpha2.GatewayClass, log logr.Logger) *Template {
	return &Template{
		namespace:  namespace,
		name:       name,
		baseDomain: baseDomain,
		alb2:       alb2,
		cur:        gc,
		log:        log,
	}
}

func (t *Template) Generate(options ...Option) *v1alpha2.GatewayClass {
	gvk := t.alb2.GroupVersionKind()
	group := v1alpha2.Group(gvk.Group)
	kind := v1alpha2.Kind(gvk.Kind)
	name := t.alb2.Name
	ns := v1alpha2.Namespace(t.namespace)
	gc := t.cur
	if gc == nil {
		gc = &v1alpha2.GatewayClass{}
	}
	gc.Name = t.name
	gc.Spec = v1alpha2.GatewayClassSpec{
		ControllerName: v1alpha2.GatewayController("alb2.gateway." + toolkit.FmtKeyBySep("/", t.baseDomain, t.name)),
		ParametersRef: &v1alpha2.ParametersReference{
			Group:     group,
			Kind:      kind,
			Name:      name,
			Namespace: &ns,
		},
	}
	defaultOptions := []Option{
		defaultLabel(t.baseDomain, t.name),
	}
	for _, op := range defaultOptions {
		op(gc)
	}
	for _, op := range options {
		op(gc)
	}
	return gc
}
