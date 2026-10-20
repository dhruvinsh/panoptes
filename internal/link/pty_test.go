//go:build !windows

package link

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"

	"github.com/dhruvinsh/panoptes/internal/frame"
)

type fakeSrc struct{}

func (fakeSrc) Prime() {}
func (fakeSrc) Collect() *frame.Obj {
	return frame.New("t", "s", "ts", time.Now().Unix(), "cpu", frame.New("pct", 1.0))
}

func rawMode(t *testing.T, f *os.File) {
	t.Helper()
	tio, err := unix.IoctlGetTermios(int(f.Fd()), ioctlGetTermios)
	if err != nil {
		t.Logf("termios: %v", err)
		return
	}
	tio.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	tio.Oflag &^= unix.OPOST
	tio.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	tio.Cflag &^= unix.CSIZE | unix.PARENB
	tio.Cflag |= unix.CS8
	_ = unix.IoctlSetTermios(int(f.Fd()), ioctlSetTermios, tio)
}

// TestHandshakeOverPty: hello -> (boot garbage, log, hi) -> cfg -> frames, plus a
// reboot (a second hi) that must trigger a cfg re-send.
func TestHandshakeOverPty(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("no pty: %v", err)
	}
	defer master.Close()
	defer slave.Close()
	rawMode(t, master)
	rawMode(t, slave)
	port := slave.Name()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &Runner{Source: fakeSrc{}, FixedPort: port, Baud: 115200, Interval: 100 * time.Millisecond,
		Cfg: frame.New("t", "cfg", "bright", 9), Candidates: func() []string { return []string{port} }}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); r.Run(ctx) }()

	type line struct {
		m   map[string]any
		raw string
	}
	lines := make(chan line, 100)
	go func() {
		sc := bufio.NewScanner(master)
		for sc.Scan() {
			var m map[string]any
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
				t.Errorf("host sent invalid JSON %q", sc.Text())
				continue
			}
			lines <- line{m, sc.Text()}
		}
	}()

	var got []line
	sentHi, sentReboot := false, false
	deadline := time.After(10 * time.Second)
	frames := 0
loop:
	for frames < 6 {
		select {
		case l := <-lines:
			got = append(got, l)
			switch l.m["t"] {
			case "hello":
				if !sentHi {
					sentHi = true
					_, _ = master.Write([]byte("garbage from boot rom\xff\xfe\r\n{\"t\":\"log\",\"msg\":\"booting\"}\n" +
						`{"t":"hi","v":1,"fw":"0.1.0","board":"ESP32-2432S028R","w":320,"h":240}` + "\n"))
				}
			case "s":
				frames++
				if frames == 3 && !sentReboot {
					sentReboot = true
					_, _ = master.Write([]byte(`{"t":"hi","v":1,"fw":"0.1.0"}` + "\n"))
				}
			}
		case <-deadline:
			break loop
		}
	}
	cancel()
	wg.Wait()

	var types []string
	for _, l := range got {
		types = append(types, l.m["t"].(string))
	}
	if len(types) == 0 || types[0] != "hello" {
		t.Fatalf("first message should be hello: %v", types)
	}
	if got[0].raw != `{"t":"hello","v":1,"agent":"`+Agent+`"}` {
		t.Fatalf("hello: %s", got[0].raw)
	}
	idxCfg, idxS := -1, -1
	cfgCount := 0
	for i, ty := range types {
		if ty == "cfg" {
			cfgCount++
			if idxCfg < 0 {
				idxCfg = i
			}
		}
		if ty == "s" && idxS < 0 {
			idxS = i
		}
	}
	if idxCfg < 0 || idxS < 0 || idxCfg > idxS {
		t.Fatalf("expected cfg before the first frame: %v", types)
	}
	if got[idxCfg].raw != `{"t":"cfg","bright":9}` {
		t.Fatalf("cfg: %s", got[idxCfg].raw)
	}
	if frames < 6 {
		t.Fatalf("only %d frames: %v", frames, types)
	}
	if cfgCount < 2 {
		t.Fatalf("cfg not re-sent after second hi: %v", types)
	}
}
