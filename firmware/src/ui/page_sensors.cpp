// Page 3: sensors + processes — temperatures, fans, top-5 table, GPU/memory.
#include <math.h>
#include <stdio.h>

#include "../format.h"
#include "pages.h"

using namespace theme;
using TD = textdatum_t;

void drawSensors(Canvas& c) {
  const Stats& s = state.s;
  char v[32], a[16], b[16];

  // --- left: temperatures + fans -------------------------------------------
  const int lx = 4, ly = CONTENT_Y, lw = 154, lh = CONTENT_B - CONTENT_Y;
  panel(c, lx, ly, lw, lh);
  caption(c, lx + 8, ly + 5, "TEMPERATURES");
  int y = ly + 26;
  if (s.nTemps == 0) {
    caption(c, lx + 8, y, "none reported", DIM, TD::middle_left);
    y += 16;
  }
  for (int i = 0; i < s.nTemps; i++, y += 16) {
    const NamedValue& t = s.temps[i];
    textFit(c, F_SMALL, lx + 8, y, 50, t.name[0] ? t.name : "?", TEXT, TD::middle_left);
    hbar(c, lx + 62, y - 3, 48, 6, t.v / 100.0f, tempColor(t.v));
    tempText(c, F_SMALL, lx + lw - 8, y, t.v, tempColor(t.v), TD::middle_right);
  }

  y += 2;
  c.drawFastHLine(lx + 8, y - 6, lw - 16, LINE);
  caption(c, lx + 8, y, "FANS");
  y += 16;
  if (s.nFans == 0) caption(c, lx + 8, y, "none reported", DIM, TD::middle_left);
  for (int i = 0; i < s.nFans && y < CONTENT_B - 6; i++, y += 14) {
    const NamedValue& f = s.fans[i];
    textFit(c, F_SMALL, lx + 8, y, 70, f.name[0] ? f.name : "fan", TEXT, TD::middle_left);
    snprintf(v, sizeof v, "%.0f rpm", f.v);
    caption(c, lx + lw - 8, y, v, f.v > 0 ? ACCENT : DIM, TD::middle_right);
  }

  // --- right: processes ----------------------------------------------------
  const int rx = 162, rw = 154, rh = 116;
  panel(c, rx, ly, rw, rh);
  caption(c, rx + 8, ly + 5, "TOP 5");
  caption(c, rx + 112, ly + 5, "CPU", DIM, TD::top_right);
  caption(c, rx + rw - 8, ly + 5, "MEM", DIM, TD::top_right);
  y = ly + 27;
  if (s.nProcs == 0) caption(c, rx + 8, y, "none reported", DIM, TD::middle_left);
  for (int i = 0; i < s.nProcs; i++, y += 18) {
    const Proc& p = s.procs[i];
    if (i % 2 == 0) c.fillRect(rx + 4, y - 8, rw - 8, 17, blend(PANEL, PANEL_HI, 0.5f));
    textFit(c, F_SMALL, rx + 8, y, 74, p.n, TEXT, TD::middle_left);
    snprintf(v, sizeof v, isnan(p.c) ? "--" : "%.1f", p.c);
    caption(c, rx + 112, y, v, isnan(p.c) ? DIM : levelColor(p.c / 100), TD::middle_right);
    snprintf(v, sizeof v, isnan(p.m) ? "--" : "%.1f", p.m);
    caption(c, rx + rw - 8, y, v, SUB, TD::middle_right);
  }

  // --- right bottom: GPU details, or memory if there is no GPU --------------
  const int gy = ly + rh + 4, gh = CONTENT_B - gy;
  panel(c, rx, gy, rw, gh);
  if (s.gpu.present) {
    caption(c, rx + 8, gy + 5, "GPU");
    if (s.gpu.name[0]) textFit(c, F_SMALL, rx + rw - 8, gy + 5, 110, s.gpu.name, DIM, TD::top_right);
    text(c, F_VAL_L, rx + 8, gy + 34, fmtPct(v, sizeof v, s.gpu.pct),
         levelColor(s.gpu.pct / 100), TD::middle_left);
    tempText(c, F_VAL, rx + rw - 8, gy + 34, s.gpu.temp, tempColor(s.gpu.temp),
             TD::middle_right);
    int yy = gy + 58;
    if (!isnan(s.gpu.memUsed) && !isnan(s.gpu.memTotal)) {
      snprintf(v, sizeof v, "VRAM %s/%s", fmtMiB(a, sizeof a, s.gpu.memUsed),
               fmtMiB(b, sizeof b, s.gpu.memTotal));
      caption(c, rx + 8, yy, v, SUB, TD::middle_left);
    }
    if (!isnan(s.gpu.power)) {
      snprintf(v, sizeof v, "%.0f W", s.gpu.power);
      caption(c, rx + rw - 8, yy, v, SUB, TD::middle_right);
    }
  } else {
    caption(c, rx + 8, gy + 5, "MEMORY");
    text(c, F_VAL_L, rx + 8, gy + 34, fmtPct(v, sizeof v, s.mem.pct),
         levelColor(s.mem.pct / 100), TD::middle_left);
    if (!isnan(s.mem.used) && !isnan(s.mem.total)) {
      snprintf(v, sizeof v, "%s / %s", fmtMiB(a, sizeof a, s.mem.used),
               fmtMiB(b, sizeof b, s.mem.total));
      caption(c, rx + rw - 8, gy + 34, v, SUB, TD::middle_right);
    }
    if (!isnan(s.mem.swapPct)) {
      snprintf(v, sizeof v, "swap %.0f%%", s.mem.swapPct);
      caption(c, rx + 8, gy + 58, v, SUB, TD::middle_left);
    }
  }
}
