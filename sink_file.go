package logger

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// RotatingFile is an io.WriteCloser with owned size-based rotation, age/count
// retention, and optional gzip compression of rotated segments — so a "last
// logger" doesn't punt rotation to lumberjack/logrotate. It also supports
// Reopen() for logrotate-style external rotation (SIGHUP).
type RotatingFile struct {
	// Path is the active log file.
	Path string
	// MaxSizeBytes triggers rotation when exceeded (default 100 MiB).
	MaxSizeBytes int64
	// MaxBackups caps retained rotated files (0 = keep all).
	MaxBackups int
	// MaxAge prunes rotated files older than this (0 = no age limit).
	MaxAge time.Duration
	// Compress gzips rotated segments.
	Compress bool

	mu   sync.Mutex
	f    *os.File
	size int64
}

// NewRotatingFile opens (creating dirs as needed) the log file.
func NewRotatingFile(path string) (*RotatingFile, error) {
	r := &RotatingFile{Path: path, MaxSizeBytes: 100 << 20, MaxBackups: 7}
	if err := r.openExisting(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *RotatingFile) openExisting() error {
	if err := os.MkdirAll(filepath.Dir(r.Path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(r.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	r.f, r.size = f, fi.Size()
	return nil
}

// Write implements io.Writer; rotates first if the line would exceed the cap.
func (r *RotatingFile) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	max := r.MaxSizeBytes
	if max <= 0 {
		max = 100 << 20
	}
	if r.size+int64(len(p)) > max && r.size > 0 {
		if err := r.rotateLocked(); err != nil {
			return 0, err
		}
	}
	n, err := r.f.Write(p)
	r.size += int64(n)
	return n, err
}

func (r *RotatingFile) rotateLocked() error {
	if err := r.f.Close(); err != nil {
		return err
	}
	ts := time.Now().Format("20060102T150405.000")
	rotated := fmt.Sprintf("%s.%s", r.Path, ts)
	if err := os.Rename(r.Path, rotated); err != nil {
		return err
	}
	if err := r.openExisting(); err != nil {
		return err
	}
	r.size = 0
	// Compression is the only slow step → off the write path. Retention
	// (prune) runs synchronously so MaxBackups/MaxAge are deterministic and
	// not racy with shutdown.
	if r.Compress {
		go func() {
			if err := gzipFile(rotated); err == nil {
				_ = os.Remove(rotated)
			}
			r.mu.Lock()
			r.prune()
			r.mu.Unlock()
		}()
	}
	r.prune()
	return nil
}

func gzipFile(path string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(path + ".gz")
	if err != nil {
		return err
	}
	defer out.Close()
	zw := gzip.NewWriter(out)
	if _, err := io.Copy(zw, in); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

// prune enforces MaxBackups + MaxAge over the rotated segments.
func (r *RotatingFile) prune() {
	dir := filepath.Dir(r.Path)
	base := filepath.Base(r.Path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type seg struct {
		path string
		mod  time.Time
	}
	var segs []seg
	for _, e := range entries {
		name := e.Name()
		if name == base || !strings.HasPrefix(name, base+".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		segs = append(segs, seg{filepath.Join(dir, name), info.ModTime()})
	}
	sort.Slice(segs, func(i, j int) bool { return segs[i].mod.After(segs[j].mod) })

	now := time.Now()
	for i, s := range segs {
		over := r.MaxBackups > 0 && i >= r.MaxBackups
		old := r.MaxAge > 0 && now.Sub(s.mod) > r.MaxAge
		if over || old {
			_ = os.Remove(s.path)
		}
	}
}

// Reopen closes and reopens the active file — for SIGHUP / logrotate
// (copytruncate-free) external rotation.
func (r *RotatingFile) Reopen() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f != nil {
		_ = r.f.Close()
	}
	return r.openExisting()
}

// Sync flushes to disk.
func (r *RotatingFile) Sync() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.f.Sync()
}

// Close closes the active file.
func (r *RotatingFile) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.f.Close()
}

// NewFileSink builds a Sink writing encoded records to a self-rotating file.
func NewFileSink(rf *RotatingFile, enc Encoder, minLevel Level) *WriterSink {
	return NewWriterSink(rf, enc, minLevel)
}
