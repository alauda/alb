package depl

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"testing"

	"alauda.io/alb2/pkg/operator/toolkit"

	gruntime "runtime"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/pkg/operator/config"
	"alauda.io/alb2/utils"
	cliu "alauda.io/alb2/utils/client"
	tu "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/xorcare/pointer"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	testEnv *envtest.Environment
	rcfg    *rest.Config
	_       = BeforeSuite(func() {
		testEnv = &envtest.Environment{}
		var err error

		rcfg, err = testEnv.Start()
		if err != nil {
			panic(err)
		}
		assert.NoError(GinkgoT(), err)
		_, filename, _, _ := gruntime.Caller(0)
		albBase := path.Join(path.Dir(filename), "../../../../")

		err = tu.InitAlbCr(albBase, rcfg)
		if err != nil {
			panic(err)
		}
	})
)

var _ = AfterSuite(func() {
	err := testEnv.Stop()
	assert.NoError(GinkgoT(), err)
})

func TestDepl(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "alb operator")
}

var _ = Describe("Operator Depl", func() {
	var base string
	var kubectl *tu.Kubectl
	var cli client.Client
	var ctx context.Context
	var kc *tu.K8sClient

	var cancel func()
	var log logr.Logger
	var t GinkgoTInterface
	BeforeEach(func() {
		log = tu.GinkgoLog()
		ctx, cancel = context.WithCancel(context.Background())
		base = tu.InitBase()
		kubectl = tu.NewKubectl(base, rcfg, log)
		kc = tu.NewK8sClient(ctx, rcfg)
		kc.CreateNsIfNotExist("cpaas-system")
		var err error
		cli, err = cliu.GetClient(ctx, rcfg, cliu.InitScheme(runtime.NewScheme()))
		t = GinkgoT()
		assert.NoError(t, err)
		_ = ctx
		_ = log
	})
	AfterEach(func() {
		cancel()
	})

	It("test pretty cr", func() {
		kubectl.AssertKubectlApply(`
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
  name: alb-v1
  namespace: cpaas-system
spec:
  address: 127.0.0.1
  type: nginx
  config:
    networkMode: host
`)
		alb := &albv2.ALB2{}
		cli.Get(ctx, client.ObjectKey{Namespace: "cpaas-system", Name: "alb-v2"}, alb)
		fmt.Printf("%v", toolkit.PrettyCr(alb))
	})
	It("test depl and load", func() {
		kubectl.AssertKubectlApply(`
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
  name: alb-v2
  namespace: cpaas-system
spec:
  address: 127.0.0.1
  type: nginx
  config:
    networkMode: host
    replicas: 1
    projects:
      - ALL_ALL
`)
		alb := &albv2.ALB2{}
		cli.Get(ctx, client.ObjectKey{Namespace: "cpaas-system", Name: "alb-v2"}, alb)
		assert.Equal(t, alb.APIVersion, "crd.alauda.io/v2beta1")
		assert.Equal(t, alb.Kind, "ALB2")
		cur, err := LoadAlbDeploy(ctx, cli, log, types.NamespacedName{Namespace: "cpaas-system", Name: "alb-v2"}, config.DEFAULT_OPERATOR_CFG)
		assert.NoError(t, err)
		conf, err := config.NewALB2Config(cur.Alb, config.DEFAULT_OPERATOR_CFG, log)
		assert.NoError(t, err)
		cfg := config.Config{
			ALB:      *conf,
			Operator: config.DEFAULT_OPERATOR_CFG,
		}
		dep := NewAlbDeployCtl(ctx, cli, cfg, log)
		assert.NoError(t, err)
		exp, err := dep.GenExpectAlbDeploy(ctx, cur)
		assert.NoError(t, err)
		_, err = dep.DoUpdate(ctx, cur, exp)
		assert.NoError(t, err)
	})

	It("test migration", func() {
		type tcase struct {
			annotaion string
			cfg       albv2.ExternalAlbConfig
		}
		cases := []tcase{
			{
				annotaion: `{"replicas":10}`,
				cfg:       albv2.ExternalAlbConfig{Replicas: pointer.Int(10)},
			},
			{
				annotaion: `{"resources":{"limits":{"cpu":"200m"}}}`,
				cfg: albv2.ExternalAlbConfig{Resources: &albv2.ExternalResources{
					ExternalResource: &albv2.ExternalResource{
						Limits: &albv2.ContainerResource{
							CPU: "200m",
						},
					},
				}},
			},
			{
				annotaion: `{"resources":{"limits":{"cpu":2}}}`,
				cfg: albv2.ExternalAlbConfig{Resources: &albv2.ExternalResources{
					ExternalResource: &albv2.ExternalResource{
						Limits: &albv2.ContainerResource{
							CPU: "2",
						},
					},
				}},
			},
			{
				annotaion: `{"resources":{"limits":{"cpu":"2"}}}`,
				cfg: albv2.ExternalAlbConfig{Resources: &albv2.ExternalResources{
					ExternalResource: &albv2.ExternalResource{
						Limits: &albv2.ContainerResource{
							CPU: "2",
						},
					},
				}},
			},
		}

		for _, c := range cases {
			cfg := albv2.ExternalAlbConfig{}
			err := json.Unmarshal([]byte(c.annotaion), &cfg)
			assert.NoError(t, err)
			t.Logf("cfg %v", utils.PrettyJson(cfg))
			assert.Equal(t, cfg, c.cfg)
		}
	})
})
