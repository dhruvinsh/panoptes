package collectors

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dhruvinsh/panoptes/internal/frame"
	"github.com/dhruvinsh/panoptes/internal/logx"
)

func TestAllCollectorsRunAndFrameIsValid(t *testing.T) {
	src := NewSource()
	src.Prime()
	f := src.Collect()
	if f.Value("t") != "s" {
		t.Fatal("t != s")
	}
	if _, ok := f.Value("ts").(int64); !ok {
		t.Fatalf("ts: %T", f.Value("ts"))
	}
	b := frame.FitFrame(f)
	if len(b) > frame.MaxLine {
		t.Fatalf("too long: %d", len(b))
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, b)
	}
	t.Logf("%d bytes: %s", len(b), b)
}

func TestEachCollectorIndividually(t *testing.T) {
	for _, c := range Defaults() {
		t.Run(c.Name(), func(t *testing.T) {
			// allowed to fail (e.g. no sensors in CI) - StatsSource isolates it - but never panic
			_ = c.Prime()
			_ = c.Collect(frame.New())
		})
	}
}

type boom struct{}

func (boom) Name() string { return "boom" }
func (boom) Prime() error { return errors.New("prime boom") }
func (boom) Collect(out *frame.Obj) error {
	out.Set("partial", 1) // must not leak into the frame
	return errors.New("boom")
}

type panicky struct{ noPrime }

func (panicky) Name() string { return "panicky" }
func (panicky) Collect(out *frame.Obj) error {
	out.Set("partial2", 1)
	var m map[string]int
	m["x"] = 1 // nil map write panics
	return nil
}

type good struct{ noPrime }

func (good) Name() string { return "good" }
func (good) Collect(out *frame.Obj) error {
	out.Sub("cpu").Set("pct", 5.0)
	return nil
}

func TestFailingCollectorIsolated(t *testing.T) {
	var logs []string
	defer logx.SetHook(func(_ logx.Level, m string) { logs = append(logs, m) })()
	src := &StatsSource{Collectors: []Collector{boom{}, panicky{}, good{}}}
	src.Prime()
	f1 := string(frame.Encode(src.Collect()))
	f2 := string(frame.Encode(src.Collect()))
	want := `{"t":"s","cpu":{"pct":5.0}}` + "\n"
	if f1 != want || f2 != want {
		t.Fatalf("got %s / %s", f1, f2)
	}
	n := 0
	for _, l := range logs {
		if strings.Contains(l, "boom") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("boom logged %d times: %v", n, logs)
	}
}

func TestFinalizeAddsGPUTempAfterCPU(t *testing.T) {
	f := frame.New("gpu", frame.New("temp", 50), "temps", []*frame.Obj{frame.New("n", "CPU", "v", 40.0), frame.New("n", "NVMe", "v", 30.0)})
	Finalize(f)
	if s := string(frame.Encode(f.Value("temps"))); s != `[{"n":"CPU","v":40.0},{"n":"GPU","v":50.0},{"n":"NVMe","v":30.0}]`+"\n" {
		t.Fatalf("got %s", s)
	}
	f = frame.New("gpu", frame.New("temp", 50))
	Finalize(f)
	if s := string(frame.Encode(f)); s != `{"gpu":{"temp":50},"temps":[{"n":"GPU","v":50.0}]}`+"\n" {
		t.Fatalf("got %s", s)
	}
}

func T(label string, v float64) TempEntry { return TempEntry{Label: label, Current: v} }

func names(l []*frame.Obj) []string {
	out := []string{}
	for _, o := range l {
		out = append(out, o.Value("n").(string))
	}
	return out
}

func TestMapTemperatures(t *testing.T) {
	raw := []TempChip{
		{"nvme", []TempEntry{T("Composite", 41), T("Sensor 1", 50), T("Composite", 38)}},
		{"k10temp", []TempEntry{T("Tctl", 60), T("Tccd1", 55)}},
		{"jc42", []TempEntry{T("", 35), T("", 36.5)}},
		{"iwlwifi_1", []TempEntry{T("", 40)}},
		{"acpitz", []TempEntry{T("", 0)}}, // invalid -> skipped
		{"amdgpu", []TempEntry{T("edge", 48), T("junction", 55)}},
		{"weirdchip", []TempEntry{T("", 33)}},
	}
	temps, cpu := MapTemperatures(raw)
	if cpu == nil || *cpu != 60 {
		t.Fatalf("cpu = %v", cpu)
	}
	if got := names(temps); !reflect.DeepEqual(got, []string{"CPU", "GPU", "NVMe", "NVMe2", "DIMM", "WiFi"}) {
		t.Fatalf("got %v", got)
	}
	if temps[1].Value("v") != 48.0 || temps[4].Value("v") != 36.5 {
		t.Fatalf("values: %s", frame.Encode(temps))
	}
}

