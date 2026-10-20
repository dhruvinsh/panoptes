package collectors

import (
	"os"
	"strconv"
	"strings"
)

// Linux reports process CPU times in USER_HZ ticks, which is 100 on every mainstream
// architecture's userspace ABI.
const clkTck = 100.0

var pageSize = uint64(os.Getpagesize())

// listProcs reads /proc/<pid>/stat directly: one small read per process, much cheaper
// than gopsutil's per-process accessors when run every second.
func listProcs() (map[procKey]procSample, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	out := make(map[procKey]procSample, len(entries))
	for _, e := range entries {
		pid, err := strconv.ParseInt(e.Name(), 10, 32)
		if err != nil {
			continue
		}
		b, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue // exited
		}
		key, s, ok := parseProcStat(int32(pid), string(b))
		if ok {
			out[key] = s
		}
	}
	return out, nil
}

// parseProcStat parses "pid (comm) state ppid ... utime stime ... starttime vsize rss ...".
func parseProcStat(pid int32, text string) (procKey, procSample, bool) {
	open := strings.IndexByte(text, '(')
	closeP := strings.LastIndexByte(text, ')')
	if open < 0 || closeP < open {
		return procKey{}, procSample{}, false
	}
	name := text[open+1 : closeP]
	f := strings.Fields(text[closeP+1:])
	if len(f) < 22 {
		return procKey{}, procSample{}, false
	}
	utime, _ := strconv.ParseFloat(f[11], 64)
	stime, _ := strconv.ParseFloat(f[12], 64)
	rss, _ := strconv.ParseUint(f[21], 10, 64)
	return procKey{pid: pid, start: f[19]},
		procSample{name: name, cpuSec: (utime + stime) / clkTck, rss: rss * pageSize}, true
}
