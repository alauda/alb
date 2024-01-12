package gateway

import (
	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/pkg/operator/toolkit"
	"github.com/go-logr/logr"
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type Template struct {
	namespace  string
	name       string
	baseDomain string
	alb2       *albv2.ALB2
	cur        *gv1.GatewayClass
	log        logr.Logger
}

func NewTemplate(namespace, name, baseDomain string, alb2 *albv2.ALB2, gc *gv1.GatewayClass, log logr.Logger) *Template {
	return &Template{
		namespace:  namespace,
		name:       name,
		baseDomain: baseDomain,
		alb2:       alb2,
		cur:        gc,
		log:        log,
	}
}

func (t *Template) Generate(options ...Option) *gv1.GatewayClass {
	gvk := t.alb2.GroupVersionKind()
	group := gv1.Group(gvk.Group)
	kind := gv1.Kind(gvk.Kind)
	name := t.alb2.Name
	ns := gv1.Namespace(t.namespace)
	gc := t.cur
	if gc == nil {
		gc = &gv1.GatewayClass{}
	}
	gc.Name = t.name
	gc.Spec = gv1.GatewayClassSpec{
		ControllerName: gv1.GatewayController("alb2.gateway." + toolkit.FmtKeyBySep("/", t.baseDomain, t.name)),
		ParametersRef: &gv1.ParametersReference{
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

func defaultLabel(baseDomain, name string) Option {
	labels := map[string]string{
		"alb2." + baseDomain + "/gatewayclass": name,
	}
	return func(gc *gv1.GatewayClass) {
		if gc == nil {
			return
		}
		gc.Labels = labels
	}
}
