package collectors

import (
	"errors"

	"golang.org/x/sys/unix"
)

func cpuModel() string {
	if s, err := unix.Sysctl("machdep.cpu.brand_string"); err == nil && s != "" {
		return s
	}
	n, _ := cpuInfoFallback()
	return n
}

// cpuFreqMHz: hw.cpufrequency exists on Intel Macs only (Apple Silicon: absent, like psutil).
func cpuFreqMHz() (float64, error) {
	hz, err := unix.SysctlUint64("hw.cpufrequency")
	if err != nil || hz == 0 {
		return 0, errors.New("hw.cpufrequency not available")
	}
	return float64(hz) / 1e6, nil
}
