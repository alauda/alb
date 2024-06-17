// utils only used in test,this package could not be used in dirver package
package test_utils

import (
	_ "alauda.io/alb2/config"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	. "alauda.io/alb2/utils"
	k8sv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DEFAULT_NS  = "ns-1"
	DEFAULT_ALB = "alb-1"
)

var DEFAULT_CONFIG_FOR_TEST = map[string]string{
	"DOMAIN":               "alauda.io",
	"TWEAK_DIRECTORY":      "../driver/", // set TWEAK_DIRECTORY to an existing path: make calculate hash happy
	"NAME":                 DEFAULT_ALB,
	"NAMESPACE":            DEFAULT_NS,
	"bindkey":              "loadbalancer.%s/bind",
	"labels.name":          "alb2.%s/name",
	"labels.frontend":      "alb2.%s/frontend",
	"labels.source_type":   "alb2.%s/source-type",
	"DEFAULT_SSL_STRATEGY": "Request",
}

type FakeResource struct {
	Alb FakeALBResource
	K8s FakeK8sResource
}

func add[T client.Object](crs []client.Object, ocrs []T) []client.Object {
	ret := crs
	for _, cr := range ocrs {
		x := cr
		ret = append(ret, x)
	}
	return ret
}

// reutrn sorted (via create order) cr
func (f FakeResource) ListCr() []client.Object {
	crs := []client.Object{}
	crs = add(crs, SliceToPointerSlice(f.K8s.Namespaces))
	crs = add(crs, SliceToPointerSlice(f.K8s.Services))
	crs = add(crs, SliceToPointerSlice(f.K8s.EndPoints))
	crs = add(crs, SliceToPointerSlice(f.K8s.Ingresses))
	crs = add(crs, SliceToPointerSlice(f.K8s.IngressesClass))
	crs = add(crs, SliceToPointerSlice(f.K8s.Secrets))
	crs = add(crs, SliceToPointerSlice(f.Alb.Albs))
	crs = add(crs, SliceToPointerSlice(f.Alb.Frontends))
	crs = add(crs, SliceToPointerSlice(f.Alb.Rules))
	return crs
}

type FakeALBResource struct {
	Albs      []albv2.ALB2
	Frontends []albv1.Frontend
	Rules     []albv1.Rule
}

type FakeK8sResource struct {
	Namespaces     []k8sv1.Namespace
	Services       []k8sv1.Service
	EndPoints      []k8sv1.Endpoints
	Ingresses      []networkingv1.Ingress
	IngressesClass []networkingv1.IngressClass
	Secrets        []k8sv1.Secret
}
