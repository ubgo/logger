package logger

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// FileConfig is the hot-reloadable on-disk config. Kept intentionally tiny —
// the level is the thing operators actually flip in production. Extend via
// OnReload for app-specific knobs.
type FileConfig struct {
	Level string `json:"level"` // band name or OTEL SeverityNumber 1..24
}

// ConfigWatcher polls a JSON config file and applies changes to a LevelVar
// without a restart — no fsnotify dependency (mtime poll keeps the core
// zero-dep). Changes are applied only when the file's mtime changes.
type ConfigWatcher struct {
	path     string
	lv       *LevelVar
	interval time.Duration
	mu       sync.Mutex // guards onReload (set after the goroutine starts)
	onReload func(FileConfig)
	stop     chan struct{}
	once     sync.Once
}

// WatchConfigFile starts a goroutine that re-reads path every interval (min
// 1s) and applies cfg.Level to lv. Returns a stop func. A missing or invalid
// file is ignored (keeps the last good level) rather than crashing — graceful
// degradation, not a silent zero.
func WatchConfigFile(path string, lv *LevelVar, interval time.Duration) (*ConfigWatcher, func()) {
	if interval < time.Second {
		interval = time.Second
	}
	w := &ConfigWatcher{path: path, lv: lv, interval: interval, stop: make(chan struct{})}
	w.applyOnce() // apply immediately so startup honors the file
	go w.run()
	return w, func() { w.Stop() }
}

// OnReload registers a callback invoked with the parsed config on every
// applied change (for app-specific settings beyond level).
func (w *ConfigWatcher) OnReload(fn func(FileConfig)) {
	w.mu.Lock()
	w.onReload = fn
	w.mu.Unlock()
}

func (w *ConfigWatcher) run() {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	var lastMod time.Time
	for {
		select {
		case <-w.stop:
			return
		case <-t.C:
			fi, err := os.Stat(w.path)
			if err != nil {
				continue // missing file: keep last good config
			}
			if m := fi.ModTime(); !m.Equal(lastMod) {
				lastMod = m
				w.apply()
			}
		}
	}
}

func (w *ConfigWatcher) applyOnce() {
	if _, err := os.Stat(w.path); err == nil {
		w.apply()
	}
}

func (w *ConfigWatcher) apply() {
	b, err := os.ReadFile(w.path)
	if err != nil {
		return
	}
	var c FileConfig
	if json.Unmarshal(b, &c) != nil {
		return // invalid JSON: keep last good config
	}
	if lvl, ok := parseLevel(c.Level); ok {
		w.lv.Set(lvl)
	}
	w.mu.Lock()
	cb := w.onReload
	w.mu.Unlock()
	if cb != nil {
		cb(c)
	}
}

// Stop ends the watcher goroutine.
func (w *ConfigWatcher) Stop() {
	w.once.Do(func() { close(w.stop) })
}
