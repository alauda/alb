package ingress

import (
	"context"
	"fmt"
	"strings"

	. "alauda.io/alb2/test/e2e/framework"
	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = ginkgo.Describe("Ingress", func() {
	var f *Framework

	ginkgo.BeforeEach(func() {
		deployCfg := Config{InstanceMode: true, RestCfg: KUBE_REST_CONFIG, Project: []string{"project1"}}
		f = NewAlb(deployCfg)
		f.InitProductNs("alb-test", "project1")
		f.InitDefaultSvc("svc-default", []string{"192.168.1.1", "192.168.2.2"})
		f.Init()
	})

	ginkgo.AfterEach(func() {
		f.Destroy()
		f = nil
	})

	GIt("empty test", func() {
		Logf("ok")
	})

	GIt("should compatible with redirect ingress", func() {
		ns := f.GetProductNs()
		// should generate rule and policy when use ingress.backend.port.number even svc not exist.
		pathType := networkingv1.PathTypeImplementationSpecific
		_, err := f.GetK8sClient().NetworkingV1().Ingresses(ns).Create(context.Background(), &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
				Name:      "redirect",
				Annotations: map[string]string{
					"nginx.ingress.kubernetes.io/temporal-redirect": "/console-platform/portal",
				},
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/&",
										PathType: &pathType,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "none",
												Port: networkingv1.ServiceBackendPort{Number: 8080},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}, metav1.CreateOptions{})

		assert.NoError(ginkgo.GinkgoT(), err)
		f.WaitNginxConfigStr("listen.*80")

		rules := f.WaitIngressRule("redirect", ns, 1)
		rule := rules[0]
		ruleName := rules[0].Name

		assert.Equal(ginkgo.GinkgoT(), rule.Spec.ServiceGroup.Services[0].Port, 8080)

		f.WaitPolicy(func(policyRaw string) bool {
			fmt.Printf("policyRaw %s", policyRaw)
			hasRule := PolicyHasRule(policyRaw, 80, ruleName)
			hasPod := PolicyHasBackEnds(policyRaw, ruleName, `[]`)
			return hasRule && hasPod
		})
	})

	ginkgo.Context("basic ingress", func() {
		GIt("should translate ingress to rule and generate nginx config,policy.new", func() {
			ns := f.GetProductNs()
			name := "ingress-a"

			f.CreateIngress(ns, name, "/a", "svc-default", 80)

			f.WaitNginxConfigStr("listen.*80")

			rules := f.WaitIngressRule(name, ns, 1)
			rule := rules[0]
			ruleName := rule.Name

			f.WaitPolicy(func(policyRaw string) bool {
				hasRule := PolicyHasRule(policyRaw, 80, ruleName)
				hasPod := PolicyHasBackEnds(policyRaw, ruleName, `[map[address:192.168.1.1 otherclusters:false port:80 weight:50] map[address:192.168.2.2 otherclusters:false port:80 weight:50]]`)
				return hasRule && hasPod
			})

		})

		GIt("should get correct backend port when ingress use port.name", func() {
			// give a svc.port tcp-81 which point to pod name pod-tcp-112 which point to pod container port 112
			// if ingress.backend.port.name=tcp-81, the backend should be xxxx:112
			ns := f.GetProductNs()
			ingressCase := IngressCase{
				Namespace: ns,
				Name:      "svc2",
				SvcPort: map[string]struct {
					Protocol   corev1.Protocol
					Port       int32
					Target     intstr.IntOrString
					TargetPort int32
					TargetName string
				}{
					"tcp-81": {
						Protocol:   corev1.ProtocolTCP,
						Port:       81,
						Target:     intstr.IntOrString{Type: intstr.String, StrVal: "pod-tcp-112"},
						TargetName: "pod-tcp-112",
						TargetPort: 112,
					},
				},
				Eps: []string{"192.168.3.1", "192.168.3.2"},
				Ingress: struct {
					Name string
					Host string
					Path string
					Port intstr.IntOrString
				}{
					Name: "svc-ingress",
					Host: "host-a",
					Path: "/a",
					Port: intstr.IntOrString{Type: intstr.String, StrVal: "tcp-81"},
				},
			}

			f.InitIngressCase(ingressCase)
			f.WaitNginxConfigStr("listen.*80")
			rules := f.WaitIngressRule(ingressCase.Ingress.Name, ingressCase.Namespace, 1)
			rule := rules[0]
			ruleName := rule.Name
			Logf("created rule %+v", rule)
			assert.Equal(ginkgo.GinkgoT(), rule.Spec.ServiceGroup.Services[0].Port, 81)
			f.WaitPolicy(func(policyRaw string) bool {
				hasRule := PolicyHasRule(policyRaw, 80, ruleName)
				hasPod := PolicyHasBackEnds(policyRaw, ruleName, `[map[address:192.168.3.1 otherclusters:false port:112 weight:50] map[address:192.168.3.2 otherclusters:false port:112 weight:50]]`)
				return hasRule && hasPod
			})
		})

		GIt("should work when ingress has longlong name", func() {
			ns := f.GetProductNs()
			_ = ns
			name := "ingress-aaa" + strings.Repeat("a", 100)

			f.CreateIngress(ns, name, "/a", "svc-default", 80)

			f.WaitNginxConfigStr("listen.*80")

			rules := f.WaitIngressRule(name, ns, 1)
			rule := rules[0]
			ruleName := rule.Name
			_ = ruleName

			f.WaitPolicy(func(policyRaw string) bool {
				hasRule := PolicyHasRule(policyRaw, 80, ruleName)
				hasPod := PolicyHasBackEnds(policyRaw, ruleName, `[map[address:192.168.1.1 otherclusters:false port:80 weight:50] map[address:192.168.2.2 otherclusters:false port:80 weight:50]]`)
				return hasRule && hasPod
			})
		})

		GIt("should work with none path ingress", func() {
			f.AssertKubectlApply(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-onlyhost
  namespace:  alb-test
spec:
  rules:
  - host: local.ares.acp.ingress.domain
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-normal
  namespace:  alb-test
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: svc-default
            port:
              number: 80
        path: /xx
        pathType: ImplementationSpecific
`)
			f.Wait(func() (bool, error) {
				rs, err := f.K8sClient.GetAlbClient().CrdV1().Rules("cpaas-system").List(f.GetCtx(), metav1.ListOptions{})
				Logf("rs %v %v", rs.Items, err)
				if err != nil {
					return false, err
				}
				return len(rs.Items) == 1, err
			})
		})

		GIt("should handle defaultbackend correctly", func() {
			f.AssertKubectlApply(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ing-nodefault
  namespace: alb-test
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: svc-default
            port:
              number: 80
        path: /xx
        pathType: ImplementationSpecific
`)
			f.Wait(func() (bool, error) {
				ft, err := f.K8sClient.GetAlbClient().CrdV1().Frontends("cpaas-system").Get(f.GetCtx(), "alb-dev-00080", metav1.GetOptions{})
				Logf("%v %v", ft.Spec.ServiceGroup, err)
				if err != nil {
					return false, err
				}
				return ft.Spec.ServiceGroup == nil, nil
			})

			f.AssertKubectlApply(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ing-hasdefault
  namespace: alb-test
spec:
  defaultBackend:
    service:
      name: svc-default
      port:
        number: 80
`)
			f.Wait(func() (bool, error) {
				ft, err := f.K8sClient.GetAlbClient().CrdV1().Frontends("cpaas-system").Get(f.GetCtx(), "alb-dev-00080", metav1.GetOptions{})
				Logf("shoule be 1 %v %v %v", ft.Spec.Source, ft.Spec.ServiceGroup, err)
				if err != nil {
					return false, err
				}
				if ft.Spec.ServiceGroup != nil && ft.Spec.Source.Name == "ing-hasdefault" {
					return true, nil
				}
				return false, nil
			})
			f.AssertKubectlApply(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ing-hasdefault1
  namespace: alb-test
spec:
  defaultBackend:
    service:
      name: svc-default
      port:
        number: 80
`)
			err := f.K8sClient.GetK8sClient().NetworkingV1().Ingresses("alb-test").Delete(f.GetCtx(), "ing-hasdefault", metav1.DeleteOptions{})
			assert.NoError(ginkgo.GinkgoT(), err)
			f.Wait(func() (bool, error) {
				ft, err := f.K8sClient.GetAlbClient().CrdV1().Frontends("cpaas-system").Get(f.GetCtx(), "alb-dev-00080", metav1.GetOptions{})
				Logf("shoule be empty %v %v %v", ft.Spec.Source, ft.Spec.ServiceGroup, err)
				return ft.Spec.ServiceGroup == nil, nil
			})
		})

	})
})
