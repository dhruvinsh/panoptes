package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dhruvinsh/panoptes/internal/config"
	"github.com/dhruvinsh/panoptes/internal/frame"
	"github.com/dhruvinsh/panoptes/internal/logx"
	"github.com/dhruvinsh/panoptes/internal/pause"
)

func mustParse(t *testing.T, args ...string) *Parsed {
	t.Helper()
	p, err := Parse(args)
	if err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return p
}

func cfgJSON(opts config.Values) string {
	c := config.CfgMessage(opts)
	if c == nil {
		return "<nil>"
	}
	return strings.TrimSpace(string(frame.Encode(c)))
}

func TestCfgPassthroughAndMerge(t *testing.T) {
	p := mustParse(t, "--bright", "120", "--no-led", "--rotate", "1", "--alert-cpu", "90")
	opts := config.Merge(p.Values, config.Values{"bright": int64(10), "beep": true, "interval": 2.0})
	if opts.Int("bright", 0) != 120 || !opts.Bool("beep") || opts.Float("interval", 0) != 2.0 || opts.Int("baud", 0) != 115200 {
		t.Fatalf("merge: %v", opts)
	}
	if got := cfgJSON(opts); got != `{"t":"cfg","bright":120,"led":false,"beep":true,"rotate":1,"alert_cpu":90.0}` {
		t.Fatalf("cfg: %s", got)
	}
	if got := p.Passthrough(); !reflect.DeepEqual(got, []string{"--bright", "120", "--no-led", "--rotate", "1", "--alert-cpu", "90.0"}) {
		t.Fatalf("passthrough: %v", got)
	}
}

func TestNoCfgWhenNothingSet(t *testing.T) {
	p := mustParse(t)
	if got := cfgJSON(config.Merge(p.Values, config.Values{})); got != "<nil>" {
		t.Fatalf("got %s", got)
	}
	if p.Command != "run" {
		t.Fatalf("command %q", p.Command)
	}
}

func TestBoolPairLastWins(t *testing.T) {
	p := mustParse(t, "--led", "--no-led", "--no-beep", "--beep", "--auto-bright")
	if got := cfgJSON(config.Merge(p.Values, nil)); got != `{"t":"cfg","auto_bright":true,"led":false,"beep":true}` {
		t.Fatalf("got %s", got)
	}
}

func TestSubcommandAnywhereAndPassthroughOrder(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "c.toml")
	p := mustParse(t, "--theme", "nord", "install-service", "-p", "/dev/panoptes", "-v", "--demo",
		"--config", cfgPath, "-i", "0.5", "-b", "9600")
	if p.Command != "install-service" {
		t.Fatalf("command %q", p.Command)
	}
	want := []string{"--port", "/dev/panoptes", "--baud", "9600", "--interval", "0.5", "--config", cfgPath,
		"--theme", "nord", "--verbose", "--demo"}
	if got := p.Passthrough(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestDryRunOnceDemo(t *testing.T) {
	var out, errb bytes.Buffer
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // no user config file
	code := Main("test", []string{"--dry-run", "--once", "--demo", "--interval", "0.1", "--bright", "50"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	var types []string
	for _, l := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("bad line %q", l)
		}
		types = append(types, m["t"].(string))
		if len(l)+1 > frame.MaxLine {
			t.Fatalf("line too long: %d", len(l)+1)
		}
	}
	if !reflect.DeepEqual(types, []string{"hello", "cfg", "s"}) {
		t.Fatalf("types %v", types)
	}
	if lines[0] != `{"t":"hello","v":1,"agent":"panoptes test"}` || lines[1] != `{"t":"cfg","bright":50}` {
		t.Fatalf("got %v", lines[:2])
	}
}

func TestVersionAndUsageErrors(t *testing.T) {
	var out, errb bytes.Buffer
	if Main("v1.2.3", []string{"--version"}, &out, &errb) != 0 || out.String() != "panoptes v1.2.3\n" {
		t.Fatalf("version: %q", out.String())
	}
	for _, args := range [][]string{{"--theme", "monokai"}, {"--theme-mode", "dim"}, {"--bright", "300"},
		{"--rotate", "2"}, {"--interval", "0"}, {"bogus"}, {"--nope"}, {"run", "extra"}} {
		errb.Reset()
		if code := Main("x", args, &out, &errb); code != 2 {
			t.Errorf("%v: exit %d", args, code)
		}
		if !strings.Contains(errb.String(), "error:") {
			t.Errorf("%v: stderr %q", args, errb.String())
		}
	}
}

func TestLoadConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte("port = \"/dev/cyd\"\nauto-bright = false\nbogus = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := config.Load(p, true); !reflect.DeepEqual(got, config.Values{"port": "/dev/cyd", "auto_bright": false}) {
		t.Fatalf("got %v", got)
	}
	if got := config.Parse("theme = \"catppuccin\"\ntheme-mode = \"auto\"\n", "x"); !reflect.DeepEqual(got,
		config.Values{"theme": "catppuccin", "theme_mode": "auto"}) {
		t.Fatalf("got %v", got)
	}
	if got := config.Parse("[panoptes]\nbright = 5\n", "x"); !reflect.DeepEqual(got, config.Values{"bright": int64(5)}) {
		t.Fatalf("table: %v", got)
	}
	if got := config.Parse("this is = not toml [", "x"); len(got) != 0 {
		t.Fatalf("invalid: %v", got)
	}
	if got := config.Load(filepath.Join(t.TempDir(), "missing.toml"), true); len(got) != 0 {
		t.Fatalf("missing: %v", got)
	}
}

