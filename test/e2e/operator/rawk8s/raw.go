package rawk8s

import (
	"context"

	. "alauda.io/alb2/test/e2e/framework"
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "alauda.io/alb2/utils/test_utils"
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = Describe("raw k8s", func() {
	var env *OperatorEnv
	var kt *Kubectl
	var kc *K8sClient
	var cli client.Client
	var ctx context.Context
	var log logr.Logger
	BeforeEach(func() {
		env = StartOperatorEnvOrDieWithOpt(OperatorEnvCfg{RunOpext: true}, func(e *OperatorEnv) {
			e.InitK8s = func(ctx context.Context, base string, cfg *rest.Config, l logr.Logger) error {
				// delete the feature crd to mock raw k8s
				kt := NewKubectl(base, cfg, l)
				l.Info(kt.AssertKubectl("delete", "crd", "features.infrastructure.alauda.io"))
				return nil
			}
		})
		kt = env.Kt
		kc = env.Kc
		ctx = env.Ctx
		cli = env.Kc.GetClient()
		log = env.Log
		_ = kc
		_ = kt
		_ = cli
		_ = ctx
		_ = log
	})

	AfterEach(func() {
		env.Stop()
	})
	It("should ok in raw k8s", func() {
		log.Info("in test")
		ns := "cpaas-system"
		name := "a1"
		alb := `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: a1
    namespace: cpaas-system
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        loadbalancerName: a1
        replicas: 1
        `
		kt.AssertKubectlApply(alb)
		EventuallySuccess(func(g gomega.Gomega) {
			_, err := kt.Kubectl("get svc -n " + ns + " " + name)
			GNoErr(g, err)
		}, log)
	})
})
