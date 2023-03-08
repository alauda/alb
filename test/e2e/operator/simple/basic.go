package simple

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"

	f "alauda.io/alb2/test/e2e/framework"
	"github.com/go-logr/logr"
	"github.com/kr/pretty"
	. "github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	. "alauda.io/alb2/utils/test_utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO 真正的operator的测试应该将controller启动起来,是否要想办法将这个移到pkg中呢,不要放在e2e里
var _ = Describe("Operator Simple", func() {
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
		cli = kc.GetClient()
	})

	AfterEach(func() {
		cancel()
	})

	f.GIt("test v1 client", func() {
		// 如果在升级到v2之后，有人继续用v1的client更新了alb。
		kt.AssertKubectlApply(`
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
  name: alb-v2
  namespace: cpaas-system
spec:
  address: 127.0.0.1
  type: nginx
  config:
    networkMode: host
    replicas: 1
    projects:
      - ALL_ALL
`)
		alb := &albv2.ALB2{}
		cli.Get(ctx, client.ObjectKey{Namespace: "cpaas-system", Name: "alb-v2"}, alb)
		fmt.Printf("createed %+v\n", alb.Spec.Config)
		albc := kc.GetAlbClient()
		v1alb, err := albc.CrdV1().ALB2s("cpaas-system").Get(ctx, "alb-v2", metav1.GetOptions{})
		fmt.Printf("v1 origin alb %v %v\n", v1alb.ResourceVersion, v1alb.Spec)
		f.GinkgoNoErr(err)
		{
			v1alb.Annotations["a"] = "b"
			v1alb, err = albc.CrdV1().ALB2s("cpaas-system").Update(ctx, v1alb, metav1.UpdateOptions{})
			fmt.Printf("v1 update1 alb %v %+v\n", v1alb.ResourceVersion, v1alb)
			fmt.Printf("v1 update1 alb spec %+v\n", v1alb.Spec)
			f.GinkgoNoErr(err)
			v1alb.Annotations["a"] = "c"
			v1alb, err = albc.CrdV1().ALB2s("cpaas-system").Update(ctx, v1alb, metav1.UpdateOptions{})
			fmt.Printf("v1 update2 alb %v %+v\n", v1alb.ResourceVersion, v1alb)
			f.GinkgoNoErr(err)
		}
		// v1alb
		v2alb, err := albc.CrdV2beta1().ALB2s("cpaas-system").Get(ctx, "alb-v2", metav1.GetOptions{})
		f.GinkgoNoErr(err)
		fmt.Printf("v2 alb %v %v \n", v2alb.ResourceVersion, v2alb)
	})

	f.GIt("test dynamic client", func() {
		kt.AssertKubectlApply(`
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
  name: alb-v2
  namespace: cpaas-system
spec:
  address: 127.0.0.1
  type: nginx
  config:
    networkMode: host
    replicas: 1
    projects:
      - ALL_ALL
`)
		alb := &albv2.ALB2{}
		cli.Get(ctx, client.ObjectKey{Namespace: "cpaas-system", Name: "alb-v2"}, alb)
		fmt.Printf("%v\n", alb.Spec.Config)
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "crd.alauda.io",
			Kind:    "ALB2",
			Version: "v2beta1",
		})
		cli.Get(ctx, client.ObjectKey{Namespace: "cpaas-system", Name: "alb-v2"}, u)
		fmt.Printf("dynamic %v\n", u)
		values := map[string]interface{}{
			"a": "b",
		}
		unstructured.SetNestedField(u.Object, values, "spec", "config")
		err := cli.Update(ctx, u)
		assert.NoError(GinkgoT(), err)

		u = &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "crd.alauda.io",
			Kind:    "ALB2",
			Version: "v2beta1",
		})
		err = cli.Get(ctx, client.ObjectKey{Namespace: "cpaas-system", Name: "alb-v2"}, u)
		assert.NoError(GinkgoT(), err)
		fmt.Printf("\n\ndynamic %v\n", u)
	})

	f.GIt("deploy normal alb", func() {
		alb := `
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
        address: 192.168.134.195
        loadbalancerName: ares-alb2
        nodeSelector:
          kubernetes.io/hostname: 192.168.134.195
        projects:
        - ALL_ALL
        resources:
          limits:
            cpu: "2400m"
            memory: 2Gi
          requests:
            cpu: 50m
            memory: 128Mi
        replicas: 1
        `
		opext.AssertDeploy(types.NamespacedName{Namespace: "cpaas-system", Name: "ares-alb2"}, alb, nil)
		a := NewAssertHelper(ctx, kc, kt)
		a.AssertResource(ExpectResource{
			ExpectExist: []Resource{
				{
					Ns:    "cpaas-system",
					Kind:  "deployment",
					Names: []string{"ares-alb2"},
				},
				{
					Ns:    "cpaas-system",
					Kind:  "feature",
					Names: []string{"ares-alb2-cpaas-system"},
				},
				{
					Ns:    "",
					Kind:  "IngressClass",
					Names: []string{"ares-alb2"},
				},
				{
					Ns:    "cpaas-system",
					Kind:  "service",
					Names: []string{"ares-alb2"},
				},
			},

			ExpectNotExist: []Resource{
				{
					Ns:    "",
					Kind:  "GatewayClass",
					Names: []string{"ares-alb2"},
				},
			},
		})
		a.AssertDeployment("cpaas-system", "ares-alb2", ExpectDeployment{
			ExpectContainlerEnv: map[string]map[string]string{"alb2": {
				"MODE":           "controller",
				"NETWORK_MODE":   "host",
				"ALB_ENABLE":     "true",
				"SERVE_INGRESS":  "true",
				"GATEWAY_ENABLE": "false",
				"CPU_PRESET":     "3",
			}},
			Test: func(dep *v1.Deployment) bool {
				spec := dep.Spec.Template.Spec
				return spec.HostNetwork &&
					spec.DNSPolicy == "ClusterFirstWithHostNet" &&
					spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil &&
					spec.NodeSelector["kubernetes.io/hostname"] == "192.168.134.195" &&
					spec.Containers[0].Resources.Limits.Cpu().String() == "2400m" &&
					spec.Tolerations[0].Operator == "Exists" &&
					dep.Spec.Template.Labels["alb2.cpaas.io/type"] == "local" && // 必须设置这个label 反亲和性才有用
					true
			},
		})
		// service上必须有servcie_name = alb2-${name}的label,监控才能采到这个alb
		svc := &corev1.Service{}
		cli.Get(ctx, client.ObjectKey{Namespace: "cpaas-system", Name: "ares-alb2"}, svc)
		assert.Equal(GinkgoT(), "alb2-ares-alb2", svc.Labels["service_name"])
		assert.Equal(GinkgoT(), "alb2-ares-alb2", svc.Spec.Selector["service_name"])
	})

	f.GIt("deploy hr-host-gateway mode alb", func() {
		alb :=
			`
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
        nodeSelector:
            kubernetes.io/hostname: 192.168.134.195
        gateway:
            enable: true
            mode: gatewayclass
        replicas: 2
        `
		name := "hr-host-gateway"
		_ = name
		_ = alb

		opext.AssertDeploy(types.NamespacedName{Namespace: "cpaas-system", Name: "ares-alb2"}, alb, nil)
		a := NewAssertHelper(ctx, kc, kt)
		a.AssertResource(ExpectResource{
			ExpectExist: []Resource{
				{
					Ns:    "cpaas-system",
					Kind:  "deployment",
					Names: []string{"ares-alb2"},
				},
				{
					Ns:    "cpaas-system",
					Kind:  "feature",
					Names: []string{"ares-alb2-cpaas-system"},
				},
				{
					Ns:    "",
					Kind:  "IngressClass",
					Names: []string{"ares-alb2"},
				},
				{
					Ns:    "",
					Kind:  "GatewayClass",
					Names: []string{"ares-alb2"},
				},
			},
		})
		a.AssertDeployment("cpaas-system", "ares-alb2", ExpectDeployment{
			ExpectContainlerEnv: map[string]map[string]string{"alb2": {
				"MODE":           "controller",
				"NETWORK_MODE":   "host",
				"ALB_ENABLE":     "true",
				"SERVE_INGRESS":  "true",
				"GATEWAY_ENABLE": "true",
				"GATEWAY_MODE":   "gatewayclass",
			}},
			Test: func(dep *v1.Deployment) bool {
				spec := dep.Spec.Template.Spec
				log.Info("depl", "spec", pretty.Formatter(spec))
				return spec.HostNetwork == true &&
					spec.DNSPolicy == "ClusterFirstWithHostNet" &&
					spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil &&
					spec.NodeSelector["kubernetes.io/hostname"] == "192.168.134.195" &&
					true
			},
		})
	})

	f.GIt("deploy like apprelease", func() {
		alb := `
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: global-alb2
    namespace: cpaas-system
    labels:
        alb.cpaas.io/managed-by: alb-operator
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        antiAffinityKey: system
        defaultSSLCert: cpaas-system/dex.tls
        defaultSSLStrategy: Both
        ingressHTTPPort: 80
        ingressHTTPSPort: 443
        loadbalancerName: global-alb2
        metricsPort: 11782
        nodeSelector:
        ingress: "true"
        projects:
            - cpaas-system
        replicas: 3
        global:
            labelBaseDomain: cpaas.io
            namespace: cpaas-system
`

		opext.AssertDeploy(types.NamespacedName{Namespace: "cpaas-system", Name: "global-alb2"}, alb, nil)
		a := NewAssertHelper(ctx, kc, kt)
		a.AssertResource(ExpectResource{
			ExpectExist: []Resource{
				{
					Ns:    "cpaas-system",
					Kind:  "deployment",
					Names: []string{"global-alb2"},
				},
				{
					Ns:    "cpaas-system",
					Kind:  "feature",
					Names: []string{"global-alb2-cpaas-system"},
				},
				{
					Ns:    "",
					Kind:  "IngressClass",
					Names: []string{"global-alb2"},
				},
			},

			ExpectNotExist: []Resource{
				{
					Ns:    "",
					Kind:  "GatewayClass",
					Names: []string{"gateway-g1"},
				},
			},
		})
		a.AssertDeployment("cpaas-system", "global-alb2", ExpectDeployment{
			ExpectContainlerEnv: map[string]map[string]string{"alb2": {
				"MODE":           "controller",
				"NETWORK_MODE":   "host",
				"ALB_ENABLE":     "true",
				"SERVE_INGRESS":  "true",
				"GATEWAY_ENABLE": "false",
			}},
			Test: func(dep *v1.Deployment) bool {
				spec := dep.Spec.Template.Spec
				return spec.HostNetwork == true &&
					spec.DNSPolicy == "ClusterFirstWithHostNet" &&
					spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil
			},
		})
	})

	f.GIt("deploy like container networking mode", func() {
		alb :=
			`
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: gateway-g1
    namespace: cpaas-system
    labels:
        alb.cpaas.io/managed-by: alb-operator
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        networkMode: container
        enableALb: false
        enableIngress: "false"
        gateway:
            enable: true
            mode: gateway
            gatewayModeCfg:
                name: "n1/g1"
        replicas: 3
            `
		opext.AssertDeploy(types.NamespacedName{Namespace: "cpaas-system", Name: "gateway-g1"}, alb, nil)
		a := NewAssertHelper(ctx, kc, kt)
		a.AssertResource(ExpectResource{
			ExpectExist: []Resource{
				{
					Ns:    "cpaas-system",
					Kind:  "deployment",
					Names: []string{"gateway-g1"},
				},
				{
					Ns:    "cpaas-system",
					Kind:  "alb2",
					Names: []string{"gateway-g1"},
				},
				// TODO service 应该是由operator创建出来的。
				{
					Ns:    "cpaas-system",
					Kind:  "service",
					Names: []string{"gateway-g1-tcp", "gateway-g1-udp"},
				},
			},
			ExpectNotExist: []Resource{
				{
					Ns:    "cpaas-system",
					Kind:  "feature",
					Names: []string{"gateway-g1-cpaas-system"},
				},
				{
					Ns:    "",
					Kind:  "IngressClass",
					Names: []string{"gateway-g1"},
				},
				{
					Ns:    "",
					Kind:  "GatewayClass",
					Names: []string{"gateway-g1"},
				},
			},
		})
		a.AssertDeployment("cpaas-system", "gateway-g1", ExpectDeployment{
			ExpectContainlerEnv: map[string]map[string]string{"alb2": {
				"MODE":           "controller",
				"NETWORK_MODE":   "container",
				"ALB_ENABLE":     "false",
				"SERVE_INGRESS":  "false",
				"GATEWAY_ENABLE": "true",
				"GATEWAY_MODE":   "gateway",
				"GATEWAY_NAME":   "n1/g1",
			}},
			Hostnetwork: false,
			Test: func(dep *v1.Deployment) bool {
				spec := dep.Spec.Template.Spec
				f.Logf("spec %# v", pretty.Formatter(spec.Affinity))
				return spec.HostNetwork == false &&
					spec.DNSPolicy == "ClusterFirst" &&
					*dep.Spec.Replicas == 3 &&
					spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil
			},
		})
	})
})
