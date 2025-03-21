package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSSLAnnotation(t *testing.T) {
	assert.Equal(t, ParseSSLAnnotation("a.com=1,a.com=1"), map[string]string{
		"a.com": "1",
	})
	assert.Equal(t, ParseSSLAnnotation("a.com=1,a.com=2"), (map[string]string)(nil))
}
