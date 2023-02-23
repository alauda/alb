package chart

// test for render alb chart
import (
	"context"
	"os"
	"path"
	"testing"

	f "alauda.io/alb2/test/e2e/framework"
	"alauda.io/alb2/utils"
	tu "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	_ "github.com/kr/pretty"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
)

var testEnv *envtest.Environment
var kubecfg *rest.Config
var albRoot string
var testBase string
var helm *f.Helm
var kt *tu.Kubectl
var kc *tu.K8sClient

func TestChart(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "alb chart suite")
}

var _ = Describe("chart", func() {
	var (
		albBase         = f.GetAlbRoot()
		chartBase       = path.Join(albBase, "/deploy/chart/alb")
		chartDefaultVal = path.Join(chartBase, "values.yaml")
	)
	var ctx context.Context
	var cancel context.CancelFunc
	var l logr.Logger
	BeforeEach(func() {
		albRoot = os.Getenv("ALB_ROOT")
		testBase = os.TempDir() + "/" + utils.RandomStr("alb-chart-test", 3)
		os.MkdirAll(testBase, os.ModePerm)
		localCfg := os.Getenv("USE_LOCAL_KUBECFG")
		ctx, cancel = context.WithCancel(context.Background())
		l = tu.GinkgoLog()
		if localCfg == "" {
			testEnv = &envtest.Environment{}
			cfg, err := testEnv.Start()
			assert.NoError(GinkgoT(), err)
			kubecfg = cfg
			f.Logf("use envtest")
		} else {
			f.Logf("use local cfg")
			cf, _ := clientcmd.BuildConfigFromFlags("", localCfg)
			kubecfg = cf
		}

		helm = f.NewHelm(testBase, kubecfg)
		kt = tu.NewKubectl(testBase, kubecfg, tu.GinkgoLog())
		kc = tu.NewK8sClient(ctx, kubecfg)
		// install feature crd
		kt.AssertKubectl("apply", "-f", path.Join(albRoot, "scripts", "yaml", "crds", "extra", "v1"))
		kc.CreateNsIfNotExist("cpaas-system")
		f.Logf("clean helm")
		helm.AssertUnInstallAll()
	})

	AfterEach(func() {
		cancel()
		if os.Getenv("USE_LOCAL_KUBECFG") == "" {
			err := testEnv.Stop()
			assert.NoError(GinkgoT(), err)
		}
		l.Info("cancel")
	})

	f.GIt("deploy alb and csv", func() {
		// TODO 注意从sentry迁移的名字变了。感觉需要优化下
		cfgs := []string{
			`
            displayName: "x"
            address: "192.168.134.195"
            global:
              labelBaseDomain: cpaas.io
              namespace: cpaas-system
              registry:
                address: build-harbor.alauda.cn
            loadbalancerName: ares-alb2
            nodeSelector:
                kubernetes.io/hostname: 192.168.134.195
            gateway:
                enable: true
                mode: gatewayclass
            replicas: 1
            `,
		}
		name := "operator"
		helm.AssertInstall(cfgs, name, chartBase, chartDefaultVal)
		alb := &albv2.ALB2{}
		err := kc.GetClient().Get(ctx, types.NamespacedName{Namespace: "cpaas-system", Name: "ares-alb2"}, alb)
		f.GinkgoNoErr(err)
		assert.Equal(GinkgoT(), "ares-alb2", *alb.Spec.Config.LoadbalancerName)
		l.Info("alb", "annotation", alb.Annotations["alb.cpaas.io/migrate-backup"])
		// TODO add test here
	})
})
