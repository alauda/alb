package framework

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	c "alauda.io/alb2/controller"
	ct "alauda.io/alb2/controller/types"
	a1t "alauda.io/alb2/pkg/apis/alauda/v1"
	"github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	. "alauda.io/alb2/utils/test_utils"
)

type (
	NgxPolicy    c.NgxPolicy
	BackendGroup ct.BackendGroup
)

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
	var psMap map[a1t.PortNumber]c.Policies
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

func (f *AlbHelper) CreateFt(port a1t.PortNumber, protocol string, svcName string, svcNs string) {
	name := fmt.Sprintf("%s-%05d", f.AlbName, port)
	if protocol == "udp" {
		name += "-udp"
	}
	ft := a1t.Frontend{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.AlbNs,
			Name:      name,
			Labels: map[string]string{
				"alb2.cpaas.io/name": f.AlbName,
			},
		},
		Spec: a1t.FrontendSpec{
			Port:     port,
			Protocol: a1t.FtProtocol(protocol),
			ServiceGroup: &a1t.ServiceGroup{Services: []a1t.Service{
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

func (f *AlbHelper) WaitFtState(name string, check func(ft *a1t.Frontend) (bool, error)) *a1t.Frontend {
	var ft *a1t.Frontend
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

func (f *AlbHelper) WaitFt(name string) *a1t.Frontend {
	return f.WaitFtState(name, func(ft *a1t.Frontend) (bool, error) {
		if ft != nil {
			return true, nil
		}
		return false, nil
	})
}
