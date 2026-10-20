package collectors

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/dhruvinsh/panoptes/internal/frame"
)

// MaxFans caps the fans list.
const MaxFans = 4

// TempEntry is one temperature input of a sensor chip.
type TempEntry struct {
	Label   string
	Current float64
}

// TempChip is one sensor chip (hwmon "name"), in discovery order. Chips with the same
// name are merged (like psutil: all NVMe drives end up under one "nvme" key).
type TempChip struct {
	Name    string
	Entries []TempEntry
}

var cpuChips = map[string]bool{"k10temp": true, "coretemp": true, "zenpower": true, "cpu_thermal": true,
	"cpu-thermal": true, "soc_thermal": true, "x86_pkg_temp": true, "cpu": true, "soc-thermal": true}

// Preferred label per chip (first match wins).
var preferredLabels = map[string][]string{
	"k10temp":  {"tdie", "tctl"},
	"zenpower": {"tdie", "tctl"},
	"coretemp": {"package id 0", "physical id 0"},
	"amdgpu":   {"edge", "junction"},
	"nvme":     {"composite"},
}

var niceNames = map[string]string{
	"amdgpu": "GPU", "radeon": "GPU", "nouveau": "GPU", "i915": "iGPU", "xe": "iGPU",
	"nvme": "NVMe", "drivetemp": "Disk", "acpitz": "ACPI", "iwlwifi": "WiFi", "mt7921_phy": "WiFi",
	"jc42": "DIMM", "spd5118": "DIMM", "pch": "PCH", "thinkpad": "TP", "dell_smm": "Dell",
	"nct6775": "Board", "nct6687": "Board", "it87": "Board", "asus": "Board", "asusec": "Board",
	"gigabyte_wmi": "Board", "bnxt_en": "NIC", "r8169": "NIC", "mlx5": "NIC",
}

// Display ordering (lower first) - temps beyond 3 may be dropped for size.
var tempOrder = map[string]int{"CPU": 0, "GPU": 1, "NVMe": 2, "iGPU": 3, "Disk": 4, "Board": 5, "DIMM": 6}

var reChipSuffix = regexp.MustCompile(`[_-]\d+$`)

// baseChip: "iwlwifi_1" -> "iwlwifi", "pch_cannonlake" -> "pch", "nvme0"... stays.
func baseChip(name string) string {
	n := reChipSuffix.ReplaceAllString(strings.ToLower(name), "")
	if strings.HasPrefix(n, "pch_") {
		return "pch"
	}
	if strings.HasPrefix(n, "mt79") {
		return "mt7921_phy"
	}
	return n
}

func validTemp(v float64) bool { return v > 0 && v < 150 }

func pickLabel(entries []TempEntry, preferred []string) (TempEntry, bool) {
	for _, want := range preferred {
		for _, e := range entries {
			if strings.ToLower(e.Label) == want && validTemp(e.Current) {
				return e, true
			}
		}
	}
	return TempEntry{}, false
}

func maxEntry(entries []TempEntry) TempEntry {
	best := entries[0]
	for _, e := range entries[1:] {
		if e.Current > best.Current {
			best = e
		}
	}
	return best
}

