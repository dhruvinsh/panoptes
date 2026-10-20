package service

import (
	"encoding/xml"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestSystemdQuote(t *testing.T) {
	for in, want := range map[string]string{
		"/usr/bin/panoptes": "/usr/bin/panoptes",
		"--port":            "--port",
		"/home/me/my dir/x": `"/home/me/my dir/x"`,
		`a"b`:               `"a\"b"`,
		"50%":               "50%%",
		"":                  `""`,
	} {
		if got := systemdQuote(in); got != want {
			t.Errorf("systemdQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSystemdUnit(t *testing.T) {
	u := SystemdUnitText([]string{"--port", "/dev/panoptes", "--bright", "120"})
	if !strings.Contains(u, "ExecStart="+systemdQuote(Executable())+" --port /dev/panoptes --bright 120\n") ||
		!strings.Contains(u, "Restart=always") || !strings.Contains(u, "WantedBy=default.target") {
		t.Fatalf("unit:\n%s", u)
	}
}

func TestLaunchdPlistIsValidXML(t *testing.T) {
	p := LaunchdPlist([]string{"--theme", "nord", "--port", "/dev/cu.a&b"})
	if !strings.Contains(p, "<string>"+LaunchdLabel+"</string>") || !strings.Contains(p, "<string>/dev/cu.a&amp;b</string>") {
		t.Fatalf("plist:\n%s", p)
	}
	var v any
	if err := xml.Unmarshal([]byte(p), &v); err != nil {
		t.Fatalf("invalid XML: %v", err)
	}
}

func TestWinQuote(t *testing.T) {
	for in, want := range map[string]string{
		`C:\panoptes.exe`:                 `C:\panoptes.exe`,
		`C:\Program Files\p\panoptes.exe`: `"C:\Program Files\p\panoptes.exe"`,
		`C:\dir with space\`:              `"C:\dir with space\\"`,
		`say "hi"`:                        `"say \"hi\""`,
		"":                                `""`,
	} {
		if got := WinQuote(in); got != want {
			t.Errorf("WinQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestWindowsLaunch(t *testing.T) {
	prog, args := windowsLaunch([]string{"--port", "COM5"}, `C:\Windows\System32\conhost.exe`)
	if prog != `C:\Windows\System32\conhost.exe` || !strings.HasPrefix(args, "--headless ") ||
		!strings.Contains(args, " --port COM5 --log-file ") {
		t.Fatalf("got %s %s", prog, args)
	}
	prog, args = windowsLaunch([]string{"--log-file", "x.log"}, "")
	if prog != Executable() || args != "--log-file x.log" {
		t.Fatalf("got %s %s", prog, args)
	}
}

func TestTaskXML(t *testing.T) {
	x := TaskXML(`PC\me`, `C:\Windows\System32\conhost.exe`, `--headless "C:\a b\panoptes.exe" --theme nord`)
	for _, want := range []string{"<UserId>PC\\me</UserId>", "<ExecutionTimeLimit>PT0S</ExecutionTimeLimit>",
		"<DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>", "&quot;C:\\a b\\panoptes.exe&quot;"} {
		if !strings.Contains(x, want) {
			t.Errorf("missing %q", want)
		}
	}
	// the XML declaration says UTF-16; parse after stripping it
	body := x[strings.Index(x, "?>")+2:]
	var v any
	if err := xml.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("invalid XML: %v", err)
	}
	b := utf16LE("a\nb")
	if b[0] != 0xFF || b[1] != 0xFE {
		t.Fatal("no BOM")
	}
	u := make([]uint16, 0)
	for i := 2; i < len(b); i += 2 {
		u = append(u, uint16(b[i])|uint16(b[i+1])<<8)
	}
	if string(utf16.Decode(u)) != "a\r\nb" {
		t.Fatal("utf16 roundtrip")
	}
}

func TestStartupCmd(t *testing.T) {
	s := StartupCmd(`C:\Windows\System32\conhost.exe`, `--headless C:\p.exe --log-file "C:\100%\x.log"`)
	if s != "@echo off\r\nstart \"\" C:\\Windows\\System32\\conhost.exe --headless C:\\p.exe --log-file \"C:\\100%%\\x.log\"\r\n" {
		t.Fatalf("got %q", s)
	}
}
