// UI controller: page state, touch gestures, toasts, overlays, rendering.
#pragma once
#include <stdint.h>

void uiBegin();
void uiPollTouch(uint32_t now);
void uiTick(uint32_t now);  // renders when a frame is due
void uiSetPage(int page, bool user);
void uiApplyRotation();
void uiInvalidate();  // render on the next tick (e.g. after a theme change)
void uiToast(const char* msg);
