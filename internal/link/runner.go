package link

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/dhruvinsh/panoptes/internal/frame"
	"github.com/dhruvinsh/panoptes/internal/logx"
	"github.com/dhruvinsh/panoptes/internal/ports"
)

// Timings (PROTOCOL.md).
var (
	HelloInterval        = 2 * time.Second
	ReconnectInterval    = 2 * time.Second
	HandshakeRotateAfter = 10 * time.Second // with several candidate ports, try the next one if no `hi`
	pollInterval         = 50 * time.Millisecond
)

// Source produces stats frames.
type Source interface {
	Prime()
	Collect() *frame.Obj
}

var errHandshakeTimeout = errors.New("handshake timeout")

// Runner is the reconnect loop.
type Runner struct {
	Source     Source
	FixedPort  string
	Baud       int
	Interval   time.Duration
	Cfg        *frame.Obj // sent after every `hi`; nil = none
	Once       bool
	Candidates func() []string // defaults to ports.Candidates

	rotate        int
	waitingLogged bool
	exitCode      int
}

func (r *Runner) candidates() []string {
	if r.Candidates != nil {
		return r.Candidates()
	}
	return ports.Candidates()
}

func (r *Runner) pickPort() string {
	if r.FixedPort != "" {
		return r.FixedPort
	}
	c := r.candidates()
	if len(c) == 0 {
		return ""
	}
	return c[r.rotate%len(c)]
}

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// Run loops until ctx is cancelled (or after one frame with Once). Returns the exit code.
func (r *Runner) Run(ctx context.Context) int {
	r.Source.Prime()
	for ctx.Err() == nil {
		port := r.pickPort()
		if port == "" {
			if !r.waitingLogged {
				logx.Infof("no board serial port found; waiting (use --list / --port)")
				r.waitingLogged = true
			}
			sleepCtx(ctx, ReconnectInterval)
			continue
		}
		r.waitingLogged = false
		dev := NewDevice(port, r.Baud)
		done, err := r.connect(ctx, dev)
		dev.Close()
		if done {
			return r.exitCode
		}
		if errors.Is(err, errHandshakeTimeout) {
			r.rotate++
			logx.Infof("no reply on %s, trying next candidate port", port)
			continue
		}
		if err != nil && ctx.Err() == nil {
			logx.Warnf("serial error on %s: %v; retrying in %.0fs", port, err, ReconnectInterval.Seconds())
			if strings.Contains(err.Error(), "ermission") {
				logx.Warnf("permission denied: install packaging/70-panoptes.rules or join the " +
					"uucp (Arch) / dialout (Debian/Ubuntu) group - see docs/host.md")
			}
		}
		sleepCtx(ctx, ReconnectInterval)
	}
	return 0
}

func (r *Runner) connect(ctx context.Context, dev *Device) (bool, error) {
	if err := dev.Open(); err != nil {
		return false, err
	}
	logx.Infof("opened %s @ %d", dev.Port, dev.Baud)
	return r.session(ctx, dev)
}

// session returns done=true when finished (--once or cancelled), or an error.
func (r *Runner) session(ctx context.Context, dev *Device) (bool, error) {
	opened := time.Now()
	var lastHello time.Time
	var seenHi int64
	nextFrame := time.Now()
	multi := r.FixedPort == "" && len(r.candidates()) > 1
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()
	for ctx.Err() == nil {
		if err := dev.Err(); err != nil {
			return false, err
		}
		now := time.Now()
		if c := dev.HiCount(); c != seenHi { // first hi, or the device rebooted and said hi again
			seenHi = c
			if r.Cfg != nil {
				if err := dev.SendMsg(r.Cfg); err != nil {
					return false, err
				}
				logx.Debugf("sent cfg %s", strings.TrimSpace(string(frame.Encode(r.Cfg))))
			}
			nextFrame = now
		}
		if seenHi == 0 {
			if lastHello.IsZero() || now.Sub(lastHello) >= HelloInterval {
				if err := dev.SendMsg(HelloMsg()); err != nil {
					return false, err
				}
				lastHello = now
				logx.Debugf("sent hello")
			}
			if multi && now.Sub(opened) > HandshakeRotateAfter {
				return false, errHandshakeTimeout
			}
			if r.Once && now.Sub(opened) > HandshakeRotateAfter {
				logx.Errorf("no `hi` from %s within %.0fs; giving up (--once)", dev.Port, HandshakeRotateAfter.Seconds())
				r.exitCode = 1
				return true, nil
			}
		} else if !now.Before(nextFrame) {
			data := frame.FitFrame(r.Source.Collect())
			if err := dev.Send(data); err != nil {
				return false, err
			}
			logx.Debugf("sent frame (%d bytes)", len(data))
			if r.Once {
				time.Sleep(200 * time.Millisecond) // let the OS flush before closing
				return true, nil
			}
			nextFrame = nextFrame.Add(r.Interval)
			if nextFrame.Before(time.Now()) { // fell behind (slow collector) - don't burst
				nextFrame = time.Now().Add(r.Interval)
			}
		}
		select {
		case <-ctx.Done():
		case <-tick.C:
		}
	}
	return true, nil
}
