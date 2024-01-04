package test_utils

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	AssertDeploymentFunc = func(*v1.Deployment) bool
	ExpectDeployment     struct {
		ExpectContainlerEnv   map[string]map[string]string
		UnexpectContainlerEnv map[string]map[string]string
		Hostnetwork           bool
		Test                  AssertDeploymentFunc
	}
)

type Resource struct {
	Ns    string
	Kind  string
	Names []string
}

type ExpectResource struct {
	ExpectExist    []Resource
	ExpectNotExist []Resource
}

type AssertHelper struct {
	kt    *Kubectl
	kc    *K8sClient
	ctx   context.Context
	omega Gomega
}

func NewAssertHelper(ctx context.Context, kc *K8sClient, kt *Kubectl) *AssertHelper {
	return &AssertHelper{
		kt:  kt,
		kc:  kc,
		ctx: ctx,
	}
}

func NewAssertHelperOmgea(ctx context.Context, kc *K8sClient, kt *Kubectl, o Gomega) *AssertHelper {
	return &AssertHelper{
		kt:    kt,
		kc:    kc,
		ctx:   ctx,
		omega: o,
	}
}

func (a *AssertHelper) AssertResource(expect ExpectResource) {
	for _, e := range expect.ExpectExist {
		for _, name := range e.Names {
			if e.Ns == "" {
				e.Ns = "''"
			}
			if a.omega != nil {
				a.kt.AssertKubectlOmgea(a.omega, "get", e.Kind, "-n", e.Ns, name)
			} else {
				a.kt.AssertKubectl("get", e.Kind, "-n", e.Ns, name)
			}
		}
	}

	for _, e := range expect.ExpectNotExist {
		for _, name := range e.Names {
			_, err := a.kt.Kubectl("get", e.Kind, "-n", e.Ns, name)
			errmsg := fmt.Sprintf("%v", err)
			a.assertContains(errmsg, "not found", fmt.Sprintf("%s %s should not exist", e.Kind, name))
		}
	}
}

func (a *AssertHelper) AssertDeployment(ns string, name string, expect ExpectDeployment) {
	c := a.kc.GetK8sClient()
	dep, err := c.AppsV1().Deployments(ns).Get(a.ctx, name, metaV1.GetOptions{})
	containers := dep.Spec.Template.Spec.Containers
	a.assertNotErr(err, "get deployment err")
	for cname, cenv := range expect.ExpectContainlerEnv {
		c, find := lo.Find(containers, func(c coreV1.Container) bool {
			return c.Name == cname
		})
		a.assertTrue(find, "container not found")
		for key, val := range cenv {
			v, find := lo.Find(c.Env, func(e coreV1.EnvVar) bool {
				return e.Name == key
			})
			a.assertTrue(find, "env %v not found expect val %v", key, val)
			a.assertTrue(v.Value == val, "env val %v != %v", v, val)
		}
	}
	if expect.Test != nil {
		ret := expect.Test(dep)
		a.assertTrue(ret, "ext test fail")
	}
}

func (a *AssertHelper) assertNotErr(err error, msg string) {
	if a.omega != nil {
		a.omega.Expect(err).To(BeNil(), msg)
	} else {
		assert.Nil(GinkgoT(), err, msg)
	}
}

func (a *AssertHelper) assertTrue(flag bool, sfmt string, opt ...any) {
	if a.omega == nil {
		assert.True(GinkgoT(), flag, fmt.Sprintf(sfmt, opt...))
	} else {
		a.omega.Expect(flag).To(BeTrue(), fmt.Sprintf(sfmt, opt...))
	}
}

func (a *AssertHelper) assertContains(str string, sub string, sfmt string, opt ...any) {
	if a.omega != nil {
		a.omega.Expect(str).To(ContainSubstring(sub))
	} else {
		assert.Contains(GinkgoT(), str, sub, fmt.Sprintf(sfmt, opt...))
	}
}
