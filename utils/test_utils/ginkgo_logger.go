package test_utils

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/onsi/ginkgo/v2"
)

type GinkgoLogSink struct {
	prefix string
}

// Deprecated
// 字符串中含有%时， %v的处理是错的..
func GinkgoLog() logr.Logger {
	return logr.New(GinkgoLogSink{})
}

func (l GinkgoLogSink) Init(logr.RuntimeInfo) {
}

func (l GinkgoLogSink) Enabled(int) bool {
	return true
}

func (l GinkgoLogSink) log(msg string) {
	fmt.Fprintf(ginkgo.GinkgoWriter, msg+"\n")
}

func (l GinkgoLogSink) Info(level int, msg string, kv ...interface{}) {
	l.log(fmt.Sprintf("%s level %d %s %s %v", nowStamp(), level, l.prefix, msg, kv))
}

func (l GinkgoLogSink) Error(err error, msg string, kv ...interface{}) {
	l.log(fmt.Sprintf("%s err %v %s %s %v ", nowStamp(), err, l.prefix, msg, kv))
}

func (l GinkgoLogSink) WithValues(kv ...interface{}) logr.LogSink {
	return GinkgoLogSink{prefix: fmt.Sprintf("%s %v", l.prefix, kv)}
}

func (l GinkgoLogSink) WithName(msg string) logr.LogSink {
	return GinkgoLogSink{prefix: fmt.Sprintf("%s %v", l.prefix, msg)}
}
