package checklist

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestALBCheckList(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "alb checklist related test")
}
