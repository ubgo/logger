package logger

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchConfigFileAppliesAndReloads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.json")
	if err := os.WriteFile(path, []byte(`{"level":"warn"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	lv := NewLevelVar(LevelInfo)
	_, stop := WatchConfigFile(path, lv, time.Second)
	defer stop()

	// applied immediately on start
	if lv.Level() != LevelWarn {
		t.Fatalf("startup config not applied: got %v want warn", lv.Level())
	}

	// change the file → watcher picks it up (mtime must differ)
	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(path, []byte(`{"level":"debug"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(4 * time.Second)
	for lv.Level() != LevelDebug {
		select {
		case <-deadline:
			t.Fatalf("hot reload did not apply: level still %v", lv.Level())
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func TestWatchConfigFileIgnoresBadInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.json")
	_ = os.WriteFile(path, []byte(`not json`), 0o644)
	lv := NewLevelVar(LevelInfo)
	_, stop := WatchConfigFile(path, lv, time.Second)
	defer stop()
	if lv.Level() != LevelInfo {
		t.Fatalf("invalid config must keep last good level, got %v", lv.Level())
	}
}
