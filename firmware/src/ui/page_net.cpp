// Page 2: network + disk — dual auto-scaling graph, disk rates, mount usage.
#include <math.h>
#include <stdio.h>

#include "../format.h"
#include "pages.h"

using namespace theme;
using TD = textdatum_t;

float niceRateMax(float v) {
  if (!(v > 0)) return 1024;  // empty graph: 1 KB/s scale
  float unit = 1;
  while (v >= 1000) {
    v /= 1024;
    unit *= 1024;
  }
  float m = niceCeil(v);
  if (m < 1) m = 1;
  return m * unit;
}

void drawNetDisk(Canvas& c) {
  const Stats& s = state.s;
  char v[24], w[24];

  // --- network graph -------------------------------------------------------
  const int px = 4, py = CONTENT_Y, pw = 312, ph = 116;
  panel(c, px, py, pw, ph);
  arrow(c, px + 12, py + 13, false, ACCENT);
  text(c, F_VAL, px + 21, py + 13, fmtRate(v, sizeof v, s.net.rx), isnan(s.net.rx) ? DIM : TEXT,
       TD::middle_left);
  arrow(c, px + 122, py + 13, true, ACCENT2);
  text(c, F_VAL, px + 131, py + 13, fmtRate(v, sizeof v, s.net.tx),
       isnan(s.net.tx) ? DIM : TEXT, TD::middle_left);
  if (s.net.iface[0])
    textFit(c, F_SMALL, px + pw - 8, py + 13, 70, s.net.iface, DIM, TD::middle_right);

  const int gx = px + 8, gy = py + 30, gw = pw - 16, gh = ph - 36;
  float mx = niceRateMax(fmaxf(state.hist.rx.maxv(), state.hist.tx.maxv()));
  gridLines(c, gx, gy, gw, gh, 4);
  graph(c, gx, gy, gw, gh, state.hist.rx, mx, ACCENT, true);
  graph(c, gx, gy, gw, gh, state.hist.tx, mx, ACCENT2, false);
  fmtRate(v, sizeof v, mx);
  caption(c, gx + 2, gy + 1, v, DIM);

  // --- disk rates ----------------------------------------------------------
  const int dy = 150;
  caption(c, 8, dy, "DISK", SUB, TD::middle_left);
  caption(c, 48, dy, "R", DIM, TD::middle_left);
  text(c, F_VAL, 60, dy, fmtRate(v, sizeof v, s.disk.r), isnan(s.disk.r) ? DIM : TEXT,
       TD::middle_left);
  caption(c, 176, dy, "W", DIM, TD::middle_left);
  text(c, F_VAL, 190, dy, fmtRate(v, sizeof v, s.disk.w), isnan(s.disk.w) ? DIM : TEXT,
       TD::middle_left);

  // --- mounts --------------------------------------------------------------
  const int my = 162, mh = CONTENT_B - my;
  panel(c, px, my, pw, mh);
  if (s.disk.nMounts == 0) {
    caption(c, px + pw / 2, my + mh / 2, "no mount data", DIM, TD::middle_center);
    return;
  }
  const int rowH = mh / 4 < 15 ? mh / 4 : 15;
  int y = my + (mh - rowH * s.disk.nMounts) / 2 + rowH / 2;
  for (int i = 0; i < s.disk.nMounts; i++, y += rowH) {
    const Mount& m = s.disk.mounts[i];
    textFit(c, F_SMALL, px + 8, y, 60, m.m[0] ? m.m : "?", TEXT, TD::middle_left);
    float f = m.pct / 100;
    hbar(c, px + 72, y - 3, 130, 7, f, levelColor(f));
    caption(c, px + 236, y, fmtPct(v, sizeof v, m.pct), SUB, TD::middle_right);
    if (!isnan(m.used) && !isnan(m.total)) {
      char u[12], t[12];
      snprintf(w, sizeof w, "%s/%s", fmtGiB(u, sizeof u, m.used), fmtGiB(t, sizeof t, m.total));
      caption(c, px + pw - 8, y, w, DIM, TD::middle_right);
    }
  }
}
