#include "widgets.h"

#include <math.h>
#include <string.h>

#include <utility>

using namespace theme;

uint32_t blend(uint32_t a, uint32_t b, float t) {
  t = t < 0 ? 0 : (t > 1 ? 1 : t);
  auto ch = [&](int s) {
    float ca = (a >> s) & 0xFF, cb = (b >> s) & 0xFF;
    return (uint32_t)lroundf(ca + (cb - ca) * t) << s;
  };
  return ch(16) | ch(8) | ch(0);
}

uint32_t levelColor(float f) {
  if (isnan(f)) return DIM;
  if (f < 0.6f) return ACCENT;
  if (f < 0.85f) return blend(ACCENT, WARN, (f - 0.6f) / 0.25f);
  return blend(WARN, CRIT, (f - 0.85f) / 0.15f);
}

uint32_t tempColor(float t) {
  if (isnan(t)) return DIM;
  if (t < 60) return ACCENT;
  if (t < 75) return blend(ACCENT, WARN, (t - 60) / 15);
  return blend(WARN, CRIT, (t - 75) / 15);
}

void panel(Canvas& c, int x, int y, int w, int h) { c.fillSmoothRoundRect(x, y, w, h, 6, PANEL); }

int textW(Canvas& c, Font f, const char* s) {
  c.setFont(f);
  return c.textWidth(s);
}

void text(Canvas& c, Font f, int x, int y, const char* s, uint32_t col, textdatum_t d) {
  c.setFont(f);
  c.setTextColor(col);
  c.setTextDatum(d);
  c.drawString(s, x, y);
}

void textFit(Canvas& c, Font f, int x, int y, int maxW, const char* s, uint32_t col,
             textdatum_t d) {
  c.setFont(f);
  if (c.textWidth(s) <= maxW) {
    text(c, f, x, y, s, col, d);
    return;
  }
  char tmp[96];
  size_t n = strlen(s);
  if (n > sizeof(tmp) - 3) n = sizeof(tmp) - 3;
  while (n > 0) {
    memcpy(tmp, s, n);
    while (n > 0 && tmp[n - 1] == ' ') n--;  // no "foo .."
    strcpy(tmp + n, "..");
    if (c.textWidth(tmp) <= maxW) break;
    n--;
  }
  text(c, f, x, y, tmp, col, d);
}

void tempText(Canvas& c, Font f, int x, int y, float t, uint32_t col, textdatum_t d,
              bool unitC) {
  char num[12];
  if (isnan(t))
    strcpy(num, "--");
  else
    snprintf(num, sizeof(num), "%.0f", t);
  c.setFont(f);
  const int fh = c.fontHeight();
  const int wn = c.textWidth(num);
  const int ringR = fh >= 20 ? 3 : 2;
  const int wc = unitC ? c.textWidth("C") : 0;
  const int total = wn + 2 + ringR * 2 + 1 + (unitC ? 1 + wc : 0);
  int h = (int)d & 3;  // 0 left, 1 center, 2 right
  int left = h == 0 ? x : (h == 1 ? x - total / 2 : x - total);
  text(c, f, left, y, num, col, textdatum_t::middle_left);
  if (isnan(t)) return;
  int rx = left + wn + 2 + ringR;
  int ry = y - fh / 2 + ringR + (fh >= 20 ? 4 : 2);
  c.drawCircle(rx, ry, ringR, col);
  if (ringR > 2) c.drawCircle(rx, ry, ringR - 1, col);
  if (unitC) text(c, f, rx + ringR + 2, y, "C", col, textdatum_t::middle_left);
}

void arrow(Canvas& c, int x, int y, bool up, uint32_t col) {
  // 9 px wide, 6 px tall, centred on (x, y)
  if (up)
    c.fillTriangle(x - 4, y + 3, x + 4, y + 3, x, y - 3, col);
  else
    c.fillTriangle(x - 4, y - 3, x + 4, y - 3, x, y + 3, col);
}

void hbar(Canvas& c, int x, int y, int w, int h, float f, uint32_t col) {
  int r = h / 2;
  c.fillSmoothRoundRect(x, y, w, h, r, PANEL_HI);
  if (isnan(f) || f <= 0) return;
  if (f > 1) f = 1;
  int fw = (int)lroundf(w * f);
  if (fw < h) fw = h;  // keep the rounded cap
  c.fillSmoothRoundRect(x, y, fw, h, r, col);
}

void vbar(Canvas& c, int x, int y, int w, int h, float f, uint32_t col) {
  int r = w >= 6 ? 2 : 0;
  c.fillRoundRect(x, y, w, h, r, PANEL_HI);
  if (isnan(f) || f <= 0) return;
  if (f > 1) f = 1;
  int fh = (int)lroundf(h * f);
  if (fh < 2) fh = 2;
  c.fillRoundRect(x, y + h - fh, w, fh, r, col);
}

namespace {
// fillArc with an angle range that may cross 360°
void arc(Canvas& c, int cx, int cy, int r0, int r1, float a0, float a1, uint32_t col) {
  if (a1 <= a0) return;
  if (a1 > 360) {
    if (a0 < 360) c.fillArc(cx, cy, r0, r1, a0, 360, col);
    c.fillArc(cx, cy, r0, r1, a0 < 360 ? 0 : a0 - 360, a1 - 360, col);
  } else {
    c.fillArc(cx, cy, r0, r1, a0, a1, col);
  }
}

void cap(Canvas& c, int cx, int cy, float rMid, float rad, float deg, uint32_t col) {
  float a = deg * (float)M_PI / 180.0f;
  c.fillSmoothCircle((int)lroundf(cx + cosf(a) * rMid), (int)lroundf(cy + sinf(a) * rMid),
                     (int)rad, col);
}
}  // namespace

