#include "format.h"

#include <math.h>
#include <stdio.h>
#include <time.h>

namespace {
const char* missing(char* out, size_t n) {
  snprintf(out, n, "--");
  return out;
}

// value with ~3 significant digits: 4.21 / 42.1 / 421
void sig3(char* out, size_t n, float v, const char* unit) {
  if (v < 9.995f)
    snprintf(out, n, "%.2f %s", v, unit);
  else if (v < 99.95f)
    snprintf(out, n, "%.1f %s", v, unit);
  else
    snprintf(out, n, "%.0f %s", v, unit);
}

void scaled(char* out, size_t n, float v, const char* const* units, int nUnits) {
  int u = 0;
  while (v >= 999.5f && u < nUnits - 1) {
    v /= 1024.0f;
    u++;
  }
  if (u == 0)
    snprintf(out, n, "%.0f %s", v, units[0]);
  else
    sig3(out, n, v, units[u]);
}
}  // namespace

const char* fmtRate(char* out, size_t n, float bps) {
  if (isnan(bps) || bps < 0) return missing(out, n);
  static const char* const U[] = {"B/s", "KB/s", "MB/s", "GB/s"};
  scaled(out, n, bps, U, 4);
  return out;
}

const char* fmtBytes(char* out, size_t n, float b) {
  if (isnan(b) || b < 0) return missing(out, n);
  static const char* const U[] = {"B", "KB", "MB", "GB", "TB"};
  scaled(out, n, b, U, 5);
  return out;
}

const char* fmtUptime(char* out, size_t n, double s) {
  if (isnan(s) || s < 0) return missing(out, n);
  uint32_t t = (uint32_t)s;
  uint32_t d = t / 86400, h = (t / 3600) % 24, m = (t / 60) % 60;
  if (d)
    snprintf(out, n, "%ud %uh", (unsigned)d, (unsigned)h);
  else if (h)
    snprintf(out, n, "%uh %um", (unsigned)h, (unsigned)m);
  else if (m)
    snprintf(out, n, "%um %us", (unsigned)m, (unsigned)(t % 60));
  else
    snprintf(out, n, "%us", (unsigned)t);
  return out;
}

const char* fmtMiB(char* out, size_t n, float mib) {
  if (isnan(mib) || mib < 0) return missing(out, n);
  if (mib < 1000)
    snprintf(out, n, "%.0fM", mib);
  else
    return fmtGiB(out, n, mib / 1024.0f);
  return out;
}

const char* fmtGiB(char* out, size_t n, float g) {
  if (isnan(g) || g < 0) return missing(out, n);
  if (g >= 999.5f) {
    g /= 1024.0f;
    snprintf(out, n, g < 9.95f ? "%.1fT" : "%.0fT", g);
  } else {
    snprintf(out, n, g < 9.95f ? "%.1fG" : "%.0fG", g);
  }
  return out;
}

const char* fmtFreq(char* out, size_t n, float mhz) {
  if (isnan(mhz) || mhz <= 0) return missing(out, n);
  if (mhz >= 1000)
    snprintf(out, n, "%.2f GHz", mhz / 1000.0f);
  else
    snprintf(out, n, "%.0f MHz", mhz);
  return out;
}

const char* fmtPct(char* out, size_t n, float p) {
  if (isnan(p)) return missing(out, n);
  snprintf(out, n, "%.0f%%", p);
  return out;
}

const char* fmtNum(char* out, size_t n, float v, int decimals) {
  if (isnan(v)) return missing(out, n);
  snprintf(out, n, "%.*f", decimals, v);
  return out;
}

const char* fmtClock(char* out, size_t n, int64_t e) {
  time_t t = (time_t)e;
  struct tm tm;
  gmtime_r(&t, &tm);
  snprintf(out, n, "%02d:%02d", tm.tm_hour, tm.tm_min);
  return out;
}

const char* fmtDate(char* out, size_t n, int64_t e) {
  static const char* const DAYS[] = {"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"};
  static const char* const MON[] = {"Jan", "Feb", "Mar", "Apr", "May", "Jun",
                                    "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"};
  time_t t = (time_t)e;
  struct tm tm;
  gmtime_r(&t, &tm);
  snprintf(out, n, "%s %d %s", DAYS[tm.tm_wday % 7], tm.tm_mday, MON[tm.tm_mon % 12]);
  return out;
}

float niceCeil(float v) {
  if (!(v > 0)) return 1;
  float e = powf(10.0f, floorf(log10f(v)));
  float m = v / e;
  float nm = m <= 1 ? 1 : m <= 2 ? 2 : m <= 5 ? 5 : 10;
  return nm * e;
}
