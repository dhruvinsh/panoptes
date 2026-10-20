//go:build !windows

package gpu

import "os/exec"

func hideWindow(*exec.Cmd) {}
