//go:build linux
// +build linux

package utils

import (
	"runtime"
)

// NumCPU returns the number of logical CPUs used by the nginx process(worker_process in nginx.conf)
// If CPU limits are pre-set, use it as the num of worker_process
// Else use min of runtime.cpu and WORKER_LIMIT
//  https://www.kernel.org/doc/Documentation/scheduler/sched-bwc.txt
func NumCPU(cpu_preset, limit int) int {
	if cpu_preset > 0 {
		return cpu_preset
	} else {
		return min(runtime.NumCPU(), limit)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
