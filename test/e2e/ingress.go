package e2e

import (
	"alauda.io/alb2/test/e2e/framework"
	"context"
	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = ginkgo.Describe("Ingress", func() {
	var f *framework.Framework

	ginkgo.BeforeEach(func() {
		f = framework.NewAlb(framework.Config{InstanceMode: true, RandomBaseDir: false, RestCfg: framework.CfgFromEnv(), Project: []string{"project1"}})
		f.EnsureNs("alb-test", "project1")
		f.Init()
	})
	ginkgo.AfterEach(func() {
		f.DestroyNs("alb-test")
		f.Destroy()
		f = nil
	})
	ginkgo.Context("basic ingress", func() {
		ginkgo.FIt("should compatible with redirect ingress", func() {
			ns := "alb-test"
			// should generate rule and policy when use ingress.backend.port.number even svc not exist.
			pathType := networkingv1.PathTypeImplementationSpecific
			_, err := f.GetK8sClient().NetworkingV1().Ingresses(ns).Create(context.Background(), &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns,
					Name:      "redirect",
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     "/&",
											PathType: &pathType,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "none",
													Port: networkingv1.ServiceBackendPort{Number: 8080},
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

			assert.NoError(ginkgo.GinkgoT(), err)

			f.WaitNginxConfig("listen.*80")

			rules := f.WaitIngressRule("redirect", ns, 1)
			rule := rules[0]
			ruleName := rules[0].Name

			assert.Equal(ginkgo.GinkgoT(), rule.Spec.ServiceGroup.Services[0].Port, 8080)

			f.WaitPolicy(func(policyRaw string) bool {
				hasRule := framework.PolicyHasRule(policyRaw, 80, ruleName)
				hasPod := framework.PolicyHasBackEnds(policyRaw, ruleName, `[]`)
				return hasRule && hasPod
			})
		})

		ginkgo.It("should translate ingress to rule and generate nginx config,policy.new", func() {
			ingressCase := framework.IngressCase{
				Namespace: "alb-test",
				Name:      "svc1",
				SvcPort: map[string]struct {
					Protocol   corev1.Protocol
					Port       int32
					Target     intstr.IntOrString
					TargetPort int32
					TargetName string
				}{
					"tcp-80": {
						Protocol:   corev1.ProtocolTCP,
						Port:       80,
						Target:     intstr.IntOrString{Type: intstr.Int, IntVal: 80},
						TargetName: "pod-tcp-80",
						TargetPort: 80,
					},
				},
				Eps: []string{"192.168.3.1", "192.168.3.2"},
				Ingress: struct {
					Name string
					Host string
					Path string
					Port intstr.IntOrString
				}{
					Name: "svc-ingress",
					Host: "host-a",
					Path: "/a",
					Port: intstr.IntOrString{IntVal: 80},
				},
			}

			f.InitIngressCase(ingressCase)
			f.WaitNginxConfig("listen.*80")
			rules := f.WaitIngressRule(ingressCase.Ingress.Name, ingressCase.Namespace, 1)
			rule := rules[0]
			ruleName := rule.Name
			f.WaitPolicy(func(policyRaw string) bool {
				hasRule := framework.PolicyHasRule(policyRaw, 80, ruleName)
				hasPod := framework.PolicyHasBackEnds(policyRaw, ruleName, `[map[address:192.168.3.1 port:80 weight:50] map[address:192.168.3.2 port:80 weight:50]]`)
				return hasRule && hasPod
			})
		})

		ginkgo.It("should get correct backend port when ingress use port.name", func() {
			// give a svc.port tcp-81 which point to pod name pod-tcp-112 which point to pod container port 112
			// if ingress.backend.port.name=tcp-81, the backend should be xxxx:112
			ingressCase := framework.IngressCase{
				Namespace: "alb-test",
				Name:      "svc2",
				SvcPort: map[string]struct {
					Protocol   corev1.Protocol
					Port       int32
					Target     intstr.IntOrString
					TargetPort int32
					TargetName string
				}{
					"tcp-81": {
						Protocol:   corev1.ProtocolTCP,
						Port:       81,
						Target:     intstr.IntOrString{Type: intstr.String, StrVal: "pod-tcp-112"},
						TargetName: "pod-tcp-112",
						TargetPort: 112,
					},
				},
				Eps: []string{"192.168.3.1", "192.168.3.2"},
				Ingress: struct {
					Name string
					Host string
					Path string
					Port intstr.IntOrString
				}{
					Name: "svc-ingress",
					Host: "host-a",
					Path: "/a",
					Port: intstr.IntOrString{Type: intstr.String, StrVal: "tcp-81"},
				},
			}

			f.InitIngressCase(ingressCase)
			f.WaitNginxConfig("listen.*80")
			rules := f.WaitIngressRule(ingressCase.Ingress.Name, ingressCase.Namespace, 1)
			rule := rules[0]
			ruleName := rule.Name
			framework.Logf("created rule %+v", rule)
			assert.Equal(ginkgo.GinkgoT(), rule.Spec.ServiceGroup.Services[0].Port, 81)
			f.WaitPolicy(func(policyRaw string) bool {
				hasRule := framework.PolicyHasRule(policyRaw, 80, ruleName)
				hasPod := framework.PolicyHasBackEnds(policyRaw, ruleName, `[map[address:192.168.3.1 port:112 weight:50] map[address:192.168.3.2 port:112 weight:50]]`)
				return hasRule && hasPod
			})
		})
	})
})
