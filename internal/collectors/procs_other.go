//go:build !linux

package collectors

import (
	"github.com/shirou/gopsutil/v4/process"
)

// listProcs uses gopsutil (Windows: per-process handles; macOS: libproc via purego).
// Processes we may not inspect (other users' on macOS, protected ones on Windows) are skipped.
func listProcs() (map[procKey]procSample, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}
	out := make(map[procKey]procSample, len(procs))
	for _, p := range procs {
		name, err := p.Name()
		if err != nil {
			continue
		}
		s := procSample{name: name}
		if t, err := p.Times(); err == nil {
			s.cpuSec = t.User + t.System
		}
		if m, err := p.MemoryInfo(); err == nil {
			s.rss = m.RSS
		}
		out[procKey{pid: p.Pid, start: name}] = s
	}
	return out, nil
}
