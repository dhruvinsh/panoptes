#include "canvas.h"

#include <esp_heap_caps.h>

namespace {
Canvas spr(&tft);
uint8_t* buf = nullptr;
int bandH = 0;
}  // namespace

bool canvasBegin() {
  static const int candidates[] = {120, 80, 60, 40, 30, 20};
  for (int h : candidates) {
    size_t bytes = (size_t)SCREEN_W * h * 2;
    buf = (uint8_t*)heap_caps_malloc(bytes, MALLOC_CAP_DMA | MALLOC_CAP_8BIT);
    if (buf) {
      bandH = h;
      return true;
    }
  }
  return false;
}

int canvasBandHeight() { return bandH; }

void renderFrame(DrawFn fn) {
  if (!buf) return;
  const int W = tft.width(), H = tft.height();
  tft.startWrite();
  for (int y0 = 0; y0 < H; y0 += bandH) {
    const int bh = (H - y0 < bandH) ? H - y0 : bandH;
    // Point the sprite at a virtual W x H image whose row y0 starts at the
    // band buffer. The clip rect guarantees only rows [y0, y0+bh) - the ones
    // backed by real memory - are ever touched.
    uint8_t* base = (uint8_t*)((uintptr_t)buf - (uintptr_t)y0 * W * 2);
    spr.setBuffer(base, W, H, 16);
    spr.setClipRect(0, y0, W, bh);
    fn(spr);
    tft.pushImage(0, y0, W, bh, (const lgfx::swap565_t*)buf);
  }
  tft.endWrite();
}
