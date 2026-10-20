package collectors

import (
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/dhruvinsh/panoptes/internal/frame"
)

const mib = 1024 * 1024

// Memory: mem.used/total (MiB) and pct; used = total - available (matches psutil's percent).
type Memory struct{ noPrime }

func (*Memory) Name() string { return "mem" }

func (*Memory) Collect(out *frame.Obj) error {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return err
	}
	used := vm.Total - vm.Available
	if vm.Available > vm.Total {
		used = 0
	}
	pct := 0.0
	if vm.Total > 0 {
		pct = float64(used) / float64(vm.Total) * 100
	}
	m := out.Sub("mem")
	m.Set("used", int64(used/mib))
	m.Set("total", int64(vm.Total/mib))
	m.Set("pct", pct)
	return nil
}

// Swap: mem.swap_pct (0.0 when there is no swap).
type Swap struct{ noPrime }

func (*Swap) Name() string { return "swap" }

func (*Swap) Collect(out *frame.Obj) error {
	sw, err := mem.SwapMemory()
	if err != nil {
		return err
	}
	pct := 0.0
	if sw.Total > 0 {
		pct = float64(sw.Used) / float64(sw.Total) * 100
	}
	out.Sub("mem").Set("swap_pct", pct)
	return nil
}
