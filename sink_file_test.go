package logger

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotatingFileRotatesAndPrunes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	rf, err := NewRotatingFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rf.MaxSizeBytes = 200 // tiny → force many rotations
	rf.MaxBackups = 2
	rf.Compress = false

	l := New(WithSink(NewFileSink(rf, NewJSONEncoder(), LevelTrace)), WithLevel(LevelTrace))
	for i := 0; i < 200; i++ {
		l.Info("rotate me", String("i", strings.Repeat("x", 20)))
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	var rotated int
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "app.log.") {
			rotated++
		}
	}
	if rotated == 0 {
		t.Fatal("expected rotated segments, got none")
	}
	if rotated > rf.MaxBackups {
		t.Fatalf("retention not enforced: %d segments > MaxBackups %d", rotated, rf.MaxBackups)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("active log file missing after rotation: %v", err)
	}
}

func TestRotatingFileReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.log")
	rf, err := NewRotatingFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = rf.Write([]byte("one\n"))
	// simulate logrotate moving the file away
	_ = os.Rename(path, filepath.Join(dir, "a.log.moved"))
	if err := rf.Reopen(); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	_, _ = rf.Write([]byte("two\n"))
	_ = rf.Close()
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "two") {
		t.Fatalf("reopen did not create a fresh file: %q", b)
	}
}

func TestStdLogRedirect(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger()
	restore := RedirectStdLog(l, LevelWarn)
	log.Print("legacy line")
	restore()
	log.Print("after restore") // must NOT reach our buffer
	if !strings.Contains(buf.String(), "legacy line") {
		t.Fatalf("std log not redirected: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"level":"warn"`) {
		t.Fatalf("std log level wrong: %s", buf.String())
	}
	if strings.Contains(buf.String(), "after restore") {
		t.Fatal("restore() failed: std log still captured")
	}
}
