package frame

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

func bigFrame(nCores, nProcs, procNameLen, nTemps, nFans int) *Obj {
	cores := make([]float64, nCores)
	for i := range cores {
		cores[i] = 12.3456 + float64(i)
	}
	mounts := make([]*Obj, 4)
	for i := range mounts {
		mounts[i] = New("m", fmt.Sprintf("/mnt/m%d", i), "pct", 41.0, "used", 200.55, "total", 488.0)
	}
	temps := make([]*Obj, nTemps)
	for i := range temps {
		temps[i] = New("n", fmt.Sprintf("T%d", i), "v", 40.0+float64(i))
	}
	fans := make([]*Obj, nFans)
	for i := range fans {
		fans[i] = New("n", fmt.Sprintf("fan%d", i), "rpm", 1000+i)
	}
	procs := make([]*Obj, nProcs)
	for i := range procs {
		procs[i] = New("n", strings.Repeat("p", procNameLen), "c", 12.345, "m", 4.5)
	}
	return New(
		"t", "s", "ts", 1760950000, "tz", -14400, "host", strings.Repeat("h", 40),
		"os", "Some Very Long Linux Distro 2026", "kern", "6.18.57-2-lts", "up", 123456,
		"cpu", New("name", "AMD Ryzen Threadripper PRO 7995WX", "pct", 23.456, "cores", cores,
			"freq", 3600, "temp", 55.04, "load", []float64{1.234, 0.9, 0.7}),
		"mem", New("used", 8123, "total", 32000, "pct", 25.44, "swap_pct", 0.0),
		"gpu", New("name", "RTX 3070", "pct", 12, "temp", 48, "mem_used", 1200, "mem_total", 8192, "power", 45.25),
		"net", New("iface", "enp5s0", "ip", "192.168.1.20", "rx", 125000.123, "tx", 4200.0),
		"disk", New("r", 0.0, "w", 52000.0, "mounts", mounts),
		"temps", temps,
		"fans", fans,
		"procs", procs,
		"bat", New("pct", 87, "plug", true),
	)
}

func defaultBig() *Obj { return bigFrame(32, 5, 15, 6, 4) }

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	if !bytes.HasSuffix(b, []byte("\n")) || bytes.Count(b, []byte("\n")) != 1 {
		t.Fatalf("not a single line: %q", b)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("invalid json %q: %v", b, err)
	}
	return m
}

func TestCompactAndRounded(t *testing.T) {
	b := Encode(New("t", "s", "x", 1.26, "y", []any{0.04, -0.04}, "z", nil, "w", math.NaN()))
	if string(b) != `{"t":"s","x":1.3,"y":[0.0,0.0]}`+"\n" {
		t.Fatalf("got %q", b)
	}
}

func TestFloatFormatting(t *testing.T) {
	cases := map[float64]string{55.0: "55.0", 0.25: "0.2", 0.35: "0.3", 2.675: "2.7", 12000000.0: "12000000.0",
		-0.04: "0.0", 1e16: "1e+16", 3.14159: "3.1"}
	for in, want := range cases {
		if got := FormatFloat(in); got != want {
			t.Errorf("FormatFloat(%v) = %s, want %s", in, got, want)
		}
	}
	if s := string(Encode([]float64{math.Inf(1), 1})); s != "[1.0]\n" {
		t.Errorf("inf not dropped: %s", s)
	}
}

func TestBoolsSurviveAndStringsCapped(t *testing.T) {
	b := Encode(New("plug", true, "n", 0, "s", strings.Repeat("é", 60), "q", "a\"b\\c\n\x01<>"))
	want := `{"plug":true,"n":0,"s":"` + strings.Repeat("é", 48) + `","q":"a\"b\\c\n\u0001<>"}` + "\n"
	if string(b) != want {
		t.Fatalf("got %q", b)
	}
}

func TestNormalFrameUntouched(t *testing.T) {
	b := FitFrame(defaultBig())
	if len(b) > MaxLine {
		t.Fatalf("too long: %d", len(b))
	}
	d := decode(t, b)
	if len(d["procs"].([]any)) != 5 || len(d["cpu"].(map[string]any)["cores"].([]any)) != 32 || len(d["temps"].([]any)) != 6 {
		t.Fatalf("frame was truncated: %s", b)
	}
}

