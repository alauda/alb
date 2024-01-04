package test_utils

import (
	"fmt"

	"github.com/go-logr/logr"
)

type ConsoleLogSink struct {
	prefix string
}

func ConsoleLog() logr.Logger {
	return logr.New(ConsoleLogSink{})
}

func (l ConsoleLogSink) Init(logr.RuntimeInfo) {
}

func (l ConsoleLogSink) Enabled(int) bool {
	return true
}

func (l ConsoleLogSink) log(msg string) {
	fmt.Println(msg)
}

func (l ConsoleLogSink) Info(level int, msg string, kv ...interface{}) {
	l.log(fmt.Sprintf("%s level %d %s %s %v ", nowStamp(), level, l.prefix, msg, kv))
}

func (l ConsoleLogSink) Error(err error, msg string, kv ...interface{}) {
	l.log(fmt.Sprintf("%s err %v %s %s %v ", nowStamp(), err, l.prefix, msg, kv))
}

func (l ConsoleLogSink) WithValues(kv ...interface{}) logr.LogSink {
	return ConsoleLogSink{prefix: fmt.Sprintf("%s %v", l.prefix, kv)}
}

func (l ConsoleLogSink) WithName(msg string) logr.LogSink {
	return ConsoleLogSink{prefix: fmt.Sprintf("%s %v", l.prefix, msg)}
}
