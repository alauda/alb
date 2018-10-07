package modules

import (
	"fmt"
	"math/rand"
	"strings"
)

type AlaudaLoadBalancer struct {
	Alb2Spec
	Name      string
	Namespace string
	Frontends []*Frontend
}

func (alb *AlaudaLoadBalancer) NewFrontend(port int, protocol string) (*Frontend, error) {
	ft := &Frontend{
		Name: fmt.Sprintf("%s-%d-%s", alb.Name, port, protocol),
		FrontendSpec: FrontendSpec{
			Port:     port,
			Protocol: protocol,
		},
		LB: alb,
	}
	alb.Frontends = append(alb.Frontends, ft)
	return ft, nil
}

func (alb *AlaudaLoadBalancer) ListDomains() []string {
	domains := make([]string, 0, len(alb.Domains))
	for _, d := range alb.Domains {
		offset := 0
		for idx, c := range d {
			if c != '*' && c != '.' && c != ' ' {
				offset = idx
				break
			}
		}
		domains = append(domains, d[offset:])
	}
	return domains
}

type Frontend struct {
	Name string
	FrontendSpec
	Rules []*Rule

	LB *AlaudaLoadBalancer
}

const ALPHANUM = "0123456789abcdefghijklmnopqrstuvwxyz"

func RandomStr(pixff string, length int) string {
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = ALPHANUM[rand.Intn(len(ALPHANUM))]
	}
	return pixff + "-" + string(result)
}

func (ft *Frontend) NewRule(domain, url, dsl, typ string) (*Rule, error) {
	if domain != "" || url != "" {
		dsl = GetDSL(domain, url)
	}
	r := Rule{
		Name: RandomStr(ft.Name, 4),
		RuleSpec: RuleSpec{
			Domain: domain,
			URL:    url,
			DSL:    dsl,
			Type:   typ,
		},
		FT: ft,
	}
	ft.Rules = append(ft.Rules, &r)
	return &r, nil
}

const (
	RuleTypeBind = "bind"
)

type Rule struct {
	RuleSpec
	Name string

	FT *Frontend
}

func GetDSL(domain, url string) string {
	var dsl string
	if domain != "" && url != "" {
		if strings.HasPrefix(url, "^") {
			dsl = fmt.Sprintf("(AND (EQ HOST %s) (REGEX URL %s))", domain, url)
		} else {
			dsl = fmt.Sprintf("(AND (EQ HOST %s) (STARTS_WITH URL %s))", domain, url)
		}
	} else {
		if domain != "" {
			dsl = fmt.Sprintf("(EQ HOST %s)", domain)
		} else {
			if strings.HasPrefix(url, "^") {
				dsl = fmt.Sprintf("(REGEX URL %s)", url)
			} else {
				dsl = fmt.Sprintf("(STARTS_WITH URL %s)", url)
			}
		}
	}
	return dsl
}
