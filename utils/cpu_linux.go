//go:build linux
// +build linux

package utils

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
)

// NumCPU returns the number of logical CPUs usable by the current process.
// If CPU cgroups limits are configured, use cfs_quota_us / cfs_period_us
// as formula
//  https://www.kernel.org/doc/Documentation/scheduler/sched-bwc.txt
func NumCPU(cpu_preset, limit int) int {
	return min(cpu_preset, limit)
}

func readCgroupFileToInt64(cgroupPath, cgroupFile string) int64 {
	contents, err := ioutil.ReadFile(filepath.Join(cgroupPath, cgroupFile))
	if err != nil {
		return -1
	}

	strValue := strings.TrimSpace(string(contents))
	if value, err := strconv.ParseInt(strValue, 10, 64); err == nil {
		return value
	}

	return -1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
