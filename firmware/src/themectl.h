// Theme controller: which family is shown (cfg.theme, header tap) and in which
// variant (cfg.theme_mode dark / light / auto from the LDR, with hysteresis
// and a dwell time). Reads `settings` live; the palettes are in ui/theme.cpp.
#pragma once
#include <stdint.h>

enum ThemeMode : uint8_t { THEME_MODE_DARK = 0, THEME_MODE_LIGHT, THEME_MODE_AUTO, THEME_MODE_COUNT };

void themeBegin();               // apply the persisted theme before the first frame
void themeUpdate(uint32_t now);  // follow settings and the LDR; runs the swap fade
void themeCycleFamily();         // next family + toast (header tap), persisted

int themeModeFind(const char* name);  // "dark"/"light"/"auto" -> ThemeMode, -1 if unknown
const char* themeModeName(uint8_t mode);
const char* themeLabel();  // active family, e.g. "Nord"
bool themeIsLight();       // variant currently on screen
void themeSendEvent();     // `ev` "theme" with the current state
