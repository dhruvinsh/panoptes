#include "sensors.h"

#include <Arduino.h>

#include "app.h"
#include "config.h"
#include "display.h"
#include "protocol.h"
#include "settings.h"

namespace {
float ldrAvg = -1;     // EMA of raw ADC
float blCur = -1;      // smoothed backlight
float blGain = 1;      // backlightSetGain()
int blApplied = -1;   // -1 = not applied yet
uint32_t lastSample = 0;
uint32_t lastLightEvent = 0;

void applyBacklight() {
  if (blCur < 0) return;
  int v = (int)lroundf(blCur * blGain);
  if (v != blApplied) {
    blApplied = v;
    tft.setBrightness((uint8_t)v);
  }
}
}  // namespace

void sensorsBegin() {
  pinMode(PIN_LDR, INPUT);
  analogSetPinAttenuation(PIN_LDR, ADC_0db);  // the LDR divider has a small swing
}

int ldrRaw() { return ldrAvg < 0 ? 0 : (int)lroundf(ldrAvg); }
uint8_t backlightLevel() { return blApplied < 0 ? 0 : (uint8_t)blApplied; }

float ldrLevel() {
  if (ldrAvg < 0) return -1;
  // handles either direction of LDR_RAW_*
  float light = (ldrAvg - LDR_RAW_DARK) / (float)(LDR_RAW_BRIGHT - LDR_RAW_DARK);
  return light < 0 ? 0 : (light > 1 ? 1 : light);
}

void backlightSetGain(float gain) {
  blGain = gain < 0 ? 0 : (gain > 1 ? 1 : gain);
  applyBacklight();
}

void sensorsUpdate(uint32_t now) {
  if (now - lastSample < 50) return;
  lastSample = now;

  int raw = analogRead(PIN_LDR);
  ldrAvg = ldrAvg < 0 ? raw : ldrAvg + (raw - ldrAvg) * 0.04f;  // ~1.2 s time constant

  // periodic reading for remote calibration of LDR_RAW_* / THEME_*
  if (hostConnected() && now - lastLightEvent >= LIGHT_EVENT_MS) {
    lastLightEvent = now;
    sendLightEvent(ldrRaw(), ldrLevel());
  }

  float target;
  if (nightMode) {
    target = settings.blNight;
  } else if (settings.autoBright) {
    float light = sqrtf(ldrLevel());  // perceptual-ish: rise faster out of the dark
    target = settings.blMin + (255 - settings.blMin) * light;
  } else {
    target = settings.bright;
  }

  blCur = blCur < 0 ? target : blCur + (target - blCur) * 0.2f;
  if (fabsf(blCur - target) < 0.5f) blCur = target;
  applyBacklight();
}
