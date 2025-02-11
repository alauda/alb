package ingressnginx

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// 在兼容ingress-nginx的情况下，用户常问的问题是，在xxx配置下，ingress-nginx 会是什么行为
// conformance test 应该能够给出一个快速的有力的回应。
// 同时，这下面的case 也是我们的e2e测试用例
func TestIngressNginx(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ingress nginx related e2e")
}
