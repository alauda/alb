package http

import (
	"testing"

	"github.com/openlyinc/pointy"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"alauda.io/alb2/config"
	albType "alauda.io/alb2/controller/types"
	"alauda.io/alb2/gateway"
	"alauda.io/alb2/gateway/nginx/types"
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func MockCtx() HttpCtx {
	return HttpCtx{
		listener: &types.Listener{
			Gateway: client.ObjectKey{Name: "", Namespace: ""},
		},
		httpRoute:  &gateway.HTTPRoute{},
		ruleIndex:  0,
		rule:       &gv1.HTTPRouteRule{},
		matchIndex: 0,
	}
}

func TestHttpFilterHeaderModify(t *testing.T) {
	rule := albType.InternalRule{}
	h := NewHttpProtocolTranslate(nil, log.Log.WithName("test"), config.DefaultMock())
	err := h.applyHttpFilterOnRule(MockCtx(), &rule, []gv1.HTTPRouteFilter{
		{
			Type: gv1.HTTPRouteFilterRequestHeaderModifier,
			RequestHeaderModifier: &gv1.HTTPHeaderFilter{
				Add: []gv1.HTTPHeader{
					{
						Name:  "a",
						Value: "a1",
					},
					{
						Name:  "a",
						Value: "a2",
					},
				},
				Set: []gv1.HTTPHeader{
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
		},
	})
	assert.NoError(t, err)
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
	host := gv1.PreciseHostname("a.com")
	port := gv1.PortNumber(90)

	{
		rule := albType.InternalRule{}
		h.applyHttpFilterOnRule(MockCtx(), &rule, []gv1.HTTPRouteFilter{
			{
				Type: gv1.HTTPRouteFilterRequestRedirect,
				RequestRedirect: &gv1.HTTPRequestRedirectFilter{
					Scheme:     pointy.String("https"),
					Hostname:   &host,
					Port:       &port,
					StatusCode: pointy.Int(302),
				},
			},
		})
		t.Logf("%+v", rule)
		rd := rule.Config.Redirect
		assert.Equal(t, *rd.RedirectScheme, "https")
		assert.Equal(t, *rd.RedirectHost, "a.com")
		assert.Equal(t, *rd.RedirectPort, 90)
		assert.Equal(t, rd.RedirectCode, 302)
	}
	{
		rule := albType.InternalRule{}
		h.applyHttpFilterOnRule(MockCtx(), &rule, []gv1.HTTPRouteFilter{
			{
				Type: gv1.HTTPRouteFilterRequestRedirect,
				RequestRedirect: &gv1.HTTPRequestRedirectFilter{
					Scheme:   pointy.String("https"),
					Hostname: &host,
					Port:     &port,
					Path: &gv1.HTTPPathModifier{
						ReplaceFullPath: pointy.String("/abc"),
					},
					StatusCode: pointy.Int(302),
				},
			},
		})
		t.Logf("%+v", rule)
		rd := rule.Config.Redirect
		assert.Equal(t, *rd.RedirectScheme, "https")
		assert.Equal(t, *rd.RedirectHost, "a.com")
		assert.Equal(t, rd.RedirectURL, "/abc")
		assert.Equal(t, *rd.RedirectPort, 90)
		assert.Equal(t, rd.RedirectCode, 302)
	}
	{
		ctx := MockCtx()
		prefix := gv1.PathMatchPathPrefix
		ctx.httpRoute.Spec.Rules = []gv1.HTTPRouteRule{
			{
				Matches: []gv1.HTTPRouteMatch{
					{
						Path: &gv1.HTTPPathMatch{
							Type:  &prefix,
							Value: pointy.String("/abc"),
						},
					},
				},
			},
		}
		rule := albType.InternalRule{}
		h.applyHttpFilterOnRule(ctx, &rule, []gv1.HTTPRouteFilter{
			{
				Type: gv1.HTTPRouteFilterRequestRedirect,
				RequestRedirect: &gv1.HTTPRequestRedirectFilter{
					Scheme:   pointy.String("https"),
					Hostname: &host,
					Port:     &port,
					Path: &gv1.HTTPPathModifier{
						ReplacePrefixMatch: pointy.String("/xxx"),
					},
					StatusCode: pointy.Int(302),
				},
			},
		})
		t.Logf("%+v", rule)
		rd := rule.Config.Redirect
		assert.Equal(t, *rd.RedirectScheme, "https")
		assert.Equal(t, *rd.RedirectHost, "a.com")
		assert.Equal(t, *rd.RedirectPrefixMatch, "/abc")
		assert.Equal(t, *rd.RedirectReplacePrefix, "/xxx")
		assert.Equal(t, *rd.RedirectPort, 90)
		assert.Equal(t, rd.RedirectCode, 302)
	}
}

func TestHttpFilterRewrite(t *testing.T) {
	h := NewHttpProtocolTranslate(nil, log.Log.WithName("test"), config.DefaultMock())
	ctx := MockCtx()
	prefix := gv1.PathMatchPathPrefix
	ctx.httpRoute.Spec.Rules = []gv1.HTTPRouteRule{
		{
			Matches: []gv1.HTTPRouteMatch{
				{
					Path: &gv1.HTTPPathMatch{
						Type:  &prefix,
						Value: pointy.String("/abc"),
					},
				},
			},
		},
	}
	rule := albType.InternalRule{}
	h.applyHttpFilterOnRule(ctx, &rule, []gv1.HTTPRouteFilter{
		{
			Type: gv1.HTTPRouteFilterURLRewrite,
			URLRewrite: &gv1.HTTPURLRewriteFilter{
				Path: &gv1.HTTPPathModifier{
					ReplacePrefixMatch: pointy.String("/xxx"),
				},
			},
		},
	})
	t.Logf("%+v", rule)
	rw := rule.Config.Rewrite
	assert.Equal(t, *rw.RewritePrefixMatch, "/abc")
	assert.Equal(t, *rw.RewriteReplacePrefix, "/xxx")
}
