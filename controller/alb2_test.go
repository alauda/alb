package controller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLock(t *testing.T) {
	a := assert.New(t)
	l1 := newLock("a", 10*time.Millisecond)
	a.NotEmpty(l1)
	a.False(locked(l1, "a")) //treat as unlocked for owner
	a.True(locked(l1, "b"))
	time.Sleep(11 * time.Millisecond)
	a.False(locked(l1, "a"))
	a.False(locked(l1, "b"))
}
