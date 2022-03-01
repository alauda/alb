package log

import (
	"testing"

	klogv2 "k8s.io/klog/v2"
)

func TestLog(t *testing.T) {
	Init()
	klogv2.Info("klog v2 info")
	l := L()
	l.V(-100).Info("test v-100")
	l.V(-3).Info("test v-3")
	l.V(-2).Info("test v-2")
	l.V(-1).Info("test v-1")
	l.V(0).Info("test v0")
	l.V(1).Info("test v1")
	l.V(2).Info("test v2")
	l.V(3).Info("test v3")
	l.V(4).Info("test v4")
	l.V(5).Info("test v5")

	l.Info("test taged", "a", "b")
	l.WithName("name1").Info("test with name", "a", "b")
	l.WithName("name1").WithName("name2").Info("test with name", "a", "b")
}
