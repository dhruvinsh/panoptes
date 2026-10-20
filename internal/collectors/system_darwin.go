package collectors

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func prettyOSName() string {
	if v, err := unix.Sysctl("kern.osproductversion"); err == nil && v != "" {
		return "macOS " + v
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "sw_vers", "-productVersion").Output(); err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			return "macOS " + v
		}
	}
	return "macOS"
}

// kernelVersion is the Darwin release (e.g. 24.1.0), like Python's platform.release().
func kernelVersion() string {
	var u unix.Utsname
	if err := unix.Uname(&u); err != nil {
		return ""
	}
	return unix.ByteSliceToString(u.Release[:])
}
