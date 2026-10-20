// Speaker on GPIO26: short non-blocking chirps.
#pragma once
#include <stdint.h>

void speakerBegin();
// Play the alert chirp, unless one was played in the last 60 s.
void speakerChirp();
void speakerUpdate(uint32_t now);
