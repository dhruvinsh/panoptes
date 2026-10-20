package service

import (
	"os/exec"
	"syscall"
)

const (
	detachedProcess       = 0x00000008
	createNewProcessGroup = 0x00000200
)

// setRawCmdLine passes an exact command line and detaches the child from our console.
func setRawCmdLine(cmd *exec.Cmd, line string) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: line, CreationFlags: detachedProcess | createNewProcessGroup}
}
