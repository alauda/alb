package types

import (
	"encoding/json"
	"testing"

	otelt "alauda.io/alb2/pkg/controller/ext/otel/types"
	"github.com/stretchr/testify/assert"
)

func TestToMaps(t *testing.T) {
	p := PolicyExt{
		Otel: &otelt.OtelConf{
			Exporter: &otelt.Exporter{},
		},
		RewriteResponse: &RewriteResponseConfig{},
	}
	m := p.ToMaps()
	js, err := json.MarshalIndent(m, "  ", "  ")
	assert.NoError(t, err)
	t.Logf("m %s", string(js))
	assert.Equal(t, 2, len(m))
}

func TestClean(t *testing.T) {
	p := PolicyExt{
		Otel: &otelt.OtelConf{
			Exporter: &otelt.Exporter{},
		},
		RewriteResponse: &RewriteResponseConfig{},
	}
	p.Clean("otel")
}
