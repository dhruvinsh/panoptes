// Page 0: overview — CPU / RAM / GPU gauges, network sparkline, temperature.
#include <math.h>
#include <stdio.h>

#include "../format.h"
#include "pages.h"

using namespace theme;
using TD = textdatum_t;

bool dashShowsGpu() { return state.s.gpu.present; }

namespace {
void gauge(Canvas& c, int cx, float f, uint32_t col, const char* value, bool isTemp, float temp,
           const char* label, const char* sub) {
  const int cy = 82, r = 42;
  arcGauge(c, cx, cy, r, 9, f, col);
  if (isTemp) {
    tempText(c, F_VAL_L, cx, cy - 1, temp, isnan(temp) ? DIM : TEXT, TD::middle_center, false);
  } else {
    text(c, F_VAL_L, cx, cy - 1, value, isnan(f) ? DIM : TEXT, TD::middle_center);
  }
  text(c, F_SMALL, cx, cy + 32, label, SUB, TD::middle_center);
  if (sub && sub[0]) textFit(c, F_SMALL, cx, cy + 50, 100, sub, DIM, TD::middle_center);
}
}  // namespace

void drawDashboard(Canvas& c) {
  const Stats& s = state.s;
  char v[24], sub[32], a[16], b[16];

  // --- gauges --------------------------------------------------------------
  fmtPct(v, sizeof v, s.cpu.pct);
  fmtFreq(sub, sizeof sub, s.cpu.freq);
  gauge(c, 55, anim.gauge[0], levelColor(anim.gauge[0]), v, false, NAN, "CPU",
        isnan(s.cpu.freq) ? "" : sub);

  fmtPct(v, sizeof v, s.mem.pct);
  if (!isnan(s.mem.used) && !isnan(s.mem.total))
    snprintf(sub, sizeof sub, "%s / %s", fmtMiB(a, sizeof a, s.mem.used),
             fmtMiB(b, sizeof b, s.mem.total));
  else
    sub[0] = 0;
  gauge(c, 160, anim.gauge[1], levelColor(anim.gauge[1]), v, false, NAN, "RAM", sub);

  if (dashShowsGpu()) {
    fmtPct(v, sizeof v, s.gpu.pct);
    if (!isnan(s.gpu.temp))
      snprintf(sub, sizeof sub, "%.0f C", s.gpu.temp);
    else
      sub[0] = 0;
    gauge(c, 265, anim.gauge[2], levelColor(anim.gauge[2]), v, false, NAN, "GPU", sub);
  } else {
    gauge(c, 265, anim.gauge[2], tempColor(s.cpu.temp), nullptr, true, s.cpu.temp, "TEMP",
          "");
  }

  // --- network panel -------------------------------------------------------
  const int py = 146, ph = CONTENT_B - py;
  panel(c, 4, py, 216, ph);
  caption(c, 12, py + 6, "NETWORK");
  if (s.net.iface[0]) textFit(c, F_SMALL, 212, py + 6, 70, s.net.iface, DIM, TD::top_right);

  arrow(c, 16, py + 28, false, ACCENT);
  text(c, F_VAL, 25, py + 28, fmtRate(v, sizeof v, s.net.rx), isnan(s.net.rx) ? DIM : TEXT,
       TD::middle_left);
  arrow(c, 121, py + 28, true, ACCENT2);
  text(c, F_VAL, 130, py + 28, fmtRate(v, sizeof v, s.net.tx), isnan(s.net.tx) ? DIM : TEXT,
       TD::middle_left);

  const int gx = 10, gy = py + 42, gw = 204, gh = ph - 48;
  float mx = niceRateMax(fmaxf(state.hist.rx.maxv(), state.hist.tx.maxv()));
  gridLines(c, gx, gy, gw, gh, 2);
  graph(c, gx, gy, gw, gh, state.hist.rx, mx, ACCENT, true);
  graph(c, gx, gy, gw, gh, state.hist.tx, mx, ACCENT2, false);

  // --- right panel: CPU temperature (or uptime if the gauge already shows it)
  panel(c, 224, py, 92, ph);
  if (dashShowsGpu()) {
    caption(c, 270, py + 6, "CPU TEMP", SUB, TD::top_center);
    tempText(c, F_BIG, 270, py + 40, s.cpu.temp, tempColor(s.cpu.temp), TD::middle_center,
             false);
    if (!isnan(s.cpu.load[0])) {
      snprintf(sub, sizeof sub, "load %.2f", s.cpu.load[0]);
      caption(c, 270, py + 62, sub, DIM, TD::top_center);
    }
  } else {
    caption(c, 270, py + 6, "UPTIME", SUB, TD::top_center);
    text(c, F_VAL_L, 270, py + 38, fmtUptime(v, sizeof v, s.up), isnan(s.up) ? DIM : TEXT,
         TD::middle_center);
    if (!isnan(s.cpu.load[0])) {
      snprintf(sub, sizeof sub, "load %.2f", s.cpu.load[0]);
      caption(c, 270, py + 62, sub, DIM, TD::top_center);
    }
  }
}
