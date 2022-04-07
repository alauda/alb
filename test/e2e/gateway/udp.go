package gateway

import (
	"context"

	ct "alauda.io/alb2/controller/types"
	. "alauda.io/alb2/test/e2e/framework"
	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
)

var _ = ginkgo.Describe("udp", func() {
	var f *Framework
	var ctx context.Context
	var ns string

	ginkgo.BeforeEach(func() {
		deployCfg := Config{InstanceMode: true, RestCfg: CfgFromEnv(), Project: []string{"project1"}, Gateway: true}
		f = NewAlb(deployCfg)
		f.InitProductNs("alb-test", "project1")
		f.InitDefaultSvc("svc-1", []string{"192.168.1.1", "192.168.1.2"})
		f.InitDefaultSvc("svc-2", []string{"192.168.2.1"})
		f.InitDefaultSvc("svc-udp", []string{"192.168.3.1", "192.168.3.2"})
		f.Init()
		ctx = context.Background()
		ns = f.GetProductNs()
	})

	ginkgo.AfterEach(func() {
		f.Destroy()
		_ = ctx
		f = nil
	})

	GIt("i want my app been access by udp", func() {
		_, err := f.KubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1alpha2
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
