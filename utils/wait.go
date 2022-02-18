package utils

import (
	"context"
	"k8s.io/klog/v2"
	"time"
)

// UtilWithContextAndTimeout block and run fn after every delay, stop util ctx is done or fn runtime reach the timeout
//
// wait a period first, and then run fn
//
// if ctx is done, this function may not return immediately, but keep running util fn complete or fn timeout
func UtilWithContextAndTimeout(ctx context.Context, fn func(), timeout time.Duration, delay time.Duration) (isTimeout bool) {
	for {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(delay):
			isTimeout := UtilTimeout(fn, timeout)
			if isTimeout {
				return true
			}
		}
	}
}

// UtilTimeout block fn until fn is complete or timeout
// notice that this function will leak a goroutine which run fn in background, until timeout
func UtilTimeout(fn func(), timeout time.Duration) (isTimeout bool) {
	signalCh := make(chan struct{})
	// there is no way to stop this go routine, the only thing we could do is os.Exit when timeout.
	go func(signalCh chan struct{}) {
		defer close(signalCh)
		fn()
	}(signalCh)
	select {
	case <-signalCh:
		return false
	case <-time.After(timeout):
		klog.Errorf("run fn but reach timeout %v", timeout)
		return true
	}
}
