package controller

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strconv"
	"strings"

	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getCertMap(alb *LoadBalancer, d *driver.KubernetesDriver) map[string]Certificate {
	certProtocol := map[albv1.FtProtocol]bool{
		albv1.FtProtocolHTTPS: true,
		albv1.FtProtocolgRPC:  true,
	}

	getPortDefaultCert := func(alb *LoadBalancer, d *driver.KubernetesDriver) map[string]client.ObjectKey {
		cm := make(map[string]client.ObjectKey)
		for _, ft := range alb.Frontends {
			if ft.Conflict || !certProtocol[ft.Protocol] || ft.CertificateName == "" {
				continue
			}
			ns, name, err := ParseCertificateName(ft.CertificateName)
			if err != nil {
				klog.Warningf("get cert %s failed, %+v", ft.CertificateName, err)
				continue
			}
			cm[strconv.Itoa(int(ft.Port))] = client.ObjectKey{Namespace: ns, Name: name}
		}
		return cm
	}

	getCertFromRule := func(alb *LoadBalancer, d *driver.KubernetesDriver) map[string]client.ObjectKey {
		cm := make(map[string]client.ObjectKey)
		for _, ft := range alb.Frontends {
			if ft.Conflict || !certProtocol[ft.Protocol] {
				continue
			}
			for _, rule := range ft.Rules {
				if rule.Domain == "" || rule.CertificateName == "" {
					continue
				}
				ns, name, err := ParseCertificateName(rule.CertificateName)
				if err != nil {
					klog.Warningf("get cert %s failed, %+v", rule.CertificateName, err)
					continue
				}
				cm[rule.Domain] = client.ObjectKey{Namespace: ns, Name: name}
			}
		}
		return cm
	}

	portDefaultCert := getPortDefaultCert(alb, d)
	certFromRule := getCertFromRule(alb, d)

	secretMap := make(map[string]client.ObjectKey)
	klog.Infof("port-cert %v %v", portDefaultCert, certFromRule)

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

	certMap := make(map[string]Certificate)
	certCache := make(map[string]Certificate)

	for domain, secret := range secretMap {
		secretkey := secret.String()
		if cert, ok := certCache[secretkey]; ok {
			certMap[domain] = cert
			continue
		}
		klog.V(3).Infof("get cert for domain %v %v", secretkey, domain)
		cert, err := getCertificateFromSecret(d, secret.Namespace, secret.Name)
		if err != nil {
			klog.Errorf("get cert %s failed, %+v", secret, err)
			continue
		}
		certMap[domain] = *cert
		certCache[secretkey] = *cert
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
