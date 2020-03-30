package driver

import (
	"flag"
	"k8s.io/klog"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	klog.InitFlags(nil)
	flag.Set("logtostderr", "true")
	flag.Parse()
	code := m.Run()
	os.Exit(code)
}
