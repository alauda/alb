package simple

import (
	"context"
	"fmt"

	"alauda.io/alb2/pkg/operator/config"
	f "alauda.io/alb2/test/e2e/framework"
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
	var base string
	var opext *f.AlbOperatorExt
	var kt *Kubectl
	var kc *K8sClient
	var cli client.Client
	var ctx context.Context
	var cancel func()

	var log logr.Logger
	BeforeEach(func() {
		log = GinkgoLog()
		ctx, cancel = context.WithCancel(context.Background())
		base = InitBase()
		kt = NewKubectl(base, KUBE_REST_CONFIG, log)
		kc = NewK8sClient(ctx, KUBE_REST_CONFIG)
		msg := kt.AssertKubectl("create ns cpaas-system")
		log.Info("init ns", "ret", msg)
		opext = f.NewAlbOperatorExt(ctx, base, KUBE_REST_CONFIG)
		cli = kc.GetDirectClient()
	})

	AfterEach(func() {
		cancel()
	})

	f.GIt("should work normaly when update", func() {
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
			f.GinkgoNoErr(err)
			assert.Equal(GinkgoT(), common.Data["bind_nic"], tc.expect.bindNic)
			port := &corev1.ConfigMap{}
			err = cli.Get(ctx, client.ObjectKey{Name: "ares-alb2-port-info", Namespace: "cpaas-system"}, port)
			f.GinkgoNoErr(err)
			assert.Equal(GinkgoT(), tc.expect.portInfo, port.Data["range"])
			alb := &albv2.ALB2{}
			err = cli.Get(ctx, key, alb)
			f.GinkgoNoErr(err)
			assert.Equal(GinkgoT(), tc.expect.albLabel, alb.Labels, "index %v", i)

			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "infrastructure.alauda.io",
				Kind:    "Feature",
				Version: "v1alpha1",
			})
			featureKey := client.ObjectKey{Namespace: "", Name: fmt.Sprintf("%s-%s", "ares-alb2", "cpaas-system")}
			err = cli.Get(ctx, featureKey, u)
			f.GinkgoNoErr(err)
			addr, find, err := unstructured.NestedString(u.Object, "spec", "accessInfo", "host")
			f.GinkgoNoErr(err)
			f.GinkgoAssertTrue(find, "")
			log.Info("addr in test", "addr", addr)
			assert.Equal(GinkgoT(), tc.expect.featureaddress, addr, "index %v", i)
		}
	})
})