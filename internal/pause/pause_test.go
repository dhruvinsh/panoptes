package pause

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUntil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pause")
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if got := until(path, now); !got.IsZero() {
		t.Fatalf("no file: got %v, want zero", got)
	}
	end := now.Add(time.Minute)
	if err := os.WriteFile(path, []byte(end.Format(time.RFC3339)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := until(path, now); !got.Equal(end) {
		t.Fatalf("active: got %v, want %v", got, end)
	}
	if got := until(path, end); !got.IsZero() {
		t.Fatalf("expired: got %v, want zero", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expired pause file not removed: %v", err)
	}
	if err := os.WriteFile(path, []byte("soon"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := until(path, now); !got.IsZero() {
		t.Fatalf("garbage: got %v, want zero", got)
	}
}

func TestSetClear(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	end := time.Now().Add(time.Hour).Truncate(time.Second)
	if err := Set(end); err != nil {
		t.Fatal(err)
	}
	if got := Until(); !got.Equal(end) {
		t.Fatalf("got %v, want %v", got, end)
	}
	if err := Clear(); err != nil {
		t.Fatal(err)
	}
	if got := Until(); !got.IsZero() {
		t.Fatalf("after Clear: got %v", got)
	}
	if err := Clear(); err != nil {
		t.Fatalf("second Clear: %v", err)
	}
}
