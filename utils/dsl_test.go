package utils

import (
	"testing"
)

func TestDSL2DSLX(t *testing.T) {
	_, _ = DSL2DSLX("(REGEX URL ^/clusters/global/prometheus-0($|/)(.*))")
}
