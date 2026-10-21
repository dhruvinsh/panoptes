// Package pause lets one panoptes process ask a running agent (usually the login service)
// to let go of the serial port for a while, e.g. so a flasher can use it. The request is
// a file in the config directory holding the time the pause ends; the agent polls it.
package pause

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhruvinsh/panoptes/internal/config"
)

// Default is how long `panoptes pause` lasts when no duration is given.
const Default = 10 * time.Minute

// Path is the pause file, Dir()/pause.
func Path() string { return filepath.Join(config.Dir(), "pause") }

// Set pauses the agent until the given time.
func Set(until time.Time) error {
	if err := os.MkdirAll(filepath.Dir(Path()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(Path(), []byte(until.UTC().Format(time.RFC3339)+"\n"), 0o644)
}

// Clear ends a pause. Clearing when not paused is not an error.
func Clear() error {
	if err := os.Remove(Path()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Until is when the current pause ends, or the zero time when not paused. An unreadable
// file counts as not paused, and an expired one is removed.
func Until() time.Time { return until(Path(), time.Now()) }

func until(path string, now time.Time) time.Time {
	b, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(b)))
	if err != nil {
		return time.Time{}
	}
	if !now.Before(t) {
		_ = os.Remove(path)
		return time.Time{}
	}
	return t
}
