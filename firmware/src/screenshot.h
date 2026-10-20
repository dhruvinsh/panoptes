// Screen capture over serial (protocol `shot` -> `px` rows + `shot_end`).
#pragma once

// Blocking: reads the panel's frame memory back row by row and streams it to
// the host. Takes ~25 s at 115200 baud; the UI is frozen meanwhile.
void screenshotSend();
