// Human-readable formatting helpers. All write into a caller buffer and
// return it, and render "--" for missing (NAN) values.
#pragma once
#include <stddef.h>
#include <stdint.h>

const char* fmtRate(char* out, size_t n, float bytesPerSec);   // "125 KB/s"
const char* fmtBytes(char* out, size_t n, float bytes);        // "1.2 MB"
const char* fmtUptime(char* out, size_t n, double seconds);    // "3d 4h"
const char* fmtMiB(char* out, size_t n, float mib);            // "7.9G" / "512M"
const char* fmtGiB(char* out, size_t n, float gib);            // "200G" / "1.8T"
const char* fmtFreq(char* out, size_t n, float mhz);           // "3.60 GHz"
const char* fmtPct(char* out, size_t n, float pct);            // "23%"
const char* fmtNum(char* out, size_t n, float v, int decimals);
const char* fmtClock(char* out, size_t n, int64_t localEpoch); // "14:05"
const char* fmtDate(char* out, size_t n, int64_t localEpoch);  // "Tue 20 Oct"

// A "nice" axis maximum >= v from the 1-2-5 sequence (for graph scaling).
float niceCeil(float v);
