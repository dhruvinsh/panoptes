package collectors

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func cpuModel() string {
	b, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		n, _ := cpuInfoFallback()
		return n
	}
	return parseCPUInfoModel(string(b))
}

// cpuFreqMHz averages scaling_cur_freq over all CPUs (like psutil), falling back to
// the "cpu MHz" lines in /proc/cpuinfo.
func cpuFreqMHz() (float64, error) {
	paths, _ := filepath.Glob("/sys/devices/system/cpu/cpufreq/policy*/scaling_cur_freq")
	if len(paths) == 0 {
		paths, _ = filepath.Glob("/sys/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_cur_freq")
	}
	var sum float64
	n := 0
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64); err == nil {
			sum += v / 1000 // kHz -> MHz
			n++
		}
	}
	if n > 0 {
		return sum / float64(n), nil
	}
	b, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(k) == "cpu MHz" {
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				sum += f
				n++
			}
		}
	}
	if n == 0 {
		return 0, errors.New("no CPU frequency available")
	}
	return sum / float64(n), nil
}
