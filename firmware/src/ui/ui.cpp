#include "ui.h"

#include <Arduino.h>
#include <math.h>
#include <stdio.h>
#include <string.h>

#include "../alert.h"
#include "../app.h"
#include "../format.h"
#include "../settings.h"
#include "../themectl.h"
#include "pages.h"

using namespace theme;
using TD = textdatum_t;

UiAnim anim;

namespace {
int page = 0;
uint32_t lastRender = 0;
uint32_t lastPageChange = 0;
uint32_t lastInteraction = 0;

char toastMsg[40] = "";
uint32_t toastUntil = 0;
constexpr uint32_t TOAST_MS = 1600;

// touch state machine
struct Touch {
  bool down = false;       // finger currently down (debounced)
  bool longFired = false;  // long press already handled for this contact
  int16_t x0 = 0, y0 = 0;  // first contact
  int16_t x = 0, y = 0;    // latest
  uint32_t t0 = 0;
  uint32_t lastSeen = 0;   // last raw "pressed" reading
  uint32_t lastPoll = 0;
} touch;
constexpr uint32_t RELEASE_DEBOUNCE_MS = 60;

#if TOUCH_DEBUG
int16_t dbgX = -1, dbgY = -1;
#endif

const char* const TITLES[PAGE_COUNT] = {"OVERVIEW", "CPU", "NETWORK & DISK", "SENSORS",
                                        "SYSTEM"};

void ease(float& cur, float target, float k) {
  if (isnan(target)) {
    cur = NAN;
  } else if (isnan(cur)) {
    cur = 0;  // grow in from empty
  } else {
    cur += (target - cur) * k;
    if (fabsf(target - cur) < 0.002f) cur = target;
  }
}

void stepAnimations() {
  const Stats& s = state.s;
  const float k = 0.22f;
  ease(anim.gauge[0], isnan(s.cpu.pct) ? NAN : s.cpu.pct / 100, k);
  ease(anim.gauge[1], isnan(s.mem.pct) ? NAN : s.mem.pct / 100, k);
  float third = dashShowsGpu() ? s.gpu.pct / 100 : s.cpu.temp / 100;
  ease(anim.gauge[2], third, k);
  for (int i = 0; i < MAX_CORES; i++) {
    float t = i < s.cpu.nCores ? s.cpu.cores[i] / 100 : 0;
    if (isnan(anim.cores[i])) anim.cores[i] = 0;
    anim.cores[i] += (t - anim.cores[i]) * k;
  }
}

void drawAlertBanner(Canvas& c) {
  // blink between two reds at 1 Hz; 2 px red frame around the whole screen.
  // The second red moves away from the text colour, so it never loses contrast.
  bool hi = (anim.now / 500) % 2 == 0;
  uint32_t col = hi ? CRIT : blend(CRIT, ON_CRIT ^ 0xFFFFFF, 0.45f);
  c.fillRect(0, 0, SCREEN_W, HEADER_H, col);
  c.fillTriangle(12, 18, 20, 5, 28, 18, ON_CRIT);
  c.fillRect(19, 9, 2, 5, col);
  c.fillRect(19, 15, 2, 2, col);
  char msg[48];
  snprintf(msg, sizeof msg, "ALERT  %s", alert.reason);
  text(c, F_VAL, SCREEN_W / 2 + 8, HEADER_H / 2, msg, ON_CRIT, TD::middle_center);
  for (int i = 0; i < 2; i++) c.drawRect(i, i, SCREEN_W - 2 * i, SCREEN_H - 2 * i, CRIT);
}

void drawToast(Canvas& c) {
  if (!toastMsg[0] || (int32_t)(anim.now - toastUntil) >= 0) return;
  int w = textW(c, F_BODY, toastMsg) + 32;
  int x = (SCREEN_W - w) / 2, y = 186;
  c.fillSmoothRoundRect(x - 1, y - 1, w + 2, 30, 15, blend(PANEL_HI, ACCENT, 0.6f));
  c.fillSmoothRoundRect(x, y, w, 28, 14, PANEL_HI);
  text(c, F_BODY, SCREEN_W / 2, y + 14, toastMsg, TEXT, TD::middle_center);
}

void drawScreen(Canvas& c) {
  if (!hostConnected()) {
    drawWaiting(c);
  } else {
    c.fillScreen(BG);
    drawHeader(c, page);
    switch (page) {
      case PAGE_DASH: drawDashboard(c); break;
      case PAGE_CPU: drawCpu(c); break;
      case PAGE_NET: drawNetDisk(c); break;
      case PAGE_SENSORS: drawSensors(c); break;
      default: drawSystem(c); break;
    }
    pageDots(c, page, PAGE_COUNT);
    if (alert.active) drawAlertBanner(c);
  }
  if (nightMode) c.fillSmoothCircle(SCREEN_W - 7, SCREEN_H - 7, 2, DIM);  // night indicator
  drawToast(c);
#if TOUCH_DEBUG
  if (dbgX >= 0) {
    c.drawCircle(dbgX, dbgY, 6, WARN);
    c.drawFastHLine(dbgX - 9, dbgY, 19, WARN);
    c.drawFastVLine(dbgX, dbgY - 9, 19, WARN);
  }
#endif
}

// ---- gestures ---------------------------------------------------------------

void onTap(int x, int y) {
  if (!hostConnected()) return;
  if (x < SCREEN_W / 3)
    uiSetPage(page - 1, true);
  else if (x > SCREEN_W * 2 / 3)
    uiSetPage(page + 1, true);
  else if (y < HEADER_H + 8)  // centre of the header bar (a little slack for fingers)
    themeCycleFamily();
}

void onSwipe(int dx) {
  if (!hostConnected()) return;
  uiSetPage(page + (dx < 0 ? 1 : -1), true);
}
}  // namespace

