#include "app.h"

#include <Arduino.h>
#include <esp32-hal-bt.h>

#include "alert.h"
#include "display.h"
#include "led.h"
#include "protocol.h"
#include "sensors.h"
#include "settings.h"
#include "speaker.h"
#include "themectl.h"
#include "ui/canvas.h"
#include "ui/ui.h"

bool nightMode = false;

namespace {
bool wasConnected = false;

void radiosOff() {
  // USB-only device: no WiFi/BT code is used or linked. WiFi is never
  // initialised (so its RF stays unpowered), and the Arduino core already
  // releases the BT controller memory at boot when BT is unused; btStop() makes
  // sure the controller is disabled in any case.
  btStop();
}
}  // namespace

void toggleNightMode() {
  nightMode = !nightMode;
  uiToast(nightMode ? "Night mode on" : "Night mode off");
  sendLog(nightMode ? "night mode on" : "night mode off");
}

void onStatsFrame() {
  if (alertOnFrame() && settings.beep) speakerChirp();
}

void onConfig(uint32_t mask, int forcedPage) {
  if (mask & CFG_ROTATE) uiApplyRotation();
  if ((mask & CFG_PAGE) && forcedPage >= 0) uiSetPage(forcedPage, true);
  // brightness, LED, beep, cycle, thresholds and theme are read live from `settings`
}

void onHello() { themeSendEvent(); }  // the host may have missed earlier changes

void appSetup() {
  radiosOff();
  settingsLoad();
  displayBegin(settings.rotate);
  tft.setBrightness(0);  // sensorsUpdate() fades it in
  ledBegin();
  speakerBegin();
  sensorsBegin();
  bool ok = canvasBegin();
  themeBegin();
  uiBegin();
  protocolBegin();  // sends `hi`
  if (!ok) sendLog("frame buffer allocation failed");
}

void appLoop() {
  protocolPoll();
  // Read the time after polling: a frame parsed above stamps lastFrameMs with
  // millis(), and an earlier `now` would make (now - lastFrameMs) wrap.
  uint32_t now = millis();

  bool connected = hostConnected();
  if (wasConnected && !connected) alertReset();  // stale data must not keep alerting
  wasConnected = connected;

  uiPollTouch(now);
  sensorsUpdate(now);
  themeUpdate(now);
  ledUpdate(now);
  speakerUpdate(now);
  settingsService(now);
  uiTick(now);
}
