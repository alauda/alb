package ingress

import (
	"testing"

	"alauda.io/alb2/test/e2e/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var testEnv *envtest.Environment
var KUBE_REST_CONFIG *rest.Config
var _ = BeforeSuite(func() {
	testEnv = &envtest.Environment{}
	var err error
	KUBE_REST_CONFIG, err = testEnv.Start()
	assert.NoError(GinkgoT(), err)
	// 兼容一下老的写法，在每个suit_test测试中应该保证kubecfg的初始化,等所有的cfgfromenv都删掉后，可以删除这个
	framework.AlbBeforeSuite(KUBE_REST_CONFIG)
})

var _ = AfterSuite(func() {
	framework.AlbAfterSuite()
	err := testEnv.Stop()
	assert.NoError(GinkgoT(), err)
})

func TestIngress(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ingress related suite")
}
