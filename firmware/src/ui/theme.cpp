#include "theme.h"

#include <math.h>
#include <string.h>

namespace theme {
// Tuned for the glossy ST7789: pure-black background (least visible glare),
// near-white text, and mid-tones lifted so captions survive reflections.
uint32_t BG = 0x000000;
uint32_t PANEL = 0x0F161F;
uint32_t PANEL_HI = 0x1A2431;
uint32_t LINE = 0x2E3B4D;
uint32_t TEXT = 0xFFFFFF;
uint32_t SUB = 0xB4C0CE;
uint32_t DIM = 0x7A879B;
uint32_t ACCENT = 0x22D3EE;   // cyan
uint32_t ACCENT2 = 0x8B9CFF;  // periwinkle
uint32_t GOOD = 0x34D399;
uint32_t WARN = 0xFBBF24;
uint32_t CRIT = 0xF43F5E;
uint32_t ON_CRIT = 0xFFFFFF;
}  // namespace theme

// Official palette colours, mapped to the roles as
//   BG/PANEL/PANEL_HI/LINE  the family's background ramp
//   TEXT/SUB/DIM            its foreground ramp
//   ACCENT                  its signature blue/aqua, ACCENT2 its purple/mauve
//   GOOD/WARN/CRIT          its green, yellow/orange and red.
// Where a family has no suitable step (a gauge track between two surfaces, a
// readable "comment" grey) the value is a mix of two official colours, noted
// inline. Light variants keep TEXT >= ~7:1 and SUB >= ~4.5:1 against both BG
// and PANEL, as far as the official colours allow (catppuccin latte's darkest
// text colour reaches 6.9:1 on its mantle).
const ThemeFamily THEME_FAMILIES[] = {
    {"default", "Default",
     // dark: the glare-tuned original
     {0x000000, 0x0F161F, 0x1A2431, 0x2E3B4D, 0xFFFFFF, 0xB4C0CE, 0x7A879B, 0x22D3EE, 0x8B9CFF,
      0x34D399, 0xFBBF24, 0xF43F5E},
     // light: slate greys, cyan-700 / indigo-600 accents, amber-700 warning
     {0xFFFFFF, 0xF1F5F9, 0xDDE3EA, 0xCBD5E1, 0x0F172A, 0x334155, 0x64748B, 0x0E7490, 0x4F46E5,
      0x059669, 0xB45309, 0xE11D48}},
    {"gruvbox", "Gruvbox",
     // dark: bg0_h bg0 bg1 bg2 / fg0 fg2 fg4 / bright blue purple green yellow red
     {0x1D2021, 0x282828, 0x3C3836, 0x504945, 0xFBF1C7, 0xD5C4A1, 0xA89984, 0x83A598, 0xD3869B,
      0xB8BB26, 0xFABD2F, 0xFB4934},
     // light: bg0 bg0_s bg1 bg2 / fg0 fg2 fg4 / faded blue purple green yellow red
     {0xFBF1C7, 0xF2E5BC, 0xEBDBB2, 0xD5C4A1, 0x282828, 0x504945, 0x7C6F64, 0x076678, 0x8F3F71,
      0x79740E, 0xB57614, 0x9D0006}},
    {"catppuccin", "Catppuccin",
     // mocha: crust base surface0 surface1 / text subtext1 overlay1 / blue mauve green yellow red
     {0x11111B, 0x1E1E2E, 0x313244, 0x45475A, 0xCDD6F4, 0xBAC2DE, 0x7F849C, 0x89B4FA, 0xCBA6F7,
      0xA6E3A1, 0xF9E2AF, 0xF38BA8},
     // latte: base mantle surface0 surface1 / text subtext1 overlay2 / blue mauve green peach red
     {0xEFF1F5, 0xE6E9EF, 0xCCD0DA, 0xBCC0CC, 0x4C4F69, 0x5C5F77, 0x7C7F93, 0x1E66F5, 0x8839EF,
      0x40A02B, 0xFE640B, 0xD20F39}},
    {"solarized", "Solarized",
     // dark: base03 base02 (base02+30% base01) base01 / base2 base1 (base00/base0) /
     // blue violet green yellow red. Body text is base2 rather than base0, which
     // is only 4.7:1 and washes out under reflections.
     {0x002B36, 0x073642, 0x1F4751, 0x586E75, 0xEEE8D5, 0x93A1A1, 0x74888C, 0x268BD2, 0x6C71C4,
      0x859900, 0xB58900, 0xDC322F},
     // light: base3 base2 (base2+30% base1) base1 / base02 base01 base00 / accents as dark
     {0xFDF6E3, 0xEEE8D5, 0xD3D3C5, 0x93A1A1, 0x073642, 0x586E75, 0x657B83, 0x268BD2, 0x6C71C4,
      0x859900, 0xB58900, 0xDC322F}},
    {"nord", "Nord",
     // polar night: nord0-3 / nord6 nord4 (nord3+55% nord4) / nord8 nord15 nord14 nord13 nord11
     {0x2E3440, 0x3B4252, 0x434C5E, 0x4C566A, 0xECEFF4, 0xD8DEE9, 0x99A1B0, 0x88C0D0, 0xB48EAD,
      0xA3BE8C, 0xEBCB8B, 0xBF616A},
     // snow storm: nord6 nord5 nord4 (nord4+25% nord3) / nord0 nord3 (nord3+25% nord4) /
     // nord10 nord15 nord14 nord12 nord11
     {0xECEFF4, 0xE5E9F0, 0xD8DEE9, 0xB5BCC9, 0x2E3440, 0x4C566A, 0x6F788A, 0x5E81AC, 0xB48EAD,
      0xA3BE8C, 0xD08770, 0xBF616A}},
    {"dracula", "Dracula",
     // dracula: bg darker, bg, bg lighter, current line / fg, fg-comment mixes /
     // cyan purple green orange red
     {0x191A21, 0x282A36, 0x343746, 0x44475A, 0xF8F8F2, 0xC4C9D7, 0x8F9ABB, 0x8BE9FD, 0xBD93F9,
      0x50FA7B, 0xFFB86C, 0xFF5555},
     // alucard (Dracula's official light theme): bg, floating, selection-ish surfaces /
     // fg comment (comment lightened 15%) / cyan purple green yellow red
     {0xFFFBEB, 0xECE9DF, 0xDEDCCF, 0xCFCFDE, 0x1F1F1F, 0x6C664B, 0x827C63, 0x036A96, 0x644AC9,
      0x14710A, 0x846E15, 0xCB3A2A}},
    {"tokyonight", "Tokyo Night",
     // night: bg_dark bg bg_highlight fg_gutter / fg fg_dark dark5 / blue magenta green yellow red
     {0x16161E, 0x1A1B26, 0x292E42, 0x3B4261, 0xC0CAF5, 0xA9B1D6, 0x737AA2, 0x7AA2F7, 0xBB9AF7,
      0x9ECE6A, 0xE0AF68, 0xF7768E},
     // day: bg (bg/bg_dark) bg_highlight fg_gutter / fg mixed 60% and 25% toward the night
     // bg (day's own fg is only 4.5:1), dark5 / blue magenta green orange red
     {0xE1E2E7, 0xD8DCE5, 0xC4C8DA, 0xA8AECB, 0x263763, 0x304F99, 0x68709A, 0x2E7DE9, 0x9854F1,
      0x587539, 0xB15C00, 0xF52A65}},
};
const int THEME_COUNT = sizeof(THEME_FAMILIES) / sizeof(THEME_FAMILIES[0]);

