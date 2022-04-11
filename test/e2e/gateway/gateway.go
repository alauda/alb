package gateway

import (
	"context"
	"os"

	c "alauda.io/alb2/controller"
	ct "alauda.io/alb2/controller/types"
	. "alauda.io/alb2/test/e2e/framework"
	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = ginkgo.Describe("Gateway", func() {
	var f *Framework
	var ctx context.Context
	var ns string

	ginkgo.BeforeEach(func() {
		os.Setenv("DEV_MODE", "true")
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
		f = nil
	})

	GIt("when i deploy an alb, it should create a default gatewayclass and controll all gateways attach to it", func() {

		// should create a gatewayclass, and mark it's as accept
		f.Wait(func() (bool, error) {
			c := f.GetGatewayClient().GatewayV1alpha2().GatewayClasses()
			class, err := c.Get(ctx, f.AlbName, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return false, nil
			}
			if len(class.Status.Conditions) != 1 {
				return false, nil
			}
			condition := class.Status.Conditions[0]
			return condition.Type == "Accepted" && condition.Status == "True", nil
		})
		// our gateway controller controls all gateways, which has specified classname
		_, err := f.KubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
    name: g1
    namespace: {{.ns}}
spec:
    gatewayClassName:  {{.class}}
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
    gatewayClassName:  {{.class}}
    listeners:
    - name: http
      port: 8235
      protocol: HTTP
      allowedRoutes:
        namespaces:
          from: All
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
    name: g3
    namespace: {{.ns}}
spec:
    gatewayClassName:  xxxxxx 
    listeners:
    - name: http
      port: 8236
      protocol: HTTP
      allowedRoutes:
        namespaces:
          from: All
`, map[string]interface{}{"ns": ns, "class": f.AlbName}))
		assert.NoError(ginkgo.GinkgoT(), err)

		gatewayShouldBeControll := func(key client.ObjectKey) (bool, error) {
			g, err := f.GetGatewayClient().GatewayV1alpha2().Gateways(key.Namespace).Get(ctx, key.Name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if !Gateway(*g).SameAddress(f.GetAlbPodIp()) {
				return false, nil
			}
			if !Gateway(*g).Ready() {
				return false, nil
			}
			return true, nil
		}

		f.Wait(func() (bool, error) {
			return gatewayShouldBeControll(client.ObjectKey{Name: "g1", Namespace: ns})
		})
		f.Wait(func() (bool, error) {
			return gatewayShouldBeControll(client.ObjectKey{Name: "g2", Namespace: ns})
		})
		// g3 should be ignore
		g, err := f.GetGatewayClient().GatewayV1alpha2().Gateways(ns).Get(ctx, "g3", metav1.GetOptions{})
		assert.NoError(ginkgo.GinkgoT(), err)
		assert.True(ginkgo.GinkgoT(), Gateway(*g).WaittingController())
	})

	GIt("when i deployed route, it should update route and gateway status", func() {
		// TODO 当gateway不允许某些ns的话，这些ns的route不能attach到其上面，
		// 增加 gateway.spec.listeners.allowedRoutes.namespaces.selector 用例
		gateway_router_status := func(key client.ObjectKey, desired_router int32) (bool, error) {
			g, err := f.GetGatewayClient().GatewayV1alpha2().Gateways(key.Namespace).Get(ctx, key.Name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if !Gateway(*g).SameAddress(f.GetAlbPodIp()) {
				return false, nil
			}
			if !Gateway(*g).Ready() {
				return false, nil
			}
			if Gateway(*g).Ls_attached_routes()["listener-test"] != desired_router {
				return false, nil
			}
			return true, nil
		}
		// HTTPRoute:h1可以attach到gateway:g1的listeners:listener-test
		// HTTPRoute:h2不可以attach到gateway:g1的listeners:listener-test，
		// 因为parentRefs.sectionName: aaa和listener-test不对应
		// HTTPRoute:h3不可以attach到gateway:g1的listeners:listener-test，
		// 因为g1 listener.allowedRoutes.namespaces.from: Same，而h3 namespace: fake-ns不对应
		_, err := f.KubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
    name: g1
    namespace: {{.ns}}
spec:
    gatewayClassName:  {{.class}}
    listeners:
    - name: listener-test
      port: 8234
      protocol: HTTP
      hostname: a.com
      allowedRoutes:
        namespaces:
          from: Same
`, map[string]interface{}{"ns": ns, "class": f.AlbName}))
		assert.NoError(ginkgo.GinkgoT(), err)
		f.Wait(func() (bool, error) {
			return gateway_router_status(client.ObjectKey{Name: "g1", Namespace: ns}, 0)
		})

		_, err2 := f.KubectlApply(Template(`
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
        sectionName: listener-test
    rules:
    - matches:
      - path:
          value: "/foo"
        headers:
        - name: "version"
          value: "v2"
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
        sectionName: aaa
    rules:
    - matches:
      - path:
          value: "/bar"
      backendRefs:
        - kind: Service
          name: svc-2
          namespace: {{.ns}}
          port: 80
          weight: 1
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: HTTPRoute
metadata:
    name: h3
    namespace: fake-ns
spec:
    hostnames: ["a.com"]
    parentRefs:
      - kind: Gateway
        namespace: {{.ns}}
        name: g1
        sectionName: listener-test
    rules:
    - matches:
      - path:
          value: "/foo"
      backendRefs:
        - kind: Service
          name: svc-1
          namespace: {{.ns}}
          port: 80
          weight: 1`,
			map[string]interface{}{"ns": ns, "class": f.AlbName}))

		assert.NoError(ginkgo.GinkgoT(), err2)
		f.Wait(func() (bool, error) {
			return gateway_router_status(client.ObjectKey{Name: "g1", Namespace: ns}, 1)
		})
		f.WaitNginxConfigStr("listen.*8234")
	})

	GFIt("i want my app been access by tcp", func() {
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
      port: 8235
      protocol: TCP
      allowedRoutes:
        namespaces:
          from: All
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: TCPRoute
metadata:
    name: t1
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
          `, map[string]interface{}{"ns": ns, "class": f.AlbName}))
		assert.NoError(ginkgo.GinkgoT(), err)

		f.WaitNginxConfigStr("listen.*8235")
		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			name := "8235-" + ns + "-t1"
			return p.PolicyEq("tcp", name, 8235, "null", ct.BackendGroup{
				Name: name,
				Mode: "tcp",
				Backends: ct.Backends{
					{
						Address: "192.168.1.1",
						Port:    80,
						Weight:  50,
					},
					{
						Address: "192.168.1.2",
						Port:    80,
						Weight:  50,
					},
				},
			})
		})
	})

	GFIt("i want my app been access by udp", func() {
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

	GIt("i want my app been access by http", func() {
		_, err := f.KubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
    name: g1 
    namespace: {{.ns}}
spec:
    gatewayClassName:  {{.class}}
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
        headers:
        - name: "version"
          value: "v2"
      filters:
      - type: RequestHeaderModifier
        requestHeaderModifier:
          set: 
          - name: "my-header"
            value: "bar"
      backendRefs:
        - kind: Service
          name: svc-1
          namespace: {{.ns}}
          port: 80
          weight: 1
        - kind: Service
          name: svc-2
          namespace: {{.ns}}
          port: 80
          weight: 1`,
			map[string]interface{}{"ns": ns, "class": f.AlbName}))

		assert.NoError(ginkgo.GinkgoT(), err)

		f.WaitNginxConfigStr("listen.*8234")
		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			name := "8234-" + ns + "-h1-0-0"
			expectedDsl := `["AND",["IN","HOST","a.com"],["STARTS_WITH","URL","/foo"],["EQ","HEADER","version","v2"]]`
			return p.PolicyEq("http", name, 8234, expectedDsl, ct.BackendGroup{
				Name: name,
				Mode: "http",
				Backends: ct.Backends{
					{
						Address: "192.168.1.1",
						Port:    80,
						Weight:  25,
					},
					{
						Address: "192.168.1.2",
						Port:    80,
						Weight:  25,
					},
					{
						Address: "192.168.2.1",
						Port:    80,
						Weight:  50,
					},
				},
			}, func(p c.Policy) bool {
				return p.BackendProtocol == "http"
			})
		})
	})

	// TODO 当gateway的http/https listener没有hostname时，默认行为是什么，现在是拒绝了，这种行为是否正确
	GIt("i want my app been access by both http and https", func() {
		secret, err := f.CreateTlsSecret("a.com", "secret-1", ns)
		_ = secret
		assert.NoError(ginkgo.GinkgoT(), err)
		_, err = f.KubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
    name: g1 
    namespace: {{.ns}}
spec:
    gatewayClassName:  {{.class}}
    listeners:
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
          - name: secret-1
            kind: Secret
            namespace: {{.ns}}
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
        sectionName: https
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
          weight: 1`, map[string]interface{}{"ns": ns, "class": f.AlbName}))

		assert.NoError(ginkgo.GinkgoT(), err)

		f.WaitNginxConfigStr("listen.*80")
		f.WaitNginxConfigStr("listen.*443.*ssl")

		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			if !p.CertEq("a.com", secret) {
				return false, nil
			}
			name := "80-" + ns + "-h1-0-0"
			findHttpPolicy, err := p.PolicyEq("http", name, 80, `["AND",["IN","HOST","a.com"],["STARTS_WITH","URL","/foo"]]`, ct.BackendGroup{
				Name: name,
				Mode: "http",
				Backends: ct.Backends{
					{
						Address: "192.168.1.1",
						Port:    80,
						Weight:  50,
					},
					{
						Address: "192.168.1.2",
						Port:    80,
						Weight:  50,
					},
				},
			})
			if err != nil {
				return false, err
			}
			name = "443-" + ns + "-h1-0-0"
			findHttpsPolicy, err := p.PolicyEq("http", name, 443, `["AND",["IN","HOST","a.com"],["STARTS_WITH","URL","/foo"]]`, ct.BackendGroup{
				Name: name,
				Mode: "http",
				Backends: ct.Backends{
					{
						Address: "192.168.1.1",
						Port:    80,
						Weight:  50,
					},
					{
						Address: "192.168.1.2",
						Port:    80,
						Weight:  50,
					},
				},
			}, func(p c.Policy) bool {
				return p.BackendProtocol == "https"
			})
			if err != nil {
				return false, err
			}
			return findHttpPolicy && findHttpsPolicy, nil
		})
	})

	GIt("i want my multiple app use different cert in different hostname in same port", func() {
		secretInit, _ := f.CreateTlsSecret("init.com", "secret-init", ns)
		secretHarbor, _ := f.CreateTlsSecret("harbor.com", "secret-harbor", ns)
		// TODO 还有一种配置的方法是配置一个空的hostname+多个证书,然后想办法从证书中获取到hostname
		_ = secretInit
		_ = secretHarbor

		_, err := f.KubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
    name: g1 
    namespace: {{.ns}}
spec:
    gatewayClassName:  {{.class}}
    listeners:
    - name: init-https
      port: 443
      protocol: HTTPS
      hostname: init.com
      tls:
        mode: Terminate
        certificateRefs:
          - name: secret-init
            kind: Secret
            namespace: {{.ns}}
      allowedRoutes:
        namespaces:
          from: All
    - name: harbor-https
      port: 443
      protocol: HTTPS
      hostname: harbor.com
      tls:
        mode: Terminate
        certificateRefs:
          - name: secret-harbor
            kind: Secret
            namespace: {{.ns}}
      allowedRoutes:
        namespaces:
          from: All
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: HTTPRoute
metadata:
    name: h-harbor
    namespace: {{.ns}}
spec:
    parentRefs:
      - kind: Gateway
        namespace: {{.ns}}
        name: g1
        sectionName: harbor-https
    rules:
    - matches:
      - path:
          value: "/harbor/login"
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
    name: h-init
    namespace: {{.ns}}
spec:
    hostnames: ["init.com"]
    parentRefs:
      - kind: Gateway
        namespace: {{.ns}}
        name: g1
        sectionName: init-https
    rules:
    - matches:
      - path:
          value: "/int/login"
      backendRefs:
        - kind: Service
          name: svc-1
          namespace: {{.ns}}
          port: 80
          weight: 1
          `, map[string]interface{}{"ns": ns, "class": f.AlbName}))

		assert.NoError(ginkgo.GinkgoT(), err)
		f.WaitNginxConfigStr("listen.*443.*ssl")

		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			if !p.CertEq("init.com", secretInit) {
				return false, nil
			}
			if !p.CertEq("harbor.com", secretHarbor) {
				return false, nil
			}
			return true, nil
		})
	})

	GIt("i should match most specific rule first(bigger complexity_priority)", func() {
		_, err := f.KubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
    name: g1
    namespace: {{.ns}}
spec:
    gatewayClassName:  {{.class}}
    listeners:
    - name: http-generic-host
      port: 8234
      protocol: HTTP
      hostname: "*.com"
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
        sectionName: http-generic-host
    rules:
    - matches:
      - path:
          type: "Exact"
          value: "/foo"
      filters:
      - type: RequestHeaderModifier
        requestHeaderModifier:
          set:
          - name: "my-header"
            value: "bar"
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
    hostnames: ["b.com"]
    parentRefs:
      - kind: Gateway
        namespace: {{.ns}}
        name: g1
        sectionName: http-generic-host
    rules:
    - matches:
      - path:
          type: "PathPrefix"
          value: "/bar"
        headers:
        - name: "version"
          value: "v2"
      backendRefs:
        - kind: Service
          name: svc-2
          namespace: {{.ns}}
          port: 80
          weight: 1`,
			map[string]interface{}{"ns": ns, "class": f.AlbName}))

		assert.NoError(ginkgo.GinkgoT(), err)

		f.WaitNginxConfigStr("listen.*8234")

		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			name := "8234-" + ns + "-h1-0-0"
			expectedDsl := `["AND",["IN","HOST","a.com"],["EQ","URL","/foo"]]`
			findHttpPolicy1, err := p.PolicyEq("http", name, 8234, expectedDsl, ct.BackendGroup{
				Name: name,
				Mode: "http",
				Backends: ct.Backends{
					{
						Address: "192.168.1.1",
						Port:    80,
						Weight:  50,
					},
					{
						Address: "192.168.1.2",
						Port:    80,
						Weight:  50,
					},
				},
			}, func(p c.Policy) bool {
				return p.BackendProtocol == "http" && p.Priority == 52000
			})

			if err != nil {
				return false, err
			}

			name = "8234-" + ns + "-h2-0-0"
			expectedDsl = `["AND",["IN","HOST","b.com"],["STARTS_WITH","URL","/bar"],["EQ","HEADER","version","v2"]]`
			findHttpPolicy2, err := p.PolicyEq("http", name, 8234, expectedDsl, ct.BackendGroup{
				Name: name,
				Mode: "http",
				Backends: ct.Backends{
					{
						Address: "192.168.2.1",
						Port:    80,
						Weight:  100,
					},
				},
			}, func(p c.Policy) bool {
				return p.BackendProtocol == "http" && p.Priority == 51100
			})
			if err != nil {
				return false, err
			}
			// HttpPolicy2 has Priority bigger than HttpPolicy1
			return findHttpPolicy1 && findHttpPolicy2, nil
		})
	})

	GIt("support generic-host in listeners", func() {
		_, err := f.KubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
    name: g1
    namespace: {{.ns}}
spec:
    gatewayClassName:  {{.class}}
    listeners:
    - name: http
      port: 8234
      protocol: HTTP
      hostname: "*.com"
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
        headers:
        - name: "version"
          value: "v2"
      filters:
      - type: RequestHeaderModifier
        requestHeaderModifier:
          set:
          - name: "my-header"
            value: "bar"
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
    hostnames: ["b.com"]
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
          name: svc-2
          namespace: {{.ns}}
          port: 80
          weight: 1`,
			map[string]interface{}{"ns": ns, "class": f.AlbName}))

		assert.NoError(ginkgo.GinkgoT(), err)

		f.WaitNginxConfigStr("listen.*8234")

		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			name := "8234-" + ns + "-h1-0-0"
			expectedDsl := `["AND",["IN","HOST","a.com"],["STARTS_WITH","URL","/foo"],["EQ","HEADER","version","v2"]]`
			findHttpPolicy1, err := p.PolicyEq("http", name, 8234, expectedDsl, ct.BackendGroup{
				Name: name,
				Mode: "http",
				Backends: ct.Backends{
					{
						Address: "192.168.1.1",
						Port:    80,
						Weight:  50,
					},
					{
						Address: "192.168.1.2",
						Port:    80,
						Weight:  50,
					},
				},
			}, func(p c.Policy) bool {
				return p.BackendProtocol == "http"
			})

			if err != nil {
				return false, err
			}

			name = "8234-" + ns + "-h2-0-0"
			expectedDsl = `["AND",["IN","HOST","b.com"],["STARTS_WITH","URL","/bar"]]`
			findHttpPolicy2, err := p.PolicyEq("http", name, 8234, expectedDsl, ct.BackendGroup{
				Name: name,
				Mode: "http",
				Backends: ct.Backends{
					{
						Address: "192.168.2.1",
						Port:    80,
						Weight:  100,
					},
				},
			}, func(p c.Policy) bool {
				return p.BackendProtocol == "http"
			})
			if err != nil {
				return false, err
			}
			return findHttpPolicy1 && findHttpPolicy2, nil
		})
	})
})
