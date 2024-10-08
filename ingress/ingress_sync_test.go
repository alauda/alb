package ingress

import (
	"context"
	"testing"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestIngressGenRule(t *testing.T) {
	ctx := context.Background()
	gcf := config.DefaultMock()
	config.UseMock(gcf)
	base := InitBase()
	l := log.InitKlogV2(log.LogCfg{ToFile: base + "/unit-test.log"})
	env := NewEnvtestExt(base, l)
	kcfg := env.AssertStart()
	defer env.Stop()
	kt := NewKubectl(base, kcfg, l)
	kc := NewK8sClient(ctx, kcfg)

	ingYaml := `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    alb.cpaas.io/ingress-rule-priority-0-0: "-1"
    alb.cpaas.io/ingress-rule-priority-0-1: "3"
  name: i1 
  namespace: cpaas-system
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: apollo
            port:
              number: 8080
        path: /p2
        pathType: ImplementationSpecific
      - backend:
          service:
            name: apollo
            port:
              number: 8080
        path: /p1
        pathType: ImplementationSpecific
`
	kt.AssertKubectlApply(ingYaml)
	drv, err := driver.NewDriver(driver.DrvOpt{
		Ctx: ctx,
		Cf:  kcfg,
		Opt: driver.Cfg2opt(gcf),
	})
	assert.NoError(t, err)
	ingc := NewController(drv, drv.Informers, gcf, log.L().WithName("ingress"))

	ing, err := kc.GetK8sClient().NetworkingV1().Ingresses("cpaas-system").Get(ctx, "i1", metav1.GetOptions{})
	assert.NoError(t, err)

	ft := &albv1.Frontend{}
	for ri, r := range ing.Spec.Rules {
		for pi := range r.HTTP.Paths {
			albrule, err := ingc.GenerateRule(ing, crcli.ObjectKey{Namespace: "x", Name: "x"}, ft, ri, pi, "")
			l.Info("rule", "cr", PrettyCr(albrule))
			assert.NoError(t, err)
		}
	}
}
