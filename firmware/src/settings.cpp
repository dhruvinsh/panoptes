#include "settings.h"

#include <Arduino.h>
#include <Preferences.h>

Settings settings;

namespace {
Preferences prefs;
Settings saved;  // what is currently in flash
bool dirty = false;
uint32_t dirtySince = 0;
constexpr uint32_t SAVE_DELAY_MS = 3000;
constexpr const char* NS = "cydmon";  // pre-rename NVS namespace, kept so saved settings survive

bool same(const Settings& a, const Settings& b) {
  return a.bright == b.bright && a.autoBright == b.autoBright && a.led == b.led &&
         a.ledBright == b.ledBright && a.blMin == b.blMin && a.blNight == b.blNight &&
         a.beep == b.beep && a.rotate == b.rotate && a.cycle == b.cycle &&
         a.alertTemp == b.alertTemp && a.alertCpu == b.alertCpu && a.page == b.page &&
         a.theme == b.theme && a.themeMode == b.themeMode;
}
}  // namespace

void settingsLoad() {
  Settings d;  // defaults
  if (prefs.begin(NS, true)) {
    d.bright = prefs.getUChar("bright", d.bright);
    d.autoBright = prefs.getBool("auto", d.autoBright);
    d.led = prefs.getBool("led", d.led);
    d.ledBright = prefs.getUChar("ledb", d.ledBright);
    d.blMin = prefs.getUChar("blmin", d.blMin);
    d.blNight = prefs.getUChar("blnight", d.blNight);
    d.beep = prefs.getBool("beep", d.beep);
    d.rotate = prefs.getUChar("rotate", d.rotate) ? 1 : 0;
    d.cycle = prefs.getUShort("cycle", d.cycle);
    d.alertTemp = prefs.getFloat("atemp", d.alertTemp);
    d.alertCpu = prefs.getFloat("acpu", d.alertCpu);
    d.page = prefs.getUChar("page", d.page);
    d.theme = prefs.getUChar("theme", d.theme);
    d.themeMode = prefs.getUChar("tmode", d.themeMode);
    prefs.end();
  }
  settings = d;
  saved = d;
}

void settingsMarkDirty() {
  if (!dirty) dirtySince = millis();
  dirty = true;
}

void settingsService(uint32_t now) {
  if (!dirty || now - dirtySince < SAVE_DELAY_MS) return;
  dirty = false;
  const Settings& s = settings;
  if (same(s, saved)) return;
  if (!prefs.begin(NS, false)) return;
  if (s.bright != saved.bright) prefs.putUChar("bright", s.bright);
  if (s.autoBright != saved.autoBright) prefs.putBool("auto", s.autoBright);
  if (s.led != saved.led) prefs.putBool("led", s.led);
  if (s.ledBright != saved.ledBright) prefs.putUChar("ledb", s.ledBright);
  if (s.blMin != saved.blMin) prefs.putUChar("blmin", s.blMin);
  if (s.blNight != saved.blNight) prefs.putUChar("blnight", s.blNight);
  if (s.beep != saved.beep) prefs.putBool("beep", s.beep);
  if (s.rotate != saved.rotate) prefs.putUChar("rotate", s.rotate);
  if (s.cycle != saved.cycle) prefs.putUShort("cycle", s.cycle);
  if (s.alertTemp != saved.alertTemp) prefs.putFloat("atemp", s.alertTemp);
  if (s.alertCpu != saved.alertCpu) prefs.putFloat("acpu", s.alertCpu);
  if (s.page != saved.page) prefs.putUChar("page", s.page);
  if (s.theme != saved.theme) prefs.putUChar("theme", s.theme);
  if (s.themeMode != saved.themeMode) prefs.putUChar("tmode", s.themeMode);
  prefs.end();
  saved = s;
}
