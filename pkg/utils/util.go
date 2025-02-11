package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ToBoolOr(x string, backup bool) bool {
	if x == "" {
		return backup
	}
	return strings.ToLower(strings.TrimSpace(x)) == "true"
}

func MergeMap(a map[string]string, b map[string]string) map[string]string {
	ret := map[string]string{}
	for k, v := range a {
		ret[k] = v
	}
	for k, v := range b {
		ret[k] = v
	}
	return ret
}

func HashBytes(s []byte) string {
	h := sha256.New()
	h.Write(s)
	return hex.EncodeToString(h.Sum(nil))
}

func Hash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func ParseStringToObjectKey(key string) (client.ObjectKey, error) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 {
		return client.ObjectKey{}, fmt.Errorf("invalid key format: %s", key)
	}
	return client.ObjectKey{
		Namespace: parts[0],
		Name:      parts[1],
	}, nil
}
