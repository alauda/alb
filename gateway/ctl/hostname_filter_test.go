package ctl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostNameFilter(t *testing.T) {
	type TestCase struct {
		host   string
		route  []string
		result []string
	}
	cases := []TestCase{
		{
			host:   "a.com",
			route:  []string{"a.com"},
			result: []string{"a.com"},
		},
		{
			host:   "*.a.com",
			route:  []string{"a.com"},
			result: []string{},
		},
		{
			host:   "*.a.com",
			route:  []string{"a.a.com", "b.a.com"},
			result: []string{"a.a.com", "b.a.com"},
		},
		{
			host:   "b.a.com",
			route:  []string{"*.a.com", "b.a.com", "a.a.com"},
			result: []string{"*.a.com", "b.a.com"},
		},
	}
	for _, test := range cases {
		result := FindIntersection(test.host, test.route)
		assert.Equal(t, test.result, result)
	}
}
