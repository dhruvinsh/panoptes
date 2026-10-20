package collectors

import (
	"math"
	"regexp"
	"strings"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"

	"github.com/dhruvinsh/panoptes/internal/frame"
)

// MaxCores caps the per-core list.
const MaxCores = 32

// CPUUsage: cpu.pct and cpu.cores (since the previous call; gopsutil keeps the baseline).
type CPUUsage struct{}

func (*CPUUsage) Name() string { return "cpu.usage" }

func (*CPUUsage) Prime() error {
	if _, err := cpu.Percent(0, false); err != nil {
		return err
	}
	_, err := cpu.Percent(0, true)
	return err
}

func (*CPUUsage) Collect(out *frame.Obj) error {
	total, err := cpu.Percent(0, false)
	if err != nil {
		return err
	}
	c := out.Sub("cpu")
	if len(total) > 0 {
		c.Set("pct", total[0])
	}
	cores, err := cpu.Percent(0, true)
	if err == nil && len(cores) > 0 {
		if len(cores) > MaxCores {
			cores = cores[:MaxCores]
		}
		c.Set("cores", cores)
	}
	return nil
}

// CPUFreq: cpu.freq (current MHz where the OS has it).
type CPUFreq struct{ noPrime }

func (*CPUFreq) Name() string { return "cpu.freq" }

func (*CPUFreq) Collect(out *frame.Obj) error {
	mhz, err := cpuFreqMHz()
	if err != nil {
		return err
	}
	if mhz > 0 {
		out.Sub("cpu").Set("freq", int(math.Round(mhz)))
	}
	return nil
}

// CPULoad: 1/5/15 min load average.
type CPULoad struct{}

func (*CPULoad) Name() string { return "cpu.load" }

func (*CPULoad) Prime() error { _, err := load.Avg(); return err } // Windows: starts gopsutil's sampler

func (*CPULoad) Collect(out *frame.Obj) error {
	a, err := load.Avg()
	if err != nil {
		return err
	}
	out.Sub("cpu").Set("load", []float64{a.Load1, a.Load5, a.Load15})
	return nil
}

var (
	reTrademark = regexp.MustCompile(`\((R|TM|tm|r)\)`)
	reAtFreq    = regexp.MustCompile(`@.*$`)
	reNCore     = regexp.MustCompile(`(?i)\b\d+-Core Processor\b`)
	reRadeon    = regexp.MustCompile(`(?i)\bwith Radeon.*$`)
	reCPUWord   = regexp.MustCompile(`\b(CPU|Processor)\b`)
	reSpaces    = regexp.MustCompile(`\s+`)
)

// ShortenCPUName turns "Intel(R) Core(TM) i7-8700K CPU @ 3.70GHz" into "Intel Core i7-8700K".
func ShortenCPUName(name string) string {
	s := reTrademark.ReplaceAllString(name, "")
	s = reAtFreq.ReplaceAllString(s, "")
	s = reNCore.ReplaceAllString(s, "")
	s = reRadeon.ReplaceAllString(s, "")
	s = reCPUWord.ReplaceAllString(s, "")
	s = strings.TrimSpace(reSpaces.ReplaceAllString(s, " "))
	if s == "" {
		return strings.TrimSpace(name)
	}
	return s
}

// CPUName: cpu.name (static: looked up once, even if that fails).
type CPUName struct {
	noPrime
	done bool
	name string
}

func (*CPUName) Name() string { return "cpu.name" }

func (c *CPUName) Collect(out *frame.Obj) error {
	if !c.done {
		c.done = true
		if n := cpuModel(); n != "" {
			c.name = ShortenCPUName(n)
		}
	}
	if c.name != "" {
		out.Sub("cpu").Set("name", c.name)
	}
	return nil
}

// cpuInfoFallback uses gopsutil's cpu.Info (model, MHz).
func cpuInfoFallback() (string, float64) {
	infos, err := cpu.Info()
	if err != nil || len(infos) == 0 {
		return "", 0
	}
	return infos[0].ModelName, infos[0].Mhz
}

func parseCPUInfoModel(text string) string {
	keys := []string{"model name", "Hardware", "Model", "cpu model", "Processor"}
	found := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if _, seen := found[k]; !seen && v != "" {
			found[k] = v
		}
	}
	for _, k := range keys {
		if v, ok := found[k]; ok {
			return v
		}
	}
	return ""
}
