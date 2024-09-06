package test_utils

import (
	"encoding/json"
	"fmt"
	"strings"

	ct "alauda.io/alb2/controller/types"
	a1t "alauda.io/alb2/pkg/apis/alauda/v1"

	corev1 "k8s.io/api/core/v1"
)

type (
	NgxPolicy    ct.NgxPolicy
	BackendGroup ct.BackendGroup
)

type PolicyAssert func(p ct.Policy) bool

func NgxPolicyFromRaw(raw string) NgxPolicy {
	p := NgxPolicy{}
	err := json.Unmarshal([]byte(raw), &p)
	if err != nil {
		panic(err)
	}
	return p
}

func (p NgxPolicy) String() string {
	ret, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(ret)
}

func (p NgxPolicy) CertEq(domain string, secret *corev1.Secret) bool {
	cert, find := p.CertificateMap[domain]
	if !find {
		return false
	}
	expectKey := string(secret.Data["tls.key"])
	expectCert := string(secret.Data["tls.crt"])

	find = cert.Cert == expectCert && cert.Key == expectKey
	return find
}

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

func (p NgxPolicy) FindPolicy(mode, rule string) (*ct.Policy, int, *ct.BackendGroup) {
	var retP *ct.Policy
	var retPort int
	var retBg *ct.BackendGroup
	var psMap map[a1t.PortNumber]ct.Policies
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

func (p NgxPolicy) FindTcpPolicy(rule string) (*ct.Policy, int, *ct.BackendGroup) {
	return p.FindPolicy("tcp", rule)
}

func (p NgxPolicy) FindUdpPolicy(rule string) (*ct.Policy, int, *ct.BackendGroup) {
	return p.FindPolicy("udp", rule)
}

func (p NgxPolicy) FindHttpPolicy(rule string) (*ct.Policy, int, *ct.BackendGroup) {
	return p.FindPolicy("http", rule)
}

func (p NgxPolicy) FindHttpPolicyOnly(rule string) *ct.Policy {
	r, _, _ := p.FindPolicy("http", rule)
	return r
}
