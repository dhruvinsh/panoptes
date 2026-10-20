#include "protocol.h"

#include <Arduino.h>
#include <ArduinoJson.h>
#include <stdio.h>
#include <string.h>

#include "app.h"
#include "screenshot.h"
#include "settings.h"
#include "themectl.h"
#include "ui/theme.h"

State state;

void Stats::clear() {
  // Zero everything (counts = 0, strings = ""), then mark numbers missing.
  memset(this, 0, sizeof(*this));
  up = NAN;
  cpu.pct = cpu.freq = cpu.temp = NAN;
  for (float& l : cpu.load) l = NAN;
  mem.used = mem.total = mem.pct = mem.swapPct = NAN;
  gpu.pct = gpu.temp = gpu.memUsed = gpu.memTotal = gpu.power = NAN;
  net.rx = net.tx = NAN;
  disk.r = disk.w = NAN;
  bat.pct = NAN;
  bat.plug = -1;
}

namespace {

char line[MAX_LINE + 1];
size_t lineLen = 0;
bool overflow = false;
JsonDocument doc;

// ---- tolerant field readers -------------------------------------------------

float num(JsonVariantConst v) {
  if (!v.is<float>()) return NAN;
  float f = v.as<float>();
  return isfinite(f) ? f : NAN;
}

float numClamp(JsonVariantConst v, float lo, float hi) {
  float f = num(v);
  if (isnan(f)) return f;
  return f < lo ? lo : (f > hi ? hi : f);
}

// Copy a JSON string into a fixed buffer. Non-ASCII code points are replaced
// by '?' because the bitmap fonts only cover printable ASCII.
void str(char* dst, size_t n, JsonVariantConst v) {
  dst[0] = 0;
  const char* s = v.as<const char*>();
  if (!s) return;
  size_t o = 0;
  for (const uint8_t* p = (const uint8_t*)s; *p && o + 1 < n; p++) {
    uint8_t c = *p;
    if (c < 0x80) {
      dst[o++] = (c >= 0x20 && c < 0x7f) ? (char)c : ' ';
    } else if ((c & 0xC0) == 0xC0) {
      dst[o++] = '?';  // lead byte of a multi-byte sequence
    }                  // continuation bytes are dropped
  }
  dst[o] = 0;
}

template <size_t N>
void str(char (&dst)[N], JsonVariantConst v) {
  str(dst, N, v);
}

// ---- outgoing ---------------------------------------------------------------

void sendHi() {
  Serial.print("{\"t\":\"hi\",\"v\":" + String(PROTO_VERSION) +
               ",\"fw\":\"" FW_VERSION "\",\"board\":\"" BOARD_NAME "\",\"w\":" +
               String(SCREEN_W) + ",\"h\":" + String(SCREEN_H) + "}\n");
}

// ---- incoming ---------------------------------------------------------------

void parseStats(JsonObjectConst o) {
  Stats& s = state.s;
  s.clear();

  JsonVariantConst ts = o["ts"];
  if (ts.is<int64_t>() || ts.is<float>()) {
    s.hasTs = true;
    s.ts = ts.is<int64_t>() ? ts.as<int64_t>() : (int64_t)ts.as<double>();
    s.tz = o["tz"].is<int32_t>() ? o["tz"].as<int32_t>() : 0;
  }
  str(s.host, o["host"]);
  str(s.os, o["os"]);
  str(s.kern, o["kern"]);
  {
    double up = o["up"].is<double>() ? o["up"].as<double>() : NAN;
    s.up = (isfinite(up) && up >= 0) ? up : NAN;
  }

  JsonObjectConst cpu = o["cpu"];
  if (cpu) {
    str(s.cpu.name, cpu["name"]);
    s.cpu.pct = numClamp(cpu["pct"], 0, 100);
    JsonArrayConst cores = cpu["cores"];
    for (JsonVariantConst c : cores) {
      if (s.cpu.nCores >= MAX_CORES) break;
      float v = numClamp(c, 0, 100);
      s.cpu.cores[s.cpu.nCores++] = isnan(v) ? 0 : v;
    }
    s.cpu.freq = num(cpu["freq"]);
    s.cpu.temp = num(cpu["temp"]);
    JsonArrayConst load = cpu["load"];
    for (int i = 0; i < 3; i++) s.cpu.load[i] = num(load[i]);
  }

  JsonObjectConst mem = o["mem"];
  if (mem) {
    s.mem.used = num(mem["used"]);
    s.mem.total = num(mem["total"]);
    s.mem.pct = numClamp(mem["pct"], 0, 100);
    s.mem.swapPct = numClamp(mem["swap_pct"], 0, 100);
    if (isnan(s.mem.pct) && s.mem.total > 0 && !isnan(s.mem.used))
      s.mem.pct = 100.0f * s.mem.used / s.mem.total;
  }

  JsonObjectConst gpu = o["gpu"];
  if (gpu) {
    str(s.gpu.name, gpu["name"]);
    s.gpu.pct = numClamp(gpu["pct"], 0, 100);
    s.gpu.temp = num(gpu["temp"]);
    s.gpu.memUsed = num(gpu["mem_used"]);
    s.gpu.memTotal = num(gpu["mem_total"]);
    s.gpu.power = num(gpu["power"]);
    s.gpu.present = !isnan(s.gpu.pct);
  }

  JsonObjectConst net = o["net"];
  if (net) {
    str(s.net.iface, net["iface"]);
    str(s.net.ip, net["ip"]);
    s.net.rx = num(net["rx"]);
    s.net.tx = num(net["tx"]);
  }

  JsonObjectConst disk = o["disk"];
  if (disk) {
    s.disk.r = num(disk["r"]);
    s.disk.w = num(disk["w"]);
    for (JsonObjectConst m : disk["mounts"].as<JsonArrayConst>()) {
      if (s.disk.nMounts >= MAX_MOUNTS) break;
      Mount& d = s.disk.mounts[s.disk.nMounts];
      str(d.m, m["m"]);
      d.pct = numClamp(m["pct"], 0, 100);
      d.used = num(m["used"]);
      d.total = num(m["total"]);
      if (isnan(d.pct) && d.total > 0 && !isnan(d.used)) d.pct = 100.0f * d.used / d.total;
      if (!isnan(d.pct)) s.disk.nMounts++;
    }
  }

  for (JsonObjectConst t : o["temps"].as<JsonArrayConst>()) {
    if (s.nTemps >= MAX_TEMPS) break;
    NamedValue& d = s.temps[s.nTemps];
    str(d.name, t["n"]);
    d.v = num(t["v"]);
    if (!isnan(d.v)) s.nTemps++;
  }
  for (JsonObjectConst f : o["fans"].as<JsonArrayConst>()) {
    if (s.nFans >= MAX_FANS) break;
    NamedValue& d = s.fans[s.nFans];
    str(d.name, f["n"]);
    d.v = num(f["rpm"]);
    if (!isnan(d.v)) s.nFans++;
  }
  for (JsonObjectConst p : o["procs"].as<JsonArrayConst>()) {
    if (s.nProcs >= MAX_PROCS) break;
    Proc& d = s.procs[s.nProcs];
    str(d.n, p["n"]);
    d.c = num(p["c"]);
    d.m = num(p["m"]);
    if (d.n[0]) s.nProcs++;
  }

  JsonObjectConst bat = o["bat"];
  if (bat) {
    s.bat.pct = numClamp(bat["pct"], 0, 100);
    if (bat["plug"].is<bool>()) s.bat.plug = bat["plug"].as<bool>() ? 1 : 0;
  }

  // link bookkeeping + history
  Link& l = state.link;
  l.frames++;
  l.lastFrameMs = millis();
  if (s.hasTs) {
    l.clockValid = true;
    l.clockLocal = s.ts + s.tz;
    l.clockMs = l.lastFrameMs;
  }
  state.hist.cpu.push(s.cpu.pct);
  state.hist.rx.push(s.net.rx);
  state.hist.tx.push(s.net.tx);

  onStatsFrame();
}

void parseCfg(JsonObjectConst o) {
  Settings& st = settings;
  uint32_t mask = 0;
  int forcedPage = -1;

  float f;
  if (!isnan(f = numClamp(o["bright"], 0, 255))) {
    st.bright = (uint8_t)f;
    mask |= CFG_BRIGHT;
  }
  if (o["auto_bright"].is<bool>()) {
    st.autoBright = o["auto_bright"].as<bool>();
    mask |= CFG_BRIGHT;
  }
  if (o["led"].is<bool>()) {
    st.led = o["led"].as<bool>();
    mask |= CFG_LED;
  }
  if (o["beep"].is<bool>()) {
    st.beep = o["beep"].as<bool>();
    mask |= CFG_OTHER;
  }
  if (!isnan(f = num(o["rotate"]))) {
    st.rotate = f >= 0.5f ? 1 : 0;
    mask |= CFG_ROTATE;
  }
  if (!isnan(f = num(o["page"])) && f >= 0) {
    forcedPage = (int)f;
    mask |= CFG_PAGE;
  }
  if (!isnan(f = numClamp(o["cycle"], 0, 3600))) {
    st.cycle = (uint16_t)f;
    mask |= CFG_OTHER;
  }
  if (!isnan(f = numClamp(o["alert_temp"], 0, 150))) {
    st.alertTemp = f;
    mask |= CFG_OTHER;
  }
  if (!isnan(f = numClamp(o["alert_cpu"], 0, 100))) {
    st.alertCpu = f;
    mask |= CFG_OTHER;
  }
  int i;  // unknown names are ignored
  if ((i = themeFind(o["theme"].as<const char*>())) >= 0) {
    st.theme = (uint8_t)i;
    mask |= CFG_OTHER;
  }
  if ((i = themeModeFind(o["theme_mode"].as<const char*>())) >= 0) {
    st.themeMode = (uint8_t)i;
    mask |= CFG_OTHER;
  }
  if (mask) {
    settingsMarkDirty();
    onConfig(mask, forcedPage);
  }
}

void processLine(char* p, size_t len) {
  // trim leading whitespace (and stray NULs from a reset glitch)
  while (len && (*p == ' ' || *p == '\t' || *p == 0)) {
    p++;
    len--;
  }
  if (!len) return;
  if (*p != '{') {  // boot noise / garbage
    state.link.parseErrors++;
    return;
  }
  DeserializationError err = deserializeJson(doc, p, len);
  if (err) {
    state.link.parseErrors++;
    return;
  }
  JsonObjectConst o = doc.as<JsonObjectConst>();
  const char* t = o["t"].as<const char*>();
  if (!t) {
    state.link.parseErrors++;
    return;
  }
  if (!strcmp(t, "s")) {
    parseStats(o);
  } else if (!strcmp(t, "hello")) {
    str(state.link.agent, o["agent"]);
    state.link.helloCount++;
    sendHi();
    onHello();
  } else if (!strcmp(t, "cfg")) {
    parseCfg(o);
  } else if (!strcmp(t, "shot")) {
    screenshotSend();
  }
  // unknown types are ignored per spec
  doc.clear();
}

}  // namespace

