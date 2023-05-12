package test_utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGenCert(t *testing.T) {
	key, cert, err := GenCert("a.b.c")
	assert.NoError(t, err)
	t.Logf("key %s", key)
	t.Logf("cert %s ", cert)
}

func TestTemplate(t *testing.T) {
	ts := `
a: {{.Values.x}}
{{- if eq .Values.b "true" }}
b: 1
{{- end}}
{{- if .Values.c }}
c: 1
{{- end}}
`
	out := Template(ts, map[string]interface{}{
		"Values": map[string]interface{}{
			"x": "1",
			"b": "false",
			"c": true,
		},
	})
	assert.Equal(t, out, `
a: 1
c: 1
`)
	t.Log(out)
}
