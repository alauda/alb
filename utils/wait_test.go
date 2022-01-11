package utils

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestUtilWithContextAndTimeout(t *testing.T) {
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
		assert.Equal(t, count >= 4, true)
		assert.Equal(t, count < 8, true)
		assert.Equal(t, isTimeout, false)
	}
	//  it should return with true , if f reach timeout
	{
		msgChan := make(chan string)
		isTimeout := UtilWithContextAndTimeout(context.Background(), func() {
			time.Sleep(100 * time.Second)
			msgChan <- "shold never send"
		}, 10*time.Millisecond, 20*time.Millisecond)
		assert.Equal(t, isTimeout, true)
		select {
		case _, _ = <-msgChan:
			assert.Fail(t, "should never receive msg")
		default:
			assert.True(t, true, "no msg from chan")
		}
	}

}