namespace {
// WCAG relative luminance of an RGB888 colour
float luminance(uint32_t c) {
  auto ch = [](uint32_t v) {
    float f = v / 255.0f;
    return f <= 0.04045f ? f / 12.92f : powf((f + 0.055f) / 1.055f, 2.4f);
  };
  return 0.2126f * ch((c >> 16) & 0xFF) + 0.7152f * ch((c >> 8) & 0xFF) + 0.0722f * ch(c & 0xFF);
}
}  // namespace

int themeFind(const char* id) {
  if (!id) return -1;
  for (int i = 0; i < THEME_COUNT; i++)
    if (!strcasecmp(id, THEME_FAMILIES[i].id)) return i;
  return -1;
}

void themeApply(const Palette& p) {
  using namespace theme;
  BG = p.bg;
  PANEL = p.panel;
  PANEL_HI = p.panelHi;
  LINE = p.line;
  TEXT = p.text;
  SUB = p.sub;
  DIM = p.dim;
  ACCENT = p.accent;
  ACCENT2 = p.accent2;
  GOOD = p.good;
  WARN = p.warn;
  CRIT = p.crit;
  // White unless it falls below 3:1 on a pastel red (catppuccin, tokyonight).
  ON_CRIT = 1.05f / (luminance(CRIT) + 0.05f) >= 3.0f ? 0xFFFFFF : 0x000000;
}
