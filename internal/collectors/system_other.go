//go:build !linux && !darwin && !windows

package collectors

import (
	"github.com/shirou/gopsutil/v4/host"
)

func prettyOSName() string {
	if i, err := host.Info(); err == nil && i.Platform != "" {
		return i.Platform + " " + i.PlatformVersion
	}
	return ""
}

func kernelVersion() string {
	if v, err := host.KernelVersion(); err == nil {
		return v
	}
	return ""
}
