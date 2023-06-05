package helper

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsNeedGcKind(t *testing.T) {
	type TestCase struct {
		prefix string
		time   int64
		need   bool
	}
	now := time.Now().Unix()
	new2h := time.Now().Add(time.Hour * 2).Unix()
	old2h := time.Now().Add(time.Hour * -2).Unix()
	cases := []TestCase{
		{
			prefix: "xx-ddd",
			time:   now,
			need:   false,
		},
		{
			prefix: "xx-ddd",
			time:   new2h,
			need:   false,
		},
		{
			prefix: "xx-ddd",
			time:   old2h,
			need:   true,
		},
	}
	for _, c := range cases {
		need, _ := isNeedGC(fmt.Sprintf("%s-%d", c.prefix, c.time))
		assert.Equal(t, need, c.need)
	}
}
