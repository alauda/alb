package controller

import (
	"os"
	"runtime"
	"sort"
	"testing"

	. "alauda.io/alb2/controller/types"
	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils"
	"github.com/stretchr/testify/assert"
)

func TestDslString(t *testing.T) {
	dslx := v1.DSLX{
		{
			Values: [][]string{{utils.OP_STARTS_WITH, "/k8s"}, {utils.OP_REGEX, "^/v1/*"}},
			Type:   utils.KEY_URL,
		},
	}
	assert.Equal(t, dslx.ToSearchableString(), "[{[[STARTS_WITH /k8s] [REGEX ^/v1/*]] URL }]")
}

func TestRuleOrder(t *testing.T) {
	type fields struct {
		Name string
		DSLX v1.DSLX
	}
	rules := []fields{
		{
			Name: "start with /abc or regex ^/v1/*",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/k8s"}, {utils.OP_REGEX, "^/v1/*"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "start with /abcde",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/abcde"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "start with /abc",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/abc"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "wildcard host and regex /abcd.*",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_ENDS_WITH, ".c.com"}},
					Type:   utils.KEY_HOST,
				},
				{
					Values: [][]string{{utils.OP_REGEX, "/abcd.*"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "wildcard host and regex /a.*",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_ENDS_WITH, ".c.com"}},
					Type:   utils.KEY_HOST,
				},
				{
					Values: [][]string{{utils.OP_REGEX, "/a.*"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "wildcard host and start with /abc",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_ENDS_WITH, ".c.com"}},
					Type:   utils.KEY_HOST,
				},
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/abc"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "wildcard host and start with /ab",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_ENDS_WITH, ".c.com"}},
					Type:   utils.KEY_HOST,
				},
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/ab"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "wildcard host and start with /",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_ENDS_WITH, ".c.com"}},
					Type:   utils.KEY_HOST,
				},
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "wildcard host",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_ENDS_WITH, ".c.com"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			Name: "start with /",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "start with /abc with host",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/abc"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "a.c"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			Name: "start with / with host",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "a.c"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			Name: "regex /a.* with host",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_REGEX, "/a.*"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "a.c"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			Name: "exact /a  with host",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_EQ, "/a"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "a.com"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			Name: "regex /abcd.*",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_REGEX, "/abcd.*"}},
					Type:   utils.KEY_URL,
				},
			},
		},
		{
			Name: "regex /a.*",
			DSLX: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_REGEX, "/a.*"}},
					Type:   utils.KEY_URL,
				},
			},
		},
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].DSLX.Priority() > rules[j].DSLX.Priority()
	})
	expectOrder := []string{
		"exact /a  with host",
		"start with /abc with host",
		"regex /a.* with host",
		"start with / with host",
		"wildcard host and regex /abcd.*",
		"wildcard host and start with /abc",
		"wildcard host and start with /ab",
		"wildcard host and regex /a.*",
		"wildcard host and start with /",
		"wildcard host",
		"start with /abc or regex ^/v1/*",
		"start with /abcde",
		"regex /abcd.*",
		"start with /abc",
		"regex /a.*",
		"start with /",
	}
	order := []string{}
	for _, r := range rules {
		order = append(order, r.Name)
		t.Logf(r.Name)
	}
	assert.Equal(t, expectOrder, order)
}

func TestSortPolicy(t *testing.T) {
	tests := []struct {
		name     string
		policies Policies
		order    []string
	}{
		{
			name: "compare policy RawPriority first",
			policies: []*Policy{
				{
					Rule:        "a",
					RawPriority: 5,
					Priority:    50000 + 1000,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/"}},
				},
				{
					Rule:        "b",
					RawPriority: 1,
					Priority:    10000 + 500,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/b"}},
				},
			},
			order: []string{"b", "a"},
		},
		{
			name: "compare policy RawPriority first",
			policies: []*Policy{
				{
					Rule:        "a",
					RawPriority: 5,
					Priority:    50000 + 1000,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/"}},
				},
				{
					Rule:        "b",
					RawPriority: 1,
					Priority:    10000 + 500,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/b"}},
				},
				{
					Rule:        "c",
					RawPriority: -1,
					Priority:    10000 + 500,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/b"}},
				},
			},
			order: []string{"c", "b", "a"},
		},
		{
			name: "same RawPriority compare priority",
			policies: []*Policy{
				{
					Rule:        "a",
					RawPriority: 5,
					Priority:    10000 + 1000 + 500 + 100,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/"}},
				},
				{
					Rule:        "b",
					RawPriority: 5,
					Priority:    50000 + 1000,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/b"}},
				},
			},
			order: []string{"b", "a"},
		},
		{
			name: "same RawPriority and priority, compare complex",
			policies: []*Policy{
				{
					Rule:        "a",
					RawPriority: 5,
					Priority:    10000 + 1000,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/"}},
				},
				{
					Rule:        "b",
					RawPriority: 5,
					Priority:    10000 + 1000,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/b"}},
				},
			},
			order: []string{"b", "a"},
		},
		// totally same priority and DSL, compare name to stabilize the order when compare policy
		{
			name: "same RawPriority/priority and complex, compare name to stabilize order",
			policies: []*Policy{
				{
					Rule:        "b",
					RawPriority: 5,
					Priority:    10000 + 1000,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/"}},
				},
				{
					Rule:        "a",
					RawPriority: 5,
					Priority:    10000 + 1000,
					InternalDSL: []interface{}{[]string{utils.OP_STARTS_WITH, utils.KEY_URL, "/"}},
				},
			},
			order: []string{"a", "b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := range tt.policies {
				tt.policies[i].InternalDSLLen = utils.InternalDSLLen((tt.policies[i].InternalDSL))
			}
			sort.Sort(tt.policies)
			realOrder := []string{}
			for _, p := range tt.policies {
				realOrder = append(realOrder, p.Rule)
			}
			assert.Equal(t, realOrder, tt.order, tt.name+"fail")
		})
	}
}

func TestIsSameFile(t *testing.T) {
	_, current_go_path, _, _ := runtime.Caller(1)
	exe_path, err := os.Executable()
	assert.NoError(t, err)
	assert.True(t, sameFiles(current_go_path, current_go_path))
	assert.False(t, sameFiles(current_go_path, exe_path))
}
