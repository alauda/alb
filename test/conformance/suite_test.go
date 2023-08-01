package conformance

import (
	"testing"

	sets "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/gateway-api/conformance/tests"

	. "alauda.io/alb2/test/kind/pkg/helper"
	. "alauda.io/alb2/utils/test_utils"
	"sigs.k8s.io/gateway-api/conformance/utils/suite"
)

var RT *testing.T

func TestAlbGatewayConformance(t *testing.T) {
	RT = t

	ctx, _ := CtxWithSignalAndTimeout(30 * 60)
	actx := NewAlbK8sCtx(ctx, NewAlbK8sCfg().
		// UseMockLBSvcCtl([]string{"192.168.0.1"}, []string{"2004::192:168:128:235"}).
		UseMetalLBSvcCtl([]string{"192.168.0.1"}, []string{"2004::192:168:128:235"}).
		DisableDefaultAlb().
		Build(),
	)
	defer actx.Destory()
	err := actx.Init()
	assert.NoError(t, err)

	l := actx.Log
	t.Log("tlog ok")
	l.Info("l log ok")
	actx.Kind.LoadImage("gcr.io/k8s-staging-ingressconformance/echoserver:v20221109-7ee2f3e")
	cSuite := suite.New(suite.Options{
		Client:           actx.Kubecliet.GetClient(),
		GatewayClassName: "exclusive-gateway",
		Debug:            true,
	})
	cSuite.Setup(t)
	cases := []string{
		"GatewayInvalidRouteKind",
		// "GatewayInvalidTLSConfiguration",
		// "GatewayObservedGenerationBump",
		// "GatewaySecretInvalidReferenceGrant",
		// "GatewaySecretMissingReferenceGrant",
		// "GatewaySecretReferenceGrantAllInNamespace",
		// "GatewaySecretReferenceGrantSpecific",
		// "GatewayWithAttachedRoutes",
		// "GatewayClassObservedGenerationBump",
		// "HTTPRouteCrossNamespace",
		// "HTTPRouteDisallowedKind",
		// "HTTPExactPathMatching",
		// "HTTPRouteHeaderMatching",
		// "HTTPRouteHostnameIntersection",
		// "HTTPRouteInvalidNonExistentBackendRef",
		// "HTTPRouteInvalidBackendRefUnknownKind",
		// "HTTPRouteInvalidCrossNamespaceBackendRef",
		// "HTTPRouteInvalidCrossNamespaceParentRef",
		// "HTTPRouteInvalidParentRefNotMatchingListenerPort",
		// "HTTPRouteInvalidParentRefNotMatchingSectionName",
		// "HTTPRouteListenerHostnameMatching",
		// "HTTPRouteMatchingAcrossRoutes",
		// "HTTPRouteMatching",
		// "HTTPRouteMethodMatching",
		// "HTTPRouteObservedGenerationBump",
		// "HTTPRoutePartiallyInvalidViaInvalidReferenceGrant",
		// "HTTPRouteQueryParamMatching",
		// "HTTPRouteRedirectHostAndStatus",
		// "HTTPRouteRedirectPath",
		// "HTTPRouteRedirectPort",
		// "HTTPRouteRedirectScheme",
		// "HTTPRouteReferenceGrant",
		// "HTTPRouteRequestHeaderModifier",
		// "HTTPRouteResponseHeaderModifier",
		// "HTTPRouteRewriteHost",
		// "HTTPRouteRewritePath",
		// "HTTPRouteSimpleSameNamespace",
		// "TLSRouteSimpleSameNamespace",
	}
	caseset := sets.NewSet(cases...)

	for i := 0; i < len(tests.ConformanceTests); i++ {
		test := tests.ConformanceTests[i]
		if caseset.Contains(test.ShortName) {
			t.Logf("test %d: %s", i, test.ShortName)
			test.Run(t, cSuite)
		}
	}
}
