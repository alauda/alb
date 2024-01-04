package policyattachment

import (
	"context"
	"encoding/json"
	"fmt"

	gatewayPolicy "alauda.io/alb2/pkg/apis/alauda/gateway/v1alpha1"
	. "alauda.io/alb2/test/e2e/framework"
	. "alauda.io/alb2/utils/test_utils"
	. "github.com/onsi/ginkgo"
	"github.com/openlyinc/pointy"
	"github.com/stretchr/testify/assert"

	. "alauda.io/alb2/test/e2e/gateway"
	corev1 "k8s.io/api/core/v1"
)

func TimeoutEq(timeout gatewayPolicy.TimeoutPolicyConfig, connect *uint, read *uint, send *uint) bool {
	timeoutNew := gatewayPolicy.TimeoutPolicyConfig{
		ProxyConnectTimeoutMs: connect,
		ProxySendTimeoutMs:    send,
		ProxyReadTimeoutMs:    read,
	}
	timeoutJson, _ := json.Marshal(timeout)
	timeoutNewJson, _ := json.Marshal(timeoutNew)
	GinkgoT().Logf("left %s right %s", timeoutJson, timeoutNewJson)
	return string(timeoutJson) == string(timeoutNewJson)
}

func initDefaultGateway(f *GatewayF) {
	_, err := f.KubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
    name: g1 
    namespace: {{.ns}}
spec:
    gatewayClassName:  {{.class}}
    listeners:
    - name: tcp
      port: 81
      protocol: TCP
      allowedRoutes:
        namespaces:
            from: All
    - name: http
      port: 80
      protocol: HTTP
      hostname: a.com
      allowedRoutes:
        namespaces:
            from: All
    - name: https
      port: 443
      protocol: HTTPS
      hostname: a.com
      tls:
        mode: Terminate
        certificateRefs:
          - name: secret-a
            kind: Secret
            namespace: {{.ns}}
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
    gatewayClassName:  {{.class}}
    listeners:
    - name: http
      port: 90
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
          value: "/bar"
      backendRefs:
        - kind: Service
          name: svc-1
          namespace: {{.ns}}
          port: 80
          weight: 1
    - matches:
      - path:
          value: "/foo"
      backendRefs:
        - kind: Service
          name: svc-1
          namespace: {{.ns}}
          port: 80
          weight: 1
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: HTTPRoute
metadata:
    name: h2
    namespace: {{.ns}}
spec:
    hostnames: ["a.com"]
    parentRefs:
    - kind: Gateway
      namespace: {{.ns}}
      name: g1
      sectionName: http
    - kind: Gateway
      namespace: {{.ns}}
      name: g1
      sectionName: https
    - kind: Gateway
      namespace: {{.ns}}
      name: g2
      sectionName: http
    rules:
    - matches:
      - path:
          value: "/bar"
      backendRefs:
        - kind: Service
          name: svc-1
          namespace: {{.ns}}
          port: 80
          weight: 1
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: TCPRoute
metadata:
    name: h3
    namespace: {{.ns}}
spec:
    parentRefs:
      - kind: Gateway
        namespace: {{.ns}}
        name: g1
        sectionName: tcp
    rules:
      -
        backendRefs:
          - kind: Service
            name: svc-1
            namespace: {{.ns}}
            port: 80
            weight: 1

`, map[string]interface{}{"ns": f.GetProductNs(), "class": f.AlbName}))
	assert.NoError(GinkgoT(), err)
}

var _ = Describe("GatewayPolicyAttachment", func() {
	var f *GatewayF
	var env *Env
	var ctx context.Context
	var ns string
	var secretA *corev1.Secret

	BeforeEach(func() {
		f, env = DefaultGatewayF()
		f.InitProductNs("alb-test", "project1")
		f.InitDefaultSvc("svc-1", []string{"192.168.1.1", "192.168.1.2"})
		f.InitDefaultSvc("svc-2", []string{"192.168.2.1"})
		ctx = env.Ctx
		ns = f.GetProductNs()

		secretA, _ = f.CreateTlsSecret("a.com", "secreta", f.GetProductNs())
		initDefaultGateway(f)
	})

	AfterEach(func() {
		env.Stop()
		_ = ctx
		_ = ns
		_ = secretA
	})

	GIt("timeoutpolicy could attach to https listener", func() {
		_, err := f.KubectlApply(Template(`
apiVersion: gateway.crd.alauda.io/v1alpha1
kind: TimeoutPolicy
metadata:
    name: h2
    namespace: {{.ns}}
spec:
    targetRef:
        group: gateway.networking.k8s.io
        kind: Gateway
        name: g1
        namespace: {{.ns}}
    default:
        proxy_connect_timeout_ms: 100
    override:
        proxy_send_timeout_ms: 101
`, map[string]interface{}{"ns": ns}))
		assert.NoError(GinkgoT(), err)
		// only rule who been attached generate corresponding config.
		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			h2_0_443, _, _ := p.FindHttpPolicy(fmt.Sprintf("%d-%s-%s-%s-%s-%s-%d-%d", 443, ns, "g1", "https", ns, "h2", 0, 0))
			return TestEq(func() bool {
				ret := TimeoutEq(*h2_0_443.Config.Timeout, pointy.Uint(100), nil, pointy.Uint(101))
				return ret
			}), nil
		})
	})

	GIt("timeoutpolicy could attach to tcp listener", func() {
		_, err := f.KubectlApply(Template(`
apiVersion: gateway.crd.alauda.io/v1alpha1
kind: TimeoutPolicy
metadata:
    name: t2
    namespace: {{.ns}}
spec:
    targetRef:
        group: gateway.networking.k8s.io
        kind: Gateway
        name: g1
        namespace: {{.ns}}
    default:
        proxy_connect_timeout_ms: 100
    override:
        proxy_send_timeout_ms: 101
`, map[string]interface{}{"ns": ns}))
		assert.NoError(GinkgoT(), err)
		// only rule who been attached generate corresponding config.
		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			h3_0_81, _, _ := p.FindTcpPolicy(fmt.Sprintf("%d-%s-%s", 81, ns, "h3"))
			return TestEq(func() bool {
				ret := TimeoutEq(*h3_0_81.Config.Timeout, pointy.Uint(100), nil, pointy.Uint(101))
				return ret
			}), nil
		})
	})

	GIt("when i use timeoutPolicy override and default,it should set the correct timeout", func() {
		out, err := f.KubectlApply(Template(`
apiVersion: gateway.crd.alauda.io/v1alpha1
kind: TimeoutPolicy
metadata:
    name: t1
    namespace: {{.ns}}
spec:
    targetRef:
        group: gateway.networking.k8s.io
        kind: Gateway
        name: g1
        namespace: {{.ns}}
    default:
        proxy_connect_timeout_ms: 100
    override:
        proxy_send_timeout_ms: 101
---
apiVersion: gateway.crd.alauda.io/v1alpha1
kind: TimeoutPolicy
metadata:
    name: t2
    namespace: {{.ns}}
spec:
    targetRef:
        group: gateway.networking.k8s.io
        kind: HTTPRoute
        name: h1
        namespace: {{.ns}}
    default:
        proxy_connect_timeout_ms: 200
        proxy_send_timeout_ms: 201
        proxy_read_timeout_ms: 203
    override:
        proxy_connect_timeout_ms: 202
`, map[string]interface{}{"ns": ns}))
		Logf("%v %v %v", out, err, ns)
		assert.NoError(GinkgoT(), err)
		// only rule who been attached generate corresponding config.
		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			h1_0, _, _ := p.FindHttpPolicy(fmt.Sprintf("%d-%s-%s-%s-%s-%s-%d-%d", 80, ns, "g1", "http", ns, "h1", 0, 0))
			h1_1, _, _ := p.FindHttpPolicy(fmt.Sprintf("%d-%s-%s-%s-%s-%s-%d-%d", 80, ns, "g1", "http", ns, "h1", 1, 0))
			h2_0, _, _ := p.FindHttpPolicy(fmt.Sprintf("%d-%s-%s-%s-%s-%s-%d-%d", 80, ns, "g1", "http", ns, "h2", 0, 0))
			h2_0_90, _, _ := p.FindHttpPolicy(fmt.Sprintf("%d-%s-%s-%s-%s-%s-%d-%d", 90, ns, "g2", "http", ns, "h2", 0, 0))
			// h2 attach to g1,t1 attach to g1,t2 attach to h1
			// connect use override in t2 ,read use default in t2 , send use override in t1
			h1_0_ok := TestEq(func() bool {
				ret := TimeoutEq(*h1_0.Config.Timeout, pointy.Uint(202), pointy.Uint(203), pointy.Uint(101))
				return ret
			})
			h1_1_ok := TestEq(func() bool {
				ret := TimeoutEq(*h1_1.Config.Timeout, pointy.Uint(202), pointy.Uint(203), pointy.Uint(101))
				return ret
			})
			// h2 use g1
			h2_0_ok := TestEq(func() bool {
				ret := TimeoutEq(*h2_0.Config.Timeout, pointy.Uint(100), nil, pointy.Uint(101))
				return ret
			})
			// h2_90 attach to g2, no config
			h2_0_90_ok := TestEq(func() bool {
				return h2_0_90.Config == nil
			})
			return h1_0_ok && h1_1_ok && h2_0_ok && h2_0_90_ok, nil
		})
	})

	GIt("when i use timeoutPolicy,it should set the correct timeout", func() {
		out, err := f.KubectlApply(Template(`
apiVersion: gateway.crd.alauda.io/v1alpha1
kind: TimeoutPolicy
metadata:
    name: t1
    namespace: {{.ns}}
spec:
    targetRef:
        group: gateway.networking.k8s.io
        kind: HTTPRoute
        name: h1
        namespace: {{.ns}}
    override:
        proxy_connect_timeout_ms: 10
        proxy_read_timeout_ms: 11
`, map[string]interface{}{"ns": ns}))
		Logf("%v %v %v", out, err, ns)
		assert.NoError(GinkgoT(), err)
		// only rule who been attached generate corresponding config.
		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			h1_0, _, _ := p.FindHttpPolicy(fmt.Sprintf("%d-%s-%s-%s-%s-%s-%d-%d", 80, ns, "g1", "http", ns, "h1", 0, 0))
			h1_1, _, _ := p.FindHttpPolicy(fmt.Sprintf("%d-%s-%s-%s-%s-%s-%d-%d", 80, ns, "g1", "http", ns, "h1", 1, 0))
			h2_0, _, _ := p.FindHttpPolicy(fmt.Sprintf("%d-%s-%s-%s-%s-%s-%d-%d", 80, ns, "g1", "http", ns, "h2", 0, 0))
			return TestEq(func() bool {
				ret := TimeoutEq(*h1_0.Config.Timeout, pointy.Uint(10), pointy.Uint(11), nil)
				return ret
			}) &&
				TestEq(func() bool {
					ret := TimeoutEq(*h1_1.Config.Timeout, pointy.Uint(10), pointy.Uint(11), nil)
					return ret
				}) &&
				TestEq(func() bool {
					ret := h2_0.Config == nil
					Logf("h20 ok")
					return ret
				}), nil
		})
	})
})
