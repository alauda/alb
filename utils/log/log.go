package log

import (
	"flag"
	"os"
	"sync"

	"github.com/go-logr/logr"
	klogv2 "k8s.io/klog/v2"
	klogr "k8s.io/klog/v2/klogr"
)

var (
	_globalMu sync.RWMutex
	_globalL  *Log = nil
)

type Log struct {
	logr logr.Logger
}

func L() logr.Logger {
	_globalMu.Lock()
	defer _globalMu.Unlock()
	if _globalL != nil {
		return _globalL.logr
	}
	initKlogV2()
	logger := klogr.NewWithOptions(klogr.WithFormat("Klog"))
	_globalL = &Log{logr: logger}
	return _globalL.logr
}

func Flush() {
	klogv2.Flush()
}

func initKlogV2() {
	flags := flag.CommandLine
	klogv2.InitFlags(flags)
	err := flags.Set("add_dir_header", "true")
	if err != nil {
		panic(err)
	}
	if os.Getenv("ALB_LOG_EXT") == "true" {
		logFile := os.Getenv("ALB_LOG_FILE")
		flag.Set("log_file", logFile)
		logLevel := os.Getenv("ALB_LOG_LEVEL")
		if logLevel != "" {
			flag.Set("v", logLevel)
		}
		if os.Getenv("ALB_DISABLE_LOG_STDERR") == "true" {
			flag.Set("logtostderr", "false")
			flag.Set("alsologtostderr", "false")
		}
	}
}
