// Page 4: system info + link/device diagnostics.
#include <Arduino.h>
#include <math.h>
#include <stdio.h>

#include "../format.h"
#include "../sensors.h"
#include "../settings.h"
#include "../themectl.h"
#include "canvas.h"
#include "pages.h"

using namespace theme;
using TD = textdatum_t;

namespace {
void row(Canvas& c, int x, int y, int labelW, int valueW, const char* label, const char* value) {
  caption(c, x, y, label, DIM, TD::middle_left);
  bool missing = !value || !value[0];
  textFit(c, F_SMALL, x + labelW, y, valueW, missing ? "--" : value, missing ? DIM : TEXT,
          TD::middle_left);
}
}  // namespace

void drawSystem(Canvas& c) {
  const Stats& s = state.s;
  const Link& l = state.link;
  char v[64];

  // --- host ----------------------------------------------------------------
  const int px = 4, py = CONTENT_Y, pw = 312, ph = 106;
  panel(c, px, py, pw, ph);
  caption(c, px + 8, py + 5, "HOST");
  if (l.agent[0]) caption(c, px + pw - 8, py + 5, l.agent, DIM, TD::top_right);
  int y = py + 24;
  const int lw = 58, vw = pw - 16 - lw;
  row(c, px + 8, y, lw, vw, "Host", s.host);
  row(c, px + 8, y += 14, lw, vw, "OS", s.os);
  row(c, px + 8, y += 14, lw, vw, "Kernel", s.kern);
  row(c, px + 8, y += 14, lw, vw, "Uptime", isnan(s.up) ? "" : fmtUptime(v, sizeof v, s.up));
  if (s.net.ip[0] || s.net.iface[0])
    snprintf(v, sizeof v, "%s%s%s", s.net.ip, s.net.ip[0] && s.net.iface[0] ? "  on  " : "",
             s.net.iface);
  else
    v[0] = 0;
  row(c, px + 8, y += 14, lw, vw, "IP", v);
  if (!isnan(s.bat.pct))
    snprintf(v, sizeof v, "%.0f%%%s", s.bat.pct,
             s.bat.plug == 1 ? "  plugged in" : (s.bat.plug == 0 ? "  on battery" : ""));
  else
    v[0] = 0;
  row(c, px + 8, y += 14, lw, vw, "Battery", v);

  // --- device --------------------------------------------------------------
  const int dy = py + ph + 4, dh = CONTENT_B - dy;
  panel(c, px, dy, pw, dh);
  caption(c, px + 8, dy + 5, "DEVICE");
  caption(c, px + pw - 8, dy + 5, "Panoptes v" FW_VERSION, ACCENT, TD::top_right);

  const int cx1 = px + 8, cx2 = px + 164, clw = 66, cvw = 78;
  y = dy + 22;
  snprintf(v, sizeof v, "%lu", (unsigned long)l.frames);
  row(c, cx1, y, clw, cvw, "Frames", v);
  snprintf(v, sizeof v, "%lu", (unsigned long)l.parseErrors);
  row(c, cx2, y, clw, cvw, "Errors", v);

  y += 13;
  if (l.frames)
    snprintf(v, sizeof v, "%.1f s ago", msSince(l.lastFrameMs) / 1000.0f);
  else
    v[0] = 0;
  row(c, cx1, y, clw, cvw, "Last rx", v);
  snprintf(v, sizeof v, "%u KB", (unsigned)(ESP.getFreeHeap() / 1024));
  row(c, cx2, y, clw, cvw, "Heap", v);

  y += 13;
  snprintf(v, sizeof v, "%u / 255", (unsigned)backlightLevel());
  row(c, cx1, y, clw, cvw, "Backlight", v);
  // raw value and 0..1 level, for calibrating LDR_RAW_* and THEME_* in config.h
  snprintf(v, sizeof v, "%d   %.2f", ldrRaw(), fmaxf(ldrLevel(), 0));
  row(c, cx2, y, clw, cvw, "LDR", v);

  y += 13;
  row(c, cx1, y, clw, cvw, "Up", fmtUptime(v, sizeof v, anim.now / 1000.0));
  snprintf(v, sizeof v, "%d rows", canvasBandHeight());
  row(c, cx2, y, clw, cvw, "Band", v);

  y += 13;
  snprintf(v, sizeof v, "%s   %s%s", themeLabel(), themeModeName(settings.themeMode),
           settings.themeMode == THEME_MODE_AUTO ? (themeIsLight() ? " (light)" : " (dark)") : "");
  row(c, cx1, y, clw, pw - 16 - clw, "Theme", v);
}
