package http

import (
	"encoding/json"
	"testing"

	v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils"
	"github.com/stretchr/testify/assert"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

func TestHttpMatchesToDSLX(t *testing.T) {
	matchPathPrefix := gatewayType.PathMatchPathPrefix
	matchHeaderExact := gatewayType.HeaderMatchExact
	matchHeaderRegex := gatewayType.HeaderMatchRegularExpression
	matchQueryExact := gatewayType.QueryParamMatchExact
	matchQueryRegex := gatewayType.QueryParamMatchRegularExpression
	putMethod := gatewayType.HTTPMethod("PUT")
	type TestCase struct {
		hostnames []string
		rule      gatewayType.HTTPRouteRule
		expect    string
	}
	cases := []TestCase{
		{
			expect:    `["AND",["ENDS_WITH","HOST","*.com"],["STARTS_WITH","URL","/v1"]]`,
			hostnames: []string{"*.com"},
			rule: gatewayType.HTTPRouteRule{
				Matches: []gatewayType.HTTPRouteMatch{
					{
						Path: &gatewayType.HTTPPathMatch{
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
			rule: gatewayType.HTTPRouteRule{
				Matches: []gatewayType.HTTPRouteMatch{
					{
						Path: &gatewayType.HTTPPathMatch{
							Type:  &matchPathPrefix,
							Value: utils.StringRefs("/v1"),
						},
						Headers: []gatewayType.HTTPHeaderMatch{
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
						QueryParams: []gatewayType.HTTPQueryParamMatch{
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
