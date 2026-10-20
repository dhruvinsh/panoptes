package demo

import (
	"encoding/json"
	"testing"

	"github.com/dhruvinsh/panoptes/internal/frame"
)

func TestDemoFrameFitsAndHasEveryWidget(t *testing.T) {
	s := New()
	s.Prime()
	b := frame.FitFrame(s.Collect())
	if len(b) > frame.MaxLine {
		t.Fatalf("too long: %d", len(b))
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"cpu", "mem", "gpu", "net", "disk", "temps", "fans", "procs", "bat"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing %s", k)
		}
	}
}
