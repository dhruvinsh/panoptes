// Package service installs/uninstalls panoptes as a per-user background service
// (no root needed): a systemd --user unit on Linux, a launchd LaunchAgent on macOS and a
// Task Scheduler logon task on Windows (Startup-folder fallback).
package service

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Names.
const (
	SystemdUnit  = "panoptes.service"
	LaunchdLabel = "com.dhruvinsh.panoptes"
	WindowsTask  = "Panoptes"
)

// Out receives the human-readable progress output.
var Out io.Writer = os.Stdout

func say(format string, a ...any) { fmt.Fprintf(Out, format+"\n", a...) }

// Executable is the absolute path of the running binary, symlinks resolved.
func Executable() string {
	exe, err := os.Executable()
	if err != nil {
		return "panoptes"
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	if a, err := filepath.Abs(exe); err == nil {
		exe = a
	}
	return exe
}

// AgentCommand is the absolute command line that runs this same binary with args.
func AgentCommand(args []string) []string {
	return append([]string{Executable()}, args...)
}

func looksTemporary(exe string) bool {
	tmp := os.TempDir()
	return strings.Contains(exe, string(filepath.Separator)+"go-build") ||
		(tmp != "" && strings.HasPrefix(exe, tmp+string(filepath.Separator)))
}

// run executes a command, echoing it and its output. Returns success.
func run(name string, args ...string) bool {
	say("  $ %s", strings.Join(append([]string{name}, args...), " "))
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if s := strings.TrimSpace(string(out)); s != "" {
		say("    %s", strings.ReplaceAll(s, "\n", "\n    "))
	}
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			say("    failed: %v", err)
		}
		return false
	}
	return true
}

func have(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// Install writes and starts the service for this OS. args are baked into the command.
func Install(args []string) int {
	exe := Executable()
	if looksTemporary(exe) {
		say("Warning: %s looks like a temporary build (go run?). Install the binary first "+
			"(e.g. `go install` or a release download) so the service keeps working.", exe)
	}
	switch runtime.GOOS {
	case "linux":
		return installLinux(args)
	case "darwin":
		return installMac(args)
	case "windows":
		return installWindows(args)
	}
	say("Unsupported platform %s", runtime.GOOS)
	return 1
}

// Uninstall stops and removes the service for this OS.
func Uninstall() int {
	switch runtime.GOOS {
	case "linux":
		return uninstallLinux()
	case "darwin":
		return uninstallMac()
	case "windows":
		return uninstallWindows()
	}
	say("Unsupported platform %s", runtime.GOOS)
	return 1
}

// ------------------------------------------------------------------ Linux

// systemdQuote quotes one ExecStart argument (and escapes systemd's % and $ specifiers).
func systemdQuote(arg string) string {
	arg = strings.ReplaceAll(arg, "%", "%%")
	arg = strings.ReplaceAll(arg, "$", "$$")
	safe := arg != ""
	for _, c := range arg {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("/._-=:+,@%$", c)) {
			safe = false
			break
		}
	}
	if safe {
		return arg
	}
	return `"` + strings.ReplaceAll(strings.ReplaceAll(arg, `\`, `\\`), `"`, `\"`) + `"`
}

// SystemdUnit renders the unit file.
func SystemdUnitText(args []string) string {
	parts := []string{}
	for _, a := range AgentCommand(args) {
		parts = append(parts, systemdQuote(a))
	}
	return "[Unit]\n" +
		"Description=Panoptes - PC stats for the Cheap Yellow Display\n" +
		"\n" +
		"[Service]\n" +
		"Type=simple\n" +
		"ExecStart=" + strings.Join(parts, " ") + "\n" +
		"Restart=always\n" +
		"RestartSec=5\n" +
		"\n" +
		"[Install]\n" +
		"WantedBy=default.target\n"
}

func systemdPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "systemd", "user", SystemdUnit)
}

func installLinux(args []string) int {
	path := systemdPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		say("Cannot create %s: %v", filepath.Dir(path), err)
		return 1
	}
	if err := os.WriteFile(path, []byte(SystemdUnitText(args)), 0o644); err != nil {
		say("Cannot write %s: %v", path, err)
		return 1
	}
	say("Wrote systemd user unit %s", path)
	if have("systemctl") {
		run("systemctl", "--user", "daemon-reload")
		if run("systemctl", "--user", "enable", "--now", SystemdUnit) {
			say("Started. Logs: journalctl --user -u panoptes -f")
		} else {
			say("Could not enable it automatically; run:\n  systemctl --user daemon-reload && systemctl --user enable --now %s", SystemdUnit)
		}
	} else {
		say("systemctl not found; start `%s` from your session autostart instead.", strings.Join(AgentCommand(args), " "))
	}
	user := os.Getenv("USER")
	if user == "" {
		user = "$USER"
	}
	say("Optional: `loginctl enable-linger %s` keeps it running without an active login session "+
		"(note: with the udev uaccess rule, port access is granted to the logged-in seat user).", user)
	return 0
}

func uninstallLinux() int {
	path := systemdPath()
	if have("systemctl") {
		run("systemctl", "--user", "disable", "--now", SystemdUnit)
	}
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			say("Cannot remove %s: %v", path, err)
			return 1
		}
		say("Removed %s", path)
		if have("systemctl") {
			run("systemctl", "--user", "daemon-reload")
		}
	} else {
		say("No unit at %s", path)
	}
	return 0
}

// ------------------------------------------------------------------ macOS

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", LaunchdLabel+".plist")
}

func macLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Logs", "panoptes.log")
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// LaunchdPlist renders the LaunchAgent plist.
func LaunchdPlist(args []string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + LaunchdLabel + `</string>
	<key>ProgramArguments</key>
	<array>
`)
	for _, a := range AgentCommand(args) {
		b.WriteString("\t\t<string>" + xmlEscape(a) + "</string>\n")
	}
	log := xmlEscape(macLogPath())
	b.WriteString(`	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>ThrottleInterval</key>
	<integer>5</integer>
	<key>ProcessType</key>
	<string>Background</string>
	<key>StandardOutPath</key>
	<string>` + log + `</string>
	<key>StandardErrorPath</key>
	<string>` + log + `</string>
</dict>
</plist>
`)
	return b.String()
}

func installMac(args []string) int {
	path := plistPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		say("Cannot create %s: %v", filepath.Dir(path), err)
		return 1
	}
	_ = os.MkdirAll(filepath.Dir(macLogPath()), 0o755)
	if err := os.WriteFile(path, []byte(LaunchdPlist(args)), 0o644); err != nil {
		say("Cannot write %s: %v", path, err)
		return 1
	}
	say("Wrote LaunchAgent %s", path)
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	run("launchctl", "bootout", domain, path) // ignore failure: not loaded yet
	if !run("launchctl", "bootstrap", domain, path) {
		run("launchctl", "load", "-w", path)
	}
	say("Logs: ~/Library/Logs/panoptes.log")
	return 0
}

func uninstallMac() int {
	path := plistPath()
	if _, err := os.Stat(path); err != nil {
		say("No LaunchAgent at %s", path)
		return 0
	}
	if !run("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), path) {
		run("launchctl", "unload", "-w", path)
	}
	if err := os.Remove(path); err != nil {
		say("Cannot remove %s: %v", path, err)
		return 1
	}
	say("Removed %s", path)
	return 0
}
