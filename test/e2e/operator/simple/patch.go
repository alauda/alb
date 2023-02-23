package simple

import (
	"context"

	f "alauda.io/alb2/test/e2e/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "alauda.io/alb2/utils/test_utils"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Operator", func() {
	var base string
	var opext *f.AlbOperatorExt
	var kt *Kubectl
	var kc *K8sClient
	var cli client.Client
	var ctx context.Context
	var cancel func()

	var log logr.Logger
	BeforeEach(func() {
		log = GinkgoLog()
		ctx, cancel = context.WithCancel(context.Background())
		base = InitBase()
		kt = NewKubectl(base, KUBE_REST_CONFIG, log)
		kc = NewK8sClient(ctx, KUBE_REST_CONFIG)
		msg := kt.AssertKubectl("create ns cpaas-system")
		log.Info("init ns", "ret", msg)
		opext = f.NewAlbOperatorExt(ctx, base, KUBE_REST_CONFIG)
		cli = kc.GetDirectClient()
	})

	AfterEach(func() {
		cancel()
	})

	f.GIt("should create/update correctly when has patch", func() {
		kt.AssertKubectlApply(`
kind: ConfigMap
apiVersion: v1
metadata:
  name: cfg-1 
  namespace: cpaas-system 
data:
  http: |
    test:1
  grpc_server: |
    test:1
  stream-common: |
    test:1
  stream-tcp: |
    test:1
  stream-udp: |
    test:1
  upstream: |
    test:1
  bind_nic: 
    test:1
`)
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
        replicas: 1
        bindNIC: '{"nic":["eth0"]}'
        overwrite:
          image:
             - alb: "alb:v1.2.1"
               nginx: "nginx:v1.2.1"
          configmap:
             - name: cpaas-system/cfg-1
        `
		opext.AssertDeploy(types.NamespacedName{Namespace: "cpaas-system", Name: "ares-alb2"}, alb, nil)
		depl := &appv1.Deployment{}
		deplkey := types.NamespacedName{Namespace: "cpaas-system", Name: "ares-alb2"}
		err := cli.Get(ctx, deplkey, depl)
		f.GinkgoNoErr(err)
		assert.Equal(GinkgoT(), "nginx:v1.2.1", depl.Spec.Template.Spec.Containers[0].Image)
		assert.Equal(GinkgoT(), "alb:v1.2.1", depl.Spec.Template.Spec.Containers[1].Image)
		cfgmap := &corev1.ConfigMap{}
		cfgmapkey := types.NamespacedName{Namespace: "cpaas-system", Name: "ares-alb2"}
		err = cli.Get(ctx, cfgmapkey, cfgmap)
		f.GinkgoNoErr(err)
		assert.Equal(GinkgoT(), cfgmap.Data["http"], "test:1\n")
		assert.Equal(GinkgoT(), cfgmap.Data["bind_nic"], "{\"nic\":[\"eth0\"]}")
	})
})
