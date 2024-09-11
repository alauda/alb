package log

import (
	"testing"
)

func TestLog(_ *testing.T) {
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

	l.Info("test tagged", "a", "b")
	l.WithName("name1").Info("test with name", "a", "b")
	l.WithName("name1").WithName("name2").Info("test with name", "a", "b")

	{
		l := InitKlogV2(LogCfg{ToFile: "./test.log"})
		l.Info("test other log")
		Flush()
	}
}
