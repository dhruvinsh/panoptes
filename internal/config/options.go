package config

// Kind says where an option is consumed.
type Kind uint8

const (
	KindAgent  Kind = iota // agent option; also a command-line flag
	KindDevice             // sent to the board in a `cfg` message; also a command-line flag
	KindFile               // config file only
)

// Option describes one config key. This table is the single source of truth for the
// accepted keys, and the JSON Schemas in schema/ are generated from it (see schema.go).
type Option struct {
	Key      string
	Kind     Kind
	Type     string // JSON Schema type: string, integer, number, boolean or array (of strings)
	Doc      string
	Default  any
	Min, Max *float64
	MinExcl  bool     // Min is exclusive
	Enum     []string // allowed values (for arrays: of each item)
	MaxItems int      // arrays; 0 = no limit
	Pattern  string   // arrays: regexp each item must match
}

func f(v float64) *float64 { return &v }

// Themes and ThemeModes the firmware knows (cfg.theme / cfg.theme_mode).
var (
	Themes     = []string{"default", "gruvbox", "catppuccin", "solarized", "nord", "dracula", "tokyonight"}
	ThemeModes = []string{"dark", "light", "auto"}
)

// DisableGroups are the metric groups `disable` can switch off.
var DisableGroups = []string{"gpu", "temps", "fans", "battery", "procs", "disk", "net"}

// Defaults for the run options.
const (
	DefaultBaud              = 115200
	DefaultInterval          = 1.0
	DefaultHelloInterval     = 2.0
	DefaultReconnectInterval = 2.0
	DefaultHandshakeTimeout  = 10.0
)

// Options lists every option. Order matters: the Agent options come first, then the
// device options in the order they appear in the `cfg` message, then the file-only ones.
var Options = []Option{
	{Key: "port", Kind: KindAgent, Type: "string",
		Doc: "Serial port of the board, e.g. /dev/ttyUSB0 or COM5. Default: auto-detect."},
	{Key: "baud", Kind: KindAgent, Type: "integer", Default: DefaultBaud, Min: f(1),
		Doc: "Serial baud rate. The firmware speaks 115200."},
	{Key: "interval", Kind: KindAgent, Type: "number", Default: DefaultInterval, Min: f(0), MinExcl: true,
		Doc: "Seconds between stats frames."},
	{Key: "verbose", Kind: KindAgent, Type: "boolean", Default: false,
		Doc: "Debug logging (device boot messages, frame sizes)."},
	{Key: "log_file", Kind: KindAgent, Type: "string",
		Doc: "Also write the log to this file."},

	{Key: "bright", Kind: KindDevice, Type: "integer", Default: 200, Min: f(0), Max: f(255),
		Doc: "Backlight level, 0-255. Used when auto_bright is off."},
	{Key: "auto_bright", Kind: KindDevice, Type: "boolean", Default: true,
		Doc: "Set the backlight from the board's light sensor."},
	{Key: "auto_bright_min", Kind: KindDevice, Type: "integer", Default: 18, Min: f(0), Max: f(255),
		Doc: "Lowest backlight level auto_bright will go to in a dark room, 0-255."},
	{Key: "night_bright", Kind: KindDevice, Type: "integer", Default: 4, Min: f(0), Max: f(255),
		Doc: "Backlight level while night mode is on (long-press the screen to toggle it), 0-255."},
	{Key: "led", Kind: KindDevice, Type: "boolean", Default: true,
		Doc: "RGB status LED that follows CPU load."},
	{Key: "led_bright", Kind: KindDevice, Type: "integer", Default: 90, Min: f(0), Max: f(255),
		Doc: "Maximum brightness of the status LED, 0-255. The LED is very bright; the default keeps it subtle."},
	{Key: "beep", Kind: KindDevice, Type: "boolean", Default: false,
		Doc: "Speaker beep when an alert fires."},
	{Key: "rotate", Kind: KindDevice, Type: "integer", Default: 0, Min: f(0), Max: f(1),
		Doc: "0 = landscape, 1 = landscape flipped."},
	{Key: "page", Kind: KindDevice, Type: "integer", Min: f(-1), Max: f(4),
		Doc: "Force this page (0 dashboard, 1 CPU, 2 network, 3 sensors, 4 system). -1 leaves the board alone."},
	{Key: "cycle", Kind: KindDevice, Type: "integer", Default: 0, Min: f(0), Max: f(3600),
		Doc: "Advance to the next page every N seconds. 0 = off."},
	{Key: "alert_temp", Kind: KindDevice, Type: "number", Default: 90, Min: f(0), Max: f(150),
		Doc: "CPU temperature in °C above which the alert banner shows."},
	{Key: "alert_cpu", Kind: KindDevice, Type: "number", Default: 95, Min: f(0), Max: f(100),
		Doc: "CPU load in % above which the alert banner shows."},
	{Key: "theme", Kind: KindDevice, Type: "string", Default: "default", Enum: Themes,
		Doc: "Colour theme. Each has a dark and a light variant; theme_mode picks which one shows."},
	{Key: "theme_mode", Kind: KindDevice, Type: "string", Default: "dark", Enum: ThemeModes,
		Doc: "Which variant of the theme to show. auto follows the room light (the board's light sensor)."},

	{Key: "hello_interval", Kind: KindFile, Type: "number", Default: DefaultHelloInterval, Min: f(0), MinExcl: true,
		Doc: "Seconds between `hello` messages while waiting for the board to answer."},
	{Key: "reconnect_interval", Kind: KindFile, Type: "number", Default: DefaultReconnectInterval, Min: f(0), MinExcl: true,
		Doc: "Seconds between attempts to find and open the board after an error or unplug."},
	{Key: "handshake_timeout", Kind: KindFile, Type: "number", Default: DefaultHandshakeTimeout, Min: f(0), MinExcl: true,
		Doc: "Seconds to wait for `hi` before trying the next candidate port."},
	{Key: "net_interface", Kind: KindFile, Type: "string",
		Doc: "Network interface to graph, e.g. enp5s0 or Wi-Fi. Default: the default-route interface."},
	{Key: "usb_ids", Kind: KindFile, Type: "array", Pattern: "^[0-9a-fA-F]{4}:[0-9a-fA-F]{4}$",
		Doc: "Extra USB vendor:product IDs (hex, e.g. \"0403:6001\") to treat as the board when auto-detecting, " +
			"for boards with a USB-serial chip other than CH340, CH9102 or CP210x. They are tried first."},
	{Key: "mounts", Kind: KindFile, Type: "array", MaxItems: 4,
		Doc: "Mount points (or Windows drive letters) to show on the disk widget, at most 4. " +
			"Default: pick the real filesystems automatically."},
	{Key: "disable", Kind: KindFile, Type: "array", Enum: DisableGroups,
		Doc: "Metric groups not to collect or send; the board hides their widgets."},
}

// Key lists derived from Options.
var (
	// RunKeys are agent options with a command-line flag.
	RunKeys = keysOf(KindAgent)
	// CfgKeys are device options, sent to the board in a `cfg` message (in this order).
	CfgKeys = keysOf(KindDevice)
	// FileKeys are options only the config file sets.
	FileKeys = keysOf(KindFile)
)

func keysOf(k Kind) []string {
	var out []string
	for _, o := range Options {
		if o.Kind == k {
			out = append(out, o.Key)
		}
	}
	return out
}

// Lookup returns the option with the given (underscore) key.
func Lookup(key string) (Option, bool) {
	for _, o := range Options {
		if o.Key == key {
			return o, true
		}
	}
	return Option{}, false
}
