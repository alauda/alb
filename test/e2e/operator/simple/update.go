package simple

import (
	"context"
	"fmt"

	"alauda.io/alb2/pkg/operator/config"
	. "alauda.io/alb2/test/e2e/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	. "alauda.io/alb2/utils/test_utils"
)

var _ = Describe("Operator", func() {
	var env *OperatorEnv
	var opext *AlbOperatorExt
	var cli client.Client
	var ctx context.Context
	var log logr.Logger

	BeforeEach(func() {
		env = StartOperatorEnvOrDie()
		opext = env.Opext
		cli = env.Kc.GetDirectClient()
		ctx = env.Ctx
		log = env.Log
	})

	AfterEach(func() {
		env.Stop()
	})

	GIt("port project alb", func() {
		alb1 := `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: alb-1
    namespace: cpaas-system
    labels:
        alb.cpaas.io/managed-by: alb-operator
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        replicas: 1
        portProjects: '[{"port":"113-333","projects":["p1"]}]'
`
		alb2 := `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: alb-2
    namespace: cpaas-system
    labels:
        alb.cpaas.io/managed-by: alb-operator
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        replicas: 1
        portProjects: '[{"port":"413-533","projects":["p1"]}]'
`
		alb3 := `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: alb-3
    namespace: cpaas-system
    labels:
        alb.cpaas.io/managed-by: alb-operator
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        replicas: 1
`
		opext.AssertDeploy(types.NamespacedName{Namespace: "cpaas-system", Name: "alb-1"}, alb1, nil)
		opext.AssertDeploy(types.NamespacedName{Namespace: "cpaas-system", Name: "alb-2"}, alb2, nil)
		opext.AssertDeploy(types.NamespacedName{Namespace: "cpaas-system", Name: "alb-3"}, alb3, nil)
	})

	GIt("should work normaly when update", func() {
		type TestExpect struct {
			bindNic        string
			portInfo       string
			albLabel       map[string]string
			featureaddress string
		}
		type TestCase struct {
			alb    string
			env    *config.OperatorCfg
			expect TestExpect
		}
		cases := []TestCase{
			{
				alb: `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: ares-alb2
    namespace: cpaas-system
    labels:
        alb.cpaas.io/managed-by: alb-operator
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        replicas: 1
        `,
				expect: TestExpect{
					bindNic:  "{}",
					portInfo: "[]",
					albLabel: map[string]string{
						"alb.cpaas.io/managed-by": "alb-operator",
					},
					featureaddress: "127.0.0.1",
				},
			},
			{
				alb: `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: ares-alb2
    namespace: cpaas-system
    labels:
        alb.cpaas.io/managed-by: alb-operator
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        replicas: 1
        portProjects: '[{"port":"113-333","projects":["p1"]}]'
        bindNIC: '{"nic":["eth0"]}'
        `,
				expect: TestExpect{
					portInfo: `[{"port":"113-333","projects":["p1"]}]`,
					bindNic:  `{"nic":["eth0"]}`,
					albLabel: map[string]string{
						"alb.cpaas.io/managed-by": "alb-operator",
					},
					featureaddress: "127.0.0.1",
				},
			},
			{
				alb: `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: ares-alb2
    namespace: cpaas-system
    labels:
        alb.cpaas.io/managed-by: alb-operator
spec:
    address: "127.0.0.1,127.0.0.3"
    type: "nginx" 
    config:
        replicas: 1
        projects: ["A2"]
        `,
				expect: TestExpect{
					portInfo: `[]`,
					bindNic:  `{}`,

					albLabel: map[string]string{
						"alb.cpaas.io/managed-by": "alb-operator",
						"project.cpaas.io/A2":     "true",
					},
					featureaddress: "127.0.0.1,127.0.0.3",
				},
			},
			{
				alb: `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: ares-alb2
    namespace: cpaas-system
    labels:
        alb.cpaas.io/managed-by: alb-operator
spec:
    address: "127.0.0.1,127.0.0.2"
    type: "nginx" 
    config:
        replicas: 1
        projects: ["A1"]
        `,
				expect: TestExpect{
					portInfo: `[]`,
					bindNic:  `{}`,

					albLabel: map[string]string{
						"alb.cpaas.io/managed-by": "alb-operator",
						"project.cpaas.io/A1":     "true",
					},
					featureaddress: "127.0.0.1,127.0.0.2",
				},
			},
		}
		for i, tc := range cases {
			log.Info("loop", "index", i)
			key := types.NamespacedName{Namespace: "cpaas-system", Name: "ares-alb2"}
			opext.AssertDeploy(key, tc.alb, tc.env)
			common := &corev1.ConfigMap{}
			err := cli.Get(ctx, key, common)
			GinkgoNoErr(err)
			assert.Equal(GinkgoT(), common.Data["bind_nic"], tc.expect.bindNic)
			port := &corev1.ConfigMap{}
			err = cli.Get(ctx, client.ObjectKey{Name: "ares-alb2-port-info", Namespace: "cpaas-system"}, port)
			GinkgoNoErr(err)
			assert.Equal(GinkgoT(), tc.expect.portInfo, port.Data["range"])
			alb := &albv2.ALB2{}
			err = cli.Get(ctx, key, alb)
			GinkgoNoErr(err)
			assert.Equal(GinkgoT(), tc.expect.albLabel, alb.Labels, "index %v", i)

			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "infrastructure.alauda.io",
				Kind:    "Feature",
				Version: "v1alpha1",
			})
			featureKey := client.ObjectKey{Namespace: "", Name: fmt.Sprintf("%s-%s", "ares-alb2", "cpaas-system")}
			err = cli.Get(ctx, featureKey, u)
			GinkgoNoErr(err)
			addr, find, err := unstructured.NestedString(u.Object, "spec", "accessInfo", "host")
			GinkgoNoErr(err)
			GinkgoAssertTrue(find, "")
			log.Info("addr in test", "addr", addr)
			assert.Equal(GinkgoT(), tc.expect.featureaddress, addr, "index %v", i)
		}
	})
})
