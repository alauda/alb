package e2e

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	_ "alauda.io/alb2/test/kind/e2e/operator"
)

func TestAlbKindE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Kind e2e Suite")
}
