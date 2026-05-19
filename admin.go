package logger

import (
	"net/http"
	"strconv"
	"strings"
)

// LevelHandler is an http.Handler for runtime level control without a
// restart. GET returns the current level; PUT/POST with body or ?level= sets
// it. Levels accept band names (trace/debug/info/warn/error/fatal) or the raw
// OTEL SeverityNumber.
//
//	mux.Handle("/loglevel", logger.NewLevelHandler(lv))
type LevelHandler struct{ lv *LevelVar }

// NewLevelHandler exposes a *LevelVar over HTTP.
func NewLevelHandler(lv *LevelVar) *LevelHandler { return &LevelHandler{lv: lv} }

func parseLevel(s string) (Level, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "trace":
		return LevelTrace, true
	case "debug":
		return LevelDebug, true
	case "info":
		return LevelInfo, true
	case "warn", "warning":
		return LevelWarn, true
	case "error":
		return LevelError, true
	case "fatal":
		return LevelFatal, true
	}
	if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= 24 {
		return Level(n), true
	}
	return 0, false
}

func (h *LevelHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cur := h.lv.Level()
		b, _ := cur.MarshalText()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"level":"` + string(b) + `","severity_number":` +
			strconv.Itoa(int(cur)) + "}\n"))
	case http.MethodPut, http.MethodPost:
		val := r.URL.Query().Get("level")
		if val == "" {
			buf := make([]byte, 16)
			n, _ := r.Body.Read(buf)
			val = string(buf[:n])
		}
		lvl, ok := parseLevel(val)
		if !ok {
			http.Error(w, "bad level: "+val, http.StatusBadRequest)
			return
		}
		h.lv.Set(lvl)
		b, _ := lvl.MarshalText()
		_, _ = w.Write([]byte(`{"level":"` + string(b) + `"}` + "\n"))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
