package collectors

import (
	"errors"
	"strings"
	"sync"

	"golang.org/x/sys/windows/registry"
)

const cpuKey = `HARDWARE\DESCRIPTION\System\CentralProcessor\0`

func cpuModel() string {
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, cpuKey, registry.QUERY_VALUE); err == nil {
		defer k.Close()
		if s, _, err := k.GetStringValue("ProcessorNameString"); err == nil && strings.TrimSpace(s) != "" {
			return s
		}
	}
	n, _ := cpuInfoFallback()
	return n
}

var (
	winMHzOnce sync.Once
	winMHz     float64
)

// cpuFreqMHz: the registry's ~MHz (base clock, not live - same as psutil on most systems).
// Read once; it doesn't change.
func cpuFreqMHz() (float64, error) {
	winMHzOnce.Do(func() {
		if k, err := registry.OpenKey(registry.LOCAL_MACHINE, cpuKey, registry.QUERY_VALUE); err == nil {
			if v, _, err := k.GetIntegerValue("~MHz"); err == nil {
				winMHz = float64(v)
			}
			k.Close()
		}
		if winMHz == 0 {
			_, winMHz = cpuInfoFallback()
		}
	})
	if winMHz == 0 {
		return 0, errors.New("no CPU frequency available")
	}
	return winMHz, nil
}
