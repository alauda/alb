package framework

import (
	"context"

	. "alauda.io/alb2/utils/test_utils"
	"github.com/onsi/ginkgo"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SvcExt struct {
	kc  *K8sClient
	ctx context.Context
}

type SvcOpt struct {
	Ns    string
	Name  string
	Ep    []string
	Ports []corev1.ServicePort
}

func NewSvcExt(kc *K8sClient, ctx context.Context) *SvcExt {
	return &SvcExt{
		kc:  kc,
		ctx: ctx,
	}
}

func (s *SvcExt) initSvcWithOpt(opt SvcOpt) error {
	ns := opt.Ns
	name := opt.Name
	ep := opt.Ep
	f := s.kc

	service_spec := corev1.ServiceSpec{
		Ports: opt.Ports,
	}
	epPorts := lo.Map(opt.Ports, func(p corev1.ServicePort, _ int) corev1.EndpointPort {
		return corev1.EndpointPort{
			Name:        p.Name,
			Protocol:    p.Protocol,
			Port:        p.Port,
			AppProtocol: p.AppProtocol,
		}
	})
	_, err := f.GetK8sClient().CoreV1().Services(ns).Create(s.ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: service_spec,
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	address := []corev1.EndpointAddress{}
	for _, ip := range ep {
		address = append(address, corev1.EndpointAddress{IP: ip})
	}
	_, err = f.GetK8sClient().CoreV1().Endpoints(ns).Create(s.ctx, &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			Labels:    map[string]string{"kube-app": name},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: address,
				Ports:     epPorts,
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (f *SvcExt) InitSvcWithOpt(opt SvcOpt) {
	err := f.initSvcWithOpt(opt)
	assert.NoError(ginkgo.GinkgoT(), err)
}

func (f *SvcExt) InitSvc(ns, name string, ep []string) {
	opt := SvcOpt{
		Ns:   ns,
		Name: name,
		Ep:   ep,
		Ports: []corev1.ServicePort{
			{
				Port: 80,
			},
		},
	}
	f.initSvcWithOpt(opt)
}
