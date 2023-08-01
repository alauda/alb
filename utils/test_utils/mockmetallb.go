package test_utils

import (
	"context"
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MockMetallb struct {
	cfg    *rest.Config
	log    logr.Logger
	ctx    context.Context
	cli    kubernetes.Interface
	v4pool mapset.Set[string]
	v6pool mapset.Set[string]
	v4used mapset.Set[string]
	v6used mapset.Set[string]
}

func NewMockMetallb(ctx context.Context, cfg *rest.Config, v4 []string, v6 []string, log logr.Logger) *MockMetallb {
	cli := kubernetes.NewForConfigOrDie(cfg)
	return &MockMetallb{
		cfg:    cfg,
		cli:    cli,
		log:    log,
		ctx:    ctx,
		v4pool: mapset.NewSet(v4...),
		v6pool: mapset.NewSet(v6...),
		v4used: mapset.NewSet[string](),
		v6used: mapset.NewSet[string](),
	}
}

func (m *MockMetallb) Start() {
	cli := m.cli
	w, err := cli.CoreV1().Services("").Watch(m.ctx, metav1.ListOptions{})
	if err != nil {
		panic(fmt.Errorf("watch svc fail %v", err))
	}
	// log.Info("start watch ", "gvr", wi.gvr, "ns", wi.ns)
	for event := range w.ResultChan() {
		svc, ok := event.Object.(*corev1.Service)
		if !ok {
			continue
		}
		err := m.onSvc(client.ObjectKeyFromObject(svc))
		if err != nil {
			m.log.Error(err, "fail")
		}
	}
}

func (m *MockMetallb) onSvc(key client.ObjectKey) error {
	cli := m.cli
	svc, err := cli.CoreV1().Services(key.Namespace).Get(m.ctx, key.Name, metav1.GetOptions{})
	if err != nil {
		m.log.Error(err, "get svc fail")
		return nil
	}
	if svc.Spec.Type != "LoadBalancer" {
		return nil
	}
	if len(svc.Status.LoadBalancer.Ingress) != 0 {
		return nil
	}
	policy := *svc.Spec.IPFamilyPolicy
	if policy == corev1.IPFamilyPolicyPreferDualStack || policy == corev1.IPFamilyPolicyRequireDualStack {
		svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
			{
				IP: m.getIpv4(),
			},
			{
				IP: m.getIpv6(),
			},
		}
	} else {
		if svc.Spec.IPFamilies[0] == corev1.IPv4Protocol {
			svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
				{
					IP: m.getIpv4(),
				},
			}
		}
		if svc.Spec.IPFamilies[0] == corev1.IPv6Protocol {
			svc.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
				{
					IP: m.getIpv6(),
				},
			}
		}
	}
	nsvc, err := cli.CoreV1().Services(key.Namespace).UpdateStatus(m.ctx, svc, metav1.UpdateOptions{})
	if err != nil {
		m.log.Error(err, "update svc fail")
		return nil
	}
	m.log.Info("update svc", "diff", cmp.Diff(nsvc, svc))
	return nil
}

func (m *MockMetallb) getIpv4() string {
	ips := m.v4pool.Difference(m.v4used).ToSlice()
	if len(ips) == 0 {
		panic("no ip")
	}
	ip := ips[0]
	m.v4used.Add(ip)
	return ips[0]
}

func (m *MockMetallb) getIpv6() string {
	ips := m.v6pool.Difference(m.v6used).ToSlice()
	if len(ips) == 0 {
		panic("no ip")
	}
	ip := ips[0]
	m.v6used.Add(ip)
	return ips[0]
}

func (m *MockMetallb) getDual() (string, string) {
	return m.getIpv4(), m.getIpv6()
}
