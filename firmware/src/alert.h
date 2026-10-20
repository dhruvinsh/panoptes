// Alert mode: sustained high CPU temperature or load.
#pragma once
#include <stdint.h>

struct Alert {
  bool active = false;
  char reason[32] = "";
  uint8_t hot = 0;   // consecutive frames over threshold
  uint8_t cool = 0;  // consecutive frames under threshold
};

extern Alert alert;

// Evaluate the latest `s` frame. Returns true when the alert just started.
bool alertOnFrame();
void alertReset();
