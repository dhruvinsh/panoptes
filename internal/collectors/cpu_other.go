//go:build !linux && !darwin && !windows

package collectors

import "errors"

func cpuModel() string {
	n, _ := cpuInfoFallback()
	return n
}

func cpuFreqMHz() (float64, error) {
	_, mhz := cpuInfoFallback()
	if mhz == 0 {
		return 0, errors.New("no CPU frequency available")
	}
	return mhz, nil
}
