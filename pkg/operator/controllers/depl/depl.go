package depl

import (
	"fmt"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gateway "sigs.k8s.io/gateway-api/apis/v1alpha2"

	. "alauda.io/alb2/pkg/operator/toolkit"
)

// 集群上alb关心的cr
type AlbDeploy struct {
	Alb      *albv2.ALB2
	Deploy   *appsv1.Deployment // TODO deployment 可能不是一个而是多个，即同时存在新旧两个版本的deployment
	Common   *corev1.ConfigMap
	PortInfo *corev1.ConfigMap
	Ingress  *netv1.IngressClass
	Gateway  *gateway.GatewayClass
	Svc      *AlbDeploySvc
	Feature  *unstructured.Unstructured
}

type AlbDeploySvc struct {
	Svc    *corev1.Service
	TcpSvc *corev1.Service // for metallb lb svc
	UdpSvc *corev1.Service // for metallb lb svc
}

func (d *AlbDeploy) Show() string {
	return fmt.Sprintf("alb %v,depl %v,comm %v,port %v,ic %v,gc %v,svc %v",
		showCr(d.Alb),
		showCr(d.Deploy),
		showCr(d.Common),
		showCr(d.PortInfo),
		showCr(d.Ingress),
		showCr(d.Gateway),
		d.Svc.show())
}

func showCr(obj client.Object) string {
	if IsNil(obj) {
		return "isnil"
	}
	return fmt.Sprintf("name %v kind %v version %v", obj.GetName(), obj.GetObjectKind().GroupVersionKind().String(), obj.GetResourceVersion())
}

func (a *AlbDeploySvc) show() string {
	return fmt.Sprintf("svc %v tcp-svc %v udp-svc %v", showCr(a.Svc), showCr(a.TcpSvc), showCr(a.UdpSvc))
}
