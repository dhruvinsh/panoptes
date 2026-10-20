//go:build !windows

package collectors

func windowsLocalDrive(string) bool { return true }
