package e2e

import (
	"alauda.io/alb2/test/e2e/framework"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"testing"
)

var cfg *rest.Config
var testEnv *envtest.Environment

var _ = BeforeSuite(func() {
	albCrdDir := filepath.Join("..", "../", "chart", "crds")

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{albCrdDir},
	}

	cfg, err := testEnv.Start()
	framework.Logf("start up envtest host %v", cfg.Host)

	kubeconfig := os.Getenv("ENV_TEST_KUBECONFIG")
	if kubeconfig != "" {
		os.RemoveAll(kubeconfig)
		os.WriteFile(kubeconfig, []byte(fmt.Sprintf(`apiVersion: v1
clusters:
- cluster:
    server: http://%s
  name: env-test
contexts:
- context:
    cluster: env-test
    user: env
  name: env
current-context: env
kind: Config
preferences: {}
users:
- name: env`, cfg.Host)), os.ModePerm)
	}

	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())
	framework.EnvTestCfgToEnv(cfg)
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
