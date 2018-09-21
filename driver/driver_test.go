package driver

import (
	"flag"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	flag.Set("logtostderr", "true")
	flag.Parse()
	code := m.Run()
	os.Exit(code)
}
