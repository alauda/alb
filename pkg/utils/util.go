package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
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

func Hash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
