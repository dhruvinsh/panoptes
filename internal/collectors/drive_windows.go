package collectors

import (
	"strings"

	"golang.org/x/sys/windows"
)

// windowsLocalDrive keeps fixed and removable drives (skips network shares, CD-ROMs, RAM disks).
func windowsLocalDrive(mp string) bool {
	root := strings.TrimRight(mp, `\`) + `\`
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return false
	}
	switch windows.GetDriveType(p) {
	case windows.DRIVE_FIXED, windows.DRIVE_REMOVABLE:
		return true
	}
	return false
}
