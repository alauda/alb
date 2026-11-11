package test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"alauda.io/alb2/pkg/utils/test_utils/ing_utils"
	ing_ph "alauda.io/alb2/pkg/utils/test_utils/ing_utils"

	u "alauda.io/alb2/utils"
	. "alauda.io/alb2/utils/test_utils"
)

func TestProxyHeader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "proxy_header related test")
}

var _ = Describe("proxy_header related test", func() {
	l := ConsoleLog()
	var env *EnvtestExt
	var kt *Kubectl
	var kc *K8sClient

	BeforeEach(func() {
		env = NewEnvtestExt(InitBase(), GinkgoLog())
		env.AssertStart()
		kt = env.Kubectl()
		l.Info("start")
		l.Info("x", "cmd", kt.AssertKubectl("get crds"))
		kc = NewK8sClient(context.Background(), env.GetRestCfg())
		_ = l
		_ = kt
		_ = kc
	})

	AfterEach(func() {
		env.Stop()
	})

	Context("unit", func() {
		It("ingress x_forward to policy should ok", func() {
			ingYaml := `
apiVersion: crd.alauda.io/v2
kind: ALB2
metadata:
    name: alb-t
    namespace: cpaas-system
spec:
    type: "nginx"
    config:
        projects: ["ALL_ALL"]
---
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  name: alb-t-00080
  namespace: cpaas-system
  labels:
    "alb2.cpaas.io/name": "alb-t"
spec:
  port: 80
  protocol: http
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
    name: demo1
    namespace: cpaas-system
    annotations:
      alb.ingress.cpaas.io/use-forwarded-headers: "true"
spec:
    rules:
    - host: demo1.cpaas.io
      http:
        paths:
        - path: /
          pathType: ImplementationSpecific
          backend:
            service:
              name: demo
              port:
                number: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
    name: demo
    namespace: cpaas-system
spec:
    rules:
    - host: demo.cpaas.io
      http:
        paths:
        - path: /
          pathType: ImplementationSpecific
          backend:
            service:
              name: demo
              port:
                number: 80
`
			kt.AssertKubectlApply(ingYaml)
			ns := "cpaas-system"
			policy, err := ing_ph.SyncIngressAndGetPolicyFromK8s(env.GetRestCfg(), ns, "alb-t", l)
			Expect(err).NotTo(HaveOccurred())
			l.Info("policy", "policy", u.PrettyJson(policy))
			l.Info("get rule", "rule", kt.AssertKubectl("get rule -A -o yaml"))
			rules, err := ing_utils.WaitIngressRule(l, kc, ns, "demo1", "cpaas-system", 1)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(rules)).To(Equal(1))
			rule := rules[0]
			Expect(rule.Spec.Config.ProxyHeader.UseForwardedHeaders).To(Equal(true))
			ref := rule.Name + "/proxy_header"
			Expect(policy.GetHttpPolicy(rule.Name).Config.Refs["proxy_header"]).To(Equal(ref))
			GinkgoAssertJsonEq(policy.SharedConfig[ref], `
			{
				"type": "proxy_header",
				"proxy_header": {
					"use_forwarded_headers": true
				}
			}
			`, "policy proxy_header not match")
		})
	})
})
