package alb

import (
	"testing"

	"alauda.io/alb2/test/e2e/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var testEnv *envtest.Environment
var _ = BeforeSuite(func() {
	testEnv = &envtest.Environment{}
	cfg, err := testEnv.Start()
	assert.NoError(GinkgoT(), err)
	framework.AlbBeforeSuite(cfg)
})

var _ = AfterSuite(func() {
	framework.AlbAfterSuite()
	err := testEnv.Stop()
	assert.NoError(GinkgoT(), err)
})

func TestALB(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "alb suite")
}
