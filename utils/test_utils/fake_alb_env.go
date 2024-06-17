package test_utils

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alauda.io/alb2/utils/log"
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"

	. "github.com/onsi/gomega"

	. "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FakeAlbEnv is a env which is could apply FakeResource
type FakeAlbEnv struct {
	ctx  context.Context
	e    *EnvtestExt
	base string
	l    logr.Logger
	kc   *K8sClient
}

func NewFakeEnv() FakeAlbEnv {
	return FakeAlbEnv{
		ctx:  context.Background(),
		l:    log.L(),
		base: InitBase(),
	}
}

func (a *FakeAlbEnv) GetCfg() *rest.Config {
	return a.e.cfg
}

func (a *FakeAlbEnv) AssertStart() {
	a.e = NewEnvtestExt(a.base, a.l)
	a.e.AssertStart()

	a.kc = NewK8sClient(a.ctx, a.e.cfg)
}

func (a *FakeAlbEnv) ApplyFakes(fake FakeResource) error {
	for _, res := range fake.ListCr() {
		err := a.kc.ctlClient.Create(a.ctx, res)
		fmt.Printf("create cr %+v %s %s\n", PrettyCr(res), res.GetNamespace(), res.GetName())
		if err != nil {
			return fmt.Errorf("crate cr %+v fail %v", res, err)
		}
	}
	return nil
}

func (a *FakeAlbEnv) ClearFakes(fake FakeResource) error {
	crs := fake.ListCr()
	a.deleteAll(crs...)

	// make sure ns been deleted.
	// https://book.kubebuilder.io/reference/envtest#namespace-usage-limitation
	for _, ns := range fake.K8s.Namespaces {
		for {
			ns, err := a.kc.k8sClient.CoreV1().Namespaces().Get(a.ctx, ns.GetName(), metav1.GetOptions{})
			fmt.Printf("clearfakes ns %v err %v\n", ns, err)
			if apierrors.IsNotFound(err) {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}
	return nil
}

func (a *FakeAlbEnv) Stop() {
	a.e.Stop()
}

func (a *FakeAlbEnv) deleteAll(objs ...client.Object) {
	ctx := a.ctx
	cfg := a.e.cfg
	k8sClient := a.kc.GetClient()
	timeout := 10 * time.Second
	interval := 1 * time.Second

	RegisterFailHandler(Fail)
	// copy from  https://book.kubebuilder.io/reference/envtest#namespace-usage-limitation
	clientGo, err := kubernetes.NewForConfig(cfg)
	Expect(err).ShouldNot(HaveOccurred())
	for _, obj := range objs {
		Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, obj))).Should(Succeed())

		if ns, ok := obj.(*corev1.Namespace); ok {
			// Normally the kube-controller-manager would handle finalization
			// and garbage collection of namespaces, but with envtest, we aren't
			// running a kube-controller-manager. Instead we're gonna approximate
			// (poorly) the kube-controller-manager by explicitly deleting some
			// resources within the namespace and then removing the `kubernetes`
			// finalizer from the namespace resource so it can finish deleting.
			// Note that any resources within the namespace that we don't
			// successfully delete could reappear if the namespace is ever
			// recreated with the same name.

			// Look up all namespaced resources under the discovery API
			_, apiResources, err := clientGo.Discovery().ServerGroupsAndResources()
			Expect(err).ShouldNot(HaveOccurred())
			namespacedGVKs := make(map[string]schema.GroupVersionKind)
			for _, apiResourceList := range apiResources {
				defaultGV, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
				Expect(err).ShouldNot(HaveOccurred())
				for _, r := range apiResourceList.APIResources {
					if !r.Namespaced || strings.Contains(r.Name, "/") {
						// skip non-namespaced and subresources
						continue
					}
					gvk := schema.GroupVersionKind{
						Group:   defaultGV.Group,
						Version: defaultGV.Version,
						Kind:    r.Kind,
					}
					if r.Group != "" {
						gvk.Group = r.Group
					}
					if r.Version != "" {
						gvk.Version = r.Version
					}
					namespacedGVKs[gvk.String()] = gvk
				}
			}

			// Delete all namespaced resources in this namespace
			for _, gvk := range namespacedGVKs {
				var u unstructured.Unstructured
				u.SetGroupVersionKind(gvk)
				err := k8sClient.DeleteAllOf(ctx, &u, client.InNamespace(ns.Name))
				Expect(client.IgnoreNotFound(ignoreMethodNotAllowed(err))).ShouldNot(HaveOccurred())
			}

			Eventually(func() error {
				key := client.ObjectKeyFromObject(ns)
				if err := k8sClient.Get(ctx, key, ns); err != nil {
					return client.IgnoreNotFound(err)
				}
				// remove `kubernetes` finalizer
				const kubernetes = "kubernetes"
				finalizers := []corev1.FinalizerName{}
				for _, f := range ns.Spec.Finalizers {
					if f != kubernetes {
						finalizers = append(finalizers, f)
					}
				}
				ns.Spec.Finalizers = finalizers

				// We have to use the k8s.io/client-go library here to expose
				// ability to patch the /finalize subresource on the namespace
				_, err = clientGo.CoreV1().Namespaces().Finalize(ctx, ns, metav1.UpdateOptions{})
				return err
			}, timeout, interval).Should(Succeed())
		}

		Eventually(func() metav1.StatusReason {
			key := client.ObjectKeyFromObject(obj)
			if err := k8sClient.Get(ctx, key, obj); err != nil {
				return apierrors.ReasonForError(err)
			}
			return ""
		}, timeout, interval).Should(Equal(metav1.StatusReasonNotFound))
	}
}

func ignoreMethodNotAllowed(err error) error {
	if err != nil {
		if apierrors.ReasonForError(err) == metav1.StatusReasonMethodNotAllowed {
			return nil
		}
	}
	return err
}
