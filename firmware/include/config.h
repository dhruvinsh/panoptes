// =============================================================================
//  Panoptes — board configuration (ESP32-2432S028R "Cheap Yellow Display")
//
//  Everything that differs between board revisions lives in this one file:
//  pins, panel driver, colour order/inversion, touch calibration, LDR range.
//  Any of the #ifndef'd values can also be overridden from platformio.ini
//  build_flags (see the [env:cyd-st7789] environment for an example).
// =============================================================================
#pragma once

#ifndef FW_VERSION  // CI release builds pass -DFW_VERSION=\"<tag>\"
#define FW_VERSION "0.2.0"
#endif
#define BOARD_NAME "ESP32-2432S028R"
#define PROTO_VERSION 1

// -----------------------------------------------------------------------------
//  Panel driver
//
//  Most single-USB (micro-USB) CYDs use an ILI9341. Some batches, and most
//  dual-USB (micro + USB-C) "CYD2USB" boards, ship an ST7789 instead.
//
//  How to switch:
//    * build the ST7789 environment:  pio run -e cyd-st7789 -t upload
//      (it just adds -DPANEL_DRIVER=2), or
//    * change PANEL_DRIVER below.
//
//  Symptoms and fixes:
//    * colours look like a photo negative (black background shows white)
//      -> flip PANEL_INVERT
//    * red and blue are swapped (the cyan accent shows as orange/yellow)
//      -> flip PANEL_RGB_ORDER
//    * picture garbled / shifted, but backlight works -> wrong PANEL_DRIVER
// -----------------------------------------------------------------------------
#define PANEL_DRIVER_ILI9341 1
#define PANEL_DRIVER_ST7789 2

//  This board (verified 2026-10-20 with diag/panel_test.cpp, mode 6) is an
//  ST7789 without inversion, despite the ILI9341 most guides list. If your
//  picture is scrambled, run `pio run -e paneltest -t upload` and tap the
//  mode that looks right.
#ifndef PANEL_DRIVER
#define PANEL_DRIVER PANEL_DRIVER_ST7789
#endif

#ifndef PANEL_INVERT
#define PANEL_INVERT false
#endif

#ifndef PANEL_RGB_ORDER  // false = BGR (normal for these panels), true = RGB
#define PANEL_RGB_ORDER false
#endif

#ifndef PANEL_SPI_FREQ  // 40 MHz is reliable on CYDs; 55/80 MHz often works too
#define PANEL_SPI_FREQ 40000000
#endif

// -----------------------------------------------------------------------------
//  Display (HSPI)
// -----------------------------------------------------------------------------
#define PIN_TFT_SCLK 14
#define PIN_TFT_MOSI 13
#define PIN_TFT_MISO 12
#define PIN_TFT_CS 15
#define PIN_TFT_DC 2
#define PIN_TFT_RST -1
#define PIN_TFT_BL 21

#define SCREEN_W 320  // landscape
#define SCREEN_H 240

// -----------------------------------------------------------------------------
//  Touch XPT2046 (separate VSPI bus)
// -----------------------------------------------------------------------------
#define PIN_TOUCH_CLK 25
#define PIN_TOUCH_MOSI 32
#define PIN_TOUCH_MISO 39
#define PIN_TOUCH_CS 33
#define PIN_TOUCH_IRQ 36

//  Raw ADC range of the touch controller across the visible area. Values are
//  raw XPT2046 readings (0..4095) in the panel's native (portrait) frame.
//  If a tap lands a few pixels off near the edges, nudge these. Enable
//  TOUCH_DEBUG to draw a marker where the firmware thinks you touched.
#ifndef TOUCH_X_MIN
#define TOUCH_X_MIN 200
#endif
#ifndef TOUCH_X_MAX
#define TOUCH_X_MAX 3700
#endif
#ifndef TOUCH_Y_MIN
#define TOUCH_Y_MIN 240
#endif
#ifndef TOUCH_Y_MAX
#define TOUCH_Y_MAX 3800
#endif
//  Touch orientation relative to the panel (0..7, LovyanGFX offset_rotation).
//  0-3 rotate by 90° steps, 4-7 are the same with one axis mirrored.
//  If left/right taps are swapped try 2; if the axes are swapped try 1 or 3;
//  if only one axis is mirrored try 4..7.
#ifndef TOUCH_OFFSET_ROTATION
#define TOUCH_OFFSET_ROTATION 0
#endif
#ifndef TOUCH_DEBUG
#define TOUCH_DEBUG 0
#endif

