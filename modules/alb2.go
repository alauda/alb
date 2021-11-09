package modules

import (
	"fmt"
	"math/rand"
	"strings"

	"alauda.io/alb2/config"
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils"
	"github.com/thoas/go-funk"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
)

type AlaudaLoadBalancer struct {
	UID       types.UID
	Labels    map[string]string
	Spec      alb2v1.ALB2Spec
	Name      string
	Namespace string
	Frontends []*Frontend
	TweakHash string
}

func (alb *AlaudaLoadBalancer) NewFrontend(port int, protocol string, certificateName string) (*Frontend, error) {
	ft := &Frontend{
		Name: fmt.Sprintf("%s-%05d", alb.Name, port),
		FrontendSpec: alb2v1.FrontendSpec{
			Port:     port,
			Protocol: protocol,
		},
		LB: alb,
	}
	if certificateName != "" {
		ft.CertificateName = certificateName
	}
	alb.Frontends = append(alb.Frontends, ft)
	return ft, nil
}

func (alb *AlaudaLoadBalancer) ListDomains() []string {
	domains := make([]string, 0, len(alb.Spec.Domains))
	for _, d := range alb.Spec.Domains {
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
	UID    types.UID
	Lables map[string]string
	Name   string
	alb2v1.FrontendSpec
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

func (ft *Frontend) IsTcpOrUdp() bool {
	return funk.ContainsString([]string{ProtoTCP, ProtoUDP}, ft.Protocol)
}

func (ft *Frontend) IsHttpOrHttps() bool {
	return funk.ContainsString([]string{ProtoHTTP, ProtoHTTPS}, ft.Protocol)
}

func (ft *Frontend) NewRule(ingressInfo, domain, url, rewriteTarget, backendProtocol, certificateName string,
	enableCORS bool, corsAllowHeaders string, corsAllowOrigin string, redirectURL string, redirectCode int, vhost string, priority int, pathType networkingv1.PathType, ingressVersion string) (*Rule, error) {

	var (
		dsl  string
		dslx alb2v1.DSLX
	)
	if domain != "" || url != "" {
		dsl = GetDSL(domain, url)
		dslx = GetDSLX(domain, url, pathType)
	}

	sourceIngressVersion := fmt.Sprintf(config.Get("labels.source_ingress_version"), config.Get("DOMAIN"))
	r := Rule{
		Name:        RandomStr(ft.Name, 4),
		Annotations: map[string]string{sourceIngressVersion: ingressVersion},
		RuleSpec: alb2v1.RuleSpec{
			Domain:           domain,
			URL:              url,
			DSL:              dsl,
			DSLX:             dslx,
			Priority:         priority,
			RewriteBase:      url,
			RewriteTarget:    rewriteTarget,
			BackendProtocol:  backendProtocol,
			CertificateName:  certificateName,
			EnableCORS:       enableCORS,
			CORSAllowHeaders: corsAllowHeaders,
			CORSAllowOrigin:  corsAllowOrigin,
			RedirectURL:      redirectURL,
			RedirectCode:     redirectCode,
			VHost:            vhost,
			Description:      ingressInfo,
		},
		FT: ft,
	}
	ft.Rules = append(ft.Rules, &r)
	return &r, nil
}

type Rule struct {
	alb2v1.RuleSpec
	Name        string
	Labels      map[string]string
	Annotations map[string]string

	FT *Frontend
}

func (ft Frontend) AllowNoAddr() bool {
	return false
}

func (rl Rule) AllowNoAddr() bool {
	if rl.RedirectURL != "" {
		return true
	}
	return false
}

// Deprecated: instead by GetDSLX.
func GetDSL(domain, url string) string {
	var dsl string
	if domain != "" && url != "" {
		if strings.IndexAny(url, "^$():?[]*\\") != -1 {
			if !strings.HasPrefix(url, "^") {
				url = "^" + url
			}
			dsl = fmt.Sprintf("(AND (EQ HOST %s) (REGEX URL %s))", domain, url)
		} else {
			dsl = fmt.Sprintf("(AND (EQ HOST %s) (STARTS_WITH URL %s))", domain, url)
		}
	} else {
		if domain != "" {
			dsl = fmt.Sprintf("(EQ HOST %s)", domain)
		} else {
			if strings.IndexAny(url, "^$():?[]*\\") != -1 {
				if !strings.HasPrefix(url, "^") {
					url = "^" + url
				}
				dsl = fmt.Sprintf("(REGEX URL %s)", url)
			} else {
				dsl = fmt.Sprintf("(STARTS_WITH URL %s)", url)
			}
		}
	}
	return dsl
}

func GetDSLX(domain, url string, pathType networkingv1.PathType) alb2v1.DSLX {
	var dslx alb2v1.DSLX
	if url != "" {
		if pathType == networkingv1.PathTypeExact {
			dslx = append(dslx, alb2v1.DSLXTerm{
				Values: [][]string{
					{utils.OP_EQ, url},
				},
				Type: utils.KEY_URL,
			})
		} else if pathType == networkingv1.PathTypePrefix {
			dslx = append(dslx, alb2v1.DSLXTerm{
				Values: [][]string{
					{utils.OP_STARTS_WITH, url},
				},
				Type: utils.KEY_URL,
			})
		} else {
			// path is regex
			if strings.ContainsAny(url, "^$():?[]*\\") {
				if !strings.HasPrefix(url, "^") {
					url = "^" + url
				}

				dslx = append(dslx, alb2v1.DSLXTerm{
					Values: [][]string{
						{utils.OP_REGEX, url},
					},
					Type: utils.KEY_URL,
				})
			} else {
				dslx = append(dslx, alb2v1.DSLXTerm{
					Values: [][]string{
						{utils.OP_STARTS_WITH, url},
					},
					Type: utils.KEY_URL,
				})
			}
		}
	}
	if domain != "" {
		dslx = append(dslx, alb2v1.DSLXTerm{
			Values: [][]string{
				{utils.OP_EQ, domain},
			},
			Type: utils.KEY_HOST,
		})
	}
	return dslx
}
