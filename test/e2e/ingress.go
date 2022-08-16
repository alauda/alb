package e2e

import (
	"context"
	"fmt"
	"strings"

	"alauda.io/alb2/test/e2e/framework"
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
		deployCfg := framework.Config{InstanceMode: true, RestCfg: framework.CfgFromEnv(), Project: []string{"project1"}}
		f = framework.NewAlb(deployCfg)
		f.InitProductNs("alb-test", "project1")
		f.InitDefaultSvc("svc-default", []string{"192.168.1.1", "192.168.2.2"})
		f.Init()
	})

	ginkgo.AfterEach(func() {
		f.Destroy()
		f = nil
	})

	framework.GIt("should compatible with redirect ingress", func() {
		ns := f.GetProductNs()
		// should generate rule and policy when use ingress.backend.port.number even svc not exist.
		pathType := networkingv1.PathTypeImplementationSpecific
		_, err := f.GetK8sClient().NetworkingV1().Ingresses(ns).Create(context.Background(), &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      "redirect",
				Annotations: map[string]string{
					"nginx.ingress.kubernetes.io/temporal-redirect": "/console-platform/portal",
				},
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
		f.WaitNginxConfigStr("listen.*80")

		rules := f.WaitIngressRule("redirect", ns, 1)
		rule := rules[0]
		ruleName := rules[0].Name

		assert.Equal(ginkgo.GinkgoT(), rule.Spec.ServiceGroup.Services[0].Port, 8080)

		f.WaitPolicy(func(policyRaw string) bool {
			fmt.Printf("policyRaw %s", policyRaw)
			hasRule := framework.PolicyHasRule(policyRaw, 80, ruleName)
			hasPod := framework.PolicyHasBackEnds(policyRaw, ruleName, `[]`)
			return hasRule && hasPod
		})
	})

	ginkgo.Context("basic ingress", func() {
		framework.GIt("should translate ingress to rule and generate nginx config,policy.new", func() {
			ns := f.GetProductNs()
			name := "ingress-a"

			f.CreateIngress(name, "/a", "svc-default", 80)

			f.WaitNginxConfigStr("listen.*80")

			rules := f.WaitIngressRule(name, ns, 1)
			rule := rules[0]
			ruleName := rule.Name

			f.WaitPolicy(func(policyRaw string) bool {
				hasRule := framework.PolicyHasRule(policyRaw, 80, ruleName)
				hasPod := framework.PolicyHasBackEnds(policyRaw, ruleName, `[map[address:192.168.1.1 otherclusters:false port:80 weight:50] map[address:192.168.2.2 otherclusters:false port:80 weight:50]]`)
				return hasRule && hasPod
			})

		})

		framework.GIt("should get correct backend port when ingress use port.name", func() {
			// give a svc.port tcp-81 which point to pod name pod-tcp-112 which point to pod container port 112
			// if ingress.backend.port.name=tcp-81, the backend should be xxxx:112
			ns := f.GetProductNs()
			ingressCase := framework.IngressCase{
				Namespace: ns,
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
			f.WaitNginxConfigStr("listen.*80")
			rules := f.WaitIngressRule(ingressCase.Ingress.Name, ingressCase.Namespace, 1)
			rule := rules[0]
			ruleName := rule.Name
			framework.Logf("created rule %+v", rule)
			assert.Equal(ginkgo.GinkgoT(), rule.Spec.ServiceGroup.Services[0].Port, 81)
			f.WaitPolicy(func(policyRaw string) bool {
				hasRule := framework.PolicyHasRule(policyRaw, 80, ruleName)
				hasPod := framework.PolicyHasBackEnds(policyRaw, ruleName, `[map[address:192.168.3.1 otherclusters:false port:112 weight:50] map[address:192.168.3.2 otherclusters:false port:112 weight:50]]`)
				return hasRule && hasPod
			})
		})

		framework.GIt("should work when ingress has longlong name", func() {
			ns := f.GetProductNs()
			_ = ns
			name := "ingress-aaa" + strings.Repeat("a", 100)

			f.CreateIngress(name, "/a", "svc-default", 80)

			f.WaitNginxConfigStr("listen.*80")

			rules := f.WaitIngressRule(name, ns, 1)
			rule := rules[0]
			ruleName := rule.Name
			_ = ruleName

			f.WaitPolicy(func(policyRaw string) bool {
				hasRule := framework.PolicyHasRule(policyRaw, 80, ruleName)
				hasPod := framework.PolicyHasBackEnds(policyRaw, ruleName, `[map[address:192.168.1.1 otherclusters:false port:80 weight:50] map[address:192.168.2.2 otherclusters:false port:80 weight:50]]`)
				return hasRule && hasPod
			})
		})
	})
})
