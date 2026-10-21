# Panoptes firmware

Firmware for the ESP32-2432S028R "Cheap Yellow Display" (2.8" 320x240, single
micro-USB, CH340). It turns the board into a USB-powered hardware monitor for
the PC it is plugged into. The PC runs the `panoptes` host agent (see [docs/host.md](../docs/host.md)),
which streams stats as newline-delimited JSON over USB serial at 115200 baud.
The wire format is in [`../PROTOCOL.md`](../PROTOCOL.md).

**USB only.** The firmware has no WiFi, Bluetooth, OTA, NTP or web code.
WiFi is never initialised, so its radio/PHY stays unpowered, and `btStop()`
runs at boot so the BT controller is off too (the Arduino core already frees
the BT controller memory when BT is unused). The clock comes only from the
host's `ts`/`tz` fields.

## Pages

Tap the left or right third of the screen, or swipe, to change page.
Long-press anywhere for 1 s to toggle **night mode** (very dim backlight,
dimmed LED). Tap the centre of the header bar to cycle the **colour theme**
(a toast shows its name).

| # | Page | Content |
|---|------|---------|
| 0 | Overview | clock/date, CPU, RAM, GPU gauges (the GPU gauge becomes CPU temp if there is no GPU), network sparkline, CPU temp / uptime |
| 1 | CPU | model, frequency, per-core bars (up to 32), 60 s history, load average |
| 2 | Network & Disk | rx/tx graph with auto-scaling, disk read/write, usage of up to 4 mounts |
| 3 | Sensors | temperatures, fans, top 5 processes, GPU details (memory if there is no GPU) |
| 4 | System | host/OS/kernel/uptime/IP/battery; firmware version, frames received, parse errors, free heap, backlight, LDR raw value and level, theme and mode |

If no stats frame arrives for 5 s, the **Waiting for host** screen appears
with the `panoptes` hint.

RGB LED: colour follows CPU load (green, then yellow, then red). It breathes
slowly blue while waiting for the host and flashes red during an alert. The
host can turn it off with `cfg.led`.

**Alert mode** starts after 5 consecutive frames with CPU temp >= `alert_temp`
(default 90 °C) or CPU load >= `alert_cpu` (default 95 %). The screen shows a
red banner and border, and the LED flashes. If `cfg.beep` is set (off by
default), the speaker chirps, at most once per 60 s. The alert ends after 3
cool frames.

Settings from `cfg` (brightness, auto-brightness, LED, beep, rotation, cycle,
thresholds, theme and theme mode) and the last page you picked are stored in
NVS and survive a reboot. Writes are debounced, and only keys whose value changed are written.

## Themes

Each theme family has a dark and a light variant, built from the family's
official palette:

| `cfg.theme` | dark | light |
|-------------|------|-------|
| `default` | the original glare-tuned black theme | slate on white |
| `gruvbox` | gruvbox dark (hard) | gruvbox light |
| `catppuccin` | mocha | latte |
| `solarized` | solarized dark | solarized light |
| `nord` | polar night | snow storm |
| `dracula` | dracula | alucard |
| `tokyonight` | night | day |

