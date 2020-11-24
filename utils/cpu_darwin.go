// +build darwin
package utils

import (
	"runtime"
)

// NumCPU returns the number of logical CPUs usable by the current process.
// If CPU cgroups limits are configured, use cfs_quota_us / cfs_period_us
// as formula
//  https://www.kernel.org/doc/Documentation/scheduler/sched-bwc.txt
func NumCPU(limit int) int {
	return runtime.NumCPU()
}