func TestCoretempPackage(t *testing.T) {
	temps, cpu := MapTemperatures([]TempChip{{"coretemp", []TempEntry{T("Core 0", 50), T("Package id 0", 52)}}})
	if cpu == nil || *cpu != 52 || string(frame.Encode(temps)) != `[{"n":"CPU","v":52.0}]`+"\n" {
		t.Fatalf("got %v %s", cpu, frame.Encode(temps))
	}
}

func TestMapFans(t *testing.T) {
	fans := MapFans([]FanChip{
		{"nct6775", []FanEntry{{"", 1150}, {"", 0}, {"", 800}}},
		{"thinkpad", []FanEntry{{"", 2000}}},
		{"dell_smm", []FanEntry{{"Processor Fan", 3000}, {"Video Fan", 2500}}},
	})
	if got := names(fans); !reflect.DeepEqual(got, []string{"nct6771", "nct6773", "thinkpad", "Processor "}) {
		t.Fatalf("got %v", got)
	}
}

func TestProcsAggregate(t *testing.T) {
	top := Aggregate([]ProcRow{{"chrome", 5, 1}, {"chrome", 6, 2}, {"bash", 1, 0.1}, {"x", 20, 0}}, TopN)
	if top[0] != (ProcRow{"x", 20, 0}) || top[1] != (ProcRow{"chrome", 11, 3}) {
		t.Fatalf("got %v", top)
	}
	if cleanProcName("SearchIndexer.EXE") != "SearchIndexer" || cleanProcName("a-very-long-process-name") != "a-very-long-pro" {
		t.Fatal("cleanProcName")
	}
}

func TestProcsPrimeGivesNonzeroTotal(t *testing.T) {
	p := &TopProcesses{}
	if err := p.Prime(); err != nil {
		t.Skipf("no process list: %v", err)
	}
	x := 0
	for i := 0; i < 300_000_000; i++ { // burn a little CPU in this process
		x += i
	}
	_ = x
	out := frame.New()
	if err := p.Collect(out); err != nil {
		t.Fatal(err)
	}
	procs, _ := out.Value("procs").([]*frame.Obj)
	if len(procs) == 0 {
		t.Fatal("no procs")
	}
	nonzero := false
	for _, r := range procs {
		if r.Value("c").(float64) > 0 {
			nonzero = true
		}
	}
	if !nonzero {
		t.Fatalf("all zero: %s", frame.Encode(procs))
	}
}

