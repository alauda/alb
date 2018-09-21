package common

import "time"

const (
	TaskCheckInterval = time.Second * 1

	TaskSuccceed = 0
	TaskFailed   = 1
	TaskRunning  = 2

	TaskStatusUnknown = 9
)
