// Package collectors gathers the stats for one `s` frame.
//
// Each collector is small and fails independently: it writes into a scratch object that
// is merged into the frame only if it completed. A failing (or panicking) collector is
// logged once at debug level and simply contributes nothing to that frame.
package collectors

import (
	"fmt"
	"sync"
	"time"

	"github.com/dhruvinsh/panoptes/internal/frame"
	"github.com/dhruvinsh/panoptes/internal/logx"
)

// Collector is one independent piece of the frame.
type Collector interface {
	Name() string
	// Prime takes baseline samples (rates, cpu%) so the first real frame is meaningful.
	Prime() error
	Collect(out *frame.Obj) error
}

// MaxTemps caps the temps list.
const MaxTemps = 6

// Defaults returns the standard collector set, in frame order.
func Defaults() []Collector {
	return []Collector{
		&SystemInfo{},
		&CPUUsage{},
		&CPUFreq{},
		&CPULoad{},
		&CPUName{},
		&Memory{},
		&Swap{},
		&Network{},
		&DiskIO{},
		&DiskMounts{},
		&Temperatures{},
		&Fans{},
		&Battery{},
		NewGPU(),
		&TopProcesses{},
	}
}

// StatsSource runs all collectors and assembles one `s` frame.
type StatsSource struct {
	Collectors []Collector
	mu         sync.Mutex
	failed     map[string]bool
}

// NewSource returns a source using the default collectors.
func NewSource() *StatsSource { return &StatsSource{Collectors: Defaults()} }

func (s *StatsSource) fail(c Collector, phase string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failed == nil {
		s.failed = map[string]bool{}
	}
	if !s.failed[c.Name()] {
		s.failed[c.Name()] = true
		logx.Debugf("collector %s %s failed (%v); skipping it when it fails", c.Name(), phase, err)
	}
}

func safe(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn()
}

// Prime primes every collector.
func (s *StatsSource) Prime() {
	for _, c := range s.Collectors {
		if err := safe(c.Prime); err != nil {
			s.fail(c, "prime", err)
		}
	}
}

// Collect builds one frame.
func (s *StatsSource) Collect() *frame.Obj {
	f := frame.New("t", "s")
	for _, c := range s.Collectors {
		part := frame.New()
		if err := safe(func() error { return c.Collect(part) }); err != nil {
			s.fail(c, "collect", err)
			continue
		}
		frame.Merge(f, part)
	}
	Finalize(f)
	return f
}

// Finalize does cross-collector touch-ups: surface the GPU temp in `temps` (right after
// CPU) and enforce the list cap.
func Finalize(f *frame.Obj) {
	temps, _ := f.Value("temps").([]*frame.Obj)
	if gpu := f.Obj("gpu"); gpu != nil {
		if v, ok := gpu.Get("temp"); ok && v != nil {
			found := false
			for _, t := range temps {
				if t.Value("n") == "GPU" {
					found = true
				}
			}
			if !found {
				pos := 0
				if len(temps) > 0 && temps[0].Value("n") == "CPU" {
					pos = 1
				}
				g := frame.New("n", "GPU", "v", toFloat(v))
				temps = append(temps[:pos:pos], append([]*frame.Obj{g}, temps[pos:]...)...)
			}
			f.Set("temps", temps)
		}
	}
	if f.Has("temps") {
		if len(temps) > MaxTemps {
			temps = temps[:MaxTemps]
		}
		if len(temps) == 0 {
			f.Delete("temps")
		} else {
			f.Set("temps", temps)
		}
	}
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case float64:
		return x
	}
	return 0
}

// RateTracker turns monotonically increasing counters into per-second rates.
type RateTracker struct {
	prevT time.Time
	prev  map[string]float64
}

// Update records values and returns rates since the previous call (nil on the first).
func (r *RateTracker) Update(values map[string]float64) map[string]float64 {
	now := time.Now()
	prevT, prev := r.prevT, r.prev
	r.prevT = now
	r.prev = make(map[string]float64, len(values))
	for k, v := range values {
		r.prev[k] = v
	}
	if prev == nil {
		return nil
	}
	dt := now.Sub(prevT).Seconds()
	if dt <= 0 {
		return nil
	}
	rates := map[string]float64{}
	for k, v := range values {
		if p, ok := prev[k]; ok {
			if d := v - p; d >= 0 {
				rates[k] = d / dt
			} else {
				rates[k] = 0 // counter wrap/reset
			}
		}
	}
	return rates
}

// Reset forgets the baseline.
func (r *RateTracker) Reset() { r.prev = nil }

// noPrime can be embedded by collectors without a baseline.
type noPrime struct{}

func (noPrime) Prime() error { return nil }
