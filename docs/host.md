# Panoptes host agent

`panoptes` is a single, dependency-free binary for Linux, macOS and Windows that streams CPU,
memory, GPU, network, disk, temperature, fan, process and battery stats to an ESP32
"Cheap Yellow Display" (ESP32-2432S028R) over USB serial, using the newline-delimited JSON
protocol in [`../PROTOCOL.md`](../PROTOCOL.md).

Prebuilt binaries: linux/amd64, linux/arm64, windows/amd64, windows/arm64, darwin/amd64 (Intel),
darwin/arm64 (Apple Silicon). All are built with `CGO_ENABLED=0`, so they need no libc or
runtime beyond the OS itself.

## Install

### From GitHub Releases

Download the archive for your OS and architecture from
<https://github.com/dhruvinsh/panoptes/releases> — `panoptes_<version>_linux_amd64.tar.gz`,
`…_linux_arm64.tar.gz`, `…_darwin_amd64.tar.gz` (Intel Mac), `…_darwin_arm64.tar.gz` (Apple
Silicon), `…_windows_amd64.zip`, `…_windows_arm64.zip` — and verify it against `checksums.txt`
if you like. Each archive holds the binary, this document and the `packaging/` udev rule. Put the
binary somewhere stable on your `PATH`: the service installer (below) records its absolute path,
so don't run it from your Downloads folder.

**Linux**

```sh
tar xzf panoptes_*_linux_amd64.tar.gz
install -Dm755 panoptes ~/.local/bin/panoptes     # or: sudo install -m755 panoptes /usr/local/bin/
panoptes --version
```

**macOS**

