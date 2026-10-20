package collectors

import (
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/dhruvinsh/panoptes/internal/frame"
)

// TopN is the number of processes reported.
const TopN = 5

var skipProcNames = map[string]bool{"system idle process": true, "idle": true, "": true}

// ProcRow is one process (or same-named group): c = % of the whole machine, m = % memory.
type ProcRow struct {
	Name string
	CPU  float64
	Mem  float64
}

// Aggregate sums same-named processes and returns the top-N by CPU (then memory).
func Aggregate(rows []ProcRow, top int) []ProcRow {
	idx := map[string]int{}
	var agg []ProcRow
	for _, r := range rows {
		i, ok := idx[r.Name]
		if !ok {
			i = len(agg)
			idx[r.Name] = i
			agg = append(agg, ProcRow{Name: r.Name})
		}
		agg[i].CPU += r.CPU
		agg[i].Mem += r.Mem
	}
	sort.SliceStable(agg, func(i, j int) bool {
		if agg[i].CPU != agg[j].CPU {
			return agg[i].CPU > agg[j].CPU
		}
		return agg[i].Mem > agg[j].Mem
	})
	if len(agg) > top {
		agg = agg[:top]
	}
	return agg
}

func cleanProcName(name string) string {
	if strings.HasSuffix(strings.ToLower(name), ".exe") {
		name = name[:len(name)-4]
	}
	return truncName(name, 15)
}

// procKey identifies a process instance (pid plus start time/name, so pid reuse is noticed).
type procKey struct {
	pid   int32
	start string
}

type procSample struct {
	name   string
	cpuSec float64 // user+system CPU seconds
	rss    uint64
}

// TopProcesses reports the top 5 processes by CPU since the previous frame. Prime takes
// the baseline so the first frame isn't all zeros; processes first seen in a frame
// count as 0 % (like psutil's first cpu_percent() call).
type TopProcesses struct {
	prev  map[procKey]procSample
	prevT time.Time
	ncpu  int
}

func (*TopProcesses) Name() string { return "procs" }

func (t *TopProcesses) cpus() int {
	if t.ncpu == 0 {
		if n, err := cpu.Counts(true); err == nil && n > 0 {
			t.ncpu = n
		} else {
			t.ncpu = runtime.NumCPU()
		}
	}
	return t.ncpu
}

func (t *TopProcesses) Prime() error {
	cur, err := listProcs()
	if err != nil {
		return err
	}
	t.prev, t.prevT = cur, time.Now()
	return nil
}

func (t *TopProcesses) Collect(out *frame.Obj) error {
	cur, err := listProcs()
	if err != nil {
		return err
	}
	now := time.Now()
	var total uint64
	if vm, err := mem.VirtualMemory(); err == nil {
		total = vm.Total
	}
	dt := now.Sub(t.prevT).Seconds()
	ncpu := float64(t.cpus())
	rows := make([]ProcRow, 0, len(cur))
	for k, s := range cur {
		if k.pid == 0 || skipProcNames[strings.ToLower(s.name)] {
			continue
		}
		c := 0.0
		if p, ok := t.prev[k]; ok && dt > 0 && s.cpuSec >= p.cpuSec {
			c = (s.cpuSec - p.cpuSec) / dt * 100 / ncpu
		}
		m := 0.0
		if total > 0 {
			m = float64(s.rss) / float64(total) * 100
		}
		rows = append(rows, ProcRow{Name: cleanProcName(s.name), CPU: c, Mem: m})
	}
	t.prev, t.prevT = cur, now
	// deterministic aggregation order for ties
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	top := Aggregate(rows, TopN)
	if len(top) == 0 {
		return nil
	}
	list := make([]*frame.Obj, len(top))
	for i, r := range top {
		list[i] = frame.New("n", r.Name, "c", r.CPU, "m", r.Mem)
	}
	out.Set("procs", list)
	return nil
}
