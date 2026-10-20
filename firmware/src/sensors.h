// Light sensor (LDR) and backlight control.
#pragma once
#include <stdint.h>

void sensorsBegin();
// Samples the LDR and steers the backlight (auto / manual / night).
void sensorsUpdate(uint32_t now);
int ldrRaw();             // smoothed raw ADC value
// Light level from the LDR: 0 = LDR_RAW_DARK .. 1 = LDR_RAW_BRIGHT (linear,
// clamped), or -1 before the first sample.
float ldrLevel();
uint8_t backlightLevel(); // currently applied backlight 0..255
// Scale the backlight by 0..1 on top of auto/manual/night (theme swap fade).
void backlightSetGain(float gain);
