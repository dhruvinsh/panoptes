#include "alert.h"

#include <math.h>
#include <stdio.h>

#include "config.h"
#include "protocol.h"
#include "settings.h"

Alert alert;

bool alertOnFrame() {
  const Stats& s = state.s;
  bool hotTemp = !isnan(s.cpu.temp) && s.cpu.temp >= settings.alertTemp;
  bool hotCpu = !isnan(s.cpu.pct) && s.cpu.pct >= settings.alertCpu;

  if (hotTemp || hotCpu) {
    alert.cool = 0;
    if (alert.hot < 255) alert.hot++;
    if (hotTemp)
      snprintf(alert.reason, sizeof(alert.reason), "CPU %.0f C", s.cpu.temp);
    else
      snprintf(alert.reason, sizeof(alert.reason), "CPU LOAD %.0f%%", s.cpu.pct);
    if (!alert.active && alert.hot >= ALERT_FRAMES_ON) {
      alert.active = true;
      return true;
    }
  } else {
    alert.hot = 0;
    if (alert.cool < 255) alert.cool++;
    if (alert.active && alert.cool >= ALERT_FRAMES_OFF) alert.active = false;
  }
  return false;
}

void alertReset() {
  alert.active = false;
  alert.hot = alert.cool = 0;
}
