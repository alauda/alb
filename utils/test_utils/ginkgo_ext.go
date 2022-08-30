package test_utils

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo"
)

func log(level string, format string, args ...interface{}) {
	fmt.Fprintf(ginkgo.GinkgoWriter, nowStamp()+": "+level+": "+"ginkgo : "+format+"\n", args...)
}

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

// Logf logs to the INFO logs.
func Logf(format string, args ...interface{}) {
	log("INFO", format, args...)
}
