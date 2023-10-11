package checklist

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestALBCheckList(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "alb checklist related test")
}
