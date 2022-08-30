package ingress

import (
	"fmt"
	"strings"

	"alauda.io/alb2/config"
	m "alauda.io/alb2/modules"
	"github.com/fatih/set"
	networkingv1 "k8s.io/api/networking/v1"
)

type Need struct {
	s set.Interface
}

func getIngressFtTypes(ing *networkingv1.Ingress, c config.IConfig) Need {
	ALBSSLStrategyAnnotation := fmt.Sprintf("alb.networking.%s/enable-https", c.GetDomain())
	ALBSSLAnnotation := fmt.Sprintf("alb.networking.%s/tls", c.GetDomain())
	defaultSSLStrategy := c.GetDefaultSSLStrategy()
	ingSSLStrategy := ing.Annotations[ALBSSLStrategyAnnotation]
	sslMap := parseSSLAnnotation(ing.Annotations[ALBSSLAnnotation])
	certs := make(map[string]string)
	for host, cert := range sslMap {
		if certs[strings.ToLower(host)] == "" {
			certs[strings.ToLower(host)] = cert
		}
	}

	needFtTypes := set.New(set.NonThreadSafe)
	hasDefaultBackend := HasDefaultBackend(ing)
	if hasDefaultBackend {
		needFtTypes.Add(m.ProtoHTTP)
	}
	for _, r := range ing.Spec.Rules {
		foundTLS := false
		for _, tls := range ing.Spec.TLS {
			for _, host := range tls.Hosts {
				if strings.EqualFold(r.Host, host) {
					needFtTypes.Add(m.ProtoHTTPS)
					foundTLS = true
				}
			}
		}
		if certs[strings.ToLower(r.Host)] != "" {
			needFtTypes.Add(m.ProtoHTTPS)
			foundTLS = true
		}
		if !foundTLS {
			if defaultSSLStrategy == BothSSLStrategy && ingSSLStrategy != "false" {
				needFtTypes.Add(m.ProtoHTTPS)
				needFtTypes.Add(m.ProtoHTTP)
			} else if (defaultSSLStrategy == AlwaysSSLStrategy && ingSSLStrategy != "false") ||
				(defaultSSLStrategy == RequestSSLStrategy && ingSSLStrategy == "true") {
				needFtTypes.Add(m.ProtoHTTPS)
			} else {
				needFtTypes.Add(m.ProtoHTTP)
			}
		}
	}

	if c.GetIngressHttpPort() == 0 {
		needFtTypes.Remove(m.ProtoHTTP)
	}
	return Need{
		s: needFtTypes,
	}
}

func (n Need) AddHttp() {
	n.s.Add(m.ProtoHTTP)
}

func (n Need) AddHttps() {
	n.s.Add(m.ProtoHTTPS)
}

func (n Need) NeedHttp() bool {
	return n.s.Has(m.ProtoHTTP)
}

func (n Need) NeedHttps() bool {
	return n.s.Has(m.ProtoHTTPS)
}