void protocolBegin() {
  Serial.setRxBufferSize(SERIAL_RX_BUFFER);
  Serial.begin(SERIAL_BAUD);
  sendHi();
}

void protocolPoll() {
  // Bounded work per call so the UI stays responsive under a flood.
  int budget = 4096;
  while (budget-- > 0 && Serial.available() > 0) {
    int c = Serial.read();
    if (c < 0) break;
    if (c == '\n') {
      if (overflow) {
        state.link.parseErrors++;
      } else {
        line[lineLen] = 0;
        processLine(line, lineLen);
      }
      lineLen = 0;
      overflow = false;
    } else if (c == '\r') {
      // ignore (CRLF tolerant)
    } else if (overflow) {
      // discard until the end of the overlong line
    } else if (lineLen < MAX_LINE) {
      line[lineLen++] = (char)c;
    } else {
      overflow = true;
    }
  }
}

bool hostConnected() {
  return state.link.frames > 0 && (millis() - state.link.lastFrameMs) < HOST_TIMEOUT_MS;
}

void sendPageEvent(int page) {
  Serial.print("{\"t\":\"ev\",\"e\":\"page\",\"page\":" + String(page) + "}\n");
}

void sendThemeEvent(const char* theme, const char* mode, const char* variant) {
  char buf[96];
  snprintf(buf, sizeof buf,
           "{\"t\":\"ev\",\"e\":\"theme\",\"theme\":\"%s\",\"mode\":\"%s\",\"variant\":\"%s\"}\n",
           theme, mode, variant);
  Serial.print(buf);
}

void sendLightEvent(int raw, float level) {
  char buf[64];
  snprintf(buf, sizeof buf, "{\"t\":\"ev\",\"e\":\"light\",\"raw\":%d,\"level\":%.2f}\n", raw,
           level < 0 ? 0.0f : level);
  Serial.print(buf);
}

void sendLog(const char* msg) {
  JsonDocument d;
  d["t"] = "log";
  d["msg"] = msg;
  serializeJson(d, Serial);
  Serial.print('\n');
}
