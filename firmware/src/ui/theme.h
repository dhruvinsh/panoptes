// Colours (RGB888) and fonts. One accent per theme; amber/red only for warnings.
//
// The colours are runtime variables, not constants: themeApply() copies one
// of the palettes in THEME_FAMILIES into them, and drawing code reads them
// live every frame, so a theme change shows up on the next render. Nothing
// may cache a colour (or anything drawn with one) across frames.
#pragma once
#include <stdint.h>

#include "../display.h"

namespace theme {
// Active palette. Starts as the default dark theme, so anything drawn before
// themeBegin() still looks right.
extern uint32_t BG, PANEL, PANEL_HI, LINE;  // surfaces, darkest/lightest first
extern uint32_t TEXT, SUB, DIM;             // text, caption, "missing" grey
extern uint32_t ACCENT;                     // primary series, rx
extern uint32_t ACCENT2;                    // secondary series, tx
extern uint32_t GOOD, WARN, CRIT;
extern uint32_t ON_CRIT;  // derived: black or white, whichever reads better on CRIT
}  // namespace theme

// One palette: the 12 roles above, in that order.
struct Palette {
  uint32_t bg, panel, panelHi, line, text, sub, dim, accent, accent2, good, warn, crit;
};

// A theme family has a dark and a light variant. The order of the table is
// stored in NVS by index, so only ever append to it.
struct ThemeFamily {
  const char* id;     // protocol name, e.g. "nord"
  const char* label;  // shown in the toast and on the System page
  Palette dark, light;
};
extern const ThemeFamily THEME_FAMILIES[];
extern const int THEME_COUNT;

int themeFind(const char* id);  // index of a family by protocol name, -1 if unknown
void themeApply(const Palette& p);

// Fonts (all ASCII bitmap fonts shipped with LovyanGFX)
#define F_SMALL (&fonts::DejaVu9)          // captions, ~11 px
#define F_BODY (&fonts::DejaVu12)          // body text, ~14 px
#define F_VAL (&fonts::FreeSansBold9pt7b)  // values
#define F_VAL_L (&fonts::FreeSansBold12pt7b)
#define F_BIG (&fonts::FreeSansBold18pt7b)
#define F_MONO (&fonts::FreeMonoBold9pt7b)

// layout
constexpr int HEADER_H = 24;
constexpr int CONTENT_Y = 27;
constexpr int CONTENT_B = 224;  // page dots live below this
