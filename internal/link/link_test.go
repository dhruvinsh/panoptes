package link

import (
	"testing"

	"github.com/dhruvinsh/panoptes/internal/frame"
	"github.com/dhruvinsh/panoptes/internal/logx"
)

type rec struct {
	l   logx.Level
	msg string
}

func capture(t *testing.T) *[]rec {
	t.Helper()
	var got []rec
	restore := logx.SetHook(func(l logx.Level, m string) { got = append(got, rec{l, m}) })
	t.Cleanup(restore)
	return &got
}

func has(recs []rec, l logx.Level, msg string) bool {
	for _, r := range recs {
		if r.l == l && r.msg == msg {
			return true
		}
	}
	return false
}

func TestDeviceEventLogging(t *testing.T) {
	got := capture(t)
	d := NewDevice("/dev/null", 115200)
	d.HandleLine([]byte(`{"t":"ev","e":"theme","theme":"nord","mode":"auto","variant":"light"}`))
	d.HandleLine([]byte(`{"t":"ev","e":"light","raw":412,"level":0.43}`))
	d.HandleLine([]byte(`{"t":"ev","e":"page","page":2}`))
	d.HandleLine([]byte(`garbage {`))
	d.HandleLine([]byte("\xff\x00x\xf8xx " + `{"t":"hi","v":1,"fw":"0.1.0","board":"ESP32-2432S028R","w":320,"h":240}`))
	for _, want := range []rec{
		{logx.INFO, "device theme: nord (mode auto, showing light)"},
		{logx.DEBUG, "device light: raw=412 level=0.43"},
		{logx.INFO, "device event: {'e': 'page', 'page': 2}"},
		{logx.DEBUG, "device (non-JSON): garbage {"},
		{logx.INFO, "device hi: fw=0.1.0 board=ESP32-2432S028R 320x240 proto=v1"},
	} {
		if !has(*got, want.l, want.msg) {
			t.Errorf("missing log %v %q in %v", want.l, want.msg, *got)
		}
	}
	if d.HiCount() != 1 {
		t.Fatalf("hi count %d", d.HiCount())
	}
	select {
	case <-d.HiChan():
	default:
		t.Fatal("hi channel not closed")
	}
}

func TestHelloMsg(t *testing.T) {
	old := Agent
	Agent = "panoptes 1.2.3"
	defer func() { Agent = old }()
	if s := string(encodeHello()); s != `{"t":"hello","v":1,"agent":"panoptes 1.2.3"}`+"\n" {
		t.Fatalf("got %s", s)
	}
}

func encodeHello() []byte { return frame.Encode(HelloMsg()) }
