package collectors

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dhruvinsh/panoptes/internal/frame"
)

func readFileTrim(p string) (string, bool) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}

func readFloat(p string) (float64, bool) {
	s, ok := readFileTrim(p)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

// hwmonBases returns the sorted "<dir>/<kind>N" bases of every <kind>N_input under
// /sys/class/hwmon (and hwmonX/device/ for old drivers, unless already present) -
// the same discovery order as psutil.
func hwmonBases(kind string) []string {
	main, _ := filepath.Glob("/sys/class/hwmon/hwmon*/" + kind + "*_input")
	dev, _ := filepath.Glob("/sys/class/hwmon/hwmon*/device/" + kind + "*_input")
	have := map[string]bool{}
	var bases []string
	for _, p := range main {
		b := strings.TrimSuffix(p, "_input")
		have[b] = true
		bases = append(bases, b)
	}
	for _, p := range dev {
		b := strings.TrimSuffix(p, "_input")
		alt := strings.Replace(b, "/device/", "/", 1)
		if !have[alt] {
			bases = append(bases, b)
		}
	}
	sort.Strings(bases)
	return bases
}

func chipName(base string) string {
	dir := filepath.Dir(base)
	if n, ok := readFileTrim(filepath.Join(dir, "name")); ok && n != "" {
		return n
	}
	if n, ok := readFileTrim(filepath.Join(filepath.Dir(dir), "name")); ok && n != "" {
		return n
	}
	return filepath.Base(dir)
}

func readTemperatures() ([]*frame.Obj, *float64, error) {
	var chips []TempChip
	index := map[string]int{}
	add := func(name string, e TempEntry) {
		i, ok := index[name]
		if !ok {
			i = len(chips)
			index[name] = i
			chips = append(chips, TempChip{Name: name})
		}
		chips[i].Entries = append(chips[i].Entries, e)
	}
	bases := hwmonBases("temp")
	for _, b := range bases {
		v, ok := readFloat(b + "_input")
		if !ok {
			continue
		}
		label, _ := readFileTrim(b + "_label")
		add(chipName(b), TempEntry{Label: label, Current: v / 1000})
	}
	if len(bases) == 0 { // no hwmon at all: fall back to thermal zones (like psutil)
		zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
		sort.Strings(zones)
		for _, z := range zones {
			v, ok := readFloat(filepath.Join(z, "temp"))
			name, ok2 := readFileTrim(filepath.Join(z, "type"))
			if ok && ok2 {
				add(name, TempEntry{Current: v / 1000})
			}
		}
	}
	temps, cpu := MapTemperatures(chips)
	return temps, cpu, nil
}

func readFans() ([]*frame.Obj, error) {
	var chips []FanChip
	index := map[string]int{}
	for _, b := range hwmonBases("fan") {
		v, ok := readFloat(b + "_input")
		if !ok {
			continue
		}
		label, _ := readFileTrim(b + "_label")
		name := chipName(b)
		i, seen := index[name]
		if !seen {
			i = len(chips)
			index[name] = i
			chips = append(chips, FanChip{Name: name})
		}
		chips[i].Entries = append(chips[i].Entries, FanEntry{Label: label, RPM: v})
	}
	return MapFans(chips), nil
}

// readBattery mirrors psutil: BAT*/battery supply, percent from energy/charge/capacity,
// plugged from an AC adapter's "online" or the battery status.
func readBattery() (*BatteryInfo, error) {
	const root = "/sys/class/power_supply"
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil
	}
	var bat string
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, "BAT") || strings.Contains(strings.ToLower(n), "battery") {
			if t, _ := readFileTrim(filepath.Join(root, n, "type")); t == "" || t == "Battery" {
				if scope, _ := readFileTrim(filepath.Join(root, n, "scope")); scope == "Device" {
					continue // e.g. a wireless mouse
				}
				bat = filepath.Join(root, n)
				break
			}
		}
	}
	if bat == "" {
		return nil, nil
	}
	var pct float64
	if now, ok := readFloat(filepath.Join(bat, "energy_now")); ok {
		if full, ok := readFloat(filepath.Join(bat, "energy_full")); ok && full > 0 {
			pct = now / full * 100
		}
	} else if now, ok := readFloat(filepath.Join(bat, "charge_now")); ok {
		if full, ok := readFloat(filepath.Join(bat, "charge_full")); ok && full > 0 {
			pct = now / full * 100
		}
	}
	if pct == 0 {
		c, ok := readFloat(filepath.Join(bat, "capacity"))
		if !ok {
			return nil, nil
		}
		pct = c
	}
	if pct > 100 {
		pct = 100
	}
	info := &BatteryInfo{Percent: pct}
	for _, ac := range []string{"AC0", "AC", "ADP1", "ACAD"} {
		if v, ok := readFileTrim(filepath.Join(root, ac, "online")); ok {
			info.Plugged = boolPtr(v == "1")
			break
		}
	}
	if info.Plugged == nil {
		switch st, _ := readFileTrim(filepath.Join(bat, "status")); strings.ToLower(st) {
		case "charging", "full":
			info.Plugged = boolPtr(true)
		case "discharging":
			info.Plugged = boolPtr(false)
		}
	}
	return info, nil
}