Downloaded binaries are quarantined by Gatekeeper ("cannot be opened because the developer
cannot be verified"). Remove the quarantine attribute once:

```sh
tar xzf panoptes_*_darwin_arm64.tar.gz
xattr -d com.apple.quarantine ./panoptes
sudo mv ./panoptes /usr/local/bin/      # or ~/bin, anything on PATH
panoptes --version
```

**Windows**

Extract the zip and put `panoptes.exe` in e.g. `%LOCALAPPDATA%\Programs\panoptes\`, then add that
folder to your user `PATH` (Settings → System → About → Advanced system settings → Environment
Variables). SmartScreen may warn about an unrecognised app the first time; choose
*More info → Run anyway*.

### With Go

Go 1.24 or newer:

```sh
go install github.com/dhruvinsh/panoptes/cmd/panoptes@latest
```

The binary lands in `$(go env GOPATH)/bin` (usually `~/go/bin`). `--version` prints `dev` for
such builds; release builds set it with `-ldflags "-X main.version=vX.Y.Z"`.

### From source

```sh
go build -o panoptes ./cmd/panoptes     # from the repository root
go test ./...
```

## Usage

```sh
panoptes --list                 # show serial ports; * marks candidates
panoptes                        # auto-detect the board, handshake, stream every 1 s
panoptes --port /dev/ttyUSB0    # or COM5, /dev/cu.wchusbserial1410 ...
panoptes --dry-run              # print frames to stdout, no hardware needed
panoptes --dry-run --once       # one frame, then exit
panoptes --demo                 # synthetic, animated data (exercises every widget)
panoptes -v                     # debug logging (device boot messages, frame sizes)
panoptes screenshot -o shot.png # save what the display shows as a PNG (~25 s)
panoptes pause                  # make the running agent release the port for 10 min (e.g. to flash)
panoptes resume                 # take it back now
```

All options: `panoptes --help`. Options and the subcommand may come in any order
(`panoptes install-service --port COM5` and `panoptes --port COM5 install-service` are the same).

Port auto-detection prefers, in order: `/dev/panoptes`, then `/dev/cyd` (udev symlinks, Linux),
CH340 `1a86:7523`, CH9102 `1a86:55d4`, CP210x `10c4:ea60`, then ports whose description looks like
a USB-serial bridge. Other serial ports are never opened. If several candidates exist and one
doesn't answer the handshake within 10 s, the next one is tried.

On connect, the port is opened with DTR/RTS deasserted (to avoid resetting the ESP32 where the
OS allows it), `hello` is sent every 2 s until the device answers `hi`, then a `cfg` message (if
any device options are set) and one `s` frame per interval. `hi`, `ev` and `log` messages from the
device are logged (the 10 s `light` events only with `-v`). On any serial error or unplug the port
is closed and re-scanned every 2 s. If the device reboots and says `hi` again, `cfg` is re-sent.
Ctrl-C exits cleanly.

### Device options

Sent as a `cfg` message after every handshake:

| option | meaning |
|---|---|
| `--bright 0-255` | backlight level |
| `--auto-bright` / `--no-auto-bright` | use the onboard light sensor |
| `--led` / `--no-led` | RGB status LED |
| `--led-bright 0-255` | maximum LED brightness (default 90) |
| `--auto-bright-min 0-255` | lowest backlight auto-brightness uses (default 18) |
| `--night-bright 0-255` | backlight in night mode (default 4) |
| `--beep` / `--no-beep` | speaker alerts |
| `--rotate 0\|1` | landscape / landscape flipped |
| `--page N` | force page (-1 = leave alone) |
| `--cycle SECONDS` | auto-advance pages (0 = off) |
| `--alert-temp °C`, `--alert-cpu %` | alert thresholds |
| `--theme NAME` | `default`, `gruvbox`, `catppuccin`, `solarized`, `nord`, `dracula`, `tokyonight` |
| `--theme-mode dark\|light\|auto` | theme variant; `auto` follows the room light (onboard LDR) |

The device remembers the theme and mode across reboots. It reports its theme as an `ev` message,
logged as e.g. `device theme: nord (mode auto, showing light)`; with `-v` you also see
`device light: raw=412 level=0.43` every 10 s, for calibrating the auto thresholds
(see `firmware/README.md`).

## Config file

Optional TOML file; command-line options override it. The agent reads it on start, so it applies
the same whether you run `panoptes` by hand or as a login service (restart the service after
editing: `systemctl --user restart panoptes`, `launchctl kickstart -k gui/$(id -u)/com.dhruvinsh.panoptes`,
or `schtasks /End /TN Panoptes` then `/Run`). Location:

| OS | path |
|---|---|
| Linux | `~/.config/panoptes/config.toml` (`$XDG_CONFIG_HOME` honoured) |
| macOS | `~/.config/panoptes/config.toml` if that directory exists, else `~/Library/Application Support/panoptes/config.toml` |
| Windows | `~/.config/panoptes/config.toml` (i.e. `%USERPROFILE%\.config\panoptes`) if that directory exists, else `%AppData%\panoptes\config.toml` |

`$XDG_CONFIG_HOME`, when set to an absolute path, wins on every OS. `panoptes --help` prints the
path in use, and `--config PATH` points at another file. [`config.example.toml`](config.example.toml)
lists every option; copy it as a starting point.

```toml
#:schema https://raw.githubusercontent.com/dhruvinsh/panoptes/main/schema/config.schema.json

theme = "catppuccin"   # default, gruvbox, catppuccin, solarized, nord, dracula, tokyonight
theme_mode = "auto"    # dark, light, or auto (follows the board's light sensor)
```

### Options

Device options are sent to the board after every handshake (see PROTOCOL.md `cfg`). Everything
here is optional.

| key | default | meaning |
|---|---|---|
| `theme` | `"default"` | colour theme (see above) |
| `theme_mode` | `"dark"` | `dark`, `light` or `auto` |
| `bright` | `200` | backlight 0-255 (when `auto_bright` is off) |
| `auto_bright` | `true` | backlight from the light sensor |
| `auto_bright_min` | `18` | lowest backlight level auto-brightness goes to, 0-255 |
| `night_bright` | `4` | backlight level in night mode (long-press the screen), 0-255 |
| `led` | `true` | RGB status LED |
| `led_bright` | `90` | maximum LED brightness, 0-255 |
| `beep` | `false` | speaker beep on alerts |
| `rotate` | `0` | `0` landscape, `1` flipped |
| `page` | | force a page (0-4); `-1` leaves the board alone |
| `cycle` | `0` | auto-advance pages every N seconds (0 = off) |
| `alert_temp` | `90` | alert at this CPU temperature, °C |
| `alert_cpu` | `95` | alert at this CPU load, % |
| `port` | auto-detect | serial port, e.g. `/dev/ttyUSB0`, `COM5` |
| `baud` | `115200` | |
| `interval` | `1.0` | seconds between frames |
| `verbose` | `false` | debug logging |
| `log_file` | | also log to this file |

These have no command-line flag, only the config file:

| key | default | meaning |
|---|---|---|
| `disable` | `[]` | metric groups not to collect or send: `gpu`, `temps`, `fans`, `battery`, `procs`, `disk`, `net`. The board hides their widgets. Handy to save CPU (`procs` is the costliest) or to keep process names off the screen. |
| `net_interface` | default-route interface | interface to graph, e.g. `"enp5s0"`, `"Wi-Fi"` |
| `mounts` | auto | up to 4 mount points (or drive letters) to show instead of the automatic pick, e.g. `["/", "/mnt/data"]` |
| `usb_ids` | `[]` | extra `"vid:pid"` (hex) of USB-serial chips to treat as the board, e.g. `["0403:6001"]` for an FTDI. Tried before the built-in CH340 / CH9102 / CP210x |
| `hello_interval` | `2.0` | seconds between `hello` messages while waiting for the board |
| `reconnect_interval` | `2.0` | seconds between reconnect attempts |
| `handshake_timeout` | `10.0` | seconds to wait for `hi` before trying the next candidate port |

Keys may use `-` or `_` and may also live in a `[panoptes]` table. An unknown key, or an unknown
`theme` / `theme_mode`, is reported as a warning and ignored; the agent keeps running.
`install-service --theme X` bakes the flag into the service, and a flag beats the config file, so
leave `--theme` off `install-service` if you want the config file to choose.

### Editor support

The accepted keys are described by a JSON Schema in [`schema/config.schema.json`](../schema/config.schema.json),
generated from the agent's own option table (a test fails if it goes stale). Point your editor at
the raw GitHub URL for completion, hover docs and typo warnings:

- **taplo (the TOML language server behind Even Better TOML, and neovim's `taplo` LSP):**
  put `#:schema https://raw.githubusercontent.com/dhruvinsh/panoptes/main/schema/config.schema.json`
  on the first line, as in the example above.
- **Per project, with taplo:** a `.taplo.toml` containing
  ```toml
  [[rule]]
  include = ["**/panoptes/config.toml"]
  schema.path = "https://raw.githubusercontent.com/dhruvinsh/panoptes/main/schema/config.schema.json"
  ```

To pin the schema to a release, replace `main` with the tag in the URL.

## Run at login

```sh
panoptes install-service [options]    # options (e.g. --port, --bright, --theme) are baked in
panoptes uninstall-service
```

The service runs the absolute path of the binary you ran `install-service` with (symlinks
resolved), so install the binary first. No root needed.

- **Linux:** writes `~/.config/systemd/user/panoptes.service` and runs
  `systemctl --user enable --now panoptes.service`. Logs: `journalctl --user -u panoptes -f`.
  Optionally `loginctl enable-linger $USER` to keep it running without a login session (then use
  group membership rather than the `uaccess` rule for port access, since `uaccess` ACLs follow the
  active seat session).
- **macOS:** writes `~/Library/LaunchAgents/com.dhruvinsh.panoptes.plist` and bootstraps it with
  `launchctl`. Logs: `~/Library/Logs/panoptes.log`.
- **Windows:** registers a Task Scheduler task `Panoptes` (logon trigger, current user, no
  execution time limit, keeps running on battery, restarts on failure) and starts it. The task runs
  `conhost.exe --headless "…\panoptes.exe" …` so no console window appears at logon while
  `panoptes.exe` stays a normal console program (`--list`, `--dry-run` still print in a terminal).
  Logs: `%AppData%\panoptes\panoptes.log` (the installer adds `--log-file` unless you pass one).
  If `schtasks` is refused (some systems need an elevated prompt for logon tasks), a
  `Panoptes.cmd` launcher is written to the per-user Startup folder instead (a console window
  flashes briefly at logon in that case). `uninstall-service` removes both and stops a running
  agent. Without `conhost.exe` (pre-1809 Windows 10) the task runs `panoptes.exe` directly and a
  console window stays open.

## Flashing while the agent runs

The agent holds the serial port, so a flasher (the web flasher, esptool, `pio run -t upload`)
can't open it. Instead of stopping the service:

```sh
panoptes pause        # release the port for 10 minutes; also 90s, 30m, 1h, or a number of minutes
# ...flash the board...
panoptes resume       # reconnect now instead of waiting for the pause to run out
```

`pause` writes the end time to a `pause` file in the config directory. A running agent checks for
the file twice a second, closes the port and waits; `resume` deletes the file. While a flasher has
the port, the agent logs `... is busy` once and keeps retrying, so it reconnects by itself after
the pause even if you don't run `resume`.

## Serial port setup per OS

### Linux — Arch

The CH340 driver (`ch341`) is in the kernel; the board appears as `/dev/ttyUSB0`, owned by group
`uucp`. Either:

- install the udev rule (recommended: no group change, gives you `/dev/panoptes` and `/dev/cyd`):
  ```sh
  sudo install -m 0644 packaging/70-panoptes.rules /etc/udev/rules.d/
  sudo udevadm control --reload && sudo udevadm trigger --subsystem-match=tty
  ```
  It sets `MODE="0660"` and `TAG+="uaccess"` (the logged-in user gets an ACL). Remove an older
  `/etc/udev/rules.d/99-cyd.rules` if you had one. Or:
- `sudo usermod -aG uucp $USER` and log out/in.

### Linux — Ubuntu / Debian

Same as Arch, but the group is `dialout`: `sudo usermod -aG dialout $USER` (or use the udev rule).
If the port disappears a second after plugging in, remove `brltty` (`sudo apt remove brltty`) —
it grabs CH340 devices on some releases.

### macOS

macOS 10.14+ (including Apple Silicon) ships a CH34x driver; the port is `/dev/cu.usbserial-*`
or `/dev/cu.wchusbserial*` (panoptes uses `cu.*`, never the blocking `tty.*`). CP2102 boards show
up as `/dev/cu.SLAB_USBtoUART` or `/dev/cu.usbserial-*`. If no port appears, install WCH's CH34x
driver and approve it under System Settings → Privacy & Security. No permission setup needed.

How detection works without cgo: the release binaries are built with `CGO_ENABLED=0`, so they
can't call IOKit through cgo. Instead panoptes globs `/dev/cu.*` and runs
`/usr/sbin/ioreg -r -c IOUSBHostDevice -l -w0`, walking the printed device tree to map each
`IOCalloutDevice` (`/dev/cu.…`) to the `idVendor` / `idProduct` / product name of the USB device
above it. If `ioreg` is unavailable, ports named `cu.usbserial*`, `cu.wchusbserial*` or
`cu.SLAB_USBtoUART*` are still picked up by name.

### Windows

Install the WCH CH340/CH341 driver if Device Manager shows an unknown "USB2.0-Serial" device; the
board then appears as `COMn` ("USB-SERIAL CH340 (COMn)"). Only one program can have a COM port open,
so close the Arduino IDE / serial monitors first. `panoptes --list` shows which COM port is the board.

## Sensors per OS

| metric | Linux | macOS | Windows |
|---|---|---|---|
| CPU %, per-core (≤32), load avg | ✓ | ✓ | ✓ (load avg is emulated; reads 0 for the first ~5 s) |
| CPU frequency | ✓ current (cpufreq) | ✓ Intel; absent on Apple Silicon | base clock (registry), not live |
| CPU model | `/proc/cpuinfo` | `sysctl machdep.cpu.brand_string` | registry |
| Memory / swap | ✓ | ✓ | ✓ (swap = pagefile) |
| Network rate + IPv4 | ✓ default-route interface | ✓ | ✓ |
| Disk I/O + mounts (≤4) | ✓ | ✓ | ✓ (local drive letters) |
| Temperatures (≤6), `cpu.temp` | ✓ hwmon (k10temp/coretemp/zenpower → CPU, nvme, amdgpu, acpitz, jc42, …) | ✗ | only via LibreHardwareMonitor / OpenHardwareMonitor WMI, if running |
| Fans (≤4) | ✓ hwmon (if your board's sensor driver is loaded, e.g. `nct6775`, `it87`) | ✗ | via LHM/OHM WMI, as above |
| GPU — NVIDIA | ✓ `nvidia-smi` | ✗ | ✓ `nvidia-smi` (PATH, `System32`, or `NVIDIA Corporation\NVSMI`) |
| GPU — AMD | ✓ sysfs (`gpu_busy_percent`, VRAM, hwmon temp/power) | ✗ | ✗ |
| GPU — Intel / Apple | ✗ | ✗ | ✗ |
| Top 5 processes | ✓ | ✓ (other users' processes hidden without root) | ✓ (protected processes skipped) |
| Battery | ✓ laptops | ✓ laptops (`pmset`) | ✓ laptops |

Absent metrics are simply omitted from the frame; the display hides those widgets.

Units and conventions: memory in MiB, disk sizes in GiB, rates in bytes/s, temps in °C. Process
`c` is percent of the **whole machine** (0–100, Task Manager style, comparable to `cpu.pct`),
not per-core like `top`; same-named processes are summed (e.g. all `firefox` processes).

Frames are kept under the protocol's 2048-byte line limit: compact JSON, floats rounded to one
decimal, strings capped at 48 characters; if still too long, `procs`, then `fans`, then temps beyond
the first 3, then per-core CPU values are dropped. A typical frame is ~1 KB.

## Troubleshooting

- **`Permission denied` opening the port (Linux):** install the udev rule or join
  `uucp`/`dialout` (see above), then replug. Check with `ls -l /dev/ttyUSB0; getfacl /dev/ttyUSB0`.
- **`no board serial port found`:** run `panoptes --list`. If your board isn't marked `*`, pass
  `--port` explicitly. On Ubuntu check `brltty`; on Windows/macOS check the CH340 driver.
- **Board reboots when panoptes connects:** expected on some OS/driver combinations (DTR/RTS
  toggling on open drives the ESP32 auto-reset circuit). panoptes re-sends `hello` until the board
  is back, so it recovers within a few seconds. On Linux, `stty -F /dev/ttyUSB0 -hupcl` can stop the
  reset on subsequent opens.
- **"Waiting for host" on the display:** the device got no frame for 5 s. Run `panoptes -v`; you
  should see `device hi: ...`. If you only see `sent hello` and `device (non-JSON): ...` lines, the
  board is running different firmware or a different baud rate.
- **Port busy:** another program (Arduino IDE serial monitor, `screen`, a second panoptes) has it
  open. On Linux/macOS panoptes opens the port exclusively. To flash firmware while the agent (or
  its service) is running, use `panoptes pause` ([above](#flashing-while-the-agent-runs)).
- **No temperatures on Linux:** run `sensors` (lm_sensors); if it shows nothing, load the right
  hwmon driver (`sensors-detect`).
- **No GPU:** NVIDIA needs `nvidia-smi` (ships with the driver). Intel and Apple GPUs aren't
  supported.
- **Windows temps/fans:** start LibreHardwareMonitor (or OpenHardwareMonitor) and keep it running;
  panoptes reads its WMI sensors (`root\LibreHardwareMonitor`) automatically and retries every 60 s.
- **Windows service not starting:** check *Task Scheduler → Panoptes → History* and
  `%AppData%\panoptes\panoptes.log`; `schtasks /Run /TN Panoptes` starts it by hand.
- **macOS: "panoptes cannot be opened":** `xattr -d com.apple.quarantine $(which panoptes)`.
- **See exactly what's sent:** `panoptes --dry-run -v` prints each frame plus its byte size.
