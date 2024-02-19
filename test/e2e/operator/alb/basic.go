package alb

import (
	"context"

	. "alauda.io/alb2/test/e2e/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/xorcare/pointer"

	ctl "alauda.io/alb2/controller"
	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/pkg/operator/controllers/depl/resources/workload"
	. "alauda.io/alb2/utils/test_utils"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Operator ContainerMode", func() {
	var env *OperatorEnv
	var kt *Kubectl
	var kc *K8sClient
	var ctx context.Context
	var log logr.Logger
	BeforeEach(func() {
		env = StartOperatorEnvOrDieWithOpt(OperatorEnvCfg{RunOpext: true})
		kt = env.Kt
		kc = env.Kc
		ctx = env.Ctx
		log = env.Log
	})

	AfterEach(func() {
		env.Stop()
	})

	GIt("container network alb", func() {
		alb := `
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
        loadbalancerName: ares-alb2
        networkMode: container
        vip:
            enableLbSvc: true
        enableALb: true
        enableIngress: "true"
        nodeSelector:
          kubernetes.io/hostname: 192.168.134.195
        projects:
        - ALL_ALL
        replicas: 1
        `
		ns := "cpaas-system"
		name := "alb-1"
		kt.AssertKubectlApply(alb)
		// do it manually
		go MakeDeploymentReady(ctx, kc.GetK8sClient(), ns, name)
		go MakeLbSvcReady(ctx, log, kc.GetK8sClient(), ns, name, "127.0.0.1", "fe80::42:f7ff:fe11:7195")
		Wait(func() (bool, error) {
			// wait alb status normal
			alb, err := kc.GetAlbClient().CrdV2beta1().ALB2s(ns).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			log.Info("check alb status", "status", PrettyJson(alb.Status))
			return alb.Status.State == albv2.ALB2StateRunning, nil
		})
		// check svc type default is true
		Wait(func() (bool, error) {
			svc, err := ctl.GetLbSvc(ctx, kc.GetK8sClient(), crcli.ObjectKey{Namespace: ns, Name: name}, "cpaas.io")
			if err != nil {
				return false, err
			}
			log.Info("check lb svc node port", "svc", PrettyCr(svc))
			return svc.Spec.Type == "LoadBalancer" && (svc.Spec.AllocateLoadBalancerNodePorts == nil || *svc.Spec.AllocateLoadBalancerNodePorts == true), nil
		})

		// check sa
		sa := &corev1.ServiceAccount{}
		kc.GetClient().Get(ctx, client.ObjectKey{Namespace: "cpaas-system", Name: "alb-1-serviceaccount"}, sa)
		assert.Equal(GinkgoT(), sa.ImagePullSecrets[0].Name, "mock")

		// switch to false
		{
			alb, err := kc.GetAlbClient().CrdV2beta1().ALB2s(ns).Get(ctx, name, metav1.GetOptions{})
			GinkgoNoErr(err)
			alb.Spec.Config.Vip.AllocateLoadBalancerNodePorts = pointer.Bool(false)
			_, err = kc.GetAlbClient().CrdV2beta1().ALB2s(ns).Update(ctx, alb, metav1.UpdateOptions{})
			GinkgoNoErr(err)
			Wait(func() (bool, error) {
				svc, err := ctl.GetLbSvc(ctx, kc.GetK8sClient(), crcli.ObjectKey{Namespace: ns, Name: name}, "cpaas.io")
				if err != nil {
					return false, err
				}
				log.Info("lb svc should be false", "svc", PrettyCr(svc))
				return svc.Spec.Type == "LoadBalancer" && (svc.Spec.AllocateLoadBalancerNodePorts != nil && *svc.Spec.AllocateLoadBalancerNodePorts == false), nil
			})
		}

		Wait(func() (bool, error) {
			alb, err := kc.GetAlbClient().CrdV2beta1().ALB2s(ns).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			log.Info("check address status", "status", alb.Status.Detail.AddressStatus)
			v4eq := StringsEq(alb.Status.Detail.AddressStatus.Ipv4, []string{"127.0.0.1"})
			v6eq := StringsEq(alb.Status.Detail.AddressStatus.Ipv6, []string{"fe80::42:f7ff:fe11:7195"})
			return alb.Status.Detail.AddressStatus.Ok && v4eq && v6eq, nil
		})

		log.Info("add lb svc annotation")
		kt.AssertKubectlApply(`
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
        loadbalancerName: ares-alb2
        vip:
            enableLbSvc: true
            lbSvcAnnotations:
                a: b
        networkMode: container
        enableALb: true
        enableIngress: "true"
        nodeSelector:
            kubernetes.io/hostname: 192.168.134.195
        projects:
        - ALL_ALL
        replicas: 1
`)
		Wait(func() (bool, error) {
			svc, err := ctl.GetLbSvc(ctx, kc.GetK8sClient(), crcli.ObjectKey{Namespace: ns, Name: name}, "cpaas.io")
			if err != nil {
				log.Error(err, "get svc err")
				return false, err
			}
			log.Info("check svc annotation", "annotation", svc.Annotations)
			return svc.Annotations["a"] == "b", nil
		})

		log.Info("disable lb svc")
		// disable lb svc
		{
			cli := kc.GetAlbClient().CrdV2beta1().ALB2s(ns)
			alb, err := cli.Get(ctx, name, metav1.GetOptions{})
			GinkgoNoErr(err)
			alb.Spec.Config.Vip.EnableLbSvc = false
			_, err = cli.Update(ctx, alb, metav1.UpdateOptions{})
			GinkgoNoErr(err)
			Wait(func() (bool, error) {
				// wait alb status normal
				alb, err := kc.GetAlbClient().CrdV2beta1().ALB2s(ns).Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				log.Info("check alb status", "status", PrettyJson(alb.Status))
				return alb.Status.State == albv2.ALB2StateRunning && len(alb.Status.Detail.AddressStatus.Ipv4) == 0, nil
			})
			_, err = ctl.GetLbSvc(ctx, kc.GetK8sClient(), crcli.ObjectKey{Namespace: ns, Name: name}, "cpaas.io")
			assert.True(GinkgoT(), k8serrors.IsNotFound(err))
		}

		log.Info("enable again,and change annotations")
		{
			cli := kc.GetAlbClient().CrdV2beta1().ALB2s(ns)
			alb, err := cli.Get(ctx, name, metav1.GetOptions{})
			GinkgoNoErr(err)
			alb.Spec.Config.Vip.EnableLbSvc = true
			alb.Spec.Config.Vip.LbSvcAnnotations = map[string]string{
				"b": "1",
			}
			_, err = cli.Update(ctx, alb, metav1.UpdateOptions{})
			GinkgoNoErr(err)
			Wait(func() (bool, error) {
				alb, err := kc.GetAlbClient().CrdV2beta1().ALB2s(ns).Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				// log.Info("check alb status", "status", PrettyCr(alb))
				return alb.Status.State == albv2.ALB2StateRunning, nil
			})
			svc, err := ctl.GetLbSvc(ctx, kc.GetK8sClient(), crcli.ObjectKey{Namespace: ns, Name: name}, "cpaas.io")
			GinkgoNoErr(err)
			assert.Equal(GinkgoT(), map[string]string{"b": "1", "alb.cpaas.io/origin_annotation": "{\"b\":\"1\"}"}, svc.Annotations)
		}

		log.Info("check volume cfg")
		{
			dep := &appv1.Deployment{}
			kc.GetClient().Get(ctx, crcli.ObjectKey{Namespace: "cpaas-system", Name: "alb-1"}, dep, &crcli.GetOptions{})
			vol := workload.VolumeCfgFromDepl(dep)
			log.Info("vol", "xx", PrettyJson(vol))
			assert.Equal(GinkgoT(), vol.Mounts, map[string]map[string]string{
				"alb2": {
					"share-conf": "/etc/alb2/nginx/",
					"tweak-conf": "/alb/tweak/",
				},
				"nginx": {
					"nginx-run":  "/alb/nginx/run/",
					"share-conf": "/etc/alb2/nginx/",
					"tweak-conf": "/alb/tweak/",
				},
			})
		}
		log.Info("enable allocateLoadBalancerNodePorts")
		{
			cli := kc.GetAlbClient().CrdV2beta1().ALB2s(ns)
			alb, err := cli.Get(ctx, name, metav1.GetOptions{})
			GinkgoNoErr(err)
			alb.Spec.Config.Vip.AllocateLoadBalancerNodePorts = pointer.Bool(true)
			_, err = cli.Update(ctx, alb, metav1.UpdateOptions{})
			GinkgoNoErr(err)
			Wait(func() (bool, error) {
				svc, err := ctl.GetLbSvc(ctx, kc.GetK8sClient(), crcli.ObjectKey{Namespace: ns, Name: name}, "cpaas.io")
				if err != nil {
					return false, err
				}
				log.Info("check enable lb svc", PrettyCr(svc))
				if svc.Spec.AllocateLoadBalancerNodePorts == nil {
					return false, err
				}
				return svc.Spec.Type == "LoadBalancer" && *svc.Spec.AllocateLoadBalancerNodePorts == true, nil
			})
		}

		log.Info("change lbsvc ipFamilyPolicy")
		{
			svc, err := ctl.GetLbSvc(ctx, kc.GetK8sClient(), crcli.ObjectKey{Namespace: ns, Name: name}, "cpaas.io")
			GinkgoNoErr(err)
			log.Info("svc", PrettyCr(svc))
			assert.Equal(GinkgoT(), string(*svc.Spec.IPFamilyPolicy), "PreferDualStack")

			cli := kc.GetAlbClient().CrdV2beta1().ALB2s(ns)
			alb, err := cli.Get(ctx, name, metav1.GetOptions{})
			GinkgoNoErr(err)
			policy := corev1.IPFamilyPolicySingleStack
			alb.Spec.Config.Vip.LbSvcIpFamilyPolicy = &policy
			_, err = cli.Update(ctx, alb, metav1.UpdateOptions{})
			GinkgoNoErr(err)
			EventuallySuccess(func(g gomega.Gomega) {
				svc, err := ctl.GetLbSvc(ctx, kc.GetK8sClient(), crcli.ObjectKey{Namespace: ns, Name: name}, "cpaas.io")
				GinkgoNoErr(err)
				log.Info("check svc", PrettyCr(svc))
				g.Expect(string(*svc.Spec.IPFamilyPolicy)).Should(gomega.Equal("SingleStack"))
			}, log)
		}
		log.Info("check ingress")
		{
			changeAndAssert := func(enable bool) {
				enableStr := "false"
				if enable {
					enableStr = "true"
				}
				cli := kc.GetAlbClient().CrdV2beta1().ALB2s(ns)
				alb, err := cli.Get(ctx, name, metav1.GetOptions{})
				GinkgoNoErr(err)
				alb.Spec.Config.DefaultIngressClass = pointer.Bool(enable)
				alb, err = cli.Update(ctx, alb, metav1.UpdateOptions{})
				GinkgoNoErr(err)
				log.Info("update-alb", "alb", PrettyCr(alb))
				EventuallySuccess(func(g gomega.Gomega) {
					class, err := kc.GetK8sClient().NetworkingV1().IngressClasses().Get(ctx, name, metav1.GetOptions{})
					log.Info("check ingressclass", "class", PrettyCr(class), "enable", enableStr)
					g.Expect(err).ShouldNot(gomega.HaveOccurred())
					g.Expect(class.Annotations["ingressclass.kubernetes.io/is-default-class"]).Should(gomega.Equal(enableStr))
				}, log)
			}

			class, err := kc.GetK8sClient().NetworkingV1().IngressClasses().Get(ctx, name, metav1.GetOptions{})
			GinkgoNoErr(err)
			log.Info("check ingress", "class", PrettyCr(class))
			GinkgoAssertTrue(class.Annotations["ingressclass.kubernetes.io/is-default-class"] == "false", "")

			changeAndAssert(true)
			changeAndAssert(false)
		}
	})

	GIt("configmap should update when overwrite configmap change", func() {
		log.Info("hello")
		kt.AssertKubectlApply(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg-1
  namespace: cpaas-system
data:
  http: '{}'
---
apiVersion: crd.alauda.io/v2beta1
kind: ALB2
metadata:
    name: alb-1
    namespace: cpaas-system
spec:
    address: "127.0.0.1"
    type: "nginx" 
    config:
        overwrite:
          configmap:
             - name: cpaas-system/cfg-1
        replicas: 1
`)
		EventuallySuccess(func(g gomega.Gomega) {
			cm := &corev1.ConfigMap{}
			err := kc.GetClient().Get(ctx, crcli.ObjectKey{Namespace: "cpaas-system", Name: "alb-1"}, cm)
			GNoErr(g, err)
			log.Info("cm is ", "cm", PrettyCr(cm), "http", cm.Data["http"])
			GEqual(g, cm.Data["http"], "{}")
		}, log)

		kt.AssertKubectlApply(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: cfg-1
  namespace: cpaas-system
data:
  http: '{"xx":"xx"}'
`)
		EventuallySuccess(func(g gomega.Gomega) {
			cm := &corev1.ConfigMap{}
			err := kc.GetClient().Get(ctx, crcli.ObjectKey{Namespace: "cpaas-system", Name: "alb-1"}, cm)
			GNoErr(g, err)
			log.Info("updated cm is ", "cm", PrettyCr(cm))
			GEqual(g, cm.Data["http"], `{"xx":"xx"}`)
		}, log)
	})
})
