// Reusable drawing primitives. All take screen coordinates.
#pragma once
#include "../protocol.h"
#include "canvas.h"
#include "theme.h"

using lgfx::textdatum_t;
using Font = const lgfx::IFont*;

uint32_t blend(uint32_t a, uint32_t b, float t);  // t=0 -> a, t=1 -> b
uint32_t levelColor(float frac);                  // 0..1 load -> accent/amber/red
uint32_t tempColor(float celsius);

void panel(Canvas& c, int x, int y, int w, int h);
void text(Canvas& c, Font f, int x, int y, const char* s, uint32_t col,
          textdatum_t d = textdatum_t::top_left);
// Like text(), but truncates with ".." to fit maxW pixels.
void textFit(Canvas& c, Font f, int x, int y, int maxW, const char* s, uint32_t col,
             textdatum_t d = textdatum_t::top_left);
inline void caption(Canvas& c, int x, int y, const char* s, uint32_t col = theme::SUB,
                    textdatum_t d = textdatum_t::top_left) {
  text(c, F_SMALL, x, y, s, col, d);
}
// Temperature with a drawn degree ring, e.g. "55°C". Vertical alignment is
// always "middle"; horizontal follows d (left/center/right).
void tempText(Canvas& c, Font f, int x, int y, float celsius, uint32_t col,
              textdatum_t d = textdatum_t::middle_left, bool unitC = true);
int textW(Canvas& c, Font f, const char* s);

// Small up/down triangles for rx/tx.
void arrow(Canvas& c, int x, int y, bool up, uint32_t col);

void hbar(Canvas& c, int x, int y, int w, int h, float frac, uint32_t col);
void vbar(Canvas& c, int x, int y, int w, int h, float frac, uint32_t col);

// 270° arc gauge. frac NAN -> empty grey track.
void arcGauge(Canvas& c, int cx, int cy, int r, int thick, float frac, uint32_t col);

// Time-series graph, newest sample at the right edge. Values are divided by
// maxv; NAN samples leave gaps.
void graph(Canvas& c, int x, int y, int w, int h, const Ring<HIST_LEN>& r, float maxv,
           uint32_t col, bool fill);
// Anti-aliased 1 px line that respects the band clip rect.
void aaLine(Canvas& c, int x0, int y0, int x1, int y1, uint32_t col);
void gridLines(Canvas& c, int x, int y, int w, int h, int divisions);

void pageDots(Canvas& c, int cur, int n);
