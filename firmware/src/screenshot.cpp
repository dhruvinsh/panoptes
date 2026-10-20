#include "screenshot.h"

#include <Arduino.h>
#include <mbedtls/base64.h>

#include "display.h"

void screenshotSend() {
  const int w = tft.width(), h = tft.height();
  static lgfx::rgb888_t px[SCREEN_W > SCREEN_H ? SCREEN_W : SCREEN_H];
  static uint8_t rgb[sizeof(px)];
  static unsigned char b64[((sizeof(rgb) + 2) / 3) * 4 + 1];

  // Reading back what the panel holds (not the canvas) shows rotation and
  // driver problems too.
  for (int y = 0; y < h; y++) {
    tft.readRect(0, y, w, 1, px);
    for (int x = 0; x < w; x++) {
      rgb[x * 3] = px[x].R8();
      rgb[x * 3 + 1] = px[x].G8();
      rgb[x * 3 + 2] = px[x].B8();
    }
    size_t n = 0;
    mbedtls_base64_encode(b64, sizeof(b64), &n, rgb, w * 3);
    b64[n] = 0;
    Serial.printf("{\"t\":\"px\",\"y\":%d,\"w\":%d,\"h\":%d,\"d\":\"%s\"}\n", y, w, h, (const char*)b64);
  }
  Serial.print("{\"t\":\"shot_end\"}\n");
}
