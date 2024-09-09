package utils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUtilWithContextAndTimeout(t *testing.T) {
	t.Skip("unstable... skip")
	// it should run f each period
	{
		msgChan := make(chan string, 100)
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(120 * time.Millisecond)
			cancel()
			t.Logf("ctx is done")
		}()
		isTimeout := UtilWithContextAndTimeout(ctx, func() {
			msgChan <- "msg"
		}, 10*time.Millisecond, 20*time.Millisecond)
		close(msgChan)
		count := 0
		for msg := range msgChan {
			_ = msg
			count++
		}
		t.Logf("count is %v\n", count)
		assert.Equal(t, isTimeout, false)
	}
	//  it should return with true , if f reach timeout
	{
		msgChan := make(chan string)
		isTimeout := UtilWithContextAndTimeout(context.Background(), func() {
			time.Sleep(100 * time.Second)
			msgChan <- "should never send"
		}, 10*time.Millisecond, 20*time.Millisecond)
		assert.Equal(t, isTimeout, true)
		select {
		case <-msgChan:
			assert.Fail(t, "should never receive msg")
		default:
			assert.True(t, true, "no msg from chan")
		}
	}
}
