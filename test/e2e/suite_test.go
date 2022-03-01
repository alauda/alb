package e2e

import (
	"alauda.io/alb2/test/e2e/framework"
	"github.com/stretchr/testify/assert"

	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var cfg *rest.Config
var testEnv *envtest.Environment

var _ = BeforeSuite(func() {

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "../", "chart", "crds/alb"),
			filepath.Join("..", "../", "chart", "crds/gateway/v1alpha2/experimental"),
		},
	}

	cfg, err := testEnv.Start()
	assert.NoError(GinkgoT(), err)
	p, err := framework.InitKubeCfg(cfg)
	assert.NoError(GinkgoT(), err)
	framework.Logf("start up envtest %+v in kubecfg %v", cfg.Host, p)
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	assert.NotNil(GinkgoT(), testEnv, "test env is nil")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func TestALB(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "alb suite")
}
