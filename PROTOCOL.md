# Panoptes — Serial Protocol v1

Transport: USB serial (CH340, `1a86:7523`; some CYD variants use CP2102 `10c4:ea60`),
**115200 baud, 8N1**. Messages are **newline-delimited JSON** (one object per line, `\n`
terminated, UTF-8, max 2048 bytes per line). Every message has a `"t"` (type) field.
Unknown fields MUST be ignored by both sides; missing fields mean "not available" and
the UI must hide/grey that widget rather than show zeros.

Opening the port on a CH340 toggles DTR/RTS which resets the ESP32. The host opens
the port with DTR/RTS deasserted where the OS allows it, and in any case tolerates a
reboot: it keeps sending `hello` every 2 s until it receives `hi`.

## Host → Device

### `hello` — handshake (repeat every 2 s until `hi` is received)
```json
{"t":"hello","v":1,"agent":"panoptes 0.2.0"}
```

### `s` — stats frame (default every 1 s)
All fields optional. Units are in the key comments.
```jsonc
{
  "t": "s",
  "ts": 1760950000,          // unix epoch seconds (UTC)
  "tz": -14400,              // local UTC offset in seconds (device renders local clock)
  "host": "archbox",
  "os": "Arch Linux",        // pretty OS name
  "kern": "6.18.57-2-lts",
  "up": 123456,              // uptime seconds
  "cpu": {
    "name": "AMD Ryzen 7 5800X",
    "pct": 23.4,             // total %
    "cores": [12.0, 40.5],   // per logical core %, max 32 entries (host truncates)
    "freq": 3600,            // current MHz
    "temp": 55.0,            // °C package temp
    "load": [1.2, 0.9, 0.7]      // 1/5/15 min load avg (host rounds all floats to 1 dp)
  },
  "mem":  {"used": 8123, "total": 32000, "pct": 25.4, "swap_pct": 0.0},   // MiB
  "gpu":  {"name": "RTX 3070", "pct": 12, "temp": 48, "mem_used": 1200,
           "mem_total": 8192, "power": 45.2},                             // MiB, W
  "net":  {"iface": "enp5s0", "ip": "192.168.1.20",
           "rx": 125000.0, "tx": 4200.0},                                 // bytes/s
  "disk": {"r": 0.0, "w": 52000.0,                                        // bytes/s
           "mounts": [{"m": "/", "pct": 41.0, "used": 200.5, "total": 488.0}]},  // GiB, max 4
  "temps": [{"n": "CPU", "v": 55.0}, {"n": "NVMe", "v": 41.0}],           // °C, max 6
  "fans":  [{"n": "cpu", "rpm": 1150}],                                   // max 4
  "procs": [{"n": "firefox", "c": 12.3, "m": 4.5}],                       // top 5 by cpu; c=% of whole machine (0-100), m=%mem
  "bat":   {"pct": 87, "plug": true}                                      // laptops only
}
```

### `cfg` — runtime config (optional, any subset)
```json
{"t":"cfg","bright":200,"auto_bright":true,"led":true,"beep":false,
 "rotate":0,"page":-1,"cycle":0,"alert_temp":90,"alert_cpu":95,
 "theme":"nord","theme_mode":"auto"}
```
- `bright` 0–255 backlight; `auto_bright` use the onboard LDR.
- `led` RGB status LED on/off; `beep` speaker alerts on/off.
- `rotate` 0/1 → landscape / landscape-flipped.
- `page` force page index (-1 = leave alone); `cycle` auto-advance seconds (0 = off).
- `alert_temp` °C, `alert_cpu` % thresholds for alert mode.
- `theme` colour theme family: `"default"`, `"gruvbox"`, `"catppuccin"`, `"solarized"`,
  `"nord"`, `"dracula"`, `"tokyonight"` (case-insensitive). Unknown names are ignored.
- `theme_mode` `"dark"` | `"light"` | `"auto"`. `auto` picks the variant from the onboard
  light sensor (with hysteresis and a 20 s dwell, so it never flaps).

Like the other settings, `theme` and `theme_mode` are stored in flash and survive a reboot.

## Device → Host

### `hi` — reply to `hello`, also sent unsolicited on boot
```json
{"t":"hi","v":1,"fw":"0.1.0","board":"ESP32-2432S028R","w":320,"h":240}
```

### `ev` — UI events (informational; host may log or ignore)
```json
{"t":"ev","e":"page","page":2}
{"t":"ev","e":"theme","theme":"nord","mode":"auto","variant":"light"}
{"t":"ev","e":"light","raw":412,"level":0.43}
```
- `page`: the page changed (tap, swipe, auto-cycle or `cfg.page`).
- `theme`: the theme family, mode or shown variant (`"dark"`/`"light"`) changed. Also sent
  right after each `hi` that answers a `hello`, so the host always knows the current theme.
- `light`: every 10 s while frames are arriving. `raw` is the smoothed LDR ADC value
  (higher = darker on most boards), `level` the 0.0 (dark) – 1.0 (bright) value derived from
  it that auto-brightness and `theme_mode: "auto"` use. Meant for calibrating the thresholds.

### `log` — debug text
```json
{"t":"log","msg":"..."}
```

## Clarifications
- The 2048-byte line limit counts UTF-8 bytes including the `\n`. Host caps strings at 48 chars.
- The host sends `s`/`cfg` frames only after it has received a `hi`, so the device MUST send `hi`
  on boot and in reply to every `hello`. The host re-sends `cfg` whenever it sees a fresh `hi`.

## Timeouts
- Device shows the "Waiting for host" screen if no `s` frame arrives for **5 s**.
- Host reconnects (re-scans ports) on any serial error, retrying every 2 s.
