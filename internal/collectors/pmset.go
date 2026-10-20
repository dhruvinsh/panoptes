package collectors

import (
	"regexp"
	"strconv"
	"strings"
)

var rePmsetPct = regexp.MustCompile(`(\d+)%`)

// parsePmset parses `pmset -g batt` output (macOS).
func parsePmset(text string) *BatteryInfo {
	var info *BatteryInfo
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, "InternalBattery") {
			continue
		}
		m := rePmsetPct.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pct, _ := strconv.ParseFloat(m[1], 64)
		info = &BatteryInfo{Percent: pct}
		break
	}
	if info == nil {
		return nil
	}
	switch {
	case strings.Contains(text, "'AC Power'"):
		info.Plugged = boolPtr(true)
	case strings.Contains(text, "'Battery Power'"):
		info.Plugged = boolPtr(false)
	}
	return info
}