func TestThemeOptions(t *testing.T) {
	p := mustParse(t, "--theme", "nord", "--theme-mode", "auto")
	opts := config.Merge(p.Values, config.Values{"theme": "gruvbox", "theme_mode": "light"})
	if got := cfgJSON(opts); got != `{"t":"cfg","theme":"nord","theme_mode":"auto"}` {
		t.Fatalf("got %s", got)
	}
	if got := p.Passthrough(); !reflect.DeepEqual(got, []string{"--theme", "nord", "--theme-mode", "auto"}) {
		t.Fatalf("got %v", got)
	}
}

func TestThemeFromConfigFileValidated(t *testing.T) {
	var logs []string
	defer logx.SetHook(func(_ logx.Level, m string) { logs = append(logs, m) })()
	p := mustParse(t)
	if got := cfgJSON(config.Merge(p.Values, config.Values{"theme": "Dracula", "theme_mode": "AUTO"})); got != `{"t":"cfg","theme":"dracula","theme_mode":"auto"}` {
		t.Fatalf("got %s", got)
	}
	if got := cfgJSON(config.Merge(p.Values, config.Values{"theme": "monokai", "bright": int64(5)})); got != `{"t":"cfg","bright":5}` {
		t.Fatalf("got %s", got)
	}
	if !strings.Contains(strings.Join(logs, "\n"), "monokai") {
		t.Fatalf("no warning: %v", logs)
	}
	if got := cfgJSON(config.Merge(p.Values, config.Values{"theme_mode": "dim"})); got != "<nil>" {
		t.Fatalf("got %s", got)
	}
	// ints from the file stay ints, floats from the file stay floats
	if got := cfgJSON(config.Merge(p.Values, config.Values{"alert_temp": int64(90), "alert_cpu": 95.5})); got != `{"t":"cfg","alert_temp":90,"alert_cpu":95.5}` {
		t.Fatalf("got %s", got)
	}
}

// Every option in the registry that isn't file-only must have a command-line flag
// (so --help and the config file stay in step).
func TestEveryNonFileOptionHasAFlag(t *testing.T) {
	fs := newFlagSet(&Parsed{}, map[string]**bool{})
	for _, o := range config.Options {
		if o.Kind == config.KindFile {
			if fs.Lookup(strings.ReplaceAll(o.Key, "_", "-")) != nil {
				t.Errorf("%s is file-only but has a flag", o.Key)
			}
			continue
		}
		if fs.Lookup(strings.ReplaceAll(o.Key, "_", "-")) == nil {
			t.Errorf("option %s has no flag", o.Key)
		}
	}
}

