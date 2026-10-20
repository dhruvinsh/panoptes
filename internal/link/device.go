// Package link is the serial connection: port opening, the reader goroutine, the
// hello/hi handshake and the reconnect loop.
package link

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.bug.st/serial"

	"github.com/dhruvinsh/panoptes/internal/frame"
	"github.com/dhruvinsh/panoptes/internal/logx"
)

// ProtocolVersion is the protocol spoken by this agent (PROTOCOL.md v1).
const ProtocolVersion = 1

// Agent is the agent string sent in `hello`; main sets it to "panoptes <version>".
var Agent = "panoptes dev"

const (
	readTimeout  = 200 * time.Millisecond
	writeTimeout = 2 * time.Second
	maxGarbage   = 4096 // drop a newline-less buffer (e.g. boot ROM output at another baud)
)

// HelloMsg is the handshake message.
func HelloMsg() *frame.Obj {
	return frame.New("t", "hello", "v", ProtocolVersion, "agent", Agent)
}

// Msg is one decoded device->host message (numbers kept as json.Number).
type Msg map[string]any

// Type is the "t" field.
func (m Msg) Type() string { s, _ := m["t"].(string); return s }

// Device is one open serial port plus a background reader for device->host lines.
type Device struct {
	Port string
	Baud int

	// Listener, if set before Open, is called (from the reader goroutine) for every JSON message.
	Listener func(Msg)

	port    serial.Port
	wmu     sync.Mutex
	errMu   sync.Mutex
	err     error
	hiCount atomic.Int64
	hiOnce  sync.Once
	hiCh    chan struct{}
	infoMu  sync.Mutex
	info    Msg
	closing atomic.Bool
	done    chan struct{}
}

// NewDevice returns an unopened device.
func NewDevice(port string, baud int) *Device {
	return &Device{Port: port, Baud: baud, hiCh: make(chan struct{})}
}

// Open opens the port without resetting the ESP32 where the OS allows it, and starts the reader.
//
// The ESP32 auto-reset circuit pulls EN (reset) low only while RTS is asserted and DTR is
// not. Windows applies the initial DTR/RTS state atomically in the DCB, so both lines
// simply stay deasserted. On Linux/macOS the kernel asserts both at open (harmless: EN
// stays high); we then drop RTS first (DTR still high) and DTR after it, so the lines
// never pass through the RTS-only state that would reset the board. The handshake
// tolerates a reboot anyway if the OS/driver toggles the lines differently.
func (d *Device) Open() error {
	mode := &serial.Mode{BaudRate: d.Baud, DataBits: 8, Parity: serial.NoParity, StopBits: serial.OneStopBit}
	if runtime.GOOS == "windows" {
		mode.InitialStatusBits = &serial.ModemOutputBits{DTR: false, RTS: false}
	}
	p, err := serial.Open(d.Port, mode) // POSIX: also takes TIOCEXCL (no second agent on the port)
	if err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		_ = p.SetRTS(false) // errors ignored: ptys / adapters without modem lines
		_ = p.SetDTR(false)
	}
	if err := p.SetReadTimeout(readTimeout); err != nil {
		p.Close()
		return err
	}
	d.port = p
	d.done = make(chan struct{})
	go d.reader()
	return nil
}

// Close closes the port and waits (briefly) for the reader to stop.
func (d *Device) Close() {
	d.closing.Store(true)
	if d.port != nil {
		_ = d.port.Close()
		select {
		case <-d.done:
		case <-time.After(time.Second):
		}
	}
}

// Err is the reader's error, if the port failed.
func (d *Device) Err() error {
	d.errMu.Lock()
	defer d.errMu.Unlock()
	return d.err
}

func (d *Device) setErr(err error) {
	d.errMu.Lock()
	if d.err == nil {
		d.err = err
	}
	d.errMu.Unlock()
}

// HiCount is the number of `hi` messages received so far.
func (d *Device) HiCount() int64 { return d.hiCount.Load() }

// HiChan is closed when the first `hi` arrives.
func (d *Device) HiChan() <-chan struct{} { return d.hiCh }

// Info is the last `hi` message.
func (d *Device) Info() Msg {
	d.infoMu.Lock()
	defer d.infoMu.Unlock()
	return d.info
}

var errWriteTimeout = errors.New("write timed out")