func TestShortenCPUName(t *testing.T) {
	for in, want := range map[string]string{
		"AMD Ryzen 7 5800X 8-Core Processor":       "AMD Ryzen 7 5800X",
		"Intel(R) Core(TM) i7-8700K CPU @ 3.70GHz": "Intel Core i7-8700K",
		"Apple M2 Pro": "Apple M2 Pro",
		"AMD Ryzen 7 7840U w/ Radeon 780M Graphics": "AMD Ryzen 7 7840U w/ Radeon 780M Graphics",
		"AMD Ryzen 5 5600G with Radeon Graphics":    "AMD Ryzen 5 5600G",
	} {
		if got := ShortenCPUName(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestParseCPUInfoModel(t *testing.T) {
	if got := parseCPUInfoModel("processor\t: 0\nmodel name\t: AMD Ryzen 9\nmodel name\t: other\n"); got != "AMD Ryzen 9" {
		t.Fatalf("got %q", got)
	}
	if got := parseCPUInfoModel("Hardware\t: BCM2835\nModel\t: Raspberry Pi 4\n"); got != "BCM2835" {
		t.Fatalf("got %q", got)
	}
}

func TestPickPrimary(t *testing.T) {
	counters := map[string]IfCounters{"lo": {1e9, 1e9}, "docker0": {1e8, 0}, "enp5s0": {100, 100}, "wlan0": {5000, 5000},
		"tun0": {1e7, 0}}
	order := []string{"lo", "docker0", "enp5s0", "wlan0", "tun0"}
	addrs := map[string][]string{"enp5s0": {"192.168.1.2"}, "wlan0": {"10.0.0.5"}}
	up := map[string]bool{}
	for k := range counters {
		up[k] = true
	}
	if g := PickPrimary(counters, addrs, up, "enp5s0", "", order); g != "enp5s0" {
		t.Fatal(g)
	}
	if g := PickPrimary(counters, addrs, up, "tun0", "10.0.0.5", order); g != "wlan0" { // tun excluded
		t.Fatal(g)
	}
	if g := PickPrimary(counters, addrs, up, "", "", order); g != "wlan0" { // most traffic, excluding virtual
		t.Fatal(g)
	}
	up["wlan0"] = false
	if g := PickPrimary(counters, addrs, up, "", "", order); g != "enp5s0" { // down interfaces skipped
		t.Fatal(g)
	}
	for _, n := range []string{"vethab12", "virbr0", "Loopback Pseudo-Interface 1", "vEthernet (WSL)"} {
		if !IsExcludedIface(n) {
			t.Errorf("%s should be excluded", n)
		}
	}
	for _, n := range []string{"Wi-Fi", "en0", "Ethernet"} {
		if IsExcludedIface(n) {
			t.Errorf("%s should not be excluded", n)
		}
	}
}

func TestParseProcNetRoute(t *testing.T) {
	text := "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
		"wg0\t00000000\t00000000\t0001\t0\t0\t50\t00000000\t0\t0\t0\n" +
		"tun0\t00000000\t0101A8C0\t0003\t0\t0\t10\t00000000\t0\t0\t0\n" +
		"wlan0\t00000000\t0101A8C0\t0003\t0\t0\t600\t00000000\t0\t0\t0\n" +
		"enp5s0\t00000000\t0101A8C0\t0003\t0\t0\t100\t00000000\t0\t0\t0\n" +
		"enp5s0\t0001A8C0\t00000000\t0001\t0\t0\t100\t00FFFFFF\t0\t0\t0\n"
	if got := parseProcNetRoute(text); got != "wg0" {
		t.Fatalf("got %q", got)
	}
}

func Part(dev, mp, fstype, opts string) Partition {
	return Partition{Device: dev, Mountpoint: mp, Fstype: fstype, Opts: opts}
}

func TestSelectPartitionsLinux(t *testing.T) {
	parts := []Partition{
		Part("/dev/sda2", "/home", "btrfs", "rw"), Part("/dev/sda2", "/", "btrfs", "rw"),
		Part("/dev/loop0", "/snap/core/1", "squashfs", "ro"), Part("/dev/sda1", "/boot", "vfat", "rw"),
		Part("tmpfs", "/tmp", "tmpfs", "rw"), Part("overlay", "/var/lib/docker/x", "overlay", "rw"),
		Part("/dev/sdb1", "/data", "ext4", "rw"), Part("/dev/sdc1", "/mnt/a", "ext4", "rw"),
		Part("/dev/sdd1", "/mnt/b", "xfs", "rw"), Part("gvfsd-fuse", "/run/user/1000/gvfs", "fuse.gvfsd-fuse", "rw"),
	}
	if got := SelectPartitions(parts, "linux"); !reflect.DeepEqual(got, []string{"/", "/data", "/mnt/a", "/mnt/b"}) {
		t.Fatalf("got %v", got)
	}
}

func TestSelectPartitionsWindows(t *testing.T) {
	parts := []Partition{Part(`C:\`, `C:\`, "NTFS", "rw"), Part(`D:\`, `D:\`, "", "cdrom"), Part(`E:\`, `E:\`, "exFAT", "rw")}
	if got := SelectPartitions(parts, "windows"); !reflect.DeepEqual(got, []string{`C:\`, `E:\`}) {
		t.Fatalf("got %v", got)
	}
	if DisplayMount(`C:\`, "windows") != "C:" || DisplayMount("/run/media/me/USB", "linux") != "USB" ||
		DisplayMount("/Volumes/Backup/", "darwin") != "Backup" || DisplayMount("/home", "linux") != "/home" {
		t.Fatal("DisplayMount")
	}
}

func TestParseOSRelease(t *testing.T) {
	if got := parseOSRelease("NAME=\"Arch Linux\"\nPRETTY_NAME=\"Arch Linux\"\nID=arch\n"); got != "Arch Linux" {
		t.Fatal(got)
	}
	if got := parseOSRelease("NAME='Foo'\n"); got != "Foo" {
		t.Fatal(got)
	}
}

func TestParsePmset(t *testing.T) {
	b := parsePmset("Now drawing from 'Battery Power'\n -InternalBattery-0 (id=4653155)\t87%; discharging; 4:12 remaining present: true\n")
	if b == nil || b.Percent != 87 || b.Plugged == nil || *b.Plugged {
		t.Fatalf("got %+v", b)
	}
	if parsePmset("Now drawing from 'AC Power'\n") != nil { // desktop Mac: no battery
		t.Fatal("expected nil")
	}
}

func TestRateTracker(t *testing.T) {
	tick := time.Unix(1000, 0)
	r := RateTracker{now: func() time.Time { tick = tick.Add(time.Second); return tick }}
	if r.Update(map[string]float64{"a": 10}) != nil {
		t.Fatal("first update must be nil")
	}
	got := r.Update(map[string]float64{"a": 5}) // counter reset
	if got == nil || got["a"] != 0 {
		t.Fatalf("got %v", got)
	}
}

func collectorNames(cs []Collector) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Name())
	}
	return out
}

func TestDisableGroups(t *testing.T) {
	all := collectorNames(DefaultsWith(Options{}))
	if !reflect.DeepEqual(all, collectorNames(Defaults())) {
		t.Fatal("no options must equal Defaults()")
	}
	got := collectorNames(DefaultsWith(Options{Disable: []string{"gpu", "disk", "procs", "battery"}}))
	for _, gone := range []string{"gpu", "disk.io", "disk.mounts", "procs", "bat"} {
		for _, n := range got {
			if n == gone {
				t.Errorf("%s should be disabled: %v", gone, got)
			}
		}
	}
	if len(got) != len(all)-5 {
		t.Errorf("only the disabled collectors should go: %v", got)
	}
}

func TestDisableTempsDropsGPUTemp(t *testing.T) {
	src := &StatsSource{Collectors: []Collector{gpuTemp{}}, noTemps: true}
	if src.Collect().Has("temps") {
		t.Fatal("temps must not be surfaced when disabled")
	}
	src.noTemps = false
	if !src.Collect().Has("temps") {
		t.Fatal("control: temps is surfaced from the GPU temperature")
	}
}

type gpuTemp struct{ noPrime }

func (gpuTemp) Name() string { return "gpu" }
func (gpuTemp) Collect(out *frame.Obj) error {
	out.Set("gpu", frame.New("name", "X", "temp", 50))
	return nil
}

func TestNormalizeMounts(t *testing.T) {
	if got := normalizeMounts([]string{" /home ", "", "/"}, "linux"); !reflect.DeepEqual(got, []string{"/home", "/"}) {
		t.Fatalf("got %v", got)
	}
	if got := normalizeMounts([]string{"C:", `D:\`, "e:"}, "windows"); !reflect.DeepEqual(got, []string{`C:\`, `D:\`, `e:\`}) {
		t.Fatalf("got %v", got)
	}
	if got := normalizeMounts([]string{"/a", "/b", "/c", "/d", "/e"}, "linux"); len(got) != MaxMounts {
		t.Fatalf("not capped: %v", got)
	}
	if normalizeMounts(nil, "linux") != nil {
		t.Fatal("nil must stay nil: it means auto-select")
	}
}

func TestForcedIface(t *testing.T) {
	n := &Network{Iface: "WI-FI"}
	if got := n.forcedIface([]string{"lo", "Wi-Fi"}); got != "Wi-Fi" {
		t.Fatalf("got %q", got)
	}
	if got := n.forcedIface([]string{"lo", "eth0"}); got != "" {
		t.Fatalf("a missing interface must fall back to detection, got %q", got)
	}
	if got := (&Network{}).forcedIface([]string{"lo"}); got != "" {
		t.Fatalf("got %q", got)
	}
}
