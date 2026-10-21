// Package cli is the panoptes command line: flag parsing, option layering and the
// run / dry-run / install-service / uninstall-service / screenshot commands.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/pflag"

	"github.com/dhruvinsh/panoptes/internal/collectors"
	"github.com/dhruvinsh/panoptes/internal/config"
	"github.com/dhruvinsh/panoptes/internal/demo"
	"github.com/dhruvinsh/panoptes/internal/frame"
	"github.com/dhruvinsh/panoptes/internal/link"
	"github.com/dhruvinsh/panoptes/internal/logx"
	"github.com/dhruvinsh/panoptes/internal/ports"
	"github.com/dhruvinsh/panoptes/internal/screenshot"
	"github.com/dhruvinsh/panoptes/internal/service"
)

// Commands are the positional subcommands.
var Commands = []string{"run", "install-service", "uninstall-service", "screenshot"}

const defaultShotFile = "panoptes-screenshot.png"

// triBool backs a --x / --no-x pair; the last one given wins (like argparse's
// BooleanOptionalAction).
type triBool struct {
	dst  **bool
	sets bool // value stored when the flag is given as "true"
}

func (t *triBool) String() string {
	if t.dst == nil || *t.dst == nil {
		return ""
	}
	return strconv.FormatBool(**t.dst)
}
func (t *triBool) Type() string { return "bool" }
func (t *triBool) Set(s string) error {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	v := b == t.sets
	*t.dst = &v
	return nil
}

// Parsed is the result of parsing a command line.
type Parsed struct {
	Command string
	Values  config.Values // only options given on the command line
	List    bool
	DryRun  bool
	Once    bool
	Demo    bool
	Version bool
	Help    bool
	Config  string // --config, "" if not given
	Out     string

	order []string // CLI keys in canonical order, for Passthrough
}

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func newFlagSet(p *Parsed, bools map[string]**bool) *pflag.FlagSet {
	fs := pflag.NewFlagSet("panoptes", pflag.ContinueOnError)
	fs.SortFlags = false
	fs.SetOutput(io.Discard)
	fs.StringP("port", "p", "", "serial port (default: auto-detect)")
	fs.IntP("baud", "b", config.DefaultBaud, "baud rate")
	fs.Float64P("interval", "i", config.DefaultInterval, "seconds between frames")
	fs.BoolVar(&p.List, "list", false, "list serial ports and exit")
	fs.BoolVar(&p.DryRun, "dry-run", false, "print frames to stdout instead of serial")
	fs.BoolVar(&p.Once, "once", false, "send/print a single frame and exit")
	fs.BoolVar(&p.Demo, "demo", false, "send synthetic animated data")
	fs.BoolP("verbose", "v", false, "debug logging")
	fs.String("log-file", "", "also log to this file")
	fs.StringVar(&p.Config, "config", "", "config file path (default "+config.DefaultPath()+")")
	fs.StringVarP(&p.Out, "out", "o", defaultShotFile, "output PNG for the screenshot command")
	fs.BoolVar(&p.Version, "version", false, "print the version and exit")
	fs.BoolVarP(&p.Help, "help", "h", false, "show this help")

	fs.Int("bright", 0, "backlight 0-255")
	fs.Int("auto-bright-min", 0, "lowest backlight auto-brightness uses, 0-255")
	fs.Int("night-bright", 0, "backlight in night mode, 0-255")
	fs.Int("led-bright", 0, "maximum status LED brightness, 0-255")
	for _, name := range []string{"auto-bright", "led", "beep"} {
		help := map[string]string{"auto-bright": "use the onboard light sensor", "led": "RGB status LED", "beep": "speaker alerts"}[name]
		var v *bool
		bools[name] = &v
		f := fs.VarPF(&triBool{dst: bools[name], sets: true}, name, "", help+" (--no-"+name+" to disable)")
		f.NoOptDefVal = "true"
		g := fs.VarPF(&triBool{dst: bools[name], sets: false}, "no-"+name, "", "")
		g.NoOptDefVal = "true"
		g.Hidden = true
	}
	fs.Int("rotate", 0, "0 landscape, 1 landscape flipped")
	fs.Int("page", 0, "force page index (-1 = leave alone)")
	fs.Int("cycle", 0, "auto-advance pages every N s (0 = off)")
	fs.Float64("alert-temp", 0, "alert threshold, °C")
	fs.Float64("alert-cpu", 0, "alert threshold, CPU %")
	fs.String("theme", "", "colour theme: "+strings.Join(config.Themes, ", "))
	fs.String("theme-mode", "", "dark, light, or auto (follow the room light via the LDR)")
	return fs
}

