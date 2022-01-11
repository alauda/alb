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
