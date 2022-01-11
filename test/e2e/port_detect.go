package e2e

import (
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/test/e2e/framework"
	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	"os"
)

var _ = ginkgo.Describe("Ingress", func() {
	var f *framework.Framework

	ginkgo.BeforeEach(func() {
		deployCfg := framework.Config{PortProbe: true, AlbName: "alb-dev", InstanceMode: true, RestCfg: framework.CfgFromEnv(), Project: []string{"project1"}}
		f = framework.NewAlb(deployCfg)
		f.InitProductNs("alb-test", "project1")
		f.InitDefaultSvc("svc-default", []string{"192.168.1.1", "192.168.2.2"})
		f.Init()
	})

	ginkgo.AfterEach(func() {
		f.Destroy()
		f = nil
	})

	framework.GIt("should detect tcp port conflict", func() {

		host, err := os.Hostname()
		if err != nil {
			assert.NoError(ginkgo.GinkgoT(), err)
		}
		ns := f.GetProductNs()
		// tcp conflict
		{
			stopCh := make(chan struct{}, 1)
			framework.ListenTcp("5553", stopCh)
			f.CreateFt(5553, "tcp", "svc-default", ns)
			f.WaitFtState("alb-dev-05553", func(ft *alb2v1.Frontend) (bool, error) {
				framework.Logf("ft status %+v", ft.Status)
				if ft.Status.Instances[host].Conflict == true {
					return true, nil
				}
				return false, nil
			})
			// it may take 2*LOCK_TIMEOUT secs
			f.WaitAlbState("alb-dev", func(alb *alb2v1.ALB2) (bool, error) {
				framework.Logf("alb status %+v", alb.Status)
				if alb.Status.Reason == "port conflict" {
					return true, nil
				}
				return false, nil
			})
			stopCh <- struct{}{}

			f.WaitFtState("alb-dev-05553", func(ft *alb2v1.Frontend) (bool, error) {
				framework.Logf("ft status %+v", ft.Status)
				if ft.Status.Instances[host].Conflict == false {
					return true, nil
				}
				return false, nil
			})
			f.WaitAlbState("alb-dev", func(alb *alb2v1.ALB2) (bool, error) {
				framework.Logf("alb status %+v", alb.Status)
				if alb.Status.State == "ready" {
					return true, nil
				}
				return false, nil
			})
		}
	})
})
