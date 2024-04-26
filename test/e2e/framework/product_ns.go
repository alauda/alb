package framework

import (
	"fmt"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProductNs struct {
	ns  string
	env *Env
}

type ProductNsOpt struct {
	Prefix  string
	Ns      string
	Project string
	Labels  map[string]string
}

func (f *ProductNs) InitProductNsWithOpt(opt ProductNsOpt) string {
	if opt.Labels == nil {
		opt.Labels = map[string]string{}
	}
	domain := f.env.Opext.operatorCfg.GetLabelBaseDomain()
	kc := f.env.Kc
	ctx := f.env.Ctx
	if opt.Project != "" {
		opt.Labels[fmt.Sprintf("%s/project", domain)] = opt.Project
	}
	opt.Ns = opt.Prefix
	ns, err := kc.GetK8sClient().CoreV1().Namespaces().Create(
		ctx,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   opt.Ns,
				Labels: opt.Labels,
			},
		},
		metav1.CreateOptions{},
	)
	assert.Nil(ginkgo.GinkgoT(), err, "create ns fail")
	return ns.Name
}

func (p *ProductNs) GetProductNs() string {
	return p.ns
}

func (p *ProductNs) InitProductNs(ns string, project string) string {
	ns = p.InitProductNsWithOpt(ProductNsOpt{
		Prefix:  ns,
		Project: project,
	})
	p.ns = ns
	return ns
}

func (f *ProductNs) InitDefaultSvc(name string, ep []string) {
	opt := SvcOpt{
		Ns:   f.ns,
		Name: name,
		Ep:   ep,
		Ports: []corev1.ServicePort{
			{
				Port: 80,
			},
		},
	}
	if strings.Contains(name, "udp") {
		opt.Ports[0].Protocol = "UDP"
	}
	f.env.SvcExt.initSvcWithOpt(opt)
}
