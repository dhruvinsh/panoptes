// Serial protocol (see ../PROTOCOL.md) and the shared State struct.
//
// The protocol layer is the only writer of `state`; the UI, LED and alert
// code only read it. Every numeric field uses NAN (floats) or a negative /
// zero count to mean "not available" so the UI can hide that widget.
#pragma once
#include <math.h>
#include <stdint.h>

#include "config.h"

template <int N>
struct Ring {
  float v[N];
  int head = 0;   // next write index
  int count = 0;
  static constexpr int size = N;
  void push(float x) {
    v[head] = x;
    head = (head + 1) % N;
    if (count < N) count++;
  }
  // i = 0 is the oldest sample, count-1 the newest
  float at(int i) const { return v[(head - count + i + N) % N]; }
  float newest() const { return count ? at(count - 1) : NAN; }
  float maxv() const {
    float m = 0;
    for (int i = 0; i < count; i++) {
      float x = at(i);
      if (!isnan(x) && x > m) m = x;
    }
    return m;
  }
  void clear() { head = count = 0; }
};

constexpr int MAX_CORES = 32;
constexpr int MAX_MOUNTS = 4;
constexpr int MAX_TEMPS = 6;
constexpr int MAX_FANS = 4;
constexpr int MAX_PROCS = 5;
constexpr int HIST_LEN = 60;

struct NamedValue {
  char name[20];
  float v;
};

struct Mount {
  char m[24];
  float pct, used, total;  // %, GiB, GiB
};

struct Proc {
  char n[24];
  float c, m;  // %cpu, %mem
};

// One `s` frame. Reset to "all missing" before every parse, because a field
// absent from the latest frame means "not available now".
struct Stats {
  bool hasTs;
  int64_t ts;   // unix seconds UTC
  int32_t tz;   // seconds offset
  char host[49];
  char os[49];
  char kern[49];
  double up;    // seconds, NAN if missing

  struct {
    char name[56];
    float pct;
    int nCores;
    float cores[MAX_CORES];
    float freq;  // MHz
    float temp;  // °C
    float load[3];
  } cpu;

  struct {
    float used, total, pct, swapPct;  // MiB, MiB, %, %
  } mem;

  struct {
    bool present;
    char name[49];
    float pct, temp, memUsed, memTotal, power;
  } gpu;

  struct {
    char iface[16];
    char ip[40];
    float rx, tx;  // B/s
  } net;

  struct {
    float r, w;  // B/s
    int nMounts;
    Mount mounts[MAX_MOUNTS];
  } disk;

  int nTemps;
  NamedValue temps[MAX_TEMPS];
  int nFans;
  NamedValue fans[MAX_FANS];
  int nProcs;
  Proc procs[MAX_PROCS];

  struct {
    float pct;
    int8_t plug;  // -1 unknown, 0 on battery, 1 plugged in
  } bat;

  void clear();
};

struct Link {
  uint32_t frames = 0;        // valid `s` frames
  uint32_t parseErrors = 0;   // JSON errors + overlong lines
  uint32_t lastFrameMs = 0;
  uint32_t helloCount = 0;
  char agent[32] = "";        // from `hello`
  // clock: local epoch seconds at `clockMs`
  bool clockValid = false;
  int64_t clockLocal = 0;
  uint32_t clockMs = 0;
};

struct History {
  Ring<HIST_LEN> cpu;  // %
  Ring<HIST_LEN> rx;   // B/s
  Ring<HIST_LEN> tx;   // B/s
};

struct State {
  Stats s;
  Link link;
  History hist;
};

extern State state;

void protocolBegin();
// Non-blocking: drain the UART and process any complete lines.
void protocolPoll();
// True while `s` frames keep arriving (none missing for HOST_TIMEOUT_MS).
bool hostConnected();
void sendPageEvent(int page);
// {"t":"ev","e":"theme",...}: family id, "dark"/"light"/"auto", variant shown
void sendThemeEvent(const char* theme, const char* mode, const char* variant);
// {"t":"ev","e":"light",...}: smoothed LDR raw value and 0..1 level
void sendLightEvent(int raw, float level);
void sendLog(const char* msg);
