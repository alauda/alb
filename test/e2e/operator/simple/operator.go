package simple

import (
	"context"

	"alauda.io/alb2/pkg/operator/config"
	"alauda.io/alb2/pkg/operator/controllers"
	f "alauda.io/alb2/test/e2e/framework"
	"alauda.io/alb2/utils"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
)

var _ = Describe("TestReConcile", func() {
	var log logr.Logger

	var base string
	var kt *Kubectl
	var kc *K8sClient
	var cli client.Client
	var ctx context.Context
	var cancel func()

	BeforeEach(func() {
		log = GinkgoLog()
		ctx, cancel = context.WithCancel(context.Background())
		base = InitBase()
		kt = NewKubectl(base, KUBE_REST_CONFIG, log)
		kc = NewK8sClient(ctx, KUBE_REST_CONFIG)
		msg := kt.AssertKubectl("create ns cpaas-system")
		log.Info("init ns", "ret", msg)
		cli = kc.GetClient()
	})

	AfterEach(func() {
		cancel()
	})

	f.GIt("should bring config back", func() {
		// 在什么情况下 会有可能在etcd中存在一个不符合crd约束的数据呢。。。
		kt.AssertKubectlApply(`
apiVersion: crd.alauda.io/v1
kind: ALB2
metadata:
  name: alb-v2
  namespace: cpaas-system
  annotations:
    alb.cpaas.io/migrate-backup: | 
      {"replicas":10}
spec:
  address: 127.0.0.1
  type: nginx
`)
		v2alb := &albv2.ALB2{}
		key := types.NamespacedName{Namespace: "cpaas-system", Name: "alb-v2"}
		err := cli.Get(ctx, key, v2alb)
		GinkgoNoErr(err)
		log.Info("xx", "v2alb", utils.PrettyJson(v2alb))

		ctl := controllers.ALB2Reconciler{
			Client:     cli,
			OperatorCf: config.DEFAULT_OPERATOR_CFG,
			Log:        log,
		}
		reque, err := ctl.HandleBackupAnnotation(ctx, v2alb)
		log.Info("xx", "req", reque)
		assert.Equal(GinkgoT(), reque, true)
		GinkgoNoErr(err)
		err = kc.GetDirectClient().Get(ctx, key, v2alb)
		GinkgoNoErr(err)
		log.Info("xx", "v2alb", utils.PrettyJson(v2alb))
		assert.Equal(GinkgoT(), *v2alb.Spec.Config.Replicas, 10)

		kt.AssertKubectlApply(`
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
  name: alb-v2-1
  namespace: cpaas-system
  annotations:
    alb.cpaas.io/migrate-backup: | 
      {"replicas":"10"}
spec:
  address: 127.0.0.1
  type: nginx
  config:
      replicas: 1
`)
		v2alb = &albv2.ALB2{}
		key = types.NamespacedName{Namespace: "cpaas-system", Name: "alb-v2-1"}
		err = cli.Get(ctx, key, v2alb)
		GinkgoNoErr(err)
		log.Info("xx", "v2alb", utils.PrettyJson(v2alb))

		reque, err = ctl.HandleBackupAnnotation(ctx, v2alb)
		assert.Equal(GinkgoT(), reque, false)
		log.Info("xx", "req", reque)
		GinkgoNoErr(err)
		err = kc.GetDirectClient().Get(ctx, key, v2alb)
		GinkgoNoErr(err)
		assert.Equal(GinkgoT(), *v2alb.Spec.Config.Replicas, 1)

	})
})
