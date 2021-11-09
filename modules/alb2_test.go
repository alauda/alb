package modules

import (
	"testing"

	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils"
	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
)

func TestGetDSLX(t *testing.T) {

	tests := []struct {
		description string
		domain      string
		url         string
		pathType    networkingv1.PathType
		want        v1.DSLX
	}{
		{
			description: "path is regex && type is impl spec, op should be rgex",
			domain:      "alauda.io",
			url:         "^/v1/*",
			pathType:    networkingv1.PathTypeImplementationSpecific,
			want: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_REGEX, "^/v1/*"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "alauda.io"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			description: "path is regex && type is impl spec , op should be regex and add ^ prefix if it does not have",
			domain:      "alauda.io",
			url:         "/v1/*",
			pathType:    networkingv1.PathTypeImplementationSpecific,
			want: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_REGEX, "^/v1/*"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "alauda.io"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			description: "path is regex && type is exact, op should be eq",
			domain:      "alauda.io",
			url:         "/v1/*",
			pathType:    networkingv1.PathTypeExact,
			want: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_EQ, "/v1/*"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "alauda.io"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			description: "path is regex && type is prefix, op should be starts_with",
			domain:      "alauda.io",
			url:         "/v1/*",
			pathType:    networkingv1.PathTypePrefix,
			want: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/v1/*"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "alauda.io"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			description: "path is not regex and type is impl spec,op should be starts_with",
			domain:      "alauda.io",
			url:         "/v1",
			pathType:    networkingv1.PathTypeImplementationSpecific,
			want: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/v1"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "alauda.io"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			description: "path is not regex and type is prefix,op should be starts_with",
			domain:      "alauda.io",
			url:         "/v1",
			pathType:    networkingv1.PathTypePrefix,
			want: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_STARTS_WITH, "/v1"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "alauda.io"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
		{
			description: "path is not regex and type is exact,op should be eq",
			domain:      "alauda.io",
			url:         "/v1",
			pathType:    networkingv1.PathTypeExact,
			want: []v1.DSLXTerm{
				{
					Values: [][]string{{utils.OP_EQ, "/v1"}},
					Type:   utils.KEY_URL,
				},
				{
					Values: [][]string{{utils.OP_EQ, "alauda.io"}},
					Type:   utils.KEY_HOST,
				},
			},
		},
	}
	for _, test := range tests {
		dslx := GetDSLX(test.domain, test.url, test.pathType)
		assert.Equal(t, dslx, test.want, test.description)
	}
}