const char* pageTitle(int p) { return (p >= 0 && p < PAGE_COUNT) ? TITLES[p] : ""; }

void drawHeader(Canvas& c, int p) {
  const Stats& s = state.s;
  const Link& l = state.link;
  // live dot: brief flash on each received frame
  uint32_t age = msSince(l.lastFrameMs);
  uint32_t dot = age < 150 ? TEXT : (age < 2000 ? GOOD : WARN);
  c.fillSmoothCircle(10, HEADER_H / 2, 3, dot);
  textFit(c, F_VAL, 20, HEADER_H / 2, 130, s.host[0] ? s.host : "host", TEXT, TD::middle_left);

  char clk[8] = "--:--";
  char date[16] = "";
  if (l.clockValid) {
    int64_t now = l.clockLocal + msSince(l.clockMs) / 1000;
    fmtClock(clk, sizeof clk, now);
    fmtDate(date, sizeof date, now);
  }
  text(c, F_VAL, SCREEN_W - 8, HEADER_H / 2, clk, l.clockValid ? TEXT : DIM, TD::middle_right);
  int cw = textW(c, F_VAL, clk);
  // page title, or the date on the overview page
  caption(c, SCREEN_W - 16 - cw, HEADER_H / 2, p == PAGE_DASH && date[0] ? date : pageTitle(p), SUB,
          TD::middle_right);
  c.drawFastHLine(0, HEADER_H, SCREEN_W, LINE);
}

void uiBegin() {
  page = settings.page < PAGE_COUNT ? settings.page : 0;
  anim.gauge[0] = anim.gauge[1] = anim.gauge[2] = NAN;
  for (float& f : anim.cores) f = 0;
  lastPageChange = millis();
}

void uiSetPage(int p, bool user) {
  p = ((p % PAGE_COUNT) + PAGE_COUNT) % PAGE_COUNT;
  uint32_t now = millis();
  lastPageChange = now;
  if (user) lastInteraction = now;
  if (p == page) return;
  page = p;
  sendPageEvent(page);
  if (user && settings.page != page) {
    settings.page = (uint8_t)page;
    settingsMarkDirty();
  }
}

void uiApplyRotation() {
  displaySetRotation(settings.rotate);
  uiInvalidate();
}

void uiInvalidate() { lastRender = 0; }

void uiToast(const char* msg) {
  strncpy(toastMsg, msg, sizeof(toastMsg) - 1);
  toastMsg[sizeof(toastMsg) - 1] = 0;
  toastUntil = millis() + TOAST_MS;
}


void uiPollTouch(uint32_t now) {
  if (now - touch.lastPoll < 15) return;
  touch.lastPoll = now;

  lgfx::touch_point_t tp;
  bool pressed = tft.getTouch(&tp, 1) > 0;
  if (pressed && (tp.x < 0 || tp.y < 0 || tp.x >= SCREEN_W || tp.y >= SCREEN_H)) pressed = false;

  if (pressed) {
    touch.lastSeen = now;
    if (!touch.down) {
      touch.down = true;
      touch.longFired = false;
      touch.x0 = touch.x = tp.x;
      touch.y0 = touch.y = tp.y;
      touch.t0 = now;
    } else {
      touch.x = tp.x;
      touch.y = tp.y;
    }
#if TOUCH_DEBUG
    dbgX = tp.x;
    dbgY = tp.y;
#endif
    lastInteraction = now;
    // long press: held still for LONG_PRESS_MS
    int dx = touch.x - touch.x0, dy = touch.y - touch.y0;
    if (!touch.longFired && now - touch.t0 >= LONG_PRESS_MS && dx * dx + dy * dy < 20 * 20) {
      touch.longFired = true;
      toggleNightMode();
    }
    return;
  }

  // released (debounced: XPT2046 readings drop out briefly mid-press)
  if (!touch.down || now - touch.lastSeen < RELEASE_DEBOUNCE_MS) return;
  touch.down = false;
  if (touch.longFired) return;
  int dx = touch.x - touch.x0, dy = touch.y - touch.y0;
  if (abs(dx) >= SWIPE_MIN_PX && abs(dx) > abs(dy) * 3 / 2) {
    onSwipe(dx);
  } else if (touch.lastSeen - touch.t0 < 600 && abs(dx) < 25 && abs(dy) < 25) {
    onTap(touch.x0, touch.y0);
  }
}

void uiTick(uint32_t now) {
  // auto-cycle (paused for 15 s after any touch)
  if (settings.cycle > 0 && hostConnected() && now - lastPageChange >= settings.cycle * 1000UL &&
      now - lastInteraction >= 15000) {
    uiSetPage(page + 1, false);
  }

  if (now - lastRender < UI_FRAME_MS) return;
  lastRender = now;
  anim.now = now;
  stepAnimations();
  renderFrame(drawScreen);
}
