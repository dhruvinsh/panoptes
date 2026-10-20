package collectors

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func prettyOSName() string {
	v := windows.RtlGetVersion()
	if v.ProductType != 1 { // VER_NT_WORKSTATION
		return "Windows Server"
	}
	if v.MajorVersion == 10 && v.BuildNumber >= 22000 {
		return "Windows 11"
	}
	if v.MajorVersion == 10 {
		return "Windows 10"
	}
	return fmt.Sprintf("Windows %d.%d", v.MajorVersion, v.MinorVersion)
}

func kernelVersion() string {
	v := windows.RtlGetVersion()
	return fmt.Sprintf("%d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber) // e.g. 10.0.22631
}
