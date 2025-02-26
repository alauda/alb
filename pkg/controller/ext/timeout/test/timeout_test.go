package test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	ing_ph "alauda.io/alb2/pkg/utils/test_utils/ing_utils"

	u "alauda.io/alb2/utils"
	. "alauda.io/alb2/utils/test_utils"
)

func TestTimeout(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "timeout related test")
}

var _ = Describe("timeout related test", func() {
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
		It("ingress timeout to policy should ok", func() {
			kt.AssertKubectlApply(`
apiVersion: crd.alauda.io/v2
kind: ALB2
metadata:
    name: alb-t
spec:
    type: "nginx"
    config:
        projects: ["ALL_ALL"]
---
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  name: alb-t-00080
  labels:
    "alb2.cpaas.io/name": "alb-t"
spec:
  port: 80
  protocol: http
  config:
    timeout: 
      proxy_connect_timeout_ms: 3000
      proxy_send_timeout_ms: 3000
      proxy_read_timeout_ms: 3000
---
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  name: alb-t-00081
  labels:
    "alb2.cpaas.io/name": "alb-t"
spec:
  port: 81
  protocol: tcp
  serviceGroup:
    services:
    - name: demo
      namespace: default
      port: 80
      weight: 100
  config:
    timeout: 
      proxy_connect_timeout_ms: 3000
      proxy_read_timeout_ms: 3000
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: timeout-ingress
  annotations:
    nginx.ingress.kubernetes.io/proxy-connect-timeout: "10s"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "1000"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "1000"
spec:
  rules:
    - host: timeout.test.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: demo
                port:
                  number: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: timeout-ingress-1
  annotations:
    nginx.ingress.kubernetes.io/proxy-connect-timeout: "20s"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "2000"
    index.0-1.alb.ingress.cpaas.io/proxy-connect-timeout: "40s"
spec:
  rules:
    - host: timeout.test.com
      http:
        paths:
          - path: /x1
            pathType: Prefix
            backend:
              service:
                name: demo
                port:
                  number: 80
          - path: /x2
            pathType: Prefix
            backend:
              service:
                name: demo
                port:
                  number: 80
`)
			l.Info("ok", "ft", kt.AssertKubectl("get ft -A -o yaml"))
			policy, err := ing_ph.SyncIngressAndGetPolicyFromK8s(env.GetRestCfg(), "default", "alb-t", l)
			policy.CertificateMap = nil
			GinkgoNoErr(err)
			l.Info("policy", "policy", u.PrettyJson(policy))
			// policy应该有3个timeout的配置
			GinkgoAssertTrue(len(policy.SharedConfig) == 3, "")
			// 81端口(4层)的也有timeout
			GinkgoAssertTrue(policy.Stream.Tcp[v1.PortNumber(81)][0].Config.Timeout != nil, "")
		})
	})
})
