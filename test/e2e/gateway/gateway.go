package gateway

import (
	"context"
	"strings"

	. "alauda.io/alb2/test/e2e/framework"
	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	g "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

var _ = ginkgo.Describe("Gateway", func() {
	var f *Framework
	var ctx context.Context
	var ns string

	ginkgo.BeforeEach(func() {
		deployCfg := Config{InstanceMode: true, AlbAddress: "172.13.1.2", RestCfg: CfgFromEnv(), Project: []string{"project1"}, Gateway: true}
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
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
    name: g4
    namespace: {{.ns}}
spec:
    gatewayClassName: alb-dev
    listeners: []
`, map[string]interface{}{"ns": ns, "class": f.AlbName}))
		assert.NoError(ginkgo.GinkgoT(), err)

		f.Wait(func() (bool, error) {
			return f.CheckGatewayStatus(client.ObjectKey{Name: "g1", Namespace: ns}, []string{f.GetAlbAddress()})
		})
		f.Wait(func() (bool, error) {
			return f.CheckGatewayStatus(client.ObjectKey{Name: "g2", Namespace: ns}, []string{f.GetAlbAddress()})
		})
		// allow empty gateway
		f.Wait(func() (bool, error) {
			return f.CheckGatewayStatus(client.ObjectKey{Name: "g4", Namespace: ns}, []string{f.GetAlbAddress()})
		})
		// g3 should be ignore
		g, err := f.GetGatewayClient().GatewayV1alpha2().Gateways(ns).Get(ctx, "g3", metav1.GetOptions{})
		assert.NoError(ginkgo.GinkgoT(), err)
		assert.True(ginkgo.GinkgoT(), Gateway(*g).WaittingController())
	})

	GIt("allowedRoutes should ok", func() {
		gateway_router_status := func(key client.ObjectKey, desired_router int32) (bool, error) {
			g, err := f.GetGatewayClient().GatewayV1alpha2().Gateways(key.Namespace).Get(ctx, key.Name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if !Gateway(*g).SameAddress([]string{f.GetAlbAddress()}) {
				return false, nil
			}
			if !Gateway(*g).Ready() {
				return false, nil
			}
			if Gateway(*g).LsAttachedRoutes()["listener-test"] != desired_router {
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

		ns1 := f.InitProductNsWithOpt(ProductNsOpt{
			Prefix:  "fake-ns",
			Project: "project1",
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
    namespace: {{.ns1}}
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
			map[string]interface{}{"ns": ns, "ns1": ns1, "class": f.AlbName}))

		assert.NoError(ginkgo.GinkgoT(), err2)
		f.Wait(func() (bool, error) {
			return gateway_router_status(client.ObjectKey{Name: "g1", Namespace: ns}, 1)
		})
		f.WaitNginxConfigStr("listen.*8234")
	})

	GIt("ns selector should work", func() {
		ns1 := f.InitProductNsWithOpt(ProductNsOpt{
			Prefix:  "alb-test1",
			Project: "project1",
			Labels: map[string]string{
				"a": "b",
			},
		})
		_ = f.AssertKubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
    name: g1
    namespace: {{.ns}}
spec:
    gatewayClassName:  {{.class}}
    listeners:
    - name: l1
      port: 8234
      protocol: HTTP
      hostname: a.com
      allowedRoutes:
        namespaces:
          from: Selector
          selector:
              matchLabels:
                  a: b
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
        sectionName: l1 
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
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: HTTPRoute
metadata:
    name: h2
    namespace: {{.ns1}}
spec:
    hostnames: ["a.com"]
    parentRefs:
      - kind: Gateway
        namespace: {{.ns}}
        name: g1
        sectionName: l1 
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
`, map[string]interface{}{"ns": ns, "ns1": ns1, "class": f.AlbName}))
		pref := NewParentRef(ns, "g1", "l1")
		f.WaitHttpRouteStatus(ns, "h1", pref, func(status g.RouteParentStatus) (bool, error) {
			condition := status.Conditions[0]
			ret := condition.Type == "Ready" &&
				strings.Contains(condition.Message, "ns selector not match") &&
				condition.Status == "False" &&
				true
			Logf("ret %v c %+v", ret, condition)
			return ret, nil
		})

		f.WaitHttpRouteStatus(ns1, "h2", pref, func(status g.RouteParentStatus) (bool, error) {
			condition := status.Conditions[0]
			ret := condition.Type == "Ready" &&
				condition.Status == "True" &&
				true
			Logf("ret %v c %+v", ret, condition)
			return ret, nil
		})
	})

	GIt("http route with uninterocp hostname with listener should mark as reject", func() {
		_ = f.AssertKubectlApply(Template(`
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
    name: g1
    namespace: {{.ns}}
spec:
    gatewayClassName:  {{.class}}
    listeners:
    - name: l1
      port: 8234
      protocol: HTTP
      hostname: "*.a.com"
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
        sectionName: l1 
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
---
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: HTTPRoute
metadata:
    name: h2
    namespace: {{.ns}}
spec:
    hostnames: ["a.a.com"]
    parentRefs:
      - kind: Gateway
        namespace: {{.ns}}
        name: g1
        sectionName: l1 
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
		pref := NewParentRef(ns, "g1", "l1")
		f.WaitHttpRouteStatus(ns, "h1", pref, func(status g.RouteParentStatus) (bool, error) {
			condition := status.Conditions[0]
			ret := condition.Type == "Ready" &&
				strings.Contains(condition.Message, "no intersection hostname") &&
				condition.Status == "False" &&
				true
			Logf("ret %v c %+v", ret, condition)
			return ret, nil
		})

		f.WaitHttpRouteStatus(ns, "h2", pref, func(status g.RouteParentStatus) (bool, error) {
			condition := status.Conditions[0]
			ret := condition.Type == "Ready" &&
				condition.Status == "True" &&
				true
			Logf("ret %v c %+v", ret, condition)
			return ret, nil
		})
	})
})
