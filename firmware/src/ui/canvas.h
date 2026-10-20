// Flicker-free full-screen rendering through one 16-bit band buffer.
//
// A full 320x240 RGB565 frame buffer needs 150 KB, more than the ESP32's
// largest free DRAM block. Instead we keep a single band buffer
// (320 x 120 x 2 = 75 KB, falling back to smaller bands if the heap is
// tight) and render each frame in bands: the whole screen is drawn into a
// sprite whose clip rectangle covers only the current band, the band is
// pushed to the panel, and the next band is drawn. Drawing code therefore
// uses plain screen coordinates and never knows about bands; every pixel is
// written exactly once per push, so nothing ever flickers, and we keep full
// 16-bit colour (an 8-bit full-screen sprite would also fit, but RGB332
// cannot represent the dark blue-grey theme without heavy banding).
//
// RULE: drawing code must never change the clip rect, and must not call
// LovyanGFX's drawWideLine / drawSmoothLine / drawWedgeLine / drawSpot,
// which internally replace and then clear the clip rect (they would write
// outside the band buffer). Use aaLine() from widgets.h instead.
#pragma once
#include "../display.h"

using Canvas = LGFX_Sprite;
using DrawFn = void (*)(Canvas&);

bool canvasBegin();          // allocate the band buffer
int canvasBandHeight();
void renderFrame(DrawFn fn); // draw + push the whole screen
