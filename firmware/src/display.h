// LovyanGFX device for the ESP32-2432S028R.
#pragma once

#define LGFX_USE_V1
#include <LovyanGFX.hpp>

#include "config.h"

class LGFX : public lgfx::LGFX_Device {
#if PANEL_DRIVER == PANEL_DRIVER_ST7789
  lgfx::Panel_ST7789 _panel;
#else
  lgfx::Panel_ILI9341 _panel;
#endif
  lgfx::Bus_SPI _bus;
  lgfx::Light_PWM _light;
  lgfx::Touch_XPT2046 _touch;

 public:
  LGFX();
};

extern LGFX tft;

// Initialise panel, rotation (0 = landscape, 1 = landscape flipped).
void displayBegin(uint8_t rotate);
void displaySetRotation(uint8_t rotate);