// -----------------------------------------------------------------------------
//  RGB LED (active LOW), LDR, speaker
// -----------------------------------------------------------------------------
#define PIN_LED_R 4
#define PIN_LED_G 16
#define PIN_LED_B 17
#define LED_MAX_DUTY 90  // default LED brightness, 0..255 (cfg.led_bright); the LED is very bright, keep it subtle

#define PIN_LDR 34
//  Raw (smoothed) ADC readings for "bright room" and "dark room". On most
//  boards the reading goes UP as it gets darker. The System page shows the
//  live raw value: read it in your brightest and darkest conditions and put
//  those numbers here. Swap them if your board reads the other way round.
#ifndef LDR_RAW_BRIGHT
#define LDR_RAW_BRIGHT 0    // measured 2026-10-20: lit room reads 0
#endif
#ifndef LDR_RAW_DARK
#define LDR_RAW_DARK 250    // measured: covered 50-210, dark room ~265
#endif

//  Automatic light/dark theme (cfg.theme_mode = "auto"). It uses the same
//  smoothed LDR reading as auto-brightness, as a light level from 0
//  (LDR_RAW_DARK) to 1 (LDR_RAW_BRIGHT). The light variant takes over once the
//  level has stayed at or above THEME_LIGHT_ABOVE for THEME_DWELL_MS, and the
//  dark one once it has stayed at or below THEME_DARK_BELOW for as long. The
//  gap between the two is the hysteresis; with the dwell it keeps a passing
//  shadow or a lamp flicking on from swapping the theme.
//  To calibrate: set LDR_RAW_BRIGHT/DARK first. Then, in the lighting where you
//  want each variant, read the level on the System page ("LDR  raw  level"),
//  or run `panoptes -v`, which logs a `light` event every LIGHT_EVENT_MS. Put
//  the two levels here and keep them at least ~0.15 apart.
#ifndef THEME_LIGHT_ABOVE
#define THEME_LIGHT_ABOVE 0.90f  // raw <= 25: a lit room reads a flat 0
#endif
#ifndef THEME_DARK_BELOW
#define THEME_DARK_BELOW 0.75f   // raw >= 62: the backlight leaks onto the LDR, so
                                  // a dark room never reaches the 265 of a covered sensor
#endif
#define THEME_DWELL_MS 20000
#define THEME_FADE_MS 180       // backlight fade out, and again in, around an auto swap
#define LIGHT_EVENT_MS 10000    // `light` event period while the host is connected

#define PIN_SPEAKER 26

// -----------------------------------------------------------------------------
//  Behaviour
// -----------------------------------------------------------------------------
#define SERIAL_BAUD 115200
#define SERIAL_RX_BUFFER 4096   // holds ~350 ms of input while a frame renders
#define MAX_LINE 2048           // protocol: max bytes per line

#define HOST_TIMEOUT_MS 5000    // no `s` frame for this long -> waiting screen
#define UI_FRAME_MS 66          // ~15 fps render loop

#define BL_AUTO_FLOOR 18        // default floor of auto-brightness (cfg.auto_bright_min)
#define BL_NIGHT 4              // default night-mode backlight level (cfg.night_bright)

#define ALERT_FRAMES_ON 5       // consecutive hot frames to enter alert
#define ALERT_FRAMES_OFF 3      // consecutive cool frames to leave alert
#define BEEP_MIN_INTERVAL_MS 60000

#define LONG_PRESS_MS 1000
#define SWIPE_MIN_PX 40
