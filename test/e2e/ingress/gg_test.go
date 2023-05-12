package ingress

import "testing"

func TestGinkgo(t *testing.T) {
	t.Logf("test")
	a := 1
	fi1 := func() {
		a = 2
		t.Logf("test %v", a)
	}
	fi2 := func() {
		a = 3
		t.Logf("test %v", a)
	}
	fd1 := func() {
		t.Logf("fd1 %v", a)
	}
	fd2 := func() {
		t.Logf("fd2 %v", a)
	}

	_ = fi2
	fi1()
	fd1()
	fi2()
	fd2()
}
