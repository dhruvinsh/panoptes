package collectors

import (
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/yusufpapurcu/wmi"
	"golang.org/x/sys/windows"

	"github.com/dhruvinsh/panoptes/internal/frame"
)

// Windows has no built-in sensor API for temps/fans. If LibreHardwareMonitor or
// OpenHardwareMonitor is running (with its WMI provider), their sensors are used, like
// the Python agent's optional `wmi` support - here it needs no extra install.

var wmiNamespaces = []string{`root\LibreHardwareMonitor`, `root\OpenHardwareMonitor`}

type wmiSensor struct {
	Name   string
	Value  float32
	Parent string
}

var (
	wmiMu      sync.Mutex
	wmiNS      string
	wmiNextTry time.Time
)

func wmiSensors(kind string) []wmiSensor {
	wmiMu.Lock()
	defer wmiMu.Unlock()
	q := "SELECT Name, Value, Parent FROM Sensor WHERE SensorType='" + kind + "'"
	if wmiNS == "" {
		if time.Now().Before(wmiNextTry) {
			return nil
		}
		wmiNextTry = time.Now().Add(60 * time.Second)
		for _, ns := range wmiNamespaces {
			var probe []wmiSensor
			if err := wmi.QueryNamespace(q, &probe, ns); err == nil {
				wmiNS = ns
				return probe
			}
		}
		return nil
	}
	var out []wmiSensor
	if err := wmi.QueryNamespace(q, &out, wmiNS); err != nil {
		wmiNS = "" // LHM was closed; retry later
		return nil
	}
	return out
}

func readTemperatures() ([]*frame.Obj, *float64, error) {
	var temps []*frame.Obj
	var cpu *float64
	seen := map[string]bool{}
	for _, s := range wmiSensors("Temperature") {
		val := float64(s.Value)
		if !validTemp(val) {
			continue
		}
		parent, low := strings.ToLower(s.Parent), strings.ToLower(s.Name)
		if strings.Contains(parent, "/cpu") || strings.Contains(parent, "/amdcpu") || strings.Contains(parent, "/intelcpu") {
			if cpu == nil && (strings.Contains(low, "package") || strings.Contains(low, "tctl") || strings.Contains(low, "tdie")) {
				v := val
				cpu = &v
			}
			continue
		}
		var n string
		switch {
		case strings.Contains(parent, "gpu"):
			n = "GPU"
		case strings.Contains(parent, "nvme"):
			n = "NVMe"
		case strings.Contains(parent, "hdd") || strings.Contains(parent, "/storage"):
			n = "Disk"
		default:
			n = truncName(s.Name, 8)
		}
		if !seen[n] {
			seen[n] = true
			temps = append(temps, frame.New("n", n, "v", val))
		}
	}
	if cpu != nil {
		temps = append([]*frame.Obj{frame.New("n", "CPU", "v", *cpu)}, temps...)
	}
	if len(temps) > MaxTemps {
		temps = temps[:MaxTemps]
	}
	return temps, cpu, nil
}

func readFans() ([]*frame.Obj, error) {
	var fans []*frame.Obj
	for _, s := range wmiSensors("Fan") {
		if s.Value > 0 {
			fans = append(fans, frame.New("n", truncName(s.Name, 10), "rpm", int(s.Value)))
		}
	}
	if len(fans) > MaxFans {
		fans = fans[:MaxFans]
	}
	return fans, nil
}

type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

var procGetSystemPowerStatus = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetSystemPowerStatus")

// readBattery uses GetSystemPowerStatus (like psutil).
func readBattery() (*BatteryInfo, error) {
	var s systemPowerStatus
	r, _, err := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&s)))
	if r == 0 {
		return nil, err
	}
	if s.BatteryFlag == 128 || s.BatteryFlag == 255 || s.BatteryLifePercent == 255 { // no battery / unknown
		return nil, nil
	}
	info := &BatteryInfo{Percent: float64(s.BatteryLifePercent)}
	switch s.ACLineStatus {
	case 0:
		info.Plugged = boolPtr(false)
	case 1:
		info.Plugged = boolPtr(true)
	}
	return info, nil
}
