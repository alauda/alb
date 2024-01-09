package gateway

import (
	"context"

	c "alauda.io/alb2/controller"
	ct "alauda.io/alb2/controller/types"
	. "alauda.io/alb2/test/e2e/framework"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	"github.com/onsi/ginkgo"
	"github.com/openlyinc/pointy"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

var _ = ginkgo.Describe("Http", func() {
	var f *GatewayF
	var env *Env
	var ctx context.Context
	var log logr.Logger
	var ns string
	var ns1 string
	ginkgo.BeforeEach(func() {
		f, env = DefaultGatewayF()
		f.InitProductNs("alb-test", "project1")
		ns1 = f.InitProductNsWithOpt(ProductNsOpt{
			Prefix:  "alb-test1",
			Project: "project1",
		})
		f.InitDefaultSvc("svc-1", []string{"192.168.1.1", "192.168.1.2"})
		f.InitDefaultSvc("svc-2", []string{"192.168.2.1"})
		ctx = env.Ctx
		_ = ctx
		ns = f.GetProductNs()
		log = env.Log
		_ = log
	})

	ginkgo.AfterEach(func() {
		env.Stop()
		f = nil
	})

	GIt("i want my app been access by http", func() {
		_, err := f.KubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1
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
apiVersion: gateway.networking.k8s.io/v1
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
		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			name := "8234-" + ns + "-g1" + "-http-" + ns + "-h1-0-0"
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
				return p.BackendProtocol == "http" && p.Config.RewriteRequest.Headers["my-header"] == "bar"
			})
		})
	})

	GIt("wildcard https should work", func() {
		f.AssertKubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
    name: g1 
    namespace: {{.ns}}
spec:
    gatewayClassName:  {{.class}}
    listeners:
    - name: https
      port: 443
      protocol: HTTPS
      hostname: "*.com"
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
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
    name: h1
    namespace: {{.ns}}
spec:
    parentRefs:
      - kind: Gateway
        namespace: {{.ns}}
        name: g1
        sectionName: https
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
`, map[string]interface{}{"ns": ns, "class": f.AlbName}))

		secret, _ := f.CreateTlsSecret("*.com", "secret-1", ns)
		f.WaitNginxConfigStr("listen.*443.*ssl")
		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			if !p.CertEq("*.com", secret) {
				return false, nil
			}
			defaultBackend := ct.Backends{
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
			}
			defaultMatch := `["AND",["ENDS_WITH","HOST","*.com"],["STARTS_WITH","URL","/foo"]]`
			name := "443-" + ns + "-g1" + "-https-" + ns + "-h1-0-0"
			_, err := p.PolicyEq("http", name, 443, defaultMatch, ct.BackendGroup{
				Name:     name,
				Mode:     "https",
				Backends: defaultBackend,
			})
			if err != nil {
				return false, err
			}
			return true, nil
		})
	})
	GIt("i want my app been access by both http and https", func() {
		f.InitSvcWithOpt(SvcOpt{
			Ns:   ns,
			Name: "svc-3",
			Ep:   []string{"172.0.0.1"},
			Ports: []corev1.ServicePort{
				{
					Name:        "http",
					Port:        80,
					Protocol:    "TCP",
					AppProtocol: pointy.String("http"),
				},
				{
					Name:        "https",
					Port:        443,
					Protocol:    "TCP",
					AppProtocol: pointy.String("https"),
				},
			},
		})
		secret, _ := f.CreateTlsSecret("a.com", "secret-1", ns)
		f.AssertKubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1
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
apiVersion: gateway.networking.k8s.io/v1
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
          name: svc-3
          namespace: {{.ns}}
          port: 80
          weight: 1
---
apiVersion: gateway.networking.k8s.io/v1
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
    rules:
    - matches:
      - path:
          value: "/foo"
      backendRefs:
        - kind: Service
          name: svc-3
          namespace: {{.ns}}
          port: 443
          weight: 1
`, map[string]interface{}{"ns": ns, "class": f.AlbName}))

		f.WaitNginxConfigStr("listen.*80")
		f.WaitNginxConfigStr("listen.*443.*ssl")

		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			if !p.CertEq("a.com", secret) {
				return false, nil
			}
			defaultBackend := ct.Backends{
				{
					Address: "172.0.0.1",
					Port:    80,
					Weight:  100,
				},
			}
			default443Backend := ct.Backends{
				{
					Address: "172.0.0.1",
					Port:    443,
					Weight:  100,
				},
			}
			defaultMatch := `["AND",["IN","HOST","a.com"],["STARTS_WITH","URL","/foo"]]`
			name := "80-" + ns + "-g1" + "-http-" + ns + "-h1-0-0"
			_, err := p.PolicyEq("http", name, 80, defaultMatch, ct.BackendGroup{
				Name:     name,
				Mode:     "http",
				Backends: defaultBackend,
			}, func(p c.Policy) bool {
				return p.BackendProtocol == "http"
			})
			if err != nil {
				return false, err
			}
			name = "443-" + ns + "-g1" + "-https-" + ns + "-h1-0-0"
			_, err = p.PolicyEq("http", name, 443, `["AND",["IN","HOST","a.com"],["STARTS_WITH","URL","/foo"]]`, ct.BackendGroup{
				Name:     name,
				Mode:     "http",
				Backends: defaultBackend,
			}, func(p c.Policy) bool {
				return p.BackendProtocol == "http"
			})
			if err != nil {
				return false, err
			}

			name = "80-" + ns + "-g1" + "-http-" + ns + "-h2-0-0"
			_, err = p.PolicyEq("http", name, 80, defaultMatch, ct.BackendGroup{
				Name:     name,
				Mode:     "http",
				Backends: default443Backend,
			}, func(p c.Policy) bool {
				return p.BackendProtocol == "https"
			})
			if err != nil {
				return false, err
			}
			return true, nil
		})
	})

	GIt("i want my multiple app use different cert in different hostname in same port", func() {
		secretInit, _ := f.CreateTlsSecret("init.com", "secret-init", ns)
		secretHarbor, _ := f.CreateTlsSecret("harbor.com", "secret-harbor", ns)
		_ = secretInit
		_ = secretHarbor

		_, err := f.KubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1
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
apiVersion: gateway.networking.k8s.io/v1
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
apiVersion: gateway.networking.k8s.io/v1
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

	GIt("gateway listener with same hostname should mark as reject", func() {
		// TODO
	})

	GIt("http route attach to same port and same named listener", func() {
		ret := f.AssertKubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1
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
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
    name: g1
    namespace: {{.ns1}}
spec:
    gatewayClassName:  {{.class}}
    listeners:
    - name: http
      port: 80
      protocol: HTTP
      hostname: '*.com'
      allowedRoutes:
        namespaces:
          from: All
---
apiVersion: gateway.networking.k8s.io/v1
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
      - kind: Gateway
        namespace: {{.ns1}}
        name: g1
        sectionName: http
    rules:
    - matches:
      - path:
          value: "/foo"
      backendRefs:
        - kind: Service
          name: svc-2
          namespace: {{.ns}}
          port: 80
          weight: 1`,
			map[string]interface{}{"ns": ns, "ns1": ns1, "class": f.AlbName}))
		Logf("ret %s", ret)
		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			// TODO it seems werid to have such same rule.
			name1 := "80-" + ns + "-g1-http-" + ns + "-h1-0-0"
			name2 := "80-" + ns1 + "-g1-http-" + ns + "-h1-0-0"
			bg := func(name string) ct.BackendGroup {
				return ct.BackendGroup{
					Name: name,
					Mode: "http",
					Backends: ct.Backends{
						{
							Address: "192.168.2.1",
							Port:    80,
							Weight:  100,
						},
					},
				}
			}
			ret, err := p.PolicyEq("http", name1, 80, `["AND",["IN","HOST","a.com"],["STARTS_WITH","URL","/foo"]]`, bg(name1))
			if err != nil || !ret {
				return ret, err
			}
			ret, err = p.PolicyEq("http", name2, 80, `["AND",["IN","HOST","a.com"],["STARTS_WITH","URL","/foo"]]`, bg(name2))
			return ret, err
		})
	})

	GIt("http rule with multiple match should work", func() {
		_, err := f.KubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1
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
---
apiVersion: gateway.networking.k8s.io/v1
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
      - path:
          value: "/bar"
        headers:
        - name: "version"
          value: "v3"

      backendRefs:
        - kind: Service
          name: svc-2
          namespace: {{.ns}}
          port: 80
          weight: 1`,
			map[string]interface{}{"ns": ns, "class": f.AlbName}))

		assert.NoError(ginkgo.GinkgoT(), err)

		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			name1 := "80-" + ns + "-g1-http-" + ns + "-h1-0-0"
			name2 := "80-" + ns + "-g1-http-" + ns + "-h1-0-1"
			bg := func(name string) ct.BackendGroup {
				return ct.BackendGroup{
					Name: name,
					Mode: "http",
					Backends: ct.Backends{
						{
							Address: "192.168.2.1",
							Port:    80,
							Weight:  100,
						},
					},
				}
			}
			ret, err := p.PolicyEq("http", name1, 80, `["AND",["IN","HOST","a.com"],["STARTS_WITH","URL","/foo"],["EQ","HEADER","version","v2"]]`, bg(name1))
			if err != nil || !ret {
				return ret, err
			}
			ret, err = p.PolicyEq("http", name2, 80, `["AND",["IN","HOST","a.com"],["STARTS_WITH","URL","/bar"],["EQ","HEADER","version","v3"]]`, bg(name2))
			return ret, err
		})
	})

	GIt("i should match most specific rule first bigger complexity_priority", func() {
		_, err := f.KubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1
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
apiVersion: gateway.networking.k8s.io/v1
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
apiVersion: gateway.networking.k8s.io/v1
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
			name := "8234-" + ns + "-g1-http-generic-host-" + ns + "-h1-0-0"
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
				return p.BackendProtocol == "http"
			})
			if err != nil {
				return false, err
			}

			name = "8234-" + ns + "-g1-http-generic-host-" + ns + "-h2-0-0"
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
				return p.BackendProtocol == "http"
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
apiVersion: gateway.networking.k8s.io/v1
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
apiVersion: gateway.networking.k8s.io/v1
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
apiVersion: gateway.networking.k8s.io/v1
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
			name := "8234-" + ns + "-g1-http-" + ns + "-h1-0-0"
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
			name = "8234-" + ns + "-g1-http-" + ns + "-h2-0-0"
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

	GIt("http headermodify/redirect filter should work", func() {
		_ = f.AssertKubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1
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
apiVersion: gateway.networking.k8s.io/v1
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
      filters:
      - type: RequestHeaderModifier
        requestHeaderModifier:
          set:
          - name: "s1"
            value: "s1-1"
          add:
          - name: "a1"
            value: "a1-1"
          remove: ["r1","r2"] 
      - type: RequestRedirect
        requestRedirect:
            scheme: https
            hostname: "xx.com"
            port: 9090 
            statusCode: 301
      backendRefs:
        - kind: Service
          name: svc-1
          namespace: {{.ns}}
          port: 80
          weight: 1
`, map[string]interface{}{"ns": ns, "class": f.AlbName}))

		f.WaitNginxConfigStr("listen.*8234")

		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			name := "8234-" + ns + "-g1-http-" + ns + "-h1-0-0"
			policy, _, _ := p.FindHttpPolicy(name)
			Logf("policy is %+v", policy.Config.RewriteResponse)
			Logf("policy is %v", *policy.RedirectScheme)
			Logf("policy is %v", *policy.RedirectHost)
			Logf("policy is %v", *policy.RedirectPort)
			Logf("policy is %v", policy.RedirectCode)
			ret := *policy.RedirectScheme == "https" &&
				*policy.RedirectHost == "xx.com" &&
				*policy.RedirectPort == 9090 &&
				policy.RedirectCode == 301 &&
				true

			return ret, nil
		})
	})

	GIt("http route without matches", func() {
		_ = f.AssertKubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1
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
apiVersion: gateway.networking.k8s.io/v1
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
    - matches: []
      backendRefs:
        - kind: Service
          name: svc-1
          namespace: {{.ns}}
          port: 80
          weight: 1
`, map[string]interface{}{"ns": ns, "class": f.AlbName}))

		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			name := "8234-" + ns + "-g1-http-" + ns + "-h1-0-0"
			policy, _, _ := p.FindHttpPolicy(name)
			return policy != nil, nil
		})
	})

	GIt("http url rewrite filter should work", func() {
		log.Info("x  http url rewrite filter should work")
		_ = f.AssertKubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1
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
apiVersion: gateway.networking.k8s.io/v1
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
          value: "/foo/bar"
      filters:
      - type: URLRewrite
        urlRewrite:
            hostname: "xx.com"
            path: 
              type: ReplaceFullPath
              replaceFullPath: "/foo"
      backendRefs:
        - kind: Service
          name: svc-1
          namespace: {{.ns}}
          port: 80
          weight: 1
    - matches:
      - path:
          type: "PathPrefix"
          value: "/bar/foor"
      filters:
      - type: URLRewrite
        urlRewrite:
            hostname: "xx.com"
            path: 
              type: ReplacePrefixMatch
              replacePrefixMatch: "/bar"
      backendRefs:
        - kind: Service
          name: svc-1
          namespace: {{.ns}}
          port: 80
          weight: 1
`, map[string]interface{}{"ns": ns, "class": f.AlbName}))

		// prefix
		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			name := "8234-" + ns + "-g1-http-" + ns + "-h1-1-0"
			policy, _, _ := p.FindHttpPolicy(name)
			log.Info("m1", "policy", PrettyJson(policy))
			return policy != nil &&
				*policy.RewritePrefixMatch == "/bar/foor" &&
				policy.VHost == "xx.com" &&
				*policy.RewriteReplacePrefix == "/bar", nil
		})
		// full
		f.WaitNgxPolicy(func(p NgxPolicy) (bool, error) {
			name := "8234-" + ns + "-g1-http-" + ns + "-h1-0-0"
			policy, _, _ := p.FindHttpPolicy(name)
			log.Info("m1", "policy", PrettyJson(policy))
			return policy != nil &&
				policy.RewriteBase == ".*" &&
				policy.VHost == "xx.com" &&
				policy.RewriteTarget == "/foo", nil
		})
	})
})
