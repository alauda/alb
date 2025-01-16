package extctl

import (
	"encoding/json"
	"testing"

	ct "alauda.io/alb2/controller/types"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	otelt "alauda.io/alb2/pkg/controller/ext/otel/types"
	"github.com/stretchr/testify/assert"
)

func TestMergePolicy(t *testing.T) {
	ngx := &ct.NgxPolicy{
		Http: ct.HttpPolicy{
			Tcp: map[albv1.PortNumber]ct.Policies{
				80: {
					{
						Rule:        "xx",
						Upstream:    "xx",
						InternalDSL: []interface{}{},
						Config: ct.PolicyExtCfg{
							Refs: map[ct.PolicyExtKind]string{},
							PolicyExt: ct.PolicyExt{
								Otel: &otelt.OtelConf{
									Exporter: &otelt.Exporter{},
									Sampler: &otelt.Sampler{
										Name: "always_on",
									},
									Flags:    &otelt.Flags{},
									Resource: map[string]string{},
								},
							},
						},
					},
				},
			},
		},
	}
	ctl := &ExtCtl{}
	ctl.MergeSamePolicyConfig(ngx)
	js, err := json.MarshalIndent(ngx, "  ", "  ")
	assert.NoError(t, err)
	t.Logf("ngx %s", js)

	// OMG. https://github.com/golang/go/issues/37711
	type TestB struct {
		B string `json:"b"`
	}
	type TestS struct {
		A  string           `json:"a"`
		A1 string           `json:"a1,omitempty"`
		B  TestB            `json:"b"`
		C  []string         `json:"c"`
		D  []TestB          `json:"d"`
		E  map[string]TestB `json:"e"`
	}
	s := TestS{}
	s_js, _ := json.MarshalIndent(s, "  ", "  ")
	t.Logf("s is %s", s_js)
}
