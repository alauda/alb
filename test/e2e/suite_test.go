package simple

import (
	"testing"

	_ "alauda.io/alb2/test/e2e/alb"
	_ "alauda.io/alb2/test/e2e/cases"
	_ "alauda.io/alb2/test/e2e/gateway"
	_ "alauda.io/alb2/test/e2e/ingress"
	_ "alauda.io/alb2/test/e2e/operator/alb"
	_ "alauda.io/alb2/test/e2e/operator/gateway"
	_ "alauda.io/alb2/test/e2e/operator/public-cloud"
	_ "alauda.io/alb2/test/e2e/operator/rawk8s"
	_ "alauda.io/alb2/test/e2e/operator/simple"
	_ "alauda.io/alb2/test/e2e/perf"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestALB(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "alb operator related e2e")
}
