package collectors

import (
	"os"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/host"

	"github.com/dhruvinsh/panoptes/internal/frame"
)

// TZOffset is the local UTC offset in seconds.
func TZOffset() int {
	_, off := time.Now().Zone()
	return off
}

// SystemInfo: ts, tz, host, os, kern, up.
type SystemInfo struct {
	noPrime
	static *frame.Obj
	boot   uint64
}

func (*SystemInfo) Name() string { return "system" }

func (s *SystemInfo) loadStatic() {
	s.static = frame.New()
	if h, err := os.Hostname(); err == nil && h != "" {
		s.static.Set("host", strings.SplitN(h, ".", 2)[0])
	}
	if n := prettyOSName(); n != "" {
		s.static.Set("os", n)
	}
	if k := kernelVersion(); k != "" {
		s.static.Set("kern", k)
	}
}

func (s *SystemInfo) Collect(out *frame.Obj) error {
	if s.static == nil {
		s.loadStatic()
	}
	now := time.Now()
	out.Set("ts", now.Unix())
	out.Set("tz", TZOffset())
	frame.Merge(out, s.static)
	if s.boot == 0 {
		b, err := host.BootTime()
		if err != nil {
			return err
		}
		s.boot = b
	}
	out.Set("up", now.Unix()-int64(s.boot))
	return nil
}

// parseOSRelease returns PRETTY_NAME (or NAME) from os-release text.
func parseOSRelease(text string) string {
	data := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"`)
		v = strings.Trim(v, `'`)
		data[k] = v
	}
	if data["PRETTY_NAME"] != "" {
		return data["PRETTY_NAME"]
	}
	return data["NAME"]
}
