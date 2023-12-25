package test_utils

import (
	"time"

	"github.com/go-logr/logr"
	"github.com/onsi/gomega"
)

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

func EventuallySuccess(f func(g gomega.Gomega), log logr.Logger) {
	gomega.Eventually(func(g gomega.Gomega) {
		log.Info("check")
		f(g)
	}, "10m", "2s").Should(gomega.Succeed(), func(message string, callerSkip ...int) {
	})
}
func GNoErr(g gomega.Gomega, err error) {
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
}

func GEqual(g gomega.Gomega, left interface{}, right interface{}) {
	g.Expect(left).Should(gomega.Equal(right))
}
