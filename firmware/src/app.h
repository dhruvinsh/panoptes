// Application wiring: callbacks between the protocol, UI and peripherals.
#pragma once
#include <stdint.h>

// bits for onConfig()'s mask
enum : uint32_t {
  CFG_BRIGHT = 1u << 0,
  CFG_LED = 1u << 1,
  CFG_ROTATE = 1u << 2,
  CFG_PAGE = 1u << 3,
  CFG_OTHER = 1u << 4,
};

void appSetup();
void appLoop();

// called by the protocol layer
void onStatsFrame();
void onConfig(uint32_t mask, int forcedPage);
void onHello();  // after the `hi` reply to a `hello`

// night mode: backlight very low, LED dimmed (not persisted)
extern bool nightMode;
void toggleNightMode();
