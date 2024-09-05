package gateway

import (
	ct "alauda.io/alb2/controller/types"
	. "alauda.io/alb2/pkg/utils/test_utils"
	. "alauda.io/alb2/test/e2e/framework"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/assert"
)

var _ = ginkgo.Describe("udp", func() {
	var f *GatewayF
	var env *Env
	var ns string

	ginkgo.BeforeEach(func() {
		f, env = DefaultGatewayF()
		f.InitProductNs("alb-test", "project1")
		f.InitDefaultSvc("svc-1", []string{"192.168.1.1", "192.168.1.2"})
		f.InitDefaultSvc("svc-2", []string{"192.168.2.1"})
		f.InitDefaultSvc("svc-udp", []string{"192.168.3.1", "192.168.3.2"})
		ns = f.GetProductNs()
	})

	ginkgo.AfterEach(func() {
		env.Stop()
	})

	GIt("i want my app been access by udp", func() {
		_, err := f.KubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
    name: g1
    namespace: {{.ns}}
spec:
    gatewayClassName:  {{.class}}
    listeners:
    - name: udp
      port: 8235
      protocol: UDP
      allowedRoutes:
        namespaces:
          from: All
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: UDPRoute
metadata:
    name: u1
    namespace: {{.ns}}
spec:
    parentRefs:
      - kind: Gateway
        namespace: {{.ns}}
        name: g1
        sectionName: udp
    rules:
      -
        backendRefs:
          - kind: Service
            name: svc-udp
            namespace: {{.ns}}
            port: 80
            weight: 1
          `, map[string]interface{}{"ns": ns, "class": f.AlbName}))
		assert.NoError(ginkgo.GinkgoT(), err)

		f.WaitNginxConfigStr("listen.*8235")
		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			name := "8235-" + ns + "-u1"
			return p.PolicyEq("udp", name, 8235, "null", ct.BackendGroup{
				Name: name,
				Mode: "udp",
				Backends: ct.Backends{
					{
						Address: "192.168.3.1",
						Port:    80,
						Weight:  50,
					},
					{
						Address: "192.168.3.2",
						Port:    80,
						Weight:  50,
					},
				},
			})
		})
	})
})
