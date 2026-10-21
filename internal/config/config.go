// Package config handles the config file, option layering (defaults < config file < CLI)
// and building the device `cfg` message.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/dhruvinsh/panoptes/internal/frame"
	"github.com/dhruvinsh/panoptes/internal/logx"
)

// AppName is used for the config directory, log file and service names.
const AppName = "panoptes"

// Values maps option keys (underscore form) to their values.
type Values map[string]any

// Dir is the per-user config directory.
//
//	$XDG_CONFIG_HOME/panoptes                 if XDG_CONFIG_HOME is set (any OS)
//	~/.config/panoptes                        Linux; on macOS and Windows when that directory exists
//	~/Library/Application Support/panoptes    macOS
//	%AppData%\panoptes                        Windows
func Dir() string {
	home, _ := os.UserHomeDir()
	return dirFor(runtime.GOOS, os.Getenv, home, func(p string) bool {
		st, err := os.Stat(p)
		return err == nil && st.IsDir()
	})
}

func dirFor(goos string, env func(string) string, home string, isDir func(string) bool) string {
	if x := env("XDG_CONFIG_HOME"); filepath.IsAbs(x) { // the XDG spec says to ignore relative values
		return filepath.Join(x, AppName)
	}
	xdg := filepath.Join(home, ".config", AppName)
	switch goos {
	case "darwin":
		if isDir(xdg) {
			return xdg
		}
		return filepath.Join(home, "Library", "Application Support", AppName)
	case "windows":
		if isDir(xdg) {
			return xdg
		}
		if appData := env("AppData"); appData != "" {
			return filepath.Join(appData, AppName)
		}
		return filepath.Join(home, "AppData", "Roaming", AppName)
	}
	return xdg
}

// DefaultPath is Dir()/config.toml.
func DefaultPath() string { return filepath.Join(Dir(), "config.toml") }

func isKnown(k string) bool {
	_, ok := Lookup(k)
	return ok
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// Load reads a TOML config file. Unknown keys are reported and ignored; "-" and "_" are equivalent;
// keys may be top level or inside a [panoptes] table. A missing default file is silent,
// a missing explicit file logs a warning. Errors never abort the agent.
func Load(path string, explicit bool) Values {
	text, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if explicit {
				logx.Warnf("config file %s not found", path)
			}
		} else {
			logx.Warnf("cannot read config %s: %v", path, err)
		}
		return Values{}
	}
	return Parse(string(text), path)
}

// Parse decodes TOML config text (path is only used in messages).
func Parse(text, path string) Values {
	var data map[string]any
	if _, err := toml.Decode(text, &data); err != nil {
		logx.Warnf("invalid config %s: %v", path, err)
		return Values{}
	}
	for _, table := range []string{AppName, "cydmon"} {
		if sub, ok := data[table].(map[string]any); ok {
			data = sub
			break
		}
	}
	out := Values{}
	for k, v := range data {
		key := strings.ReplaceAll(k, "-", "_")
		if isKnown(key) {
			out[key] = v
		} else {
			logx.Warnf("ignoring unknown config key %q in %s", k, path)
		}
	}
	logx.Debugf("loaded config %s: %v", path, map[string]any(out))
	return out
}

// Merge layers defaults < file < cli (cli holds only options given on the command line).
func Merge(cli, file Values) Values {
	opts := Values{"baud": DefaultBaud, "interval": DefaultInterval, "verbose": false}
	for k, v := range file {
		if v != nil {
			opts[k] = v
		}
	}
	for k, v := range cli {
		if v != nil && isKnown(k) {
			opts[k] = v
		}
	}
	return opts
}

// String returns a string option ("" if unset).
func (v Values) String(k string) string {
	switch x := v[k].(type) {
	case nil:
		return ""
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}

// Bool returns a boolean option.
func (v Values) Bool(k string) bool {
	b, _ := toBool(v[k])
	return b
}

// Int returns an integer option, or def if unset/invalid.
func (v Values) Int(k string, def int) int {
	if n, ok := toInt(v[k]); ok {
		return n
	}
	if v[k] != nil {
		logx.Warnf("ignoring %s = %v (expected a number)", k, v[k])
	}
	return def
}

// Float returns a float option, or def if unset/invalid.
func (v Values) Float(k string, def float64) float64 {
	if f, ok := toFloat(v[k]); ok {
		return f
	}
	if v[k] != nil {
		logx.Warnf("ignoring %s = %v (expected a number)", k, v[k])
	}
	return def
}

// Strings returns a list option (nil if unset or not a list of strings).
func (v Values) Strings(k string) []string {
	if v[k] == nil {
		return nil
	}
	list, ok := v[k].([]any)
	if !ok {
		logx.Warnf("ignoring %s = %v (expected a list of strings)", k, v[k])
		return nil
	}
	out := make([]string, 0, len(list))
	for _, e := range list {
		s, ok := e.(string)
		if !ok {
			logx.Warnf("ignoring %s = %v (expected a list of strings)", k, v[k])
			return nil
		}
		out = append(out, s)
	}
	return out
}

func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return 0, false
		}
		return int(x), true // like Python int(): truncates
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		return n, err == nil
	}
	return 0, false
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	}
	return 0, false
}

func toBool(v any) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case int:
		return x != 0, true
	case int64:
		return x != 0, true
	case float64:
		return x != 0, true
	case string:
		return x != "", true // Python bool(str)
	}
	return false, false
}

// CfgMessage builds the `cfg` message from the merged options, or nil if no device
// option is set. Config-file values are coerced and validated here (the CLI already
// validated its own).
func CfgMessage(opts Values) *frame.Obj {
	cfg := frame.New("t", "cfg")
	for _, k := range CfgKeys {
		v := opts[k]
		if v == nil {
			continue
		}
		switch k {
		case "bright", "auto_bright_min", "night_bright", "led_bright", "rotate", "page", "cycle":
			n, ok := toInt(v)
			if !ok {
				logx.Warnf("ignoring %s = %v (expected an integer)", k, v)
				continue
			}
			cfg.Set(k, n)
		case "auto_bright", "led", "beep":
			b, ok := toBool(v)
			if !ok {
				logx.Warnf("ignoring %s = %v (expected true/false)", k, v)
				continue
			}
			cfg.Set(k, b)
		case "alert_temp", "alert_cpu":
			switch x := v.(type) {
			case int64:
				cfg.Set(k, x) // ints from the file stay ints, like the Python agent
			case int:
				cfg.Set(k, x)
			default:
				f, ok := toFloat(v)
				if !ok {
					logx.Warnf("ignoring %s = %v (expected a number)", k, v)
					continue
				}
				cfg.Set(k, f)
			}
		case "theme", "theme_mode":
			allowed := Themes
			if k == "theme_mode" {
				allowed = ThemeModes
			}
			s := strings.ToLower(fmt.Sprint(v))
			if !contains(allowed, s) {
				logx.Warnf("ignoring %s = %q (expected one of: %s)", k, fmt.Sprint(v), strings.Join(allowed, ", "))
				continue
			}
			cfg.Set(k, s)
		}
	}
	if cfg.Len() == 1 {
		return nil
	}
	return cfg
}

// PyFloat formats a float like Python's str(): "1.0", "0.5", "90.0".
func PyFloat(f float64) string {
	if a := math.Abs(f); a != 0 && (a < 1e-4 || a >= 1e16) || math.IsNaN(f) || math.IsInf(f, 0) {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if strings.Contains(s, ".") {
		return s
	}
	return s + ".0"
}
