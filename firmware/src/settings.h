// Persistent runtime configuration (NVS via Preferences).
#pragma once
#include <stdint.h>

struct Settings {
  uint8_t bright = 200;      // 0..255 manual backlight
  bool autoBright = true;    // use the LDR
  bool led = true;           // RGB status LED
  bool beep = false;         // speaker chirp on alert
  uint8_t rotate = 0;        // 0 landscape, 1 landscape flipped
  uint16_t cycle = 0;        // auto-advance seconds, 0 = off
  float alertTemp = 90.0f;   // °C
  float alertCpu = 95.0f;    // %
  uint8_t page = 0;          // last page
  uint8_t theme = 0;         // index into THEME_FAMILIES
  uint8_t themeMode = 0;     // ThemeMode: 0 dark, 1 light, 2 auto
};

extern Settings settings;

void settingsLoad();
// Call after changing `settings`; the write is debounced so a burst of
// changes (or a host re-sending the same cfg) costs at most one flash write,
// and only keys whose value actually changed are written.
void settingsMarkDirty();
void settingsService(uint32_t now);
