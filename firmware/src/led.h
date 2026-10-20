// RGB status LED (active LOW).
#pragma once
#include <stdint.h>

void ledBegin();
// Call often; animates toward the colour implied by the current state.
void ledUpdate(uint32_t now);
