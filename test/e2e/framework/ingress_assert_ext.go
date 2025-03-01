package framework

import (
	"context"
	"fmt"

	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	"github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
)

// deprecated, use KubectlApply and InitSvcWithOpt.
// ingress service and end point
type IngressCase struct {
	Namespace string
	Name      string
	SvcPort   map[string]struct { // key svc.port.name which match ep.port.name
		Protocol   corev1.Protocol
		Port       int32
		Target     intstr.IntOrString
		TargetPort int32
		TargetName string // the name match pod.port.name
	}
	Eps     []string
	Ingress struct {
		Name string
		Host string
		Path string
		Port intstr.IntOrString
	}
}

type IngressExt struct {
	kc     *K8sClient
	ns     string
	domain string
	ctx    context.Context
	log    logr.Logger
}

func (i *IngressExt) InitIngressCase(ingressCase IngressCase) {
	f := i.kc
	var svcPort []corev1.ServicePort
	for name, p := range ingressCase.SvcPort {
		svcPort = append(svcPort,
			corev1.ServicePort{
				Port:       p.Port,
				Protocol:   corev1.ProtocolTCP,
				Name:       name,
				TargetPort: p.Target,
			},
		)
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressCase.Name,
			Namespace: ingressCase.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports:    svcPort,
			Selector: map[string]string{"kube-app": ingressCase.Name},
		},
	}
	svc, err := f.GetK8sClient().CoreV1().Services(ingressCase.Namespace).Create(context.Background(), svc, metav1.CreateOptions{})
	Logf("svc port %+v", svcPort)
	Logf("created svc %+v", svc)
	assert.Nil(ginkgo.GinkgoT(), err, "")
	subSetAddress := []corev1.EndpointAddress{}
	for _, address := range ingressCase.Eps {
		subSetAddress = append(subSetAddress, corev1.EndpointAddress{
			IP: address,
		})
	}
	subSetPort := []corev1.EndpointPort{}
	for svcPortName, p := range ingressCase.SvcPort {
		subSetPort = append(subSetPort,
			corev1.EndpointPort{
				Port:     p.TargetPort,
				Protocol: corev1.ProtocolTCP,
				Name:     svcPortName,
			},
		)
	}
	subSet := corev1.EndpointSubset{
		NotReadyAddresses: []corev1.EndpointAddress{},
		Addresses:         subSetAddress,
		Ports:             subSetPort,
	}

	ep := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ingressCase.Namespace,
			Name:      ingressCase.Name,
			Labels:    map[string]string{"kube-app": ingressCase.Name},
		},
		Subsets: []corev1.EndpointSubset{subSet},
	}

	_, err = f.GetK8sClient().CoreV1().Endpoints(ingressCase.Namespace).Create(context.Background(), ep, metav1.CreateOptions{})
	assert.Nil(ginkgo.GinkgoT(), err, "")
	ingressPort := networkingv1.ServiceBackendPort{}
	if ingressCase.Ingress.Port.IntVal != 0 {
		ingressPort.Number = ingressCase.Ingress.Port.IntVal
	} else {
		ingressPort.Name = ingressCase.Ingress.Port.StrVal
	}

	_, err = f.GetK8sClient().NetworkingV1().Ingresses(ingressCase.Namespace).Create(context.Background(), &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ingressCase.Namespace,
			Name:      ingressCase.Ingress.Name,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: ingressCase.Ingress.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     ingressCase.Ingress.Path,
									PathType: (*networkingv1.PathType)(ToPointOfString(string(networkingv1.PathTypeImplementationSpecific))),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: ingressCase.Name,
											Port: ingressPort,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})

	assert.Nil(ginkgo.GinkgoT(), err, "")
}

func (i *IngressExt) CreateIngress(ns, name string, path string, svc string, port int) {
	f := i.kc
	_, err := f.GetK8sClient().NetworkingV1().Ingresses(ns).Create(context.Background(), &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     path,
									PathType: (*networkingv1.PathType)(ToPointOfString(string(networkingv1.PathTypeImplementationSpecific))),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: svc,
											Port: networkingv1.ServiceBackendPort{
												Number: int32(port),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	assert.Nil(ginkgo.GinkgoT(), err, "")
}

func (f *IngressExt) GetIngressRule(ingresName, ingressNs string, size int) []alb2v1.Rule {
	selType := fmt.Sprintf("alb2.%s/source-type=ingress", f.domain)
	sel := selType
	rules, err := f.kc.GetAlbClient().CrdV1().Rules(f.ns).List(f.ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		f.log.Info(fmt.Sprintf("get rule for ingress %s/%s sel -%s- fail %s", ingressNs, ingresName, sel, err))
	}
	return rules.Items
}

func (f *IngressExt) WaitIngressRule(ingresName, ingressNs string, size int) []alb2v1.Rule {
	rulesChan := make(chan []alb2v1.Rule, 1)
	err := wait.Poll(Poll, DefaultTimeout, func() (bool, error) {
		rules := f.GetIngressRule(ingresName, ingressNs, size)
		f.log.Info("get rule", "len", len(rules), "expect", size)
		if len(rules) == size {
			rulesChan <- rules
			return true, nil
		}
		return false, nil
	})
	assert.Nil(ginkgo.GinkgoT(), err, "wait rule fail")
	rules := <-rulesChan
	return rules
}
