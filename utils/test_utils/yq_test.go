package test_utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestYq(t *testing.T) {
	raw := `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: auth-check-cookies
  namespace: default
spec:
  rules:
  - host: "auth-check-cookies"
    http:
      paths:
      - backend:
          service:
            name: auth-server
            port:
              number: 80
        path: /
        pathType: Prefix`
	expect := strings.TrimSpace(`
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: auth-check-cookies
  namespace: default
spec:
  rules:
    - host: "auth-check-cookies"
      http:
        paths:
          - backend:
              service:
                name: auth-server
                port:
                  number: 80
            path: /
            pathType: Prefix
  ingressClassName: nginx`)
	out, err := YqDo(raw, `yq ".spec.ingressClassName=\"nginx\""`)
	t.Logf("%v | %v", out, err)
	assert.NoError(t, err)
	_ = expect
	_ = out
	assert.Equal(t, expect, out)
}
