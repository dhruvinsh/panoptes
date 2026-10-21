package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/dhruvinsh/panoptes/internal/logx"
)

func TestDirFor(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	never := func(string) bool { return false }
	always := func(string) bool { return true }
	j := filepath.Join
	x := j(os.TempDir(), "x") // absolute on every OS ("/x" has no drive letter on Windows)
	cases := []struct {
		name  string
		goos  string
		env   map[string]string
		isDir func(string) bool
		want  string
	}{
		{"linux default", "linux", nil, never, j("/h", ".config", "panoptes")},
		{"linux xdg", "linux", map[string]string{"XDG_CONFIG_HOME": x}, never, j(x, "panoptes")},
		{"relative xdg ignored", "linux", map[string]string{"XDG_CONFIG_HOME": "rel"}, never, j("/h", ".config", "panoptes")},
		{"mac native", "darwin", nil, never, j("/h", "Library", "Application Support", "panoptes")},
		{"mac existing ~/.config", "darwin", nil, always, j("/h", ".config", "panoptes")},
		{"mac xdg", "darwin", map[string]string{"XDG_CONFIG_HOME": x}, never, j(x, "panoptes")},
		{"windows native", "windows", map[string]string{"AppData": `C:\Users\u\AppData\Roaming`}, never, j(`C:\Users\u\AppData\Roaming`, "panoptes")},
		{"windows existing ~/.config", "windows", map[string]string{"AppData": "A"}, always, j("/h", ".config", "panoptes")},
		{"windows no AppData", "windows", nil, never, j("/h", "AppData", "Roaming", "panoptes")},
	}
	for _, c := range cases {
		if got := dirFor(c.goos, env(c.env), "/h", c.isDir); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestUnknownKeysWarnAndNewOptionsLoad(t *testing.T) {
	var logs []string
	defer logx.SetHook(func(l logx.Level, m string) {
		if l >= logx.WARN {
			logs = append(logs, m)
		}
	})()
	got := Parse(`theme = "nord"
themee = "typo"
disable = ["gpu", "procs"]
mounts = ["/", "/home"]
net_interface = "enp5s0"
usb_ids = ["0403:6001"]
hello-interval = 3
`, "x")
	if got["theme"] != "nord" || got["net_interface"] != "enp5s0" || got["hello_interval"] != int64(3) {
		t.Fatalf("got %v", got)
	}
	if !reflect.DeepEqual(got.Strings("disable"), []string{"gpu", "procs"}) ||
		!reflect.DeepEqual(got.Strings("mounts"), []string{"/", "/home"}) ||
		!reflect.DeepEqual(got.Strings("usb_ids"), []string{"0403:6001"}) {
		t.Fatalf("lists: %v", got)
	}
	if len(logs) != 1 || !strings.Contains(logs[0], "themee") {
		t.Fatalf("expected one warning about the typo, got %v", logs)
	}
	logs = nil
	if (Values{"disable": "gpu"}).Strings("disable") != nil || len(logs) != 1 {
		t.Fatalf("a non-list should warn and be ignored: %v", logs)
	}
}

func TestRegistry(t *testing.T) {
	seen := map[string]bool{}
	for _, o := range Options {
		if seen[o.Key] {
			t.Errorf("duplicate option %s", o.Key)
		}
		seen[o.Key] = true
		if o.Doc == "" || o.Type == "" {
			t.Errorf("option %s needs a Doc and Type", o.Key)
		}
	}
	if !reflect.DeepEqual(CfgKeys, []string{"bright", "auto_bright", "auto_bright_min", "night_bright", "led",
		"led_bright", "beep", "rotate", "page", "cycle",
		"alert_temp", "alert_cpu", "theme", "theme_mode"}) {
		t.Errorf("CfgKeys order is the cfg message order: %v", CfgKeys)
	}
}

// The schema in schema/ is generated from Options; run
//
//	UPDATE_SCHEMA=1 go test ./internal/config -run Schema
//
// after changing them.
func TestSchemaUpToDate(t *testing.T) {
	path := filepath.Join("..", "..", "schema", ConfigSchemaFile)
	want := ConfigSchema()
	if !json.Valid(want) {
		t.Fatal("generated schema is not valid JSON")
	}
	if os.Getenv("UPDATE_SCHEMA") != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with UPDATE_SCHEMA=1)", err)
	}
	// a Windows checkout may have converted the line endings
	if strings.ReplaceAll(string(got), "\r\n", "\n") != string(want) {
		t.Fatalf("%s is stale: run UPDATE_SCHEMA=1 go test ./internal/config -run Schema", path)
	}
}

func TestSchemaCoversEveryOption(t *testing.T) {
	var s struct {
		Props map[string]struct {
			Type string `json:"type"`
			Enum []string
		} `json:"properties"`
		Additional bool `json:"additionalProperties"`
	}
	if err := json.Unmarshal(ConfigSchema(), &s); err != nil {
		t.Fatal(err)
	}
	for _, o := range Options {
		if _, ok := s.Props[o.Key]; !ok {
			t.Errorf("schema is missing %s", o.Key)
		}
	}
	if s.Additional {
		t.Error("the schema must reject unknown keys so editors flag typos")
	}
}

// config.Themes must list exactly the families the firmware has, in order.
func TestThemesMatchFirmware(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "firmware", "src", "ui", "theme.cpp"))
	if err != nil {
		t.Skip("firmware sources not available")
	}
	var ids []string
	for _, m := range regexp.MustCompile(`(?m)^\s*\{"(\w+)", "[^"]+",`).FindAllStringSubmatch(string(src), -1) {
		ids = append(ids, m[1])
	}
	if !reflect.DeepEqual(ids, Themes) {
		t.Fatalf("firmware themes %v != config.Themes %v", ids, Themes)
	}
}

// docs/config.example.toml must stay valid: uncommenting every `# key = value` line gives a
// config the agent accepts without warnings and that sets every option.
func TestExampleConfig(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "docs", "config.example.toml"))
	if err != nil {
		t.Fatal(err)
	}
	commented := regexp.MustCompile(`^#\s*([a-z_]+)\s*=`)
	var lines []string
	for _, l := range strings.Split(string(src), "\n") {
		if m := commented.FindStringSubmatch(l); m != nil && isKnown(m[1]) {
			l = strings.TrimSpace(strings.TrimPrefix(l, "#"))
		}
		lines = append(lines, l)
	}
	var warned []string
	defer logx.SetHook(func(l logx.Level, m string) {
		if l >= logx.WARN {
			warned = append(warned, m)
		}
	})()
	got := Parse(strings.Join(lines, "\n"), "example")
	if len(warned) > 0 {
		t.Fatalf("warnings: %v", warned)
	}
	for _, o := range Options {
		if _, ok := got[o.Key]; !ok {
			t.Errorf("example does not mention %s", o.Key)
		}
	}
	if !strings.HasPrefix(string(src), "#:schema "+SchemaBaseURL+ConfigSchemaFile) {
		t.Error("example should start with the #:schema line")
	}
}