func TestDropOrder(t *testing.T) {
	f := defaultBig()
	full := len(Encode(f))

	noProcs := f.Clone()
	noProcs.Delete("procs")
	limProcs := len(Encode(noProcs))
	d := decode(t, Fit(f, limProcs))
	cpu := d["cpu"].(map[string]any)
	if _, ok := d["procs"]; ok {
		t.Fatal("procs not dropped")
	}
	if _, ok := d["fans"]; !ok || len(d["temps"].([]any)) != 6 || cpu["cores"] == nil {
		t.Fatal("dropped too much at step 1")
	}

	noFans := noProcs.Clone()
	noFans.Delete("fans")
	d = decode(t, Fit(f, len(Encode(noFans))))
	if _, ok := d["fans"]; ok || d["procs"] != nil || len(d["temps"].([]any)) != 6 {
		t.Fatal("step 2 wrong")
	}

	t3 := noFans.Clone()
	t3.Set("temps", t3.Value("temps").([]*Obj)[:3])
	d = decode(t, Fit(f, len(Encode(t3))))
	temps := d["temps"].([]any)
	if len(temps) != 3 || temps[0].(map[string]any)["n"] != "T0" || temps[2].(map[string]any)["n"] != "T2" {
		t.Fatalf("step 3 wrong: %v", temps)
	}
	if d["cpu"].(map[string]any)["cores"] == nil {
		t.Fatal("cores dropped too early")
	}

	d = decode(t, Fit(f, len(Encode(t3))-1))
	cpu = d["cpu"].(map[string]any)
	if cpu["cores"] != nil || len(d["temps"].([]any)) != 3 || cpu["pct"] == nil {
		t.Fatal("step 4 wrong")
	}
	if full <= limProcs {
		t.Fatal("full frame should be longer")
	}
	// the input frame is never mutated
	if !f.Has("procs") || len(f.Obj("cpu").Value("cores").([]float64)) != 32 {
		t.Fatal("input mutated")
	}
}

func TestHugeFrameFits(t *testing.T) {
	f := bigFrame(32, 40, 200, 30, 30)
	if len(Encode(f)) <= MaxLine {
		t.Fatal("test frame should be oversized")
	}
	b := FitFrame(f)
	if len(b) > MaxLine {
		t.Fatalf("too long: %d", len(b))
	}
	d := decode(t, b)
	if d["procs"] != nil || d["mem"] == nil || d["t"] != "s" {
		t.Fatalf("unexpected: %s", b)
	}
}

func TestPathologicalAlwaysFits(t *testing.T) {
	cores := make([]float64, 5000)
	for i := range cores {
		cores[i] = 1
	}
	xs := make([]any, 500)
	for i := range xs {
		xs[i] = strings.Repeat("y", 40)
	}
	b := FitFrame(New("t", "s", "ts", 1, "cpu", New("cores", cores), "x", xs))
	if len(b) > MaxLine || decode(t, b)["t"] != "s" {
		t.Fatalf("bad: %d", len(b))
	}
}

func TestUTF8BytesCounted(t *testing.T) {
	f := defaultBig()
	f.Set("host", strings.Repeat("é", 40))
	if b := Fit(f, 900); len(b) > 900 {
		t.Fatalf("too long: %d", len(b))
	}
}

func TestStableKeyOrder(t *testing.T) {
	o := New("b", 1, "a", 2)
	o.Set("b", 3)
	o.Set("c", 4)
	if s := string(Encode(o)); s != `{"b":3,"a":2,"c":4}`+"\n" {
		t.Fatalf("got %s", s)
	}
	dst := New("cpu", New("pct", 1.0))
	Merge(dst, New("cpu", New("temp", 50.0), "mem", New("pct", 2.0)))
	if s := string(Encode(dst)); s != `{"cpu":{"pct":1.0,"temp":50.0},"mem":{"pct":2.0}}`+"\n" {
		t.Fatalf("merge: %s", s)
	}
}
