#include "led.h"

#include <Arduino.h>
#include <math.h>

#include "alert.h"
#include "app.h"
#include "config.h"
#include "protocol.h"
#include "settings.h"

namespace {
constexpr uint32_t LED_PWM_FREQ = 5000;
constexpr uint8_t LED_PWM_BITS = 8;
float cur[3] = {0, 0, 0};  // smoothed 0..1 per channel
uint32_t lastMs = 0;

void write(int pin, float v) {
  v = v < 0 ? 0 : (v > 1 ? 1 : v);
  uint32_t duty = (uint32_t)lroundf(v * LED_MAX_DUTY);
  ledcWrite(pin, 255 - duty);  // active LOW
}

// green -> yellow -> red over 0..100 %
void loadColor(float pct, float out[3]) {
  float t = pct / 100.0f;
  t = t < 0 ? 0 : (t > 1 ? 1 : t);
  if (t < 0.5f) {
    out[0] = t * 2;
    out[1] = 1;
  } else {
    out[0] = 1;
    out[1] = 1 - (t - 0.5f) * 2;
  }
  out[2] = 0;
}
}  // namespace

void ledBegin() {
  for (int pin : {PIN_LED_R, PIN_LED_G, PIN_LED_B}) {
    ledcAttach(pin, LED_PWM_FREQ, LED_PWM_BITS);
    ledcWrite(pin, 255);  // off
  }
}

void ledUpdate(uint32_t now) {
  if (now - lastMs < 20) return;
  lastMs = now;

  float target[3] = {0, 0, 0};
  bool instant = false;
  if (!settings.led) {
    // off
  } else if (!hostConnected()) {
    // slow breathing blue, 4 s period
    float ph = (now % 4000) / 4000.0f;
    float b = 0.5f - 0.5f * cosf(ph * 2 * (float)M_PI);
    target[2] = 0.04f + 0.5f * b * b;
  } else if (alert.active) {
    // red flash: 120 ms on, 380 ms off
    target[0] = (now % 500) < 120 ? 1.0f : 0.0f;
    instant = true;
  } else if (!isnan(state.s.cpu.pct)) {
    loadColor(state.s.cpu.pct, target);
    for (float& t : target) t *= 0.6f;
  } else {
    target[0] = target[1] = target[2] = 0.15f;  // connected, no cpu data
  }

  float scale = nightMode ? 0.12f : 1.0f;
  const int pins[3] = {PIN_LED_R, PIN_LED_G, PIN_LED_B};
  for (int i = 0; i < 3; i++) {
    float t = target[i] * scale;
    cur[i] = instant ? t : cur[i] + (t - cur[i]) * 0.15f;
    write(pins[i], cur[i]);
  }
}
