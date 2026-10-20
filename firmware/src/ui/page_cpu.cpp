// Page 1: CPU — per-core bars, 60 s history, load average, frequency.
#include <math.h>
#include <stdio.h>

#include "../format.h"
#include "pages.h"

using namespace theme;
using TD = textdatum_t;

void drawCpu(Canvas& c) {
  const Stats& s = state.s;
  char v[24];

  // --- title row: model + frequency ---------------------------------------
  textFit(c, F_BODY, 6, 36, 214, s.cpu.name[0] ? s.cpu.name : "CPU", s.cpu.name[0] ? TEXT : DIM,
          TD::middle_left);
  text(c, F_VAL, 314, 36, fmtFreq(v, sizeof v, s.cpu.freq), isnan(s.cpu.freq) ? DIM : ACCENT,
       TD::middle_right);

  // --- per-core grid -------------------------------------------------------
  const int px = 4, py = 48, pw = 312, ph = 76;
  panel(c, px, py, pw, ph);
  const int n = s.cpu.nCores;
  if (n == 0) {
    caption(c, px + pw / 2, py + ph / 2, "no per-core data", DIM, TD::middle_center);
  } else {
    const int rows = n > 16 ? 2 : 1;
    const int cols = (n + rows - 1) / rows;
    const int ix = px + 8, iw = pw - 16;
    const bool labels = n <= 12;
    const int top = py + 8, avail = ph - 16 - (labels ? 12 : 0);
    const int rowGap = 6;
    const int bh = (avail - (rows - 1) * rowGap) / rows;
    const int gap = cols > 12 ? 3 : 5;
    const int bw = (iw - (cols - 1) * gap) / cols;
    const int used = cols * bw + (cols - 1) * gap;
    const int x0 = ix + (iw - used) / 2;
    for (int i = 0; i < n; i++) {
      int r = i / cols, col = i % cols;
      int x = x0 + col * (bw + gap);
      int y = top + r * (bh + rowGap);
      float f = anim.cores[i];
      vbar(c, x, y, bw, bh, f, levelColor(f));
      if (labels) {
        snprintf(v, sizeof v, "%d", i);
        caption(c, x + bw / 2, y + bh + 2, v, DIM, TD::top_center);
      }
    }
  }

  // --- history graph -------------------------------------------------------
  const int gy = 128, gh = 70;
  panel(c, px, gy, pw, gh);
  caption(c, px + 8, gy + 5, "TOTAL  -  60 s");
  text(c, F_VAL, px + pw - 8, gy + 11, fmtPct(v, sizeof v, s.cpu.pct),
       isnan(s.cpu.pct) ? DIM : levelColor(s.cpu.pct / 100), TD::middle_right);
  const int ax = px + 8, ay = gy + 22, aw = pw - 16, ah = gh - 27;
  gridLines(c, ax, ay, aw, ah, 4);
  graph(c, ax, ay, aw, ah, state.hist.cpu, 100, ACCENT, true);

  // --- bottom row: load average + temperature ------------------------------
  const int by = 212;
  caption(c, 8, by, "LOAD", SUB, TD::middle_left);
  int x = 44;
  for (int i = 0; i < 3; i++) {
    fmtNum(v, sizeof v, s.cpu.load[i], 2);
    text(c, F_VAL, x, by, v, isnan(s.cpu.load[i]) ? DIM : (i == 0 ? TEXT : SUB),
         TD::middle_left);
    x += textW(c, F_VAL, v) + 12;
  }
  caption(c, 236, by, "TEMP", SUB, TD::middle_left);
  tempText(c, F_VAL, 314, by, s.cpu.temp, tempColor(s.cpu.temp), TD::middle_right);
}
