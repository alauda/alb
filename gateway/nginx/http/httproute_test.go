package http

import (
	"encoding/json"
	"testing"

	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils"
	"github.com/stretchr/testify/assert"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func TestHttpMatchesToDSLX(t *testing.T) {
	matchPathPrefix := gv1b1t.PathMatchPathPrefix
	matchHeaderExact := gv1b1t.HeaderMatchExact
	matchHeaderRegex := gv1b1t.HeaderMatchRegularExpression
	matchQueryExact := gv1b1t.QueryParamMatchExact
	matchQueryRegex := gv1b1t.QueryParamMatchRegularExpression
	putMethod := gv1b1t.HTTPMethod("PUT")
	type TestCase struct {
		hostnames []string
		rule      gv1b1t.HTTPRouteRule
		expect    string
	}
	cases := []TestCase{
		{
			expect:    `["AND",["ENDS_WITH","HOST","*.com"],["STARTS_WITH","URL","/v1"]]`,
			hostnames: []string{"*.com"},
			rule: gv1b1t.HTTPRouteRule{
				Matches: []gv1b1t.HTTPRouteMatch{
					{
						Path: &gv1b1t.HTTPPathMatch{
							Type:  &matchPathPrefix,
							Value: utils.StringRefs("/v1"),
						},
					},
				},
			},
		},
		{
			expect:    `["AND",["IN","HOST","a.com","a.b.com"],["EQ","PARAM","page","1"],["REGEX","PARAM","location","c*"],["STARTS_WITH","URL","/v1"],["EQ","HEADER","version","1.1"],["REGEX","HEADER","name","w*"],["EQ","METHOD","PUT"]]`,
			hostnames: []string{"a.com", "a.b.com"},
			rule: gv1b1t.HTTPRouteRule{
				Matches: []gv1b1t.HTTPRouteMatch{
					{
						Path: &gv1b1t.HTTPPathMatch{
							Type:  &matchPathPrefix,
							Value: utils.StringRefs("/v1"),
						},
						Headers: []gv1b1t.HTTPHeaderMatch{
							{
								Type:  &matchHeaderExact,
								Value: "1.1",
								Name:  "version",
							},
							{
								Type:  &matchHeaderRegex,
								Value: "w*",
								Name:  "name",
							},
						},
						QueryParams: []gv1b1t.HTTPQueryParamMatch{
							{
								Type:  &matchQueryExact,
								Value: "1",
								Name:  "page",
							},
							{
								Type:  &matchQueryRegex,
								Value: "c*",
								Name:  "location",
							},
						},
						Method: &putMethod,
					},
				},
			},
		},
	}
	for _, c := range cases {
		dslx, err := HttpRuleMatchToDSLX(c.hostnames, c.rule.Matches[0])
		assert.NoError(t, err)
		internalDslStr, err := toInternalDslJsonStr(dslx)
		assert.NoError(t, err)
		assert.Equal(t, internalDslStr, c.expect)
	}
}

func TestJoinHostname(t *testing.T) {
	type TestCase struct {
		listenHostName string
		routeHostName  []string
		expected       []string
	}

	cases := []TestCase{
		{
			listenHostName: "*.com",
			routeHostName:  []string{"a.com"},
			expected:       []string{"a.com"},
		},
		{
			listenHostName: "*.com",
			routeHostName:  []string{},
			expected:       []string{"*.com"},
		},
		{
			listenHostName: "*.com",
			routeHostName:  []string{"*.a.com"},
			expected:       []string{"*.a.com"},
		},
		{
			listenHostName: "",
			routeHostName:  []string{"*.a.com"},
			expected:       []string{"*.a.com"},
		},
		{
			listenHostName: "a.com",
			routeHostName:  []string{"a.com"},
			expected:       []string{"a.com"},
		},
	}
	for _, c := range cases {
		assert.Equal(t, c.expected, JoinHostnames(&c.listenHostName, c.routeHostName))
	}
}

func toInternalDslJsonStr(dslx v1.DSLX) (string, error) {
	dsl, err := utils.DSLX2Internal(dslx)
	if err != nil {
		return "", err
	}
	ret, err := json.Marshal(dsl)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}
