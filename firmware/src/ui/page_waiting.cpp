// "Waiting for host" screen: orbiting comet around a breathing core.
#include <math.h>
#include <stdio.h>

#include "pages.h"

using namespace theme;
using TD = textdatum_t;

void drawWaiting(Canvas& c) {
  const uint32_t t = anim.now;
  const int cx = SCREEN_W / 2, cy = 88;
  c.fillScreen(BG);

  // subtle radial halo
  for (int r = 70; r > 52; r -= 3) c.drawCircle(cx, cy, r, blend(BG, ACCENT, 0.03f * (70 - r) / 18));

  // track ring
  c.fillArc(cx, cy, 44, 48, 0, 360, PANEL_HI);

  // comet: a bright head with a fading tail, one revolution every 1.6 s
  const float head = fmodf(t * (360.0f / 1600.0f), 360.0f);
  const int segs = 14;
  for (int i = segs; i >= 0; i--) {
    float a1 = head - i * 9.0f, a0 = a1 - 9.5f;
    uint32_t col = blend(PANEL_HI, ACCENT, 1.0f - (float)i / segs);
    float s0 = fmodf(a0 + 720, 360), s1 = s0 + (a1 - a0);
    if (s1 > 360) {
      c.fillArc(cx, cy, 44, 48, s0, 360, col);
      c.fillArc(cx, cy, 44, 48, 0, s1 - 360, col);
    } else {
      c.fillArc(cx, cy, 44, 48, s0, s1, col);
    }
  }
  {
    float a = head * (float)M_PI / 180;
    c.fillSmoothCircle(cx + (int)lroundf(cosf(a) * 46), cy + (int)lroundf(sinf(a) * 46), 4, TEXT);
  }

  // breathing core (2.4 s)
  float ph = 0.5f - 0.5f * cosf((t % 2400) / 2400.0f * 2 * (float)M_PI);
  int r = 14 + (int)lroundf(ph * 8);
  c.fillSmoothCircle(cx, cy, r + 6, blend(BG, ACCENT, 0.10f + 0.10f * ph));
  c.fillSmoothCircle(cx, cy, r, blend(PANEL_HI, ACCENT, 0.35f + 0.45f * ph));
  c.fillSmoothCircle(cx, cy, 5, TEXT);

  // text
  text(c, F_VAL_L, cx, 160, "Waiting for host", TEXT, TD::middle_center);

  char msg[64];
  const Link& l = state.link;
  if (l.frames > 0) {
    snprintf(msg, sizeof msg, "connection lost %lu s ago",
             (unsigned long)((t - l.lastFrameMs) / 1000));
  } else if (l.helloCount > 0) {
    snprintf(msg, sizeof msg, "host found, waiting for data...");
  } else {
    snprintf(msg, sizeof msg, "on the PC this is plugged into:");
  }
  caption(c, cx, 182, msg, SUB, TD::middle_center);

  // command pill
  const char* cmd = "panoptes";
  int w = textW(c, F_MONO, cmd) + textW(c, F_MONO, "$ ") + 28;
  c.fillSmoothRoundRect(cx - w / 2, 194, w, 24, 12, PANEL_HI);
  c.drawRoundRect(cx - w / 2, 194, w, 24, 12, blend(PANEL_HI, ACCENT, 0.5f));
  int x = cx - w / 2 + 14;
  text(c, F_MONO, x, 206, "$ ", DIM, TD::middle_left);
  text(c, F_MONO, x + textW(c, F_MONO, "$ "), 206, cmd, ACCENT, TD::middle_left);

  caption(c, cx, 232, "Panoptes v" FW_VERSION "   USB serial 115200", DIM, TD::middle_center);
}
