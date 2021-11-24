package e2e

import (
	"alauda.io/alb2/test/e2e/framework"
	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
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

		ginkgo.FIt("should get correct backend port when ingress use port.name", func() {
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
