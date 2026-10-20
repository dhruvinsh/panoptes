package ports

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Device name prefixes considered serial ports (same set as pyserial).
var linuxPrefixes = []string{"ttyS", "ttyUSB", "ttyXRUSB", "ttyACM", "ttyAMA", "rfcomm", "ttyAP", "ttyGS"}

// osPorts reads /sys/class/tty. Unlike go.bug.st/serial's lister it never opens a port
// (that one probes every ttyS* by opening it, every 2 s while we wait for the board).
func osPorts() ([]Port, error) {
	entries, err := os.ReadDir("/sys/class/tty")
	if err != nil {
		return nil, err
	}
	var out []Port
	for _, e := range entries {
		name := e.Name()
		if !hasAnyPrefix(name, linuxPrefixes) {
			continue
		}
		p, ok := sysfsPort("/sys/class/tty/"+name, "/dev/"+name)
		if ok {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Device < out[j].Device })
	return out, nil
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// sysfsPort inspects one /sys/class/tty/<name> entry.
func sysfsPort(sysdir, dev string) (Port, bool) {
	devLink := filepath.Join(sysdir, "device")
	devPath, err := filepath.EvalSymlinks(devLink)
	if err != nil {
		return Port{}, false // virtual tty (no hardware behind it)
	}
	subsystem := ""
	if s, err := filepath.EvalSymlinks(filepath.Join(devPath, "subsystem")); err == nil {
		subsystem = filepath.Base(s)
	}
	if subsystem == "platform" { // placeholder ttyS* ports (older kernels)
		return Port{}, false
	}
	// Newer kernels put every 8250 slot on "serial-base"; unused slots report UART type 0.
	if strings.HasPrefix(filepath.Base(dev), "ttyS") && readTrim(filepath.Join(sysdir, "type")) == "0" {
		return Port{}, false
	}
	if _, err := os.Stat(dev); err != nil {
		return Port{}, false
	}
	p := Port{Device: dev, Description: "n/a", HWID: "n/a"}
	// Walk up to the USB device (the directory that has idVendor).
	iface := ""
	d := devPath
	for i := 0; i < 5 && strings.HasPrefix(d, "/sys/devices"); i++ {
		if _, err := os.Stat(filepath.Join(d, "idVendor")); err == nil {
			vid, _ := strconv.ParseInt(readTrim(filepath.Join(d, "idVendor")), 16, 32)
			pid, _ := strconv.ParseInt(readTrim(filepath.Join(d, "idProduct")), 16, 32)
			p.VID, p.PID = int(vid), int(pid)
			p.SerialNumber = readTrim(filepath.Join(d, "serial"))
			product := readTrim(filepath.Join(d, "product"))
			ifText := ""
			if iface != "" {
				ifText = readTrim(filepath.Join(iface, "interface"))
			}
			switch {
			case product != "" && ifText != "":
				p.Description = product + " - " + ifText
			case product != "":
				p.Description = product
			case ifText != "":
				p.Description = ifText
			}
			loc := filepath.Base(iface)
			if iface == "" {
				loc = filepath.Base(d)
			}
			p.HWID = fmt.Sprintf("USB VID:PID=%04X:%04X", p.VID, p.PID)
			if p.SerialNumber != "" {
				p.HWID += " SER=" + p.SerialNumber
			}
			p.HWID += " LOCATION=" + loc
			break
		}
		if _, err := os.Stat(filepath.Join(d, "bInterfaceNumber")); err == nil {
			iface = d
		}
		d = filepath.Dir(d)
	}
	return p, true
}
