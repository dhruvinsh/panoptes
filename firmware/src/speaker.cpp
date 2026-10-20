#include "speaker.h"

#include <Arduino.h>

#include "config.h"

namespace {
struct Note {
  uint16_t hz;
  uint16_t ms;
};
constexpr Note CHIRP[] = {{1800, 50}, {0, 30}, {2600, 70}};
constexpr int CHIRP_LEN = sizeof(CHIRP) / sizeof(CHIRP[0]);

int step = -1;  // index into CHIRP, -1 = idle
uint32_t stepStart = 0;
bool everPlayed = false;
uint32_t lastChirp = 0;
}  // namespace

void speakerBegin() {
  // 10-bit resolution gives the speaker its own LEDC timer (the LEDs use 8).
  ledcAttach(PIN_SPEAKER, 1000, 10);
  ledcWriteTone(PIN_SPEAKER, 0);
}

void speakerChirp() {
  uint32_t now = millis();
  if (everPlayed && now - lastChirp < BEEP_MIN_INTERVAL_MS) return;
  everPlayed = true;
  lastChirp = now;
  step = 0;
  stepStart = now;
  ledcWriteTone(PIN_SPEAKER, CHIRP[0].hz);
}

void speakerUpdate(uint32_t now) {
  if (step < 0 || now - stepStart < CHIRP[step].ms) return;
  step++;
  stepStart = now;
  if (step >= CHIRP_LEN) {
    step = -1;
    ledcWriteTone(PIN_SPEAKER, 0);
  } else {
    ledcWriteTone(PIN_SPEAKER, CHIRP[step].hz);
  }
}
