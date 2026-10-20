#include "display.h"

LGFX tft;

LGFX::LGFX() {
  {
    auto cfg = _bus.config();
    cfg.spi_host = SPI2_HOST;  // HSPI
    cfg.spi_mode = 0;
    cfg.freq_write = PANEL_SPI_FREQ;
    cfg.freq_read = 16000000;
    cfg.spi_3wire = false;
    cfg.use_lock = true;
    cfg.dma_channel = SPI_DMA_CH_AUTO;
    cfg.pin_sclk = PIN_TFT_SCLK;
    cfg.pin_mosi = PIN_TFT_MOSI;
    cfg.pin_miso = PIN_TFT_MISO;
    cfg.pin_dc = PIN_TFT_DC;
    _bus.config(cfg);
    _panel.setBus(&_bus);
  }
  {
    auto cfg = _panel.config();
    cfg.pin_cs = PIN_TFT_CS;
    cfg.pin_rst = PIN_TFT_RST;
    cfg.pin_busy = -1;
    cfg.panel_width = 240;  // native portrait
    cfg.panel_height = 320;
    cfg.offset_x = 0;
    cfg.offset_y = 0;
    cfg.offset_rotation = 0;
    cfg.dummy_read_pixel = 8;
    cfg.dummy_read_bits = 1;
    cfg.readable = true;
    cfg.invert = PANEL_INVERT;
    cfg.rgb_order = PANEL_RGB_ORDER;
    cfg.dlen_16bit = false;
    cfg.bus_shared = false;  // the display has HSPI to itself
    _panel.config(cfg);
  }
  {
    auto cfg = _light.config();
    cfg.pin_bl = PIN_TFT_BL;
    cfg.invert = false;
    cfg.freq = 12000;
    cfg.pwm_channel = 7;  // only used by pre-3.x Arduino cores
    _light.config(cfg);
    _panel.setLight(&_light);
  }
  {
    auto cfg = _touch.config();
    cfg.x_min = TOUCH_X_MIN;
    cfg.x_max = TOUCH_X_MAX;
    cfg.y_min = TOUCH_Y_MIN;
    cfg.y_max = TOUCH_Y_MAX;
    cfg.pin_int = PIN_TOUCH_IRQ;
    cfg.bus_shared = false;  // touch has its own bus (VSPI)
    cfg.offset_rotation = TOUCH_OFFSET_ROTATION;
    cfg.spi_host = SPI3_HOST;  // VSPI
    cfg.freq = 1000000;
    cfg.pin_sclk = PIN_TOUCH_CLK;
    cfg.pin_mosi = PIN_TOUCH_MOSI;
    cfg.pin_miso = PIN_TOUCH_MISO;
    cfg.pin_cs = PIN_TOUCH_CS;
    _touch.config(cfg);
    _panel.setTouch(&_touch);
  }
  setPanel(&_panel);
}

void displaySetRotation(uint8_t rotate) { tft.setRotation(rotate ? 3 : 1); }

void displayBegin(uint8_t rotate) {
  tft.init();
  displaySetRotation(rotate);
  tft.setColorDepth(16);
  tft.fillScreen(0);
}