void arcGauge(Canvas& c, int cx, int cy, int r, int th, float f, uint32_t col) {
  constexpr float A0 = 135, SWEEP = 270;
  const int ri = r - th + 1;
  const float rMid = (r + ri) / 2.0f, capR = (th - 1) / 2.0f;
  arc(c, cx, cy, ri, r, A0, A0 + SWEEP, PANEL_HI);
  cap(c, cx, cy, rMid, capR, A0, PANEL_HI);
  cap(c, cx, cy, rMid, capR, A0 + SWEEP, PANEL_HI);
  if (isnan(f)) return;
  f = f < 0 ? 0 : (f > 1 ? 1 : f);
  float a1 = A0 + SWEEP * f;
  // soft glow under the value arc
  arc(c, cx, cy, ri - 3, ri - 1, A0, a1, blend(BG, col, 0.18f));
  arc(c, cx, cy, ri, r, A0, a1, col);
  cap(c, cx, cy, rMid, capR, A0, col);
  cap(c, cx, cy, rMid, capR, a1, col);
}

namespace {
// Blend `col` into one pixel at coverage a (0..1). Checks the clip rect
// itself because readPixel() does not.
void plot(Canvas& c, int x, int y, uint32_t col, float a, int cl, int ct, int cr, int cb) {
  if (x < cl || x > cr || y < ct || y > cb || a <= 0.02f) return;
  if (a >= 0.98f) {
    c.drawPixel(x, y, col);
    return;
  }
  uint16_t raw = c.readPixel(x, y);
  uint32_t bg = ((raw >> 11) & 0x1F) * 255 / 31 << 16 | ((raw >> 5) & 0x3F) * 255 / 63 << 8 |
                (raw & 0x1F) * 255 / 31;
  c.drawPixel(x, y, blend(bg, col, a));
}
}  // namespace

// Anti-aliased line (Xiaolin Wu). LovyanGFX's drawSmoothLine()/drawWideLine()
// are not used because they replace the clip rect, which the band renderer
// relies on (see canvas.h).
void aaLine(Canvas& c, int x0, int y0, int x1, int y1, uint32_t col) {
  int32_t cx, cy, cw, ch;
  c.getClipRect(&cx, &cy, &cw, &ch);
  const int cl = cx, ct = cy, cr = cx + cw - 1, cb = cy + ch - 1;
  bool steep = abs(y1 - y0) > abs(x1 - x0);
  if (steep) {
    std::swap(x0, y0);
    std::swap(x1, y1);
  }
  if (x0 > x1) {
    std::swap(x0, x1);
    std::swap(y0, y1);
  }
  const float dx = x1 - x0, dy = y1 - y0;
  const float g = dx == 0 ? 1 : dy / dx;
  float y = y0;
  for (int x = x0; x <= x1; x++, y += g) {
    int yi = (int)floorf(y);
    float f = y - yi;
    if (steep) {
      plot(c, yi, x, col, 1 - f, cl, ct, cr, cb);
      plot(c, yi + 1, x, col, f, cl, ct, cr, cb);
    } else {
      plot(c, x, yi, col, 1 - f, cl, ct, cr, cb);
      plot(c, x, yi + 1, col, f, cl, ct, cr, cb);
    }
  }
}

void gridLines(Canvas& c, int x, int y, int w, int h, int div) {
  for (int i = 1; i < div; i++) {
    int yy = y + h * i / div;
    for (int xx = x; xx < x + w; xx += 4) c.drawPixel(xx, yy, LINE);
  }
}

void graph(Canvas& c, int x, int y, int w, int h, const Ring<HIST_LEN>& r, float maxv,
           uint32_t col, bool fill) {
  if (r.count < 1 || !(maxv > 0) || w < 2) return;
  const int N = HIST_LEN;
  const uint32_t top = blend(PANEL, col, 0.45f), bot = blend(PANEL, col, 0.04f);
  int px = -1, py = 0;
  for (int i = 0; i < w; i++) {
    // map column -> fractional sample index (newest at right edge)
    float s = (float)i * (N - 1) / (w - 1) - (N - r.count);
    if (s < 0) {
      px = -1;
      continue;
    }
    int i0 = (int)s;
    int i1 = i0 + 1 < r.count ? i0 + 1 : i0;
    float a = r.at(i0), b = r.at(i1);
    if (isnan(a) || isnan(b)) {
      px = -1;
      continue;
    }
    float v = a + (b - a) * (s - i0);
    float f = v / maxv;
    f = f < 0 ? 0 : (f > 1 ? 1 : f);
    int yy = y + h - 1 - (int)lroundf(f * (h - 1));
    if (fill && yy < y + h - 1) c.drawGradientVLine(x + i, yy, y + h - yy, top, bot);
    if (px >= 0)
      aaLine(c, px, py, x + i, yy, col);
    else
      c.drawPixel(x + i, yy, col);
    px = x + i;
    py = yy;
  }
}

void pageDots(Canvas& c, int cur, int n) {
  const int gap = 12, y = 233;
  int x = SCREEN_W / 2 - (n - 1) * gap / 2;
  for (int i = 0; i < n; i++, x += gap) {
    if (i == cur)
      c.fillSmoothRoundRect(x - 6, y - 2, 13, 5, 2, ACCENT);
    else
      c.fillSmoothCircle(x, y, 2, DIM);
  }
}
