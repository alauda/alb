package http

import (
	"testing"

	"github.com/openlyinc/pointy"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"

	albType "alauda.io/alb2/controller/types"
	"alauda.io/alb2/gateway"
	"alauda.io/alb2/gateway/nginx/types"
)

func MockCtx() HttpCtx {
	return HttpCtx{
		listener: &types.Listener{
			Gateway: client.ObjectKey{Name: "", Namespace: ""},
		},
		httpRoute:  &gateway.HTTPRoute{},
		ruleIndex:  0,
		rule:       &gatewayType.HTTPRouteRule{},
		matchIndex: 0,
	}
}
func TestHttpFilterHeaderModify(t *testing.T) {
	rule := albType.Rule{}
	h := NewHttpProtocolTranslate(nil, log.Log.WithName("test"))
	h.applyHttpFilterOnRule(MockCtx(), &rule, []gatewayType.HTTPRouteFilter{
		{

			Type: gatewayType.HTTPRouteFilterRequestHeaderModifier,
			RequestHeaderModifier: &gatewayType.HTTPRequestHeaderFilter{
				Add: []gatewayType.HTTPHeader{
					{
						Name:  "a",
						Value: "a1",
					},
					{
						Name:  "a",
						Value: "a2",
					},
				},
				Set: []gatewayType.HTTPHeader{
					{
						Name:  "sa",
						Value: "sa1",
					},
					{
						Name:  "sa",
						Value: "sa2",
					},
					{
						Name:  "sb",
						Value: "sb1",
					},
				},
				Remove: []string{"r1", "r2"},
			},
		}})
	t.Logf("%+v", rule.Config.RewriteResponse)
	assert.Equal(t, rule.Config.RewriteResponse.Headers, map[string]string{
		"sa": "sa2",
		"sb": "sb1",
	})
	assert.Equal(t, rule.Config.RewriteResponse.HeadersAdd, map[string][]string{
		"a": {"a1", "a2"},
	})

	assert.Equal(t, rule.Config.RewriteResponse.HeadersRemove, []string{
		"r1", "r2",
	})
}

func TestHttpFilterRedirect(t *testing.T) {
	rule := albType.Rule{}
	h := NewHttpProtocolTranslate(nil, log.Log.WithName("test"))
	host := gatewayType.PreciseHostname("a.com")
	port := gatewayType.PortNumber(90)

	h.applyHttpFilterOnRule(MockCtx(), &rule, []gatewayType.HTTPRouteFilter{
		{

			Type: gatewayType.HTTPRouteFilterRequestRedirect,
			RequestRedirect: &gatewayType.HTTPRequestRedirectFilter{
				Scheme:     pointy.String("https"),
				Hostname:   &host,
				Port:       &port,
				StatusCode: pointy.Int(302),
			},
		}})
	t.Logf("%+v", rule)
	assert.Equal(t, *rule.RedirectScheme, "https")
	assert.Equal(t, *rule.RedirectHost, "a.com")
	assert.Equal(t, *rule.RedirectPort, 90)
	assert.Equal(t, rule.RedirectCode, 302)
}
