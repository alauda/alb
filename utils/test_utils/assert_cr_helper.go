package test_utils

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AssertDeploymentFunc = func(*v1.Deployment) bool
type ExpectDeployment struct {
	ExpectContainlerEnv   map[string]map[string]string
	UnexpectContainlerEnv map[string]map[string]string
	Hostnetwork           bool
	Test                  AssertDeploymentFunc
}
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
	kt  *Kubectl
	kc  *K8sClient
	ctx context.Context
}

func NewAssertHelper(ctx context.Context, kc *K8sClient, kt *Kubectl) *AssertHelper {
	return &AssertHelper{
		kt:  kt,
		kc:  kc,
		ctx: ctx,
	}
}

func (a *AssertHelper) AssertResource(expect ExpectResource) {
	for _, e := range expect.ExpectExist {
		for _, name := range e.Names {
			if e.Ns == "" {
				e.Ns = "''"
			}
			a.kt.AssertKubectl("get", e.Kind, "-n", e.Ns, name)
		}
	}
	for _, e := range expect.ExpectNotExist {
		for _, name := range e.Names {
			_, err := a.kt.Kubectl("get", e.Kind, "-n", e.Ns, name)
			errmsg := fmt.Sprintf("%v", err)
			assert.Contains(ginkgo.GinkgoT(), errmsg, "not found", " %s %s should not exist", e.Kind, name)
		}
	}
}

func (a *AssertHelper) AssertDeployment(ns string, name string, expect ExpectDeployment) {
	c := a.kc.GetK8sClient()
	dep, err := c.AppsV1().Deployments(ns).Get(a.ctx, name, metaV1.GetOptions{})
	containers := dep.Spec.Template.Spec.Containers
	assert.Nil(ginkgo.GinkgoT(), err, "")
	for cname, cenv := range expect.ExpectContainlerEnv {
		c, find := lo.Find(containers, func(c coreV1.Container) bool {
			return c.Name == cname
		})
		assert.True(ginkgo.GinkgoT(), find, "container not found")
		for key, val := range cenv {
			v, find := lo.Find(c.Env, func(e coreV1.EnvVar) bool {
				return e.Name == key
			})
			assert.True(ginkgo.GinkgoT(), find, "env %v not found expect val %v", key, val)
			assert.True(ginkgo.GinkgoT(), v.Value == val, "env val %v != %v", v, val)
		}
	}
	if expect.Test != nil {
		ret := expect.Test(dep)
		assert.True(ginkgo.GinkgoT(), ret, "ext test fail")
	}
}
