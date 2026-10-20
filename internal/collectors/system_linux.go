package collectors

import (
	"os"

	"golang.org/x/sys/unix"
)

func prettyOSName() string {
	for _, p := range []string{"/etc/os-release", "/usr/lib/os-release"} {
		if b, err := os.ReadFile(p); err == nil {
			if n := parseOSRelease(string(b)); n != "" {
				return n
			}
			break
		}
	}
	return "Linux"
}

func kernelVersion() string { return unameRelease() }

func unameRelease() string {
	var u unix.Utsname
	if err := unix.Uname(&u); err != nil {
		return ""
	}
	return unix.ByteSliceToString(u.Release[:])
}