func truncName(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// MapTemperatures turns raw chips into the protocol temps list (max 6) and the CPU
// package temperature (k10temp/coretemp/zenpower/... -> "CPU").
func MapTemperatures(raw []TempChip) ([]*frame.Obj, *float64) {
	type t struct {
		order, idx int
		n          string
		v          float64
	}
	var temps []t
	var cpuTemp *float64
	for idx, chip := range raw {
		base := baseChip(chip.Name)
		var valid []TempEntry
		for _, e := range chip.Entries {
			if validTemp(e.Current) {
				valid = append(valid, e)
			}
		}
		if len(valid) == 0 {
			continue
		}
		if cpuChips[base] {
			e, ok := pickLabel(valid, preferredLabels[base])
			if !ok {
				e = maxEntry(valid)
			}
			if cpuTemp == nil {
				v := e.Current
				cpuTemp = &v
				temps = append(temps, t{tempOrder["CPU"], idx, "CPU", v})
			}
			continue
		}
		nice, ok := niceNames[base]
		if !ok {
			nice = truncName(chip.Name, 8)
		}
		if base == "nvme" {
			// one "Composite" per drive
			var comps []TempEntry
			for _, e := range valid {
				if strings.ToLower(e.Label) == "composite" {
					comps = append(comps, e)
				}
			}
			if len(comps) == 0 {
				comps = valid[:1]
			}
			for i, e := range comps {
				n := "NVMe"
				if i > 0 {
					n = fmt.Sprintf("NVMe%d", i+1)
				}
				temps = append(temps, t{tempOrder["NVMe"], idx, n, e.Current})
			}
			continue
		}
		e, ok := pickLabel(valid, preferredLabels[base])
		if !ok {
			e = maxEntry(valid)
		}
		o, ok := tempOrder[nice]
		if !ok {
			o = 7
		}
		temps = append(temps, t{o, idx, nice, e.Current})
	}
	sort.SliceStable(temps, func(i, j int) bool {
		if temps[i].order != temps[j].order {
			return temps[i].order < temps[j].order
		}
		return temps[i].idx < temps[j].idx
	})
	var out []*frame.Obj
	seen := map[string]int{}
	for _, x := range temps {
		n := x.n
		if c, ok := seen[n]; ok { // several chips mapping to the same nice name
			seen[n] = c + 1
			n = fmt.Sprintf("%s%d", n, c+1)
		} else {
			seen[n] = 1
		}
		out = append(out, frame.New("n", n, "v", x.v))
	}
	if len(out) > MaxTemps {
		out = out[:MaxTemps]
	}
	return out, cpuTemp
}

// FanEntry is one fan input.
type FanEntry struct {
	Label string
	RPM   float64
}

// FanChip groups the fans of one chip.
type FanChip struct {
	Name    string
	Entries []FanEntry
}

// MapFans turns raw fan chips into the protocol fans list (max 4, spinning fans only).
func MapFans(raw []FanChip) []*frame.Obj {
	var fans []*frame.Obj
	for _, chip := range raw {
		for i, e := range chip.Entries {
			if e.RPM <= 0 {
				continue
			}
			n := e.Label
			if n == "" {
				if len(chip.Entries) > 1 {
					n = fmt.Sprintf("%s%d", truncName(baseChip(chip.Name), 6), i+1)
				} else {
					n = truncName(chip.Name, 8)
				}
			}
			fans = append(fans, frame.New("n", truncName(n, 10), "rpm", int(e.RPM)))
		}
	}
	if len(fans) > MaxFans {
		fans = fans[:MaxFans]
	}
	return fans
}

// Temperatures: temps list plus cpu.temp.
type Temperatures struct{ noPrime }

func (*Temperatures) Name() string { return "temps" }

func (*Temperatures) Collect(out *frame.Obj) error {
	temps, cpu, err := readTemperatures()
	if err != nil {
		return err
	}
	if len(temps) > 0 {
		out.Set("temps", temps)
	}
	if cpu != nil {
		out.Sub("cpu").Set("temp", *cpu)
	}
	return nil
}

// Fans: fans list.
type Fans struct{ noPrime }

func (*Fans) Name() string { return "fans" }

func (*Fans) Collect(out *frame.Obj) error {
	fans, err := readFans()
	if err != nil {
		return err
	}
	if len(fans) > 0 {
		out.Set("fans", fans)
	}
	return nil
}

// Battery: bat.pct and bat.plug (laptops only).
type Battery struct{ noPrime }

func (*Battery) Name() string { return "bat" }

// BatteryInfo is a battery reading; Plugged is nil when unknown.
type BatteryInfo struct {
	Percent float64
	Plugged *bool
}

func (*Battery) Collect(out *frame.Obj) error {
	b, err := readBattery()
	if err != nil || b == nil {
		return err
	}
	o := frame.New("pct", int(math.RoundToEven(b.Percent)))
	if b.Plugged != nil {
		o.Set("plug", *b.Plugged)
	}
	out.Set("bat", o)
	return nil
}

func boolPtr(b bool) *bool { return &b }
