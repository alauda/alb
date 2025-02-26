package test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ing_ph "alauda.io/alb2/pkg/utils/test_utils/ing_utils"
	f "alauda.io/alb2/test/e2e/framework"
	lu "alauda.io/alb2/utils"
	. "alauda.io/alb2/utils/test_utils"
	corev1 "k8s.io/api/core/v1"
)

func TestRedirect(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "redirect related test")
}

var _ = Describe("redirect related test", func() {
	l := ConsoleLog()
	var env *EnvtestExt
	var kt *Kubectl
	var kc *K8sClient
	var ctx context.Context
	BeforeEach(func() {
		env = NewEnvtestExt(InitBase(), GinkgoLog())
		env.AssertStart()
		kt = env.Kubectl()
		l.Info("start")
		ctx = context.Background()
		kc = NewK8sClient(ctx, env.GetRestCfg())

		svcExt := f.NewSvcExt(kc, ctx)
		svcExt.InitSvcWithOpt(f.SvcOpt{
			Ns:    "default",
			Name:  "demo",
			Ep:    []string{"192.168.1.1"},
			Ports: []corev1.ServicePort{{Port: 80}},
		})
		_ = l
		_ = kt
		_ = kc
	})

	AfterEach(func() {
		env.Stop()
	})

	It("should create default policy if ssl-redirect in ft", func() {
		kt.AssertKubectlApply(`
apiVersion: crd.alauda.io/v2
kind: ALB2
metadata:
    name: alb-t
spec:
    type: "nginx"
    config:
        projects: ["ALL_ALL"]
        ingressSSLRedirect: true
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
---
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  name: alb-t-08000
  labels:
    "alb2.cpaas.io/name": "alb-t"
spec:
  port: 8000
  protocol: http
  serviceGroup:
    services:
    - name: demo
      namespace: default
      port: 80
      weight: 100
  config:
    redirect:
      scheme: https
---
apiVersion: crd.alauda.io/v1
kind: Rule
metadata:
  labels:
    alb2.cpaas.io/frontend: alb-t-08000
    alb2.cpaas.io/name: alb-t
  name: alb-t-08000-test
spec:
  dslx:
  - type: HOST
    values:
    - - EQ
      - demo.com
  priority: 1
  serviceGroup:
    services:
    - name: demo
      namespace: default
      port: 80
      weight: 100
---
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
  name: alb-t-08001
  labels:
    "alb2.cpaas.io/name": "alb-t"
spec:
  port: 8001
  protocol: http
  serviceGroup:
    services:
    - name: demo
      namespace: default
      port: 80
      weight: 100
---
apiVersion: crd.alauda.io/v1
kind: Rule
metadata:
  labels:
    alb2.cpaas.io/frontend: alb-t-08001
    alb2.cpaas.io/name: alb-t
  name: alb-t-08001-test
spec:
  dslx:
  - type: HOST
    values:
    - - EQ
      - demo.com
  priority: 1
  serviceGroup:
    services:
    - name: demo
      namespace: default
      port: 80
      weight: 100
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: common-ingress
spec:
  rules:
    - http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: demo
                port:
                  number: 80
`)
		// 测试场景
		// 1. alb开启ingress ssl-redirect。默认80端口下多一个policy，优先级最高，匹配所有rule，scheme=https
		policy, err := ing_ph.SyncIngressAndGetPolicyFromK8s(env.GetRestCfg(), "default", "alb-t", l)
		GinkgoNoErr(err)
		policy.CertificateMap = nil
		l.Info("policy", "policy", lu.PrettyJson(policy))
		// alb 配置 ssl-redirect + 80端口无配置 + 有ingress  应该有2个policy。一个是ingress的一个是redirect的
		GinkgoAssertTrue(len(policy.Http.Tcp[80]) == 2, "")
		GinkgoAssertJsonEq(policy.Http.Tcp[80][0].Config.Redirect, `{"code":308,"scheme":"https"}`, "")
		GinkgoAssertTrue(policy.Http.Tcp[80][0].Plugins[0] == "redirect", "")
		GinkgoAssertTrue(len(policy.Http.Tcp[80][1].Plugins) == 0, "")
		// ft 配置 redirect 有默认后端. 优先级会给改成最高。并且配置redirect
		// 8000 有redirect 和 8001 没有redirect.注意8000的默认policy的优先级会优先匹配
		GinkgoAssertJsonEq(policy.Http.Tcp[8000][0].Config.Redirect, `{"scheme":"https"}`, "")
	})

	It("ingress redirect annotation", func() {
		kt.VerboseKubectl("get frontends.crd.alauda.io -A --ignore-not-found")
		te := f.TlsExt{
			Kc:  kc,
			Ctx: ctx,
		}
		te.CreateTlsSecret("demo.com", "demo-secret", "default")
		kt.AssertKubectlApply(`
apiVersion: crd.alauda.io/v2
kind: ALB2
metadata:
    name: alb-t
spec:
    type: "nginx"
    config:
        projects: ["ALL_ALL"]
        defaultSSLStrategy: "Both"
        defaultSSLCert: default/demo-secret
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: common-ingress
  annotations:
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    alb.ingress.cpaas.io/redirect-code: "308"
    alb.cpaas.io/ingress-rule-priority-0-0: "1"
    alb.cpaas.io/ingress-rule-priority-1-0: "2"
    alb.cpaas.io/ingress-rule-priority-2-0: "3"
    alb.cpaas.io/ingress-rule-priority-3-0: "4"
    index.0-0.alb.ingress.cpaas.io/redirect-host: "demo.demo.com"
    index.0-0.alb.ingress.cpaas.io/redirect-port: "22343"
    index.1-0.alb.ingress.cpaas.io/permanent-redirect: "https://demo-1.demo.com"
    index.2-0.alb.ingress.cpaas.io/temporal-redirect: "https://demo-2.demo.com"
    index.3-0.alb.ingress.cpaas.io/prefix-match: "/a"
    index.3-0.alb.ingress.cpaas.io/replace-prefix: "/b"
spec:
  tls:
  - hosts:
      - demo.com
    secretName: demo-secret
  rules:
    - host: demo.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: demo
                port:
                  number: 80
    - host: demo2.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: demo
                port:
                  number: 80
    - host: demo3.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: demo
                port:
                  number: 80
    - host: demo4.com
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
apiVersion: crd.alauda.io/v1
kind: Rule
metadata:
  labels:
    alb2.cpaas.io/frontend: alb-t-00080
    alb2.cpaas.io/name: alb-t
  name: alb-t-00080-test
spec:
  dslx:
  - type: HOST
    values:
    - - EQ
      - demo.com
  priority: 100
  redirectURL: "https://old.demo.com"
  redirectCode: 308
  serviceGroup:
    services:
    - name: demo
      namespace: default
      port: 80
      weight: 100

`)
		policy, err := ing_ph.SyncIngressAndGetPolicyFromK8s(env.GetRestCfg(), "default", "alb-t", l)
		GinkgoNoErr(err)
		policy.CertificateMap = nil
		l.Info("policy", "policy", lu.PrettyJson(policy))
		// ingress 上配置ssl-redirect=true，http的规则上有redirect配置，https的没有
		GinkgoAssertTrue(len(policy.Http.Tcp[80]) == 5, "")
		GinkgoAssertJsonEq(policy.Http.Tcp[80][0].Config.Redirect, `{"port":22343,"host":"demo.demo.com","code":308,"scheme":"https"}`, "")
		GinkgoAssertJsonEq(policy.Http.Tcp[80][1].Config.Redirect, `{"code":308,"scheme":"https","url":"https://demo-1.demo.com"}`, "")
		GinkgoAssertJsonEq(policy.Http.Tcp[443][0].Config.Redirect, `{"host":"demo.demo.com","code":308,"scheme":"https","port":22343}`, "")

		// rule上配置的redirect相关测配置会在policy中 在旧的配置中设置的code和url会覆盖新的配置。如果旧的配置没有，新的配置有，会用新的配置
		GinkgoAssertJsonEq(policy.Http.Tcp[80][4].Config.Redirect, `{"code":308,"url":"https://old.demo.com"}`, "")
	})
})
