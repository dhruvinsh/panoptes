// Package demo produces synthetic, animated stats for exercising every widget on the
// display without real load.
package demo

import (
	"math"
	"time"

	"github.com/dhruvinsh/panoptes/internal/collectors"
	"github.com/dhruvinsh/panoptes/internal/frame"
)

func wave(t, period, lo, hi, phase float64) float64 {
	return lo + (hi-lo)*(0.5+0.5*math.Sin(2*math.Pi*t/period+phase))
}

// Source is the demo stats source.
type Source struct{ t0 time.Time }

// New returns a demo source starting now.
func New() *Source { return &Source{t0: time.Now()} }

// Prime does nothing.
func (*Source) Prime() {}

// Collect returns one synthetic frame.
func (s *Source) Collect() *frame.Obj {
	t := time.Since(s.t0).Seconds()
	now := time.Now()
	pct := wave(t, 20, 3, 97, 0)
	cpuTemp := wave(t, 30, 38, 92, 1.0)
	bat := int(wave(t, 120, 5, 100, 0))
	cores := make([]float64, 16)
	for i := range cores {
		cores[i] = wave(t, 7+float64(i%5), 0, 100, float64(i)*0.7)
	}
	memUsed := wave(t, 50, 4000, 30000, 0)
	return frame.New(
		"t", "s",
		"ts", now.Unix(),
		"tz", collectors.TZOffset(),
		"host", "demo-box",
		"os", "Demo OS 1.0",
		"kern", "6.0.0-demo",
		"up", int(123456+t),
		"cpu", frame.New(
			"name", "Demo CPU 9000X",
			"pct", pct,
			"cores", cores,
			"freq", int(wave(t, 15, 2200, 5200, 0)),
			"temp", cpuTemp,
			"load", []float64{wave(t, 40, 0.2, 8.0, 0), wave(t, 80, 0.5, 6.0, 0), wave(t, 160, 0.7, 4.0, 0)},
		),
		"mem", frame.New("used", int(memUsed), "total", 32000, "pct", memUsed/320.0,
			"swap_pct", wave(t, 90, 0, 40, 0)),
		"gpu", frame.New("name", "Demo GPU 4090", "pct", int(wave(t, 12, 0, 100, 2)),
			"temp", int(wave(t, 25, 35, 85, 2)), "mem_used", int(wave(t, 33, 500, 7800, 0)),
			"mem_total", 8192, "power", wave(t, 12, 15, 320, 2)),
		"net", frame.New("iface", "demo0", "ip", "192.168.1.42",
			"rx", wave(t, 9, 0, 12_000_000, 0), "tx", wave(t, 13, 0, 3_000_000, 1)),
		"disk", frame.New("r", wave(t, 11, 0, 500e6, 0), "w", wave(t, 17, 0, 200e6, 3),
			"mounts", []*frame.Obj{
				frame.New("m", "/", "pct", wave(t, 200, 30, 98, 0), "used", 300.0, "total", 488.0),
				frame.New("m", "/home", "pct", 62.0, "used", 1150.0, "total", 1863.0),
			}),
		"temps", []*frame.Obj{
			frame.New("n", "CPU", "v", cpuTemp),
			frame.New("n", "GPU", "v", wave(t, 25, 35, 85, 2)),
			frame.New("n", "NVMe", "v", wave(t, 60, 30, 70, 0)),
			frame.New("n", "DIMM", "v", 42.0),
		},
		"fans", []*frame.Obj{
			frame.New("n", "cpu", "rpm", int(wave(t, 30, 600, 2400, 1))),
			frame.New("n", "case", "rpm", int(wave(t, 45, 400, 1400, 0))),
		},
		"procs", []*frame.Obj{
			frame.New("n", "firefox", "c", wave(t, 10, 2, 60, 0), "m", 8.4),
			frame.New("n", "code", "c", wave(t, 14, 1, 30, 1), "m", 5.1),
			frame.New("n", "cc1plus", "c", wave(t, 6, 0, 99, 2), "m", 1.2),
			frame.New("n", "pipewire", "c", 1.5, "m", 0.3),
			frame.New("n", "Xorg", "c", wave(t, 8, 0, 9, 0), "m", 1.0),
		},
		"bat", frame.New("pct", bat, "plug", (int(t)/60)%2 == 0),
	)
}
