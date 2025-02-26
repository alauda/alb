package test

import (
	"context"
	"testing"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	. "alauda.io/alb2/pkg/controller/ngxconf"
	ptu "alauda.io/alb2/pkg/utils/test_utils"
	lu "alauda.io/alb2/utils"
	. "alauda.io/alb2/utils/test_utils"

	"github.com/go-logr/logr"
	"github.com/lithammer/dedent"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	f "alauda.io/alb2/test/e2e/framework"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

func TestKeepAlive(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "keepalive related test")
}

func TestNginxTemplate(t *testing.T) {
}

var _ = Describe("keepalive related test", func() {
	l := ConsoleLog()
	var env *EnvtestExt
	var kt *Kubectl
	var kc *K8sClient
	BeforeEach(func() {
		env = NewEnvtestExt(InitBase(), GinkgoLog())
		env.AssertStart()
		kt = env.Kubectl()
		l.Info("start")
		kc = NewK8sClient(context.Background(), env.GetRestCfg())

		svcExt := f.NewSvcExt(kc, context.Background())
		svcExt.InitSvcWithOpt(f.SvcOpt{
			Ns:    "cpaas-system",
			Name:  "xx",
			Ep:    []string{"192.0.0.1"},
			Ports: []corev1.ServicePort{{Port: 8080}},
		})
		_ = l
		_ = kt
		_ = kc
	})

	AfterEach(func() {
		env.Stop()
	})

	It("should parse keepalive configuration correctly", func() {
		kt.AssertKubectlApply(dedent.Dedent(`
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: a1
    namespace: cpaas-system
spec:
    type: "nginx"
---
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
    labels:
        alb2.cpaas.io/name: a1
    name: a1-00081
    namespace: cpaas-system
spec:
    port: 81
    protocol: tcp
    config:
        keepalive:
          tcp:
            idle: 60m
            interval: 30s
            count: 3
          http:
            timeout: 60m
            requests: 1000
---
apiVersion: crd.alauda.io/v1
kind: Frontend
metadata:
    labels:
        alb2.cpaas.io/name: a1
    name: a1-00080
    namespace: cpaas-system
spec:
    port: 80
    protocol: http
    config:
        keepalive:
          tcp:
            idle: 60m
            interval: 30s
            count: 3
          http:
            timeout: 60m
            requests: 1000
            `))

		l.Info("test keepalive")
		ngxconf, err := RenderNginxConfigFromK8s(env.GetRestCfg(), l, "a1", "cpaas-system")
		GinkgoNoErr(err)
		l.Info("ngxconf", "ngxconf", ngxconf)
	})
})

func RenderNginxConfigFromK8s(restCfg *rest.Config, l logr.Logger, alb, ns string) (string, error) {
	mock := config.Mock(alb, ns)
	ctx := context.Background()
	drv, err := driver.NewDriver(driver.DrvOpt{Ctx: ctx, Cf: restCfg, Opt: driver.Cfg2opt(mock)})
	if err != nil {
		return "", err
	}
	_, tmpl, err := ptu.GetPolicyAndNgx(ptu.PolicyGetCtx{
		Ctx: ctx, Name: alb, Ns: ns, Drv: drv, L: l,
		Cfg: mock,
	})
	if err != nil {
		return "", err
	}
	l.Info("tmpl", "tmpl", lu.PrettyJson(tmpl))
	ngxconf, err := RenderNginxConfigEmbed(*tmpl)
	if err != nil {
		return "", err
	}
	return ngxconf, nil
}
