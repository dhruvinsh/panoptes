package service

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/dhruvinsh/panoptes/internal/config"
)

// Windows: panoptes is a console program (so --list, --dry-run etc. print normally).
// A console program started at logon would open a console window, so the task runs it
// through `conhost.exe --headless` (Windows 10 1809+), which gives it an invisible
// console. Logs go to %AppData%\panoptes\panoptes.log (the service adds --log-file).
// The task is registered from XML so it has no 3-day execution time limit, keeps
// running on battery and restarts on failure.

// WinQuote quotes one argument for CommandLineToArgvW.
func WinQuote(arg string) string {
	if arg != "" && !strings.ContainsAny(arg, " \t\n\v\"") {
		return arg
	}
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0
	for _, c := range arg {
		switch c {
		case '\\':
			slashes++
		case '"':
			b.WriteString(strings.Repeat(`\`, 2*slashes+1))
			b.WriteRune('"')
			slashes = 0
			continue
		default:
			if slashes > 0 {
				b.WriteString(strings.Repeat(`\`, slashes))
			}
			slashes = 0
			b.WriteRune(c)
			continue
		}
	}
	b.WriteString(strings.Repeat(`\`, 2*slashes))
	b.WriteByte('"')
	return b.String()
}

// WinCommandLine joins arguments for a Windows command line.
func WinCommandLine(args []string) string {
	q := make([]string, len(args))
	for i, a := range args {
		q[i] = WinQuote(a)
	}
	return strings.Join(q, " ")
}

func windowsLogFile() string { return filepath.Join(config.Dir(), "panoptes.log") }

// withLogFile adds --log-file unless the user passed one (a headless console has no visible stderr).
func withLogFile(args []string) []string {
	for _, a := range args {
		if a == "--log-file" || strings.HasPrefix(a, "--log-file=") {
			return args
		}
	}
	return append(append([]string(nil), args...), "--log-file", windowsLogFile())
}

func conhostPath() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	p := filepath.Join(root, "System32", "conhost.exe")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// windowsLaunch returns (program, arguments) for the task action.
func windowsLaunch(args []string, conhost string) (string, string) {
	cmd := AgentCommand(withLogFile(args))
	if conhost == "" {
		return cmd[0], WinCommandLine(cmd[1:])
	}
	return conhost, "--headless " + WinCommandLine(cmd)
}

// TaskXML renders the Task Scheduler definition (user may be "" = current user).
func TaskXML(userID, command, arguments string) string {
	uid := ""
	if userID != "" {
		uid = "<UserId>" + xmlEscape(userID) + "</UserId>"
	}
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>Panoptes - streams PC stats to the Cheap Yellow Display over USB serial.</Description>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
      ` + uid + `
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      ` + uid + `
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>999</Count>
    </RestartOnFailure>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>` + xmlEscape(command) + `</Command>
      <Arguments>` + xmlEscape(arguments) + `</Arguments>
    </Exec>
  </Actions>
</Task>
`
}

// utf16LE encodes s as UTF-16LE with a BOM (what schtasks /XML expects).
func utf16LE(s string) []byte {
	u := utf16.Encode([]rune(strings.ReplaceAll(s, "\n", "\r\n")))
	out := make([]byte, 2, 2+2*len(u))
	out[0], out[1] = 0xFF, 0xFE
	for _, c := range u {
		out = append(out, byte(c), byte(c>>8))
	}
	return out
}

func startupCmdPath() string {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		home, _ := os.UserHomeDir()
		appdata = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appdata, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "Panoptes.cmd")
}

// StartupCmd renders the Startup-folder fallback launcher (% doubled for cmd.exe).
func StartupCmd(program, arguments string) string {
	line := `start "" ` + WinQuote(program) + " " + arguments
	return "@echo off\r\n" + strings.ReplaceAll(line, "%", "%%") + "\r\n"
}

func installWindows(args []string) int {
	program, arguments := windowsLaunch(args, conhostPath())
	say("Service command: %s %s", program, arguments)
	userID := ""
	if u, err := user.Current(); err == nil {
		userID = u.Username
	}
	xmlFile, err := os.CreateTemp("", "panoptes-task-*.xml")
	if err == nil {
		_, err = xmlFile.Write(utf16LE(TaskXML(userID, program, arguments)))
		xmlFile.Close()
		defer os.Remove(xmlFile.Name())
	}
	say("Creating a Task Scheduler entry that runs at logon:")
	if err == nil && run("schtasks", "/Create", "/F", "/TN", WindowsTask, "/XML", xmlFile.Name()) {
		run("schtasks", "/Run", "/TN", WindowsTask)
		say("Installed scheduled task '%s'. Logs: %s", WindowsTask, windowsLogFile())
		say("Remove with `panoptes uninstall-service`.")
		return 0
	}
	// Logon tasks can require elevation on some systems; fall back to the Startup folder.
	path := startupCmdPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		say("Cannot create %s: %v", filepath.Dir(path), err)
		return 1
	}
	if err := os.WriteFile(path, []byte(StartupCmd(program, arguments)), 0o644); err != nil {
		say("Cannot write %s: %v", path, err)
		return 1
	}
	say("schtasks failed (may need an elevated prompt); wrote Startup-folder launcher %s instead.", path)
	cmd := exec.Command(program)
	setRawCmdLine(cmd, WinQuote(program)+" "+arguments)
	if err := cmd.Start(); err != nil {
		say("Could not start it now: %v", err)
	} else {
		_ = cmd.Process.Release()
		say("Started panoptes in the background. Logs: %s", windowsLogFile())
	}
	return 0
}

func uninstallWindows() int {
	run("schtasks", "/End", "/TN", WindowsTask)
	run("schtasks", "/Delete", "/TN", WindowsTask, "/F")
	path := startupCmdPath()
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err == nil {
			say("Removed %s", path)
		}
	}
	// stop a still-running agent (but not this process)
	run("taskkill", "/F", "/IM", filepath.Base(Executable()), "/FI", "PID ne "+strconv.Itoa(os.Getpid()))
	return 0
}
