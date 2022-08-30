package config

import "sigs.k8s.io/controller-runtime/pkg/client"

func GetAlbKey(c IConfig) client.ObjectKey {
	return client.ObjectKey{
		Namespace: c.GetNs(),
		Name:      c.GetAlbName(),
	}
}
