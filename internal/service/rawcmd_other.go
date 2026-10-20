//go:build !windows

package service

import "os/exec"

func setRawCmdLine(*exec.Cmd, string) {}