// A run (manual or as a service, which runs plain `panoptes`) takes its theme from the
// config file in the user's config dir, and a flag overrides it.
func TestThemeFromConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "panoptes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "panoptes", "config.toml"),
		[]byte("theme = \"catppuccin\"\ntheme_mode = \"auto\"\nbright = 90\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgLine := func(args ...string) string {
		var out, errb bytes.Buffer
		if code := Main("test", append([]string{"--dry-run", "--once", "--demo", "--interval", "0.1"}, args...), &out, &errb); code != 0 {
			t.Fatalf("exit %d: %s", code, errb.String())
		}
		return strings.Split(out.String(), "\n")[1]
	}
	if got := cfgLine(); got != `{"t":"cfg","bright":90,"theme":"catppuccin","theme_mode":"auto"}` {
		t.Fatalf("got %s", got)
	}
	if got := cfgLine("--theme", "nord"); got != `{"t":"cfg","bright":90,"theme":"nord","theme_mode":"auto"}` {
		t.Fatalf("got %s", got)
	}
}

func TestCollectorOptionsFromConfig(t *testing.T) {
	var logs []string
	defer logx.SetHook(func(l logx.Level, m string) {
		if l >= logx.WARN {
			logs = append(logs, m)
		}
	})()
	f := config.Parse("disable = [\"GPU\", \"bogus\"]\nnet_interface = \"eth0\"\nmounts = [\"/\"]\n", "x")
	o := collectorOptions(f)
	if !reflect.DeepEqual(o.Disable, []string{"gpu"}) || o.NetIface != "eth0" || !reflect.DeepEqual(o.Mounts, []string{"/"}) {
		t.Fatalf("got %+v", o)
	}
	if len(logs) != 1 || !strings.Contains(logs[0], "bogus") {
		t.Fatalf("expected one warning about bogus: %v", logs)
	}
}

func TestBrightnessOptions(t *testing.T) {
	p := mustParse(t, "--led-bright", "40", "--night-bright", "0", "--auto-bright-min", "25")
	got := cfgJSON(config.Merge(p.Values, config.Values{"led_bright": int64(10), "bright": int64(120)}))
	if got != `{"t":"cfg","bright":120,"auto_bright_min":25,"night_bright":0,"led_bright":40}` {
		t.Fatalf("got %s", got)
	}
	if got := p.Passthrough(); !reflect.DeepEqual(got, []string{"--auto-bright-min", "25", "--night-bright", "0", "--led-bright", "40"}) {
		t.Fatalf("passthrough: %v", got)
	}
	for _, flag := range []string{"--led-bright", "--night-bright", "--auto-bright-min"} {
		for _, v := range []string{"256", "-1"} {
			if _, err := Parse([]string{flag + "=" + v}); err == nil {
				t.Errorf("%s=%s should be rejected", flag, v)
			}
		}
	}
}

func TestPauseArgs(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want time.Duration
	}{
		{[]string{"pause"}, 10 * time.Minute},
		{[]string{"pause", "90s"}, 90 * time.Second},
		{[]string{"-v", "pause", "2"}, 2 * time.Minute},
	} {
		p := mustParse(t, tc.args...)
		if p.Command != "pause" || p.For != tc.want {
			t.Fatalf("%v: got %q %v, want pause %v", tc.args, p.Command, p.For, tc.want)
		}
	}
	for _, args := range [][]string{{"pause", "soon"}, {"pause", "-5m"}, {"pause", "5m", "x"}, {"resume", "5m"}, {"run", "5m"}} {
		if _, err := Parse(args); err == nil {
			t.Fatalf("%v: expected an error", args)
		}
	}
}

func TestPauseResumeCommands(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out, errb bytes.Buffer
	if code := Main("test", []string{"pause", "5m"}, &out, &errb); code != 0 {
		t.Fatalf("pause: exit %d: %s", code, errb.String())
	}
	if until := pause.Until(); time.Until(until) < 4*time.Minute {
		t.Fatalf("pause file says %v", until)
	}
	if code := Main("test", []string{"resume"}, &out, &errb); code != 0 {
		t.Fatalf("resume: exit %d: %s", code, errb.String())
	}
	if until := pause.Until(); !until.IsZero() {
		t.Fatalf("still paused until %v", until)
	}
}
