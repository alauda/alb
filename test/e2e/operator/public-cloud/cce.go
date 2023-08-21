package pc

import (
	"context"

	. "alauda.io/alb2/test/e2e/framework"
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ctl "alauda.io/alb2/controller"
	. "alauda.io/alb2/utils/test_utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("public-cloud cce", func() {
	var env *OperatorEnv
	var kt *Kubectl
	var kc *K8sClient
	var cli client.Client
	var ctx context.Context
	var log logr.Logger
	BeforeEach(func() {
		env = StartOperatorEnvOrDieWithOpt(OperatorEnvCfg{RunOpext: true}, func(e *OperatorEnv) {
			e.InitK8s = func(ctx context.Context, base string, cfg *rest.Config, l logr.Logger) error {
				kt := NewKubectl(base, cfg, l)
				kc := NewK8sClient(ctx, cfg)
				kc.CreateNsIfNotExist("kube-public")

				kt.AssertKubectlApply(`
                  apiVersion: v1
                  data:
                    clusterName: cce
                    clusterType: HuaweiCloudCCE
                  kind: ConfigMap
                  metadata:
                      name: global-info
                      namespace: kube-public
                `)
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

	GIt("cce create lb svc with signlestack", func() {
		ns := "cpaas-system"
		name := "ares-alb2"
		alb := `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: ares-alb2
    namespace: cpaas-system
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        loadbalancerName: ares-alb2
        networkMode: container
        vip:
            enableLbSvc: true
        replicas: 1
        `
		kt.AssertKubectlApply(alb)
		EventuallySuccess(func(g Gomega) {
			log.Info("check lbsvc")
			svc, err := ctl.GetLbSvc(ctx, kc.GetK8sClient(), crcli.ObjectKey{Namespace: ns, Name: name}, "cpaas.io")

			g.Expect(err).ShouldNot(HaveOccurred())
			log.Info("svc ", "svc", PrettyCr(svc))
			g.Expect(string(*svc.Spec.IPFamilyPolicy)).Should(Equal("SingleStack"))
		}, log)

	})

})
