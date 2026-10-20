// Package screenshot implements `panoptes screenshot`: it asks the device for its panel
// memory (`shot`), collects the `px` rows and writes a PNG.
package screenshot

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"strconv"
	"time"

	"github.com/dhruvinsh/panoptes/internal/frame"
	"github.com/dhruvinsh/panoptes/internal/link"
	"github.com/dhruvinsh/panoptes/internal/logx"
	"github.com/dhruvinsh/panoptes/internal/ports"
)

// WritePNG writes an RGB8 image; rows are w*3-byte strings (missing rows stay black).
func WritePNG(path string, w, h int, rows [][]byte) error {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		var row []byte
		if y < len(rows) {
			row = rows[y]
		}
		for x := 0; x < w; x++ {
			o := img.PixOffset(x, y)
			if len(row) >= (x+1)*3 {
				img.Pix[o], img.Pix[o+1], img.Pix[o+2] = row[x*3], row[x*3+1], row[x*3+2]
			}
			img.Pix[o+3] = 0xff
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func intField(m link.Msg, k string) (int, bool) {
	switch v := m[k].(type) {
	case json.Number:
		n, err := strconv.Atoi(v.String())
		if err != nil {
			f, ferr := v.Float64()
			return int(f), ferr == nil
		}
		return n, true
	case float64:
		return int(v), true
	}
	return 0, false
}

// Capture grabs one screenshot into out. Returns the exit code.
func Capture(port string, baud int, out string, timeout time.Duration, interrupt <-chan struct{}) int {
	if port == "" {
		c := ports.Candidates()
		if len(c) == 0 {
			logx.Errorf("no device found (try --list / --port)")
			return 1
		}
		port = c[0]
	}
	msgs := make(chan link.Msg, 1024)
	dev := link.NewDevice(port, baud)
	dev.Listener = func(m link.Msg) {
		if t := m.Type(); t == "px" || t == "shot_end" {
			msgs <- m
		}
	}
	if err := dev.Open(); err != nil {
		logx.Errorf("cannot open %s: %v", port, err)
		return 1
	}
	defer dev.Close()

	// Make sure the firmware is up (opening may have reset it) before asking.
	answered := false
	for i := 0; i < 10 && !answered; i++ {
		if err := dev.SendMsg(link.HelloMsg()); err != nil {
			logx.Errorf("serial error on %s: %v", port, err)
			return 1
		}
		select {
		case <-dev.HiChan():
			answered = true
		case <-time.After(2 * time.Second):
		case <-interrupt:
			return 130
		}
	}
	if !answered {
		logx.Errorf("device on %s did not answer hello", port)
		return 1
	}
	logx.Infof("capturing from %s (takes ~25 s at %d baud)...", port, baud)
	if err := dev.SendMsg(frame.New("t", "shot")); err != nil {
		logx.Errorf("serial error on %s: %v", port, err)
		return 1
	}
	rows := map[int][]byte{}
	w, h := 0, 0
	for {
		wait := timeout
		if len(rows) > 0 {
			wait = 5 * time.Second
		}
		var m link.Msg
		select {
		case m = <-msgs:
		case <-time.After(wait):
			logx.Errorf("timed out after %d rows", len(rows))
			return 1
		case <-interrupt:
			return 130
		}
		if err := dev.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
			logx.Errorf("serial error on %s: %v", port, err)
			return 1
		}
		if m.Type() == "shot_end" {
			break
		}
		mw, ok1 := intField(m, "w")
		mh, ok2 := intField(m, "h")
		y, ok3 := intField(m, "y")
		d, _ := m["d"].(string)
		if !ok1 || !ok2 || !ok3 {
			continue
		}
		w, h = mw, mh
		data, err := base64.StdEncoding.DecodeString(d)
		if err != nil {
			data = nil
		}
		rows[y] = data
		if len(rows)%40 == 0 {
			logx.Infof("  %d/%d rows", len(rows), h)
		}
	}
	list := make([][]byte, h)
	missing := 0
	for y := 0; y < h; y++ {
		if r := rows[y]; len(r) == w*3 {
			list[y] = r
		} else {
			missing++
		}
	}
	if missing > 0 {
		logx.Warnf("%d rows missing/corrupt (left black)", missing)
	}
	if err := WritePNG(out, w, h, list); err != nil {
		logx.Errorf("cannot write %s: %v", out, err)
		return 1
	}
	logx.Infof("saved %s (%dx%d)", out, w, h)
	return 0
}
