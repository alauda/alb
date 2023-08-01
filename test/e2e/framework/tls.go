package framework

import (
	"context"

	. "alauda.io/alb2/utils/test_utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TlsExt struct {
	Kc  *K8sClient
	Ctx context.Context
}

func (t *TlsExt) CreateTlsSecret(domain, name, ns string) (*corev1.Secret, error) {
	key, crt, err := GenCert(domain)
	if err != nil {
		return nil, err
	}
	secret, err := t.Kc.GetK8sClient().CoreV1().Secrets(ns).Create(t.Ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: map[string][]byte{
			"tls.key": []byte(key),
			"tls.crt": []byte(crt),
		},
		Type: corev1.SecretTypeTLS,
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return secret, nil
}
