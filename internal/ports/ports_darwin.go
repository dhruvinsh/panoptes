package ports

import (
	"context"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
)

// osPorts on macOS works without cgo (release binaries are built with CGO_ENABLED=0, and
// go.bug.st/serial/enumerator needs cgo/IOKit there): the device nodes come from
// globbing /dev/cu.*, VID/PID/product come from parsing `ioreg` output (see ioreg.go).
// If ioreg is unavailable or fails, the name hints (cu.usbserial*, cu.wchusbserial*,
// cu.SLAB_USBtoUART*) still make the board a candidate.
func osPorts() ([]Port, error) {
	usb := map[string]Port{}
	for _, class := range []string{"IOUSBHostDevice", "IOUSBDevice"} {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		out, err := exec.CommandContext(ctx, "/usr/sbin/ioreg", "-r", "-c", class, "-l", "-w0").Output()
		cancel()
		if err == nil {
			for dev, p := range ParseIoreg(string(out)) {
				if _, ok := usb[dev]; !ok {
					usb[dev] = p
				}
			}
		}
		if len(usb) > 0 {
			break
		}
	}
	devs, _ := filepath.Glob("/dev/cu.*")
	seen := map[string]bool{}
	var res []Port
	for _, d := range devs {
		seen[d] = true
		if p, ok := usb[d]; ok {
			res = append(res, p)
		} else {
			res = append(res, Port{Device: d, Description: "n/a", HWID: "n/a"})
		}
	}
	for d, p := range usb { // e.g. ioreg knows a node the glob missed
		if !seen[d] {
			res = append(res, p)
		}
	}
	sort.SliceStable(res, func(i, j int) bool { return res[i].Device < res[j].Device })
	return res, nil
}
