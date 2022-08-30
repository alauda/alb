package alb

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"alauda.io/alb2/config"
	L "alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
)

func TestLeader(t *testing.T) {
	t.Logf("ok")
	RegisterFailHandler(Fail)
	RunSpecs(t, "leader election ")
}

var testEnv *envtest.Environment
var _ = BeforeSuite(func() {
})

var _ = AfterSuite(func() {
})

var _ = Describe("LeaderElection", func() {

	It("leader election should ok", func() {
		t := GinkgoT()
		testEnv = &envtest.Environment{}
		cfg, err := testEnv.Start()
		assert.NoError(GinkgoT(), err)
		G_CFG := cfg
		defer testEnv.Stop()

		macfg := config.DefaultMock()
		macfg.Pod = "pa"
		mbcfg := config.DefaultMock()
		mbcfg.Pod = "pb"
		k := NewKubectl("", G_CFG)
		k.AssertKubectl("create ns cpaas-system")
		log := L.L().WithName("leader-test")

		actx, acancel := context.WithCancel(context.Background())
		bctx, bcancel := context.WithCancel(context.Background())
		_ = bcancel

		la := NewLeaderElection(actx, macfg, G_CFG, log)
		lb := NewLeaderElection(bctx, mbcfg, G_CFG, log)

		quit := make(chan bool)

		go func() {
			Logf("la b %v", la.AmILeader())
			time.Sleep(1 * time.Second)
			Logf("la m %v", la.AmILeader())
			// graceful shutdown pod a
			acancel()
			Logf("la a %v", la.AmILeader())
			for {
				// pod b should be leader
				if lb.AmILeader() {
					break
				}
				time.Sleep(1 * time.Second)
			}
			assert.False(t, la.AmILeader())
			assert.True(t, lb.AmILeader())
			quit <- true
		}()
		go func() {
			err := lb.StartLeaderElectionLoop()
			assert.NoError(GinkgoT(), err)
		}()
		go func() {
			err := la.StartLeaderElectionLoop()
			assert.NoError(GinkgoT(), err)
		}()

		<-quit
		Logf("quit")
	})
})
