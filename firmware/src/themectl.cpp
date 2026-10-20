#include "themectl.h"

#include <Arduino.h>
#include <stdio.h>
#include <string.h>

#include "config.h"
#include "protocol.h"
#include "sensors.h"
#include "settings.h"
#include "ui/theme.h"
#include "ui/ui.h"

namespace {
const char* const MODE_NAMES[THEME_MODE_COUNT] = {"dark", "light", "auto"};

int family = -1;       // applied family, -1 before themeBegin()
bool light = false;    // applied variant
bool started = false;  // themeUpdate() has run (the first frame is about to render)

// last state reported to the host
int sentFamily = -1;
bool sentLight = false;
uint8_t sentMode = 0;

// auto mode
bool autoValid = false;   // autoLight has been seeded from the LDR
bool autoLight = false;   // variant auto mode currently wants
bool crossing = false;    // level is past the switch threshold...
uint32_t crossSince = 0;  // ...since this time

// backlight fade around a dark <-> light swap: out, swap, in
enum Fade : uint8_t { FADE_NONE, FADE_OUT, FADE_IN };
Fade fade = FADE_NONE;
uint32_t fadeT0 = 0;
bool fadeTo = false;

int clampFamily(int f) { return f >= 0 && f < THEME_COUNT ? f : 0; }
uint8_t clampMode(uint8_t m) { return m < THEME_MODE_COUNT ? m : (uint8_t)THEME_MODE_DARK; }

void apply(int f, bool l) {
  const ThemeFamily& t = THEME_FAMILIES[f];
  themeApply(l ? t.light : t.dark);
  family = f;
  light = l;
  uiInvalidate();
}

// Variant wanted in auto mode. The first reading picks a side straight away
// (splitting the hysteresis gap); after that the level must stay past the
// far threshold for THEME_DWELL_MS before the variant flips.
bool autoWant(uint32_t now) {
  const float level = ldrLevel();
  if (level < 0) return light;  // no sample yet
  if (!autoValid) {
    autoValid = true;
    autoLight = level >= (THEME_LIGHT_ABOVE + THEME_DARK_BELOW) / 2;
    crossing = false;
    return autoLight;
  }
  bool past = autoLight ? level <= THEME_DARK_BELOW : level >= THEME_LIGHT_ABOVE;
  if (!past) {
    crossing = false;
  } else if (!crossing) {
    crossing = true;
    crossSince = now;
  } else if (now - crossSince >= THEME_DWELL_MS) {
    autoLight = !autoLight;
    crossing = false;
  }
  return autoLight;
}

float ramp(uint32_t now) {
  float k = (float)(now - fadeT0) / THEME_FADE_MS;
  return k > 1 ? 1 : k;
}
}  // namespace

void themeBegin() {
  // auto mode starts dark; the first themeUpdate() after an LDR sample
  // corrects it before anything is rendered
  apply(clampFamily(settings.theme), clampMode(settings.themeMode) == THEME_MODE_LIGHT);
  sentFamily = family;
  sentLight = light;
  sentMode = clampMode(settings.themeMode);
}

void themeUpdate(uint32_t now) {
  const int f = clampFamily(settings.theme);
  const uint8_t m = clampMode(settings.themeMode);
  if (m != THEME_MODE_AUTO) autoValid = false;  // re-seed when auto is picked again
  const bool want = m == THEME_MODE_AUTO ? autoWant(now) : m == THEME_MODE_LIGHT;

  if (f != family) apply(f, light);  // family change: instant

  if (fade == FADE_OUT) {
    float k = ramp(now);
    backlightSetGain(1 - k);
    if (k >= 1) {
      apply(family, fadeTo);  // swap while the backlight is off
      fade = FADE_IN;
      fadeT0 = now;
    }
  } else if (fade == FADE_IN) {
    float k = ramp(now);
    backlightSetGain(k);
    if (k >= 1) fade = FADE_NONE;
  }
  if (fade == FADE_NONE && want != light) {
    if (!started) {
      apply(family, want);  // first decision at boot: nothing rendered yet
    } else {
      fade = FADE_OUT;
      fadeT0 = now;
      fadeTo = want;
    }
  }

  started = true;
  // report settled states only, not the old variant mid-fade
  if (fade == FADE_NONE && (family != sentFamily || light != sentLight || m != sentMode))
    themeSendEvent();
}

void themeCycleFamily() {
  settings.theme = (uint8_t)((clampFamily(settings.theme) + 1) % THEME_COUNT);
  settingsMarkDirty();
  char msg[40];
  snprintf(msg, sizeof msg, "Theme: %s", THEME_FAMILIES[settings.theme].label);
  uiToast(msg);
}

int themeModeFind(const char* name) {
  if (!name) return -1;
  for (int i = 0; i < THEME_MODE_COUNT; i++)
    if (!strcasecmp(name, MODE_NAMES[i])) return i;
  return -1;
}

const char* themeModeName(uint8_t mode) { return MODE_NAMES[clampMode(mode)]; }
const char* themeLabel() { return THEME_FAMILIES[clampFamily(family)].label; }
bool themeIsLight() { return light; }

void themeSendEvent() {
  sentFamily = family;
  sentLight = light;
  sentMode = clampMode(settings.themeMode);
  sendThemeEvent(THEME_FAMILIES[clampFamily(family)].id, MODE_NAMES[sentMode],
                 light ? "light" : "dark");
}
