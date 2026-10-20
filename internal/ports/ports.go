// Package ports finds the board's serial port.
//
// Preference: the udev symlinks /dev/panoptes then /dev/cyd (Linux), then known USB
// bridge VID:PIDs (CH340 1a86:7523, CH9102 1a86:55d4, CP210x 10c4:ea60), then ports whose
// description looks like a USB-serial bridge. Other serial ports are never touched.
package ports

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Port describes one serial port.
type Port struct {
	Device       string // /dev/ttyUSB0, COM5, /dev/cu.wchusbserial1410
	VID, PID     int    // 0 when unknown
	Description  string // product string ("USB Serial", "USB-SERIAL CH340 (COM5)"), "" if unknown
	HWID         string // free-form hardware id text
	SerialNumber string
}

// HasUSB reports whether the VID/PID is known.
func (p Port) HasUSB() bool { return p.VID != 0 || p.PID != 0 }

// KnownUSBIDs in order of preference.
var KnownUSBIDs = []struct {
	VID, PID int
	Label    string
}{
	{0x1A86, 0x7523, "CH340"},
	{0x1A86, 0x55D4, "CH9102"},
	{0x10C4, 0xEA60, "CP210x"},
}

// Weak fallback on description/device name when VID/PID is unavailable
// (some drivers on Windows/macOS do not report it).
var nameHints = []string{"ch340", "ch341", "ch910", "cp210", "wchusbserial", "usb-serial", "usb serial",
	"usbserial", "slab_usbtouart"}

// LinuxSymlinks are created by packaging/70-panoptes.rules; the first existing one wins.
var LinuxSymlinks = []string{"/dev/panoptes", "/dev/cyd"}

// Hooks for tests.
var (
	listPorts = osPorts
	exists    = func(p string) bool { _, err := os.Stat(p); return err == nil }
	realpath  = func(p string) string {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return r
		}
		return p
	}
)

func score(p Port, goos string) (int, bool) {
	s := -1
	for i, k := range KnownUSBIDs {
		if p.VID == k.VID && p.PID == k.PID {
			s = i * 10
			break
		}
	}
	if s < 0 {
		text := strings.ToLower(p.Device + " " + p.Description + " " + p.HWID)
		for _, h := range nameHints {
			if strings.Contains(text, h) {
				s = 100
				break
			}
		}
	}
	if s < 0 {
		return 0, false
	}
	// macOS: /dev/tty.* blocks on open waiting for carrier; prefer /dev/cu.*
	if goos == "darwin" && strings.HasPrefix(p.Device, "/dev/tty.") {
		s += 5
	}
	return s, true
}

// Rank returns the candidate ports, best first (stable for ties).
func Rank(ports []Port, goos string) []Port {
	type sp struct {
		s, i int
		p    Port
	}
	var scored []sp
	for i, p := range ports {
		if s, ok := score(p, goos); ok {
			scored = append(scored, sp{s, i, p})
		}
	}
	sort.SliceStable(scored, func(a, b int) bool { return scored[a].s < scored[b].s })
	out := make([]Port, len(scored))
	for i, x := range scored {
		out[i] = x.p
	}
	return out
}

// Label is "CH340"/"CH9102"/"CP210x" for known bridges.
func Label(p Port) string {
	for _, k := range KnownUSBIDs {
		if p.VID == k.VID && p.PID == k.PID {
			return k.Label
		}
	}
	return ""
}

// All lists the serial ports of this machine.
func All() ([]Port, error) { return listPorts() }

func symlink(goos string) string {
	if goos != "linux" {
		return ""
	}
	for _, s := range LinuxSymlinks {
		if exists(s) {
			return s
		}
	}
	return ""
}

// Candidates returns the device paths to try, best first.
func Candidates() []string {
	ports, _ := listPorts()
	var devs []string
	for _, p := range Rank(ports, runtime.GOOS) {
		devs = append(devs, p.Device)
	}
	if link := symlink(runtime.GOOS); link != "" {
		real := realpath(link)
		out := []string{link}
		for _, d := range devs {
			if realpath(d) != real {
				out = append(out, d)
			}
		}
		devs = out
	}
	return devs
}

// Describe formats a port for --list.
func Describe(p Port) string {
	parts := []string{p.Device}
	if p.HasUSB() {
		parts = append(parts, fmt.Sprintf("%04x:%04x", p.VID, p.PID))
	}
	if l := Label(p); l != "" {
		parts = append(parts, l)
	}
	if p.Description != "" && p.Description != "n/a" {
		parts = append(parts, p.Description)
	}
	if p.SerialNumber != "" {
		parts = append(parts, "sn="+p.SerialNumber)
	}
	return strings.Join(parts, "  ")
}

// PrintList prints all ports, candidates marked with "*". Returns the exit code.
func PrintList(w io.Writer) int {
	ports, err := listPorts()
	if err != nil {
		fmt.Fprintf(w, "Cannot list serial ports: %v\n", err)
	}
	link := symlink(runtime.GOOS)
	if len(ports) == 0 && link == "" {
		fmt.Fprintln(w, "No serial ports found.")
		return 1
	}
	ranked := Rank(ports, runtime.GOOS)
	fmt.Fprintln(w, "Serial ports (* = candidate, best first):")
	isCand := map[string]bool{}
	for _, p := range ranked {
		isCand[p.Device] = true
		fmt.Fprintln(w, "  * "+Describe(p))
	}
	for _, p := range ports {
		if !isCand[p.Device] {
			fmt.Fprintln(w, "    "+Describe(p))
		}
	}
	if link != "" {
		fmt.Fprintf(w, "  %s -> %s (udev symlink, preferred)\n", link, realpath(link))
	}
	return 0
}
