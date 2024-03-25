package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCert(t *testing.T) {
	assert.Equal(t,
		formatCertsMap(map[string]map[string][]client.ObjectKey{
			"80": {
				"a.com": {
					{Namespace: "a", Name: "a2"},
					{Namespace: "a", Name: "a1"},
					{Namespace: "a", Name: "a3"},
				},
			},
			"90": {
				"b.com": {
					{Namespace: "a", Name: "b1"},
				},
				"c.com": {
					{Namespace: "a", Name: "c1"},
				},
			},
			"91": {
				"b.com": {
					{Namespace: "a", Name: "b2"},
				},
			},
		}),
		map[string]client.ObjectKey{
			"a.com":    {Namespace: "a", Name: "a1"},
			"b.com/90": {Namespace: "a", Name: "b1"},
			"b.com/91": {Namespace: "a", Name: "b2"},
			"c.com":    {Namespace: "a", Name: "c1"},
		},
	)
}
