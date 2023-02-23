package test_utils

import (
	"time"
)

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}