`cfg.theme_mode` picks the variant: `dark` (the default), `light`, or `auto`.
Light variants keep body text at about 7:1 contrast or better and captions
at about 4.5:1, which reads well on the glossy panel in a bright room. A few
roles that a family has no readable colour for (a gauge track between two
surfaces, a "comment" grey, Tokyo Night Day's 4.5:1 body text) are mixes of
two of its official colours; `src/ui/theme.cpp` notes which. The alert
banner picks white or black text, whichever reads better on the theme's red.

Tap the centre of the header bar to cycle through the families. The mode
only changes via `cfg` (`panoptes --theme-mode auto`).

### Automatic light/dark

In `auto` mode the firmware turns the smoothed LDR reading into a light
level from 0 (`LDR_RAW_DARK`) to 1 (`LDR_RAW_BRIGHT`), the same scale that
auto-brightness uses. The light variant takes over once the level has
stayed at or above `THEME_LIGHT_ABOVE` (0.65) for `THEME_DWELL_MS` (20 s),
and the dark one once it has stayed at or below `THEME_DARK_BELOW` (0.45)
for as long. Readings between the two thresholds keep the current variant,
and so does any excursion shorter than the dwell time. The backlight fades
out, the palette swaps, and the backlight fades back in (about 0.4 s in all).
At boot the first reading picks a variant straight away, using the midpoint
of the two thresholds.

To calibrate, set `LDR_RAW_BRIGHT`/`LDR_RAW_DARK` first (see below). Then,
in the lighting where you want each variant, read the level on the System
page (`LDR  raw  level`), or run `panoptes -v`, which logs a
`device light: raw=... level=...` line every 10 s. Put those levels in
`THEME_LIGHT_ABOVE` and `THEME_DARK_BELOW` in `config.h`, and keep them at
least ~0.15 apart.

## Board variants

| Variant | Panel | Build env |
|---------|-------|-----------|
| ESP32-2432S028R, micro-USB (this board, verified) | ST7789, no inversion | `cyd` (default) |
| Other batches sold under the same name | ILI9341 | `cyd-ili9341` |

The panel is not printed anywhere, and listings label both chips "ILI9341".
This board was identified with the panel test.

All board-specific settings live in [`include/config.h`](include/config.h):
pins, panel driver, inversion and colour order, touch calibration, and LDR
range.

### Finding the right panel setup: `paneltest`

```sh
pio run -e paneltest -t upload
```

[`diag/panel_test.cpp`](diag/panel_test.cpp) cycles through 7 panel setups,
about 7 s each. Tap the screen when you see a black background, a white
border, "TOP" upright, and red/green/blue bars left to right. The chosen mode
is printed over serial as a `log` line. Then set `PANEL_DRIVER` and
`PANEL_INVERT` in `config.h` to match, or build `cyd-ili9341`.

If the picture is still slightly wrong:

* **Colours inverted** (the background is white or light): flip `PANEL_INVERT`.
* **Red and blue swapped** (the cyan accent looks orange): flip `PANEL_RGB_ORDER`.
* **Garbled or shifted image** (backlight works): you have the wrong `PANEL_DRIVER`.

Any of these can also be set as build flags, for example
`build_flags = ${env:cyd.build_flags} -DPANEL_INVERT=true`.

## Build, flash, monitor

PlatformIO is assumed to be at `~/.platformio/penv/bin/pio` (or on `PATH` as `pio`).

```sh
cd firmware
pio run                  # build
pio run -t upload        # flash (upload_speed 460800: the CH340 on these boards is unreliable at 921600)
pio device monitor       # 115200 baud; you should see {"t":"hi",...} after a reset
```

If the upload doesn't start, hold **BOOT**, tap **RST**, then release
**BOOT**.

Note that the serial port is the protocol channel. Close `pio device monitor`
before you start `panoptes`, because only one program can own the port. Opening
the port toggles DTR/RTS, which resets the board. The host tolerates this.

### Linux serial permissions

`/dev/ttyUSB0` is owned by a group your user must belong to:

```sh
sudo usermod -aG uucp $USER      # Arch / Manjaro
sudo usermod -aG dialout $USER   # Debian / Ubuntu / Fedora
```

Log out and back in (or reboot) for the group change to take effect. On some
distros `brltty` claims CH340 devices. If the port disappears right after you
plug in the board, remove or disable `brltty`.

## Touch calibration

The XPT2046 has its own SPI bus (VSPI: CLK 25, MOSI 32, MISO 39, CS 33, IRQ 36).
The raw calibration range is in `config.h`:

```c
#define TOUCH_X_MIN 200
#define TOUCH_X_MAX 3700
#define TOUCH_Y_MIN 240
#define TOUCH_Y_MAX 3800
#define TOUCH_OFFSET_ROTATION 0
```

* Build with `-DTOUCH_DEBUG=1` to draw a crosshair where the firmware thinks
  you touched.
* **Left/right taps swapped:** try `TOUCH_OFFSET_ROTATION 2`.
* **Axes swapped** (tapping the left edge registers at the top): try 1 or 3.
* **One axis mirrored:** try 4..7.
* **Small offsets near the edges:** adjust the MIN/MAX values. The usual range
  is about 200..3900.

## Auto-brightness (LDR)

The LDR on GPIO 34 is read with 0 dB attenuation and smoothed (time constant
of about 1 s). On most boards the reading goes **up** as the room gets darker.
The System page shows the live `LDR raw` value. Read it in your brightest and
darkest conditions and put those numbers into `LDR_RAW_BRIGHT` and
`LDR_RAW_DARK`. Swap them if your board reads the other way round.
Auto-brightness never drops below `BL_AUTO_FLOOR` (default 18; `cfg.auto_bright_min` changes it, as `cfg.night_bright` does `BL_NIGHT` and `cfg.led_bright` does `LED_MAX_DUTY`). The System page also
shows the resulting 0..1 level, which `theme_mode: "auto"` uses too.

## Code layout

```
include/config.h       pins, panel variant, calibration, tunables
src/main.cpp           setup()/loop() -> app
src/app.{h,cpp}        wiring: boot sequence, main loop, protocol callbacks, night mode
src/protocol.{h,cpp}   line reader, JSON parsing, the shared State struct, hi/ev/log
src/settings.{h,cpp}   persistent config (Preferences/NVS)
src/display.{h,cpp}    LovyanGFX device (ILI9341/ST7789 on HSPI, XPT2046 on VSPI, PWM backlight)
src/sensors.{h,cpp}    LDR + backlight control
src/themectl.{h,cpp}   theme family/mode, auto light/dark from the LDR, swap fade
src/led.{h,cpp}        RGB status LED
src/speaker.{h,cpp}    non-blocking alert chirp
src/alert.{h,cpp}      alert detection with hysteresis
src/format.{h,cpp}     human-readable units (KB/s, 3d 4h, 3.60 GHz, ...)
src/ui/canvas.*        flicker-free band renderer
src/ui/theme.*         active colours, the palettes of every theme, fonts, layout
src/ui/widgets.*       gauges, bars, graphs, text helpers
src/ui/page_*.cpp      the pages and the waiting screen
src/ui/ui.cpp          page switching, gestures, toast, alert overlay
```

### Rendering

A full 320x240 RGB565 frame buffer needs 150 KB, which is more than the
ESP32's largest free DRAM block. The firmware keeps a single 320x120 band
buffer (75 KB, with automatic fallback to smaller bands) and renders every
frame in two bands. Each band draws the whole screen into a sprite whose clip
rect covers only that band, then pushes it. The drawing code uses plain screen
coordinates, every pixel is pushed exactly once per frame, and colour stays at
16 bits. An 8-bit full-screen sprite would also fit in RAM, but RGB332 bands
badly on the dark theme. Do not call LovyanGFX's `drawWideLine`,
`drawSmoothLine` or `drawWedgeLine` from page code: they reset the clip rect.
Use `aaLine()` instead.