// Parse parses argv (without the program name).
func Parse(args []string) (*Parsed, error) {
	p := &Parsed{Values: config.Values{}}
	bools := map[string]**bool{}
	fs := newFlagSet(p, bools)
	if err := fs.Parse(args); err != nil {
		return nil, usageError{err.Error()}
	}
	switch pos := fs.Args(); len(pos) {
	case 0:
		p.Command = "run"
	case 1:
		p.Command = pos[0]
		if !contains(Commands, p.Command) {
			return nil, usageError{fmt.Sprintf("argument command: invalid choice: %q (choose from %s)",
				p.Command, strings.Join(Commands, ", "))}
		}
	default:
		return nil, usageError{"unrecognized arguments: " + strings.Join(pos[1:], " ")}
	}
	changed := func(name string) bool { return fs.Changed(name) }
	str := func(name string) string { v, _ := fs.GetString(name); return v }
	integer := func(name string) int { v, _ := fs.GetInt(name); return v }
	float := func(name string) float64 { v, _ := fs.GetFloat64(name); return v }

	for _, key := range append(append([]string{}, config.RunKeys...), config.CfgKeys...) {
		name := strings.ReplaceAll(key, "_", "-")
		switch key {
		case "auto_bright", "led", "beep":
			if v := *bools[name]; v != nil {
				p.Values[key] = *v
				p.order = append(p.order, key)
			}
			continue
		}
		if !changed(name) {
			continue
		}
		p.order = append(p.order, key)
		switch key {
		case "port", "log_file", "theme", "theme_mode":
			p.Values[key] = str(name)
		case "baud", "bright", "auto_bright_min", "night_bright", "led_bright", "rotate", "page", "cycle":
			p.Values[key] = integer(name)
		case "interval", "alert_temp", "alert_cpu":
			p.Values[key] = float(name)
		case "verbose":
			v, _ := fs.GetBool(name)
			p.Values[key] = v
		}
	}
	// validation (argparse type=/choices=)
	if v, ok := p.Values["interval"].(float64); ok && !(v > 0) {
		return nil, usageError{"argument -i/--interval: must be > 0"}
	}
	for _, key := range []string{"bright", "auto_bright_min", "night_bright", "led_bright"} {
		if v, ok := p.Values[key].(int); ok && (v < 0 || v > 255) {
			return nil, usageError{"argument --" + strings.ReplaceAll(key, "_", "-") + ": must be 0-255"}
		}
	}
	if v, ok := p.Values["rotate"].(int); ok && v != 0 && v != 1 {
		return nil, usageError{fmt.Sprintf("argument --rotate: invalid choice: %d (choose from 0, 1)", v)}
	}
	if v, ok := p.Values["theme"].(string); ok && !contains(config.Themes, v) {
		return nil, usageError{fmt.Sprintf("argument --theme: invalid choice: %q (choose from %s)", v, strings.Join(config.Themes, ", "))}
	}
	if v, ok := p.Values["theme_mode"].(string); ok && !contains(config.ThemeModes, v) {
		return nil, usageError{fmt.Sprintf("argument --theme-mode: invalid choice: %q (choose from %s)", v, strings.Join(config.ThemeModes, ", "))}
	}
	return p, nil
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func absPath(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

// Passthrough re-renders the options given on this command line for the service.
func (p *Parsed) Passthrough() []string {
	var out []string
	emit := func(key string, v any) {
		flag := "--" + strings.ReplaceAll(key, "_", "-")
		switch x := v.(type) {
		case bool:
			if x {
				out = append(out, flag)
			} else {
				out = append(out, "--no-"+strings.ReplaceAll(key, "_", "-"))
			}
		case float64:
			out = append(out, flag, config.PyFloat(x))
		case int:
			out = append(out, flag, strconv.Itoa(x))
		case string:
			out = append(out, flag, x)
		}
	}
	for _, key := range []string{"port", "baud", "interval", "log_file"} {
		if v, ok := p.Values[key]; ok {
			if key == "log_file" {
				v = absPath(v.(string))
			}
			emit(key, v)
		}
	}
	if p.Config != "" {
		out = append(out, "--config", absPath(p.Config))
	}
	for _, key := range config.CfgKeys {
		if v, ok := p.Values[key]; ok {
			emit(key, v)
		}
	}
	if v, _ := p.Values["verbose"].(bool); v {
		out = append(out, "--verbose")
	}
	if p.Demo {
		out = append(out, "--demo")
	}
	return out
}

// Usage text.
func Usage(w io.Writer) {
	p := &Parsed{}
	fs := newFlagSet(p, map[string]**bool{})
	fmt.Fprintf(w, "usage: panoptes [command] [options]\n\n"+
		"Stream PC stats to a Cheap Yellow Display (ESP32) over USB serial.\n\n"+
		"commands:\n"+
		"  run                 stream stats to the board (default)\n"+
		"  install-service     start at login (systemd --user / launchd / Task Scheduler); options are baked in\n"+
		"  uninstall-service   remove that service\n"+
		"  screenshot          save the board's screen as a PNG (-o FILE)\n\n"+
		"options:\n%s\n"+
		"Device options (--bright ... --theme-mode) are sent as a `cfg` message after every handshake.\n"+
		"Config file: %s (CLI options override it). It also takes options that have no flag;\n"+
		"schema for editors: %s%s\n", fs.FlagUsages(), config.DefaultPath(), config.SchemaBaseURL, config.ConfigSchemaFile)
}

// Main runs the CLI and returns the exit code.
func Main(version string, args []string, stdout, stderr io.Writer) int {
	link.Agent = "panoptes " + version
	p, err := Parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "usage: panoptes [command] [options]\npanoptes: error: %v\n(see panoptes --help)\n", err)
		return 2
	}
	if p.Help {
		Usage(stdout)
		return 0
	}
	if p.Version {
		fmt.Fprintln(stdout, link.Agent)
		return 0
	}

	var file config.Values
	switch {
	case p.Config != "":
		file = config.Load(p.Config, true)
	default:
		if _, err := os.Stat(config.DefaultPath()); err == nil {
			file = config.Load(config.DefaultPath(), false)
		}
	}
	opts := config.Merge(p.Values, file)
	if err := logx.Setup(opts.Bool("verbose"), opts.String("log_file")); err != nil {
		logx.Warnf("cannot open log file %s: %v", opts.String("log_file"), err)
	}

	applyTimings(opts)
	if bad := ports.PreferUSBIDs(opts.Strings("usb_ids")); len(bad) > 0 {
		logx.Warnf("ignoring usb_ids %q (expected vid:pid in hex, e.g. 0403:6001)", bad)
	}

	if p.List {
		return ports.PrintList(stdout)
	}
	switch p.Command {
	case "install-service":
		return service.Install(p.Passthrough())
	case "uninstall-service":
		return service.Uninstall()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Get EPIPE instead of being killed when stdout is a closed pipe (`panoptes --dry-run | head`).
	signal.Notify(make(chan os.Signal, 1), syscall.SIGPIPE)

	baud := opts.Int("baud", config.DefaultBaud)
	if p.Command == "screenshot" {
		return screenshot.Capture(opts.String("port"), baud, p.Out, 90*time.Second, ctx.Done())
	}

	var src link.Source
	if p.Demo {
		src = demo.New()
	} else {
		src = collectors.NewSourceWith(collectorOptions(opts))
	}
	cfg := config.CfgMessage(opts)
	interval := opts.Float("interval", config.DefaultInterval)
	if !(interval > 0) {
		logx.Warnf("ignoring interval = %v (must be > 0)", interval)
		interval = config.DefaultInterval
	}

	if p.DryRun {
		return runDry(ctx, stdout, src, interval, p.Once, cfg)
	}
	logx.Infof("%s starting (interval %.1fs)", link.Agent, interval)
	r := &link.Runner{Source: src, FixedPort: opts.String("port"), Baud: baud,
		Interval: time.Duration(interval * float64(time.Second)), Cfg: cfg, Once: p.Once}
	code := r.Run(ctx)
	if ctx.Err() != nil {
		logx.Infof("interrupted, bye")
	}
	return code
}

// collectorOptions reads the config-file-only collector options.
func collectorOptions(opts config.Values) collectors.Options {
	o := collectors.Options{NetIface: opts.String("net_interface"), Mounts: opts.Strings("mounts")}
	for _, g := range opts.Strings("disable") {
		g = strings.ToLower(g)
		if contains(config.DisableGroups, g) {
			o.Disable = append(o.Disable, g)
		} else {
			logx.Warnf("ignoring disable = %q (expected one of: %s)", g, strings.Join(config.DisableGroups, ", "))
		}
	}
	return o
}

// seconds reads a positive number of seconds option as a Duration.
func seconds(opts config.Values, key string, def float64) time.Duration {
	v := opts.Float(key, def)
	if !(v > 0) {
		logx.Warnf("ignoring %s = %v (must be > 0)", key, v)
		v = def
	}
	return time.Duration(v * float64(time.Second))
}

// applyTimings sets the link timings from the config file.
func applyTimings(opts config.Values) {
	link.HelloInterval = seconds(opts, "hello_interval", config.DefaultHelloInterval)
	link.ReconnectInterval = seconds(opts, "reconnect_interval", config.DefaultReconnectInterval)
	link.HandshakeRotateAfter = seconds(opts, "handshake_timeout", config.DefaultHandshakeTimeout)
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func runDry(ctx context.Context, w io.Writer, src link.Source, interval float64, once bool, cfg *frame.Obj) int {
	write := func(b []byte) bool {
		_, err := w.Write(b)
		return err == nil // e.g. broken pipe: exit quietly
	}
	if !write(frame.Encode(link.HelloMsg())) {
		return 0
	}
	if cfg != nil && !write(frame.Encode(cfg)) {
		return 0
	}
	src.Prime()
	iv := time.Duration(interval * float64(time.Second))
	if !sleepCtx(ctx, min(iv, time.Second)) { // let rate/cpu% baselines accumulate
		logx.Infof("interrupted, bye")
		return 0
	}
	next := time.Now()
	for {
		data := frame.FitFrame(src.Collect())
		if !write(data) {
			return 0
		}
		logx.Debugf("frame: %d bytes", len(data))
		if once {
			return 0
		}
		next = next.Add(iv)
		if !sleepCtx(ctx, time.Until(next)) {
			logx.Infof("interrupted, bye")
			return 0
		}
	}
}
