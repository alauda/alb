package utils

import (
	"encoding/json"
	"fmt"
	"testing"

	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"github.com/stretchr/testify/assert"
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

func dslxFromJson(jsonStr string) (v1.DSLX, error) {
	dslx := v1.DSLX{}
	err := json.Unmarshal([]byte(jsonStr), &dslx)
	return dslx, err
}
func TestDSLXToInternalDsl(t *testing.T) {
	testcase := []struct {
		dslx        string
		internalDsl string
	}{
		{
			dslx: `
            [
                {
                    "type": "HOST",
                    "values": [
                        ["IN","*.jrliu-test-xxx.com" ],
                        ["IN", "abc.com" ],
                        ["IN", "*.jrliu-test.org" ]
                    ]
                }
            ]`,
			internalDsl: "[[OR [IN HOST *.jrliu-test-xxx.com] [IN HOST abc.com] [IN HOST *.jrliu-test.org]]]",
		},
		{
			dslx: `
            [
                {
                    "key": "a",
                    "type": "HEADER",
                    "values": [
                        [ "EQ", "b" ]
                    ]
                },
                {
                    "key": "b",
                    "type": "HEADER",
                    "values": [
                        [ "EQ", "c" ]
                    ]
                }
            ]`,
			internalDsl: "[AND [EQ HEADER a b] [EQ HEADER b c]]",
		},
	}
	for _, c := range testcase {
		dslx, err := dslxFromJson(c.dslx)
		assert.NoError(t, err)
		internalDsl, err := DSLX2Internal(dslx)
		assert.NoError(t, err)
		internalDslStr := fmt.Sprintf("%+v", internalDsl)
		assert.Equal(t, internalDslStr, c.internalDsl)
	}
}
