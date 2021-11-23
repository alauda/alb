package e2e

import (
	"alauda.io/alb2/test/e2e/framework"
	"github.com/onsi/ginkgo"
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
		ginkgo.FIt("should translate ingress to rule and generate nginx config,policy.new", func() {
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
						Target:     intstr.IntOrString{IntVal: 80},
						TargetName: "pod-tcp-80",
						TargetPort: 80,
					},
					"tcp-81": {
						Protocol:   corev1.ProtocolTCP,
						Port:       81,
						Target:     intstr.IntOrString{StrVal: "pod-tcp-112"},
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
	})
})
