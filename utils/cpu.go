package utils

import (
	"io/ioutil"
	"math"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	libcontainercgroups "github.com/opencontainers/runc/libcontainer/cgroups"
)

// NumCPU returns the number of logical CPUs usable by the current process.
// If CPU cgroups limits are configured, use cfs_quota_us / cfs_period_us
// as formula
//  https://www.kernel.org/doc/Documentation/scheduler/sched-bwc.txt
func NumCPU() int {
	cpus := runtime.NumCPU()

	cgroupPath, err := libcontainercgroups.FindCgroupMountpoint("", "cpu")
	if err != nil {
		return cpus
	}

	cpuQuota := readCgroupFileToInt64(cgroupPath, "cpu.cfs_quota_us")
	cpuPeriod := readCgroupFileToInt64(cgroupPath, "cpu.cfs_period_us")

	if cpuQuota == -1 || cpuPeriod == -1 {
		return cpus
	}

	return int(math.Ceil(float64(cpuQuota) / float64(cpuPeriod)))
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
