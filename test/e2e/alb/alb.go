package alb

import (
	. "alauda.io/alb2/test/e2e/framework"
	. "alauda.io/alb2/utils/test_utils"
	. "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("common alb", func() {
	GIt("alb in other ns should work normally", func() {
		opt := AlbEnvOpt{
			BootYaml: `
        apiVersion: crd.alauda.io/v2beta1
        kind: ALB2
        metadata:
            name: alb-g1
            namespace: g1
            labels:
                alb.cpaas.io/managed-by: alb-operator
        spec:
            address: "127.0.0.1"
            type: "nginx"
            config:
                networkMode: container
                projects: ["ALL_ALL"]
`,
			Ns:       "g1",
			Name:     "alb-g1",
			StartAlb: true,
		}
		e := NewAlbEnvWithOpt(opt)
		defer e.Stop()
		e.InitProductNs("g2", "p2")
		e.InitSvcWithOpt(SvcOpt{
			Ns:    "g1",
			Name:  "g1-svc",
			Ep:    []string{"192.168.1.1"},
			Ports: []corev1.ServicePort{{Port: 80}},
		})
		e.InitSvcWithOpt(SvcOpt{
			Ns:    "g2",
			Name:  "g2-svc",
			Ep:    []string{"192.168.2.1"},
			Ports: []corev1.ServicePort{{Port: 80}},
		})
		l := e.Log
		e.Kt.KubectlApply(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ing1 
  namespace: g1
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: g1-svc 
            port:
              number: 80
        path: /morgan
        pathType: ImplementationSpecific
`)
		e.Kt.KubectlApply(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ing2
  namespace: g2
spec:
  rules:
  - http:
      paths:
      - backend:
          service:
            name: g2-svc 
            port:
              number: 80
        path: /morgan
        pathType: ImplementationSpecific
`)

		e.WaitNginxConfigStr("listen.*80")
		{
			rules := e.WaitIngressRule("ing1", "g1", 2)
			l.Info("x", "rules", len(rules), "", PrettyCrs(rules))
		}
		{
			rules := e.WaitIngressRule("ing2", "g2", 2)
			l.Info("x", "rules", len(rules), "", PrettyCrs(rules))
		}
	})
})
