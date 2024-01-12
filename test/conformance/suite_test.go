package conformance

import (
	"fmt"
	"os"
	"testing"

	sets "github.com/deckarep/golang-set/v2"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/gateway-api/conformance/tests"

	. "alauda.io/alb2/test/kind/pkg/helper"
	. "alauda.io/alb2/utils/test_utils"
	"sigs.k8s.io/gateway-api/conformance/utils/suite"
)

var RT *testing.T

func fakeIpv4Ips(size int) []string {
	out := []string{}
	for i := 0; i < 200; i++ {
		out = append(out, fmt.Sprintf("111.111.111.%d", i))
	}
	return out
}
func fakeIpv6Ips(size int) []string {
	out := []string{}
	for i := 0; i < 200; i++ {
		out = append(out, fmt.Sprintf("fe80::a8a1:97ff:fe91:%d", i))
	}
	return out
}

func TestAlbGatewayConformance(t *testing.T) {
	RT = t
	ctx, _ := CtxWithSignalAndTimeout(30 * 60)
	// local dev chart
	// gateway hostnetwork
	chart := os.Getenv("ALB_GATEWAY_CONFORMANCE_TEST_CHART")

	actx := NewAlbK8sCtx(ctx, NewAlbK8sCfg().
		UseMockLBSvcCtl(fakeIpv4Ips(100), fakeIpv6Ips(100)).
		// UseMetalLBSvcCtl([]string{"192.168.0.1"}, []string{"2004::192:168:128:235"}).
		WithChart(chart).
		DisableDefaultAlb().
		Build(),
	)
	err := actx.Init()
	assert.NoError(t, err)
	defer actx.Destroy()
	cli := NewK8sClient(ctx, actx.Kubecfg)
	l := actx.Log
	t.Log("tlog ok")
	l.Info("l log ok")
	actx.Kind.LoadImage("gcr.io/k8s-staging-gateway-api/echo-basic:v20231024-v1.0.0-rc1-33-g9c830e50")
	cSuite := suite.New(suite.Options{
		Client:           cli.GetClient(),
		GatewayClassName: "exclusive-gateway",
		Debug:            true,
	})
	cSuite.Setup(t)
	cases := []string{
		"GatewayInvalidRouteKind",
		// // "GatewayInvalidTLSConfiguration",
		// // "GatewayObservedGenerationBump",
		// // "GatewaySecretInvalidReferenceGrant",
		// // "GatewaySecretMissingReferenceGrant",
		// // "GatewaySecretReferenceGrantAllInNamespace",
		// // "GatewaySecretReferenceGrantSpecific",
		// "GatewayWithAttachedRoutes",
		//	// "GatewayClassObservedGenerationBump",
		// "HTTPRouteCrossNamespace",
		// "HTTPRouteDisallowedKind",
		// "HTTPExactPathMatching",
		// "HTTPRouteHeaderMatching",
		// "HTTPRouteHostnameIntersection",
		// // "HTTPRouteInvalidNonExistentBackendRef",
		// // "HTTPRouteInvalidBackendRefUnknownKind",
		// // "HTTPRouteInvalidCrossNamespaceBackendRef",
		// // "HTTPRouteInvalidCrossNamespaceParentRef",
		// // "HTTPRouteInvalidParentRefNotMatchingListenerPort",
		// // "HTTPRouteInvalidParentRefNotMatchingSectionName",
		// "HTTPRouteListenerHostnameMatching",
		// "HTTPRouteMatchingAcrossRoutes",
		// "HTTPRouteMatching",
		// "HTTPRouteMethodMatching",
		// // "HTTPRouteObservedGenerationBump",
		// // "HTTPRoutePartiallyInvalidViaInvalidReferenceGrant",
		// "HTTPRouteQueryParamMatching",
		// "HTTPRouteRedirectHostAndStatus",
		// "HTTPRouteRedirectPath",
		// "HTTPRouteRedirectPort",
		// "HTTPRouteRedirectScheme",
		// // "HTTPRouteReferenceGrant",
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
