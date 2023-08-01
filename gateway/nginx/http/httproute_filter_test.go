package http

import (
	"testing"

	"github.com/openlyinc/pointy"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayType "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"

	"alauda.io/alb2/config"
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
	h := NewHttpProtocolTranslate(nil, log.Log.WithName("test"), config.DefaultMock())
	h.applyHttpFilterOnRule(MockCtx(), &rule, []gatewayType.HTTPRouteFilter{
		{

			Type: gv1b1t.HTTPRouteFilterRequestHeaderModifier,
			RequestHeaderModifier: &gv1b1t.HTTPHeaderFilter{
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
	t.Logf("%+v", rule.Config.RewriteRequest)
	assert.Equal(t, rule.Config.RewriteRequest.Headers, map[string]string{
		"sa": "sa2",
		"sb": "sb1",
	})
	assert.Equal(t, rule.Config.RewriteRequest.HeadersAdd, map[string][]string{
		"a": {"a1", "a2"},
	})

	assert.Equal(t, rule.Config.RewriteRequest.HeadersRemove, []string{
		"r1", "r2",
	})
}

func TestHttpFilterRedirect(t *testing.T) {
	h := NewHttpProtocolTranslate(nil, log.Log.WithName("test"), config.DefaultMock())
	host := gatewayType.PreciseHostname("a.com")
	port := gatewayType.PortNumber(90)

	{
		rule := albType.Rule{}
		h.applyHttpFilterOnRule(MockCtx(), &rule, []gatewayType.HTTPRouteFilter{
			{

				Type: gv1b1t.HTTPRouteFilterRequestRedirect,
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
	{
		rule := albType.Rule{}
		h.applyHttpFilterOnRule(MockCtx(), &rule, []gatewayType.HTTPRouteFilter{
			{

				Type: gv1b1t.HTTPRouteFilterRequestRedirect,
				RequestRedirect: &gatewayType.HTTPRequestRedirectFilter{
					Scheme:   pointy.String("https"),
					Hostname: &host,
					Port:     &port,
					Path: &gv1b1t.HTTPPathModifier{
						ReplaceFullPath: pointy.String("/abc"),
					},
					StatusCode: pointy.Int(302),
				},
			}})
		t.Logf("%+v", rule)
		assert.Equal(t, *rule.RedirectScheme, "https")
		assert.Equal(t, *rule.RedirectHost, "a.com")
		assert.Equal(t, rule.RedirectURL, "/abc")
		assert.Equal(t, *rule.RedirectPort, 90)
		assert.Equal(t, rule.RedirectCode, 302)
	}
	{
		ctx := MockCtx()
		prefix := gv1b1t.PathMatchPathPrefix
		ctx.httpRoute.Spec.Rules = []gv1b1t.HTTPRouteRule{
			{
				Matches: []gv1b1t.HTTPRouteMatch{
					{
						Path: &gv1b1t.HTTPPathMatch{
							Type:  &prefix,
							Value: pointy.String("/abc"),
						},
					},
				},
			},
		}
		rule := albType.Rule{}
		h.applyHttpFilterOnRule(ctx, &rule, []gatewayType.HTTPRouteFilter{
			{
				Type: gv1b1t.HTTPRouteFilterRequestRedirect,
				RequestRedirect: &gatewayType.HTTPRequestRedirectFilter{
					Scheme:   pointy.String("https"),
					Hostname: &host,
					Port:     &port,
					Path: &gv1b1t.HTTPPathModifier{
						ReplacePrefixMatch: pointy.String("/xxx"),
					},
					StatusCode: pointy.Int(302),
				},
			}})
		t.Logf("%+v", rule)
		assert.Equal(t, *rule.RedirectScheme, "https")
		assert.Equal(t, *rule.RedirectHost, "a.com")
		assert.Equal(t, *rule.RedirectPrefixMatch, "/abc")
		assert.Equal(t, *rule.RedirectReplacePrefix, "/xxx")
		assert.Equal(t, *rule.RedirectPort, 90)
		assert.Equal(t, rule.RedirectCode, 302)
	}
}

func TestHttpFilterRewrite(t *testing.T) {
	h := NewHttpProtocolTranslate(nil, log.Log.WithName("test"), config.DefaultMock())
	ctx := MockCtx()
	prefix := gv1b1t.PathMatchPathPrefix
	ctx.httpRoute.Spec.Rules = []gv1b1t.HTTPRouteRule{
		{
			Matches: []gv1b1t.HTTPRouteMatch{
				{
					Path: &gv1b1t.HTTPPathMatch{
						Type:  &prefix,
						Value: pointy.String("/abc"),
					},
				},
			},
		},
	}
	rule := albType.Rule{}
	h.applyHttpFilterOnRule(ctx, &rule, []gatewayType.HTTPRouteFilter{
		{
			Type: gv1b1t.HTTPRouteFilterURLRewrite,
			URLRewrite: &gatewayType.HTTPURLRewriteFilter{
				Path: &gv1b1t.HTTPPathModifier{
					ReplacePrefixMatch: pointy.String("/xxx"),
				},
			},
		}})
	t.Logf("%+v", rule)
	assert.Equal(t, *rule.RewritePrefixMatch, "/abc")
	assert.Equal(t, *rule.RewriteReplacePrefix, "/xxx")
}
