package e2e

import (
	"alauda.io/alb2/test/e2e/framework"
	"github.com/onsi/ginkgo"
)

var _ = ginkgo.Describe("Ingress", func() {
	var f *framework.Framework

	ginkgo.BeforeEach(func() {
		f = framework.NewAlb(framework.Config{RandomBaseDir: false, RestCfg: framework.CfgFromEnv()})
		f.Init()
	})
	ginkgo.AfterEach(func() {
		f.Destroy()
		f = nil
	})
	ginkgo.Context("basic ingress", func() {
		ginkgo.FIt("should translate ingress to rule and generate nginx config,policy.new", func() {
			framework.Logf("ok")
			// create ingress
			//f.EnsureV1Ingress()
			//Expect(false).To(BeTrue())
			//f.WaitAndAssertNginxConfig("80")
			//f.WaitAndAssertPolicyNew("/policy")
		})

		ginkgo.It("should ok when ingress has a long long name", func() {

		})
	})
})
