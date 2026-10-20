// Page drawing functions and the animation state they read.
#pragma once
#include "widgets.h"

enum Page : int { PAGE_DASH = 0, PAGE_CPU, PAGE_NET, PAGE_SENSORS, PAGE_SYSTEM, PAGE_COUNT };

// Values eased toward the latest frame each UI tick (so gauges glide).
struct UiAnim {
  float gauge[3];          // 0..1 or NAN
  float cores[MAX_CORES];  // 0..1
  uint32_t now;            // millis() of the current frame
};
extern UiAnim anim;

// Milliseconds from `t` to the current UI frame, clamped at 0 so a timestamp
// taken slightly after anim.now never wraps to ~49 days.
inline uint32_t msSince(uint32_t t) {
  int32_t d = (int32_t)(anim.now - t);
  return d > 0 ? (uint32_t)d : 0;
}

const char* pageTitle(int page);

void drawHeader(Canvas& c, int page);
void drawDashboard(Canvas& c);
void drawCpu(Canvas& c);
void drawNetDisk(Canvas& c);
void drawSensors(Canvas& c);
void drawSystem(Canvas& c);
void drawWaiting(Canvas& c);

// Graph scale for byte rates: a 1-2-5 step in 1024-based units.
float niceRateMax(float v);
// Is a third dashboard gauge showing the GPU (true) or the CPU temp (false)?
bool dashShowsGpu();
