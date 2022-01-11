package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDSL2DSLX(t *testing.T) {
	ret, err := DSL2DSLX("(REGEX URL ^/clusters/global/prometheus-0($|/)(.*))")
	t.Logf("we could not parse this dsl string it should be err %+v %s ", ret, err)
	assert.Containsf(t, err.Error(), "invalid exp", "")
}

func TestInternalDsl(t *testing.T) {
	dslx, err := DSL2DSLX("(STARTS_WITH URL /)")
	assert.NoError(t, err)
	internalDsl, err := DSLX2Internal(dslx)
	assert.NoError(t, err)
	t.Logf("dsl %+v", internalDsl)
}
