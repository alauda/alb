package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	c "alauda.io/alb2/controller"
	ct "alauda.io/alb2/controller/types"
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	. "alauda.io/alb2/utils/test_utils"
)

type NgxPolicy c.NgxPolicy
type BackendGroup ct.BackendGroup

func (p NgxPolicy) String() string {
	ret, err := json.MarshalIndent(p, "", "  ")
	assert.Nil(ginkgo.GinkgoT(), err)
	return string(ret)
}

func (p NgxPolicy) CertEq(domain string, secret *corev1.Secret) bool {
	cert, find := p.CertificateMap[domain]
	if !find {
		Logf("could not cert for %v", domain)
		return false
	}
	expectKey := string(secret.Data["tls.key"])
	expectCert := string(secret.Data["tls.crt"])

	find = cert.Cert == expectCert && cert.Key == expectKey
	if !find {
		Logf("cert eq  %v %v %v %v %v", find, cert.Cert, expectCert, cert.Key, expectKey)
	}
	return find
}

type PolicyAssert func(p c.Policy) bool

func (p NgxPolicy) PolicyEq(mode, rule string, expectPort int, expectDsl string, expectBg ct.BackendGroup, policyasserts ...PolicyAssert) (bool, error) {
	policy, port, bg := p.FindPolicy(mode, rule)
	if policy == nil {
		return false, fmt.Errorf("policy not found %v", rule)
	}
	if port != expectPort {
		return false, fmt.Errorf("port not eq %v %v", port, expectPort)
	}
	dsl := policy.InternalDSL
	dslByte, err := json.Marshal(dsl)
	if err != nil {
		return false, nil
	}
	dslStr := string(dslByte)

	if strings.Compare(dslStr, expectDsl) != 0 {
		Logf("left -%s- right -%s-", dslStr, expectDsl)
		return false, fmt.Errorf("dsl not eq %v %v", dsl, expectDsl)
	}
	if !bg.Backends.Eq(expectBg.Backends) {
		return false, fmt.Errorf("bg not eq %v %v", bg.Backends, expectBg.Backends)
	}
	for i, a := range policyasserts {
		if !a(*policy) {
			return false, fmt.Errorf("%v assert not match", i)
		}
	}
	return true, nil
}

func (p NgxPolicy) FindPolicy(mode, rule string) (*c.Policy, int, *ct.BackendGroup) {
	var retP *c.Policy
	var retPort int
	var retBg *ct.BackendGroup
	var psMap map[albv1.PortNumber]c.Policies
	switch mode {
	case "http":
		psMap = p.Http.Tcp
	case "tcp":
		psMap = p.Stream.Tcp
	case "udp":
		psMap = p.Stream.Udp
	}
	for port, ps := range psMap {
		for _, p := range ps {
			if p.Rule == rule {
				retP = p
				retPort = int(port)
				break
			}
		}
	}
	if retP == nil {
		return nil, 0, nil
	}
	for _, bg := range p.BackendGroup {
		if bg.Name == rule {
			retBg = bg
		}
	}
	return retP, retPort, retBg
}

func (p NgxPolicy) FindTcpPolicy(rule string) (*c.Policy, int, *ct.BackendGroup) {
	return p.FindPolicy("tcp", rule)
}
func (p NgxPolicy) FindUdpPolicy(rule string) (*c.Policy, int, *ct.BackendGroup) {
	return p.FindPolicy("udp", rule)
}

func (p NgxPolicy) FindHttpPolicy(rule string) (*c.Policy, int, *ct.BackendGroup) {
	return p.FindPolicy("http", rule)
}

type AlbHelper struct {
	*K8sClient
	AlbInfo
}

func (f *AlbHelper) CreateFt(port alb2v1.PortNumber, protocol string, svcName string, svcNs string) {
	name := fmt.Sprintf("%s-%05d", f.AlbName, port)
	if protocol == "udp" {
		name = name + "-udp"
	}
	ft := alb2v1.Frontend{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.AlbNs,
			Name:      name,
			Labels: map[string]string{
				"alb2.cpaas.io/name": f.AlbName,
			},
		},
		Spec: alb2v1.FrontendSpec{
			Port:     port,
			Protocol: alb2v1.FtProtocol(protocol),
			ServiceGroup: &alb2v1.ServiceGroup{Services: []alb2v1.Service{
				{
					Name:      svcName,
					Namespace: svcNs,
					Port:      80,
				},
			}},
		},
	}
	f.GetAlbClient().CrdV1().Frontends(f.AlbNs).Create(context.Background(), &ft, metav1.CreateOptions{})
}

func (f *AlbHelper) WaitFtState(name string, check func(ft *alb2v1.Frontend) (bool, error)) *alb2v1.Frontend {
	var ft *alb2v1.Frontend
	var err error
	err = wait.Poll(Poll, DefaultTimeout, func() (bool, error) {
		ft, err = f.GetAlbClient().CrdV1().Frontends(f.AlbNs).Get(context.Background(), name, metav1.GetOptions{})
		Logf("try get ft %s/%s ft %v", f.AlbNs, name, err)
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		ok, err := check(ft)
		if err == nil {
			return ok, nil
		}
		return ok, err
	})
	assert.NoError(ginkgo.GinkgoT(), err)
	return ft
}

func (f *AlbHelper) WaitFt(name string) *alb2v1.Frontend {
	return f.WaitFtState(name, func(ft *alb2v1.Frontend) (bool, error) {
		if ft != nil {
			return true, nil
		}
		return false, nil
	})
}

// TODO
func (f *AlbHelper) WaitAlbState(name string, check func(alb *alb2v1.ALB2) (bool, error)) *alb2v1.ALB2 {
	var globalAlb *alb2v1.ALB2
	err := wait.Poll(Poll, DefaultTimeout, func() (bool, error) {
		alb, err := f.GetAlbClient().CrdV1().ALB2s(f.AlbNs).Get(context.Background(), name, metav1.GetOptions{})
		Logf("try get alb %s/%s alb %v", f.AlbNs, name, err)
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		ok, err := check(alb)
		if err == nil {
			globalAlb = alb
			return ok, nil
		}
		return ok, err
	})
	assert.NoError(ginkgo.GinkgoT(), err)
	return globalAlb
}
