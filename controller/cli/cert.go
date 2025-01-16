package cli

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	pm "alauda.io/alb2/pkg/utils/metrics"
	"github.com/go-logr/logr"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getCertMap(alb *LoadBalancer, d *driver.KubernetesDriver) map[string]Certificate {
	s := time.Now()
	defer func() {
		pm.Write("gen-cert", float64(time.Since(s).Milliseconds()))
	}()
	certProtocol := map[albv1.FtProtocol]bool{
		albv1.FtProtocolHTTPS: true,
		albv1.FtProtocolgRPC:  true,
	}

	getPortDefaultCert := func(alb *LoadBalancer) map[string]client.ObjectKey {
		cm := make(map[string]client.ObjectKey)
		for _, ft := range alb.Frontends {
			if ft.Conflict || !certProtocol[ft.Protocol] || ft.CertificateName == "" {
				continue
			}
			ns, name, err := ParseCertificateName(ft.CertificateName)
			if err != nil {
				d.Log.Info("get cert failed", "cert", ft.CertificateName, "err", err)
				continue
			}
			cm[strconv.Itoa(int(ft.Port))] = client.ObjectKey{Namespace: ns, Name: name}
		}
		return cm
	}

	portDefaultCert := getPortDefaultCert(alb)
	certFromRule := formatCertsMap(getCertsFromRule(alb, certProtocol, d.Log))

	secretMap := make(map[string]client.ObjectKey)

	for port, secret := range portDefaultCert {
		if _, ok := secretMap[port]; !ok {
			secretMap[port] = secret
		}
	}
	for domain, secret := range certFromRule {
		if _, ok := secretMap[domain]; !ok {
			secretMap[domain] = secret
		}
	}
	d.Log.Info("get secrets", "secretMap", secretMap)

	certMap := make(map[string]Certificate)
	certCache := make(map[string]Certificate)

	for domain, secret := range secretMap {
		secretKey := secret.String()
		if cert, ok := certCache[secretKey]; ok {
			certMap[domain] = cert
			continue
		}
		d.Log.V(3).Info("get cert for domain", "key", secretKey, "domain", domain)
		cert, err := getCertificateFromSecret(d, secret.Namespace, secret.Name)
		if err != nil {
			d.Log.Error(err, "get secret failed", "secret", secret)
			continue
		}
		certMap[domain] = *cert
		certCache[secretKey] = *cert
	}
	return certMap
}

func getCertificateFromSecret(driver *driver.KubernetesDriver, namespace, name string) (*Certificate, error) {
	secret, err := driver.Client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if len(secret.Data[apiv1.TLSCertKey]) == 0 || len(secret.Data[apiv1.TLSPrivateKeyKey]) == 0 {
		return nil, errors.New("invalid secret")
	}
	_, err = tls.X509KeyPair(secret.Data[apiv1.TLSCertKey], secret.Data[apiv1.TLSPrivateKeyKey])
	if err != nil {
		return nil, err
	}
	key := string(secret.Data[apiv1.TLSPrivateKeyKey])
	cert := string(secret.Data[apiv1.TLSCertKey])
	caCert := string(secret.Data[CaCert])
	if len(caCert) != 0 {
		trimNewLine := func(s string) string {
			return strings.Trim(s, "\n")
		}
		cert = trimNewLine(cert) + "\n" + trimNewLine(caCert)
	}

	return &Certificate{
		Key:  key,
		Cert: cert,
	}, nil
}

func ParseCertificateName(n string) (string, string, error) {
	// backward compatibility
	if strings.Contains(n, "_") {
		slice := strings.Split(n, "_")
		if len(slice) != 2 {
			return "", "", errors.New("invalid certificate name")
		}
		return slice[0], slice[1], nil
	}
	if strings.Contains(n, "/") {
		slice := strings.Split(n, "/")
		if len(slice) != 2 {
			return "", "", fmt.Errorf("invalid certificate name, %s", n)
		}
		return slice[0], slice[1], nil
	}
	return "", "", fmt.Errorf("invalid certificate name, %s", n)
}

func SameCertificateName(left, right string) (bool, error) {
	ln, lc, err := ParseCertificateName(left)
	if err != nil {
		return false, err
	}
	rn, rc, err := ParseCertificateName(left)
	if err != nil {
		return false, err
	}
	return ln == rn && lc == rc, nil
}

// domain / ft / cert
func getCertsFromRule(alb *LoadBalancer, certProtocol map[albv1.FtProtocol]bool, log logr.Logger) map[string]map[string][]client.ObjectKey {
	cm := make(map[string]map[string][]client.ObjectKey)
	for _, ft := range alb.Frontends {
		if ft.Conflict || !certProtocol[ft.Protocol] {
			continue
		}
		port := strconv.Itoa(int(ft.Port))
		if cm[port] == nil {
			cm[port] = make(map[string][]client.ObjectKey)
		}
		ftMap := cm[port]
		for _, rule := range ft.Rules {
			if rule.Domain == "" || rule.CertificateName == "" {
				continue
			}
			ns, name, err := ParseCertificateName(rule.CertificateName)
			if err != nil {
				log.Info("get cert fail", "cert", rule.CertificateName, "err", err)
				continue
			}
			if ftMap[rule.Domain] == nil {
				ftMap[rule.Domain] = []client.ObjectKey{}
			}
			ftMap[rule.Domain] = append(ftMap[rule.Domain], client.ObjectKey{Namespace: ns, Name: name})
		}
	}
	return cm
}

func formatCertsMap(domainCertRaw map[string]map[string][]client.ObjectKey) map[string]client.ObjectKey {
	domainFtCerts := map[string]map[string]client.ObjectKey{} // domain ft cert
	domainCerts := map[string][]client.ObjectKey{}            // domain cert

	for ft, domains := range domainCertRaw {
		for domain, certs := range domains {
			// 一个端口下一个域名只能有一个证书
			sort.Slice(certs, func(i, j int) bool {
				return certs[i].String() < certs[j].String()
			})
			cert := certs[0]
			if domainFtCerts[domain] == nil {
				domainFtCerts[domain] = map[string]client.ObjectKey{}
			}
			if domainCerts[domain] == nil {
				domainCerts[domain] = []client.ObjectKey{}
			}
			domainFtCerts[domain][ft] = cert
			domainCerts[domain] = append(domainCerts[domain], cert)
		}
	}
	ret := map[string]client.ObjectKey{}
	for domain, certs := range domainCerts {
		if len(certs) == 1 {
			ret[domain] = certs[0]
			continue
		}
		for ft, cert := range domainFtCerts[domain] {
			ret[fmt.Sprintf("%s/%s", domain, ft)] = cert
		}
	}
	return ret
}

func (p *PolicyCli) setMetricsPortCert(cert map[string]Certificate) {
	port := p.opt.MetricsPort
	cert[fmt.Sprintf("%d", port)] = genMetricsCert()
}

var (
	metricsCert Certificate
	once        sync.Once
)

func init() {
	once.Do(func() {
		cert, key, _ := certutil.GenerateSelfSignedCertKey("localhost", []net.IP{}, []string{})
		metricsCert = Certificate{
			Cert: string(cert),
			Key:  string(key),
		}
	})
}

func genMetricsCert() Certificate {
	return metricsCert
}
