// Panel bring-up test: cycles through candidate panel configurations (one per
// boot, ~7 s each). Tap the screen when the test pattern looks right; the
// chosen mode is printed over serial and the cycle stops.
//
//   pio run -e paneltest -t upload && pio device monitor
//
// A correct mode shows: black background, white 1px border, "TOP" upright at
// the top, red/green/blue bars left→right, and the mode label in the centre.
#define LGFX_USE_V1
#include <Arduino.h>
#include <LovyanGFX.hpp>
#include <esp_system.h>

#include "config.h"

struct Mode {
  const char* name;
  int panel;  // 0 ILI9341, 1 ILI9341_2, 2 ILI9342, 3 ST7789
  bool invert;
  uint8_t rot;
  uint32_t freq;
};

static const Mode MODES[] = {
    {"ILI9341   rot1 40M", 0, false, 1, 40000000},
    {"ILI9341_2 rot1 40M", 1, false, 1, 40000000},
    {"ILI9341_2 rot1 inv", 1, true, 1, 40000000},
    {"ILI9341   rot1 10M", 0, false, 1, 10000000},
    {"ILI9342   rot0 40M", 2, false, 0, 40000000},
    {"ST7789    rot1 inv", 3, true, 1, 40000000},
    {"ST7789    rot1", 3, false, 1, 40000000},
};
static constexpr int N_MODES = sizeof(MODES) / sizeof(MODES[0]);

RTC_NOINIT_ATTR static uint32_t rtcMode;
RTC_NOINIT_ATTR static uint32_t rtcMagic;

class LGFX : public lgfx::LGFX_Device {
  lgfx::Panel_ILI9341 _p0;
  lgfx::Panel_ILI9341_2 _p1;
  lgfx::Panel_ILI9342 _p2;
  lgfx::Panel_ST7789 _p3;
  lgfx::Bus_SPI _bus;
  lgfx::Light_PWM _light;
  lgfx::Touch_XPT2046 _touch;

 public:
  void setup(const Mode& m) {
    auto b = _bus.config();
    b.spi_host = SPI2_HOST;
    b.spi_mode = 0;
    b.freq_write = m.freq;
    b.freq_read = 16000000;
    b.pin_sclk = PIN_TFT_SCLK;
    b.pin_mosi = PIN_TFT_MOSI;
    b.pin_miso = PIN_TFT_MISO;
    b.pin_dc = PIN_TFT_DC;
    _bus.config(b);

    lgfx::Panel_LCD* p = m.panel == 0 ? (lgfx::Panel_LCD*)&_p0
                       : m.panel == 1 ? (lgfx::Panel_LCD*)&_p1
                       : m.panel == 2 ? (lgfx::Panel_LCD*)&_p2
                                      : (lgfx::Panel_LCD*)&_p3;
    p->setBus(&_bus);
    auto c = p->config();
    c.pin_cs = PIN_TFT_CS;
    c.pin_rst = PIN_TFT_RST;
    c.panel_width = m.panel == 2 ? 320 : 240;
    c.panel_height = m.panel == 2 ? 240 : 320;
    c.invert = m.invert;
    c.rgb_order = false;
    c.readable = false;
    c.bus_shared = false;
    p->config(c);

    auto l = _light.config();
    l.pin_bl = PIN_TFT_BL;
    l.freq = 12000;
    _light.config(l);
    p->setLight(&_light);

    auto t = _touch.config();
    t.x_min = TOUCH_X_MIN;
    t.x_max = TOUCH_X_MAX;
    t.y_min = TOUCH_Y_MIN;
    t.y_max = TOUCH_Y_MAX;
    t.pin_int = PIN_TOUCH_IRQ;
    t.bus_shared = false;
    t.spi_host = SPI3_HOST;
    t.freq = 1000000;
    t.pin_sclk = PIN_TOUCH_CLK;
    t.pin_mosi = PIN_TOUCH_MOSI;
    t.pin_miso = PIN_TOUCH_MISO;
    t.pin_cs = PIN_TOUCH_CS;
    _touch.config(t);
    p->setTouch(&_touch);
    setPanel(p);
  }
};

static LGFX tft;
static int mode;

static void pattern(const Mode& m) {
  const int w = tft.width(), h = tft.height();
  tft.fillScreen(TFT_BLACK);
  tft.drawRect(0, 0, w, h, TFT_WHITE);
  tft.fillRect(20, 40, 80, 40, TFT_RED);
  tft.fillRect(120, 40, 80, 40, TFT_GREEN);
  tft.fillRect(220, 40, 80, 40, TFT_BLUE);
  tft.setTextDatum(lgfx::top_center);
  tft.setTextColor(TFT_WHITE);
  tft.setFont(&fonts::FreeSansBold12pt7b);
  tft.drawString("TOP", w / 2, 8);
  tft.setTextDatum(lgfx::middle_center);
  tft.setTextColor(TFT_YELLOW);
  char buf[16];
  snprintf(buf, sizeof(buf), "MODE %d", mode);
  tft.drawString(buf, w / 2, h / 2 + 10);
  tft.setFont(&fonts::Font2);
  tft.setTextColor(TFT_CYAN);
  tft.drawString(m.name, w / 2, h / 2 + 40);
  tft.drawString("tap if this looks right", w / 2, h - 30);
  tft.fillTriangle(w - 20, h - 4, w - 4, h - 4, w - 4, h - 20, TFT_MAGENTA);  // bottom-right marker
}

void setup() {
  Serial.begin(115200);
  if (rtcMagic != 0xC1D0C1D0 || esp_reset_reason() == ESP_RST_POWERON) {
    rtcMagic = 0xC1D0C1D0;
    rtcMode = 0;
  }
  mode = rtcMode % N_MODES;
  const Mode& m = MODES[mode];
  tft.setup(m);
  tft.init();
  tft.setRotation(m.rot);
  tft.setBrightness(200);
  pattern(m);
  Serial.printf("{\"t\":\"log\",\"msg\":\"paneltest mode %d: %s (%dx%d)\"}\n", mode, m.name, (int)tft.width(),
                (int)tft.height());
}

void loop() {
  static uint32_t start = millis();
  static bool locked = false;
  int32_t x, y;
  if (!locked && tft.getTouch(&x, &y)) {
    locked = true;
    Serial.printf("{\"t\":\"log\",\"msg\":\"paneltest SELECTED mode %d: %s\"}\n", mode, MODES[mode].name);
    tft.fillRect(0, tft.height() - 50, tft.width(), 20, TFT_DARKGREEN);
    tft.setTextColor(TFT_WHITE);
    tft.drawString("SELECTED - thanks!", tft.width() / 2, tft.height() - 40);
  }
  if (!locked && millis() - start > 7000) {
    rtcMode = mode + 1;
    ESP.restart();
  }
  delay(20);
}
