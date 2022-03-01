package framework

import (
	"encoding/json"
	"fmt"
	"strings"

	c "alauda.io/alb2/controller"
	ct "alauda.io/alb2/controller/types"
	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

type NgxPolicy c.NgxPolicy
type BackendGroup ct.BackendGroup

func (p NgxPolicy) String() string {
	ret, err := json.MarshalIndent(p, "", "  ")
	assert.NotNil(ginkgo.GinkgoT(), err)
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
		Logf("could not find this policy %s %s", mode, rule)
		// QUESTION: return a not found error?
		return false, nil
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
	if !bg.Eq(expectBg) {
		return false, fmt.Errorf("bg not eq %v %v", bg, expectBg)
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
	var psMap map[int]c.Policies
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
				retPort = port
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
