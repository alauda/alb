package ingress

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

func TestWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// we should wait a interval
	t.Logf("start %v", time.Now())
	go wait.UntilWithContext(ctx, func(ctx context.Context) {
		t.Logf("in %v", time.Now())
	}, time.Millisecond*100)
	time.Sleep(time.Second * 1)
	cancel()
	t.Logf("end %v", time.Now())
	time.Sleep(time.Millisecond * 500)
}
