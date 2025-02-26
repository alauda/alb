package types

import (
	"encoding/json"
	"strings"
	"testing"

	otelt "alauda.io/alb2/pkg/controller/ext/otel/types"
	lu "alauda.io/alb2/utils"
	"github.com/stretchr/testify/assert"
)

func TestJson(t *testing.T) {
	p := Policy{}
	p_json := lu.PrettyJson(p)
	t.Log()
	assert.Equal(t, !strings.Contains(p_json, "null"), true)
}

func TestToMaps(t *testing.T) {
	p := PolicyExtCfg{
		PolicyExt: PolicyExt{
			Otel: &otelt.OtelConf{
				Exporter: &otelt.Exporter{},
			},
			RewriteResponse: &RewriteResponseConfig{},
		},
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
