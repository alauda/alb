package gateway

import (
	"context"
	"strings"

	ct "alauda.io/alb2/controller/types"
	. "alauda.io/alb2/test/e2e/framework"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
)

var _ = ginkgo.Describe("container-mode-gateway", func() {
	var f *Framework
	var ctx context.Context
	var ns string
	var svc1DefaultBackends ct.Backends
	ginkgo.BeforeEach(func() {
		f = NewAlb(NewContainerModeCfg("alb-test/g1", false, KUBE_REST_CONFIG, "test-gateway"))
		GinkgoAssert(f.CreateGatewayClass("gclass"), "init gatewayclass fail")
		ns = f.InitProductNs("alb-test", "")
		f.InitDefaultSvc("svc-1", []string{"192.168.1.1", "192.168.1.2"})
		f.InitDefaultSvc("svc-2", []string{"192.168.2.1"})
		svc1DefaultBackends = ct.Backends{
			{Address: "192.168.1.1", Port: 80, Weight: 50},
			{Address: "192.168.1.2", Port: 80, Weight: 50},
		}

		ctx = context.Background()
		f.Init()
	})

	ginkgo.AfterEach(func() {
		_ = ctx
		_ = ns
		f.Destroy()
		f = nil
	})

	GIt("should get address from lb svc", func() {
		f.AssertKubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
    name: g1
    namespace: {{.ns}}
spec:
    gatewayClassName: gclass
    listeners:
    - name: http
      port: 8234
      protocol: HTTP
      hostname: a.com
      allowedRoutes:
        namespaces:
          from: All
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
    name: g2
    namespace: {{.ns}}
spec:
    gatewayClassName: gclass
    listeners:
    - name: http
      port: 8235
      protocol: HTTP
      hostname: a.com
      allowedRoutes:
        namespaces:
          from: All
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: HTTPRoute
metadata:
    name: h1
    namespace: {{.ns}}
spec:
    hostnames: ["a.com"]
    parentRefs:
      - kind: Gateway
        namespace: {{.ns}}
        name: g1
        sectionName: http
    rules:
    - matches:
      - path:
          value: "/foo"
      backendRefs:
        - kind: Service
          name: svc-1
          namespace: {{.ns}}
          port: 80
          weight: 1
`, map[string]interface{}{
			"ns": ns,
		}))
		f.SetSvcLBIp("cpaas-system", "test-gateway-tcp", "192.168.3.1")
		f.WaitNginxConfigStr("listen.*8234")
		f.WaitGateway(ns, "g1", func(g Gateway) (bool, error) {
			return g.Ready() && g.SameAddress([]string{"192.168.3.1"}), nil
		})
		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			ruleName := "8234-alb-test-g1-http-alb-test-h1-0-0"
			expectedDsl := `["AND",["IN","HOST","a.com"],["STARTS_WITH","URL","/foo"]]`
			return p.PolicyEq("http", ruleName, 8234, expectedDsl, BuildBG(ruleName, "http", svc1DefaultBackends))
		})
		// only reconcile select gateway
		f.WaitPolicy(func(raw string) bool {
			return !strings.Contains(raw, "8235-alb-test")
		})

		f.AssertService("cpaas-system", "test-gateway-tcp", &ServiceAssertCfg{
			Ports: map[string]ServiceAssertPort{
				"http-8234": {
					Port:     8234,
					Protocol: "TCP",
					NodePort: nil,
				},
			},
		})
		f.AssertService("cpaas-system", "test-gateway-udp", nil, func(s *corev1.Service) (bool, error) {
			return len(s.Spec.Ports) == 1, nil
		})
	})
})