// Send writes raw bytes (with a 2 s timeout, after which the port is considered dead).
func (d *Device) Send(data []byte) error {
	if err := d.Err(); err != nil {
		return err
	}
	d.wmu.Lock()
	defer d.wmu.Unlock()
	res := make(chan error, 1)
	go func() {
		_, err := d.port.Write(data)
		res <- err
	}()
	select {
	case err := <-res:
		if err != nil {
			d.setErr(err)
		}
		return err
	case <-time.After(writeTimeout):
		d.setErr(errWriteTimeout)
		_ = d.port.Close() // unblocks the stuck write
		return errWriteTimeout
	}
}

// SendMsg encodes and writes one message.
func (d *Device) SendMsg(m *frame.Obj) error { return d.Send(frame.Encode(m)) }

func (d *Device) reader() {
	defer close(d.done)
	var buf []byte
	chunk := make([]byte, 1024)
	for {
		n, err := d.port.Read(chunk)
		if err != nil {
			if !d.closing.Load() {
				d.setErr(err)
			}
			return
		}
		if d.closing.Load() {
			return
		}
		if n == 0 {
			continue // read timeout
		}
		buf = append(buf, chunk[:n]...)
		for {
			i := bytes.IndexByte(buf, '\n')
			if i < 0 {
				break
			}
			line := buf[:i]
			d.HandleLine(line)
			buf = append(buf[:0:0], buf[i+1:]...)
		}
		if len(buf) > maxGarbage {
			buf = buf[:0]
		}
	}
}

// HandleLine processes one device->host line.
func (d *Device) HandleLine(raw []byte) {
	text := strings.TrimSpace(strings.ToValidUTF8(string(raw), "�"))
	if text == "" {
		return
	}
	// The ESP32 boot ROM prints at 74880 baud, so a reset leaves garbage glued
	// to the front of the first message (usually the boot-time `hi`).
	if !strings.HasPrefix(text, "{") {
		if i := strings.Index(text, `{"t":`); i > 0 {
			text = text[i:]
		}
	}
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil || dec.More() {
		logx.Debugf("device (non-JSON): %s", truncate(text, 200))
		return
	}
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	msg := Msg(m)
	if d.Listener != nil {
		d.Listener(msg)
	}
	switch msg.Type() {
	case "hi":
		d.infoMu.Lock()
		d.info = msg
		d.infoMu.Unlock()
		d.hiCount.Add(1)
		d.hiOnce.Do(func() { close(d.hiCh) })
		logx.Infof("device hi: fw=%s board=%s %sx%s proto=v%s", py(msg["fw"]), py(msg["board"]),
			py(msg["w"]), py(msg["h"]), py(msg["v"]))
		if v, ok := msg["v"]; ok && v != nil && !isNum(v, ProtocolVersion) {
			logx.Warnf("device speaks protocol v%s, agent speaks v%d", py(v), ProtocolVersion)
		}
	case "ev":
		LogEvent(msg)
	case "log":
		logx.Infof("device log: %s", py(msg["msg"]))
	case "px", "shot_end":
		// screenshot data, consumed by the listener
	default:
		logx.Debugf("device sent unknown message: %s", truncate(text, 200))
	}
}

// LogEvent logs an `ev` message.
func LogEvent(m Msg) {
	switch m["e"] {
	case "theme":
		logx.Infof("device theme: %s (mode %s, showing %s)", py(m["theme"]), py(m["mode"]), py(m["variant"]))
	case "light": // every 10 s; for calibrating the LDR thresholds
		logx.Debugf("device light: raw=%s level=%s", py(m["raw"]), py(m["level"]))
	default:
		keys := make([]string, 0, len(m))
		for k := range m {
			if k != "t" && k != "e" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		parts := []string{}
		if e, ok := m["e"]; ok {
			parts = append(parts, "'e': "+pyRepr(e))
		}
		for _, k := range keys {
			parts = append(parts, "'"+k+"': "+pyRepr(m[k]))
		}
		logx.Infof("device event: {%s}", strings.Join(parts, ", "))
	}
}

func isNum(v any, want int) bool {
	n, ok := v.(json.Number)
	if !ok {
		return false
	}
	f, err := n.Float64()
	return err == nil && f == float64(want)
}

// py renders a JSON value the way Python's "%s" would (None for missing).
func py(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case string:
		return x
	case json.Number:
		return x.String()
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func pyRepr(v any) string {
	if s, ok := v.(string); ok {
		return "'" + s + "'"
	}
	return py(v)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// String implements fmt.Stringer for log messages.
func (d *Device) String() string { return fmt.Sprintf("%s@%d", d.Port, d.Baud) }
