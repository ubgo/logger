package logger

import (
	"runtime"
	"strings"
)

// Source is a resolved caller location. Resolution is done lazily by encoders
// (only when a record is actually formatted) so PC capture stays cheap.
type Source struct {
	File string
	Line int
	Func string
}

// source resolves r.PC. Returns ok=false when caller capture was disabled.
func (r *Record) source() (Source, bool) {
	if r.PC == 0 {
		return Source{}, false
	}
	fs := runtime.CallersFrames([]uintptr{uintptr(r.PC)})
	f, _ := fs.Next()
	if f.File == "" {
		return Source{}, false
	}
	// trim to package/file.go:line — full paths are noise in logs
	file := f.File
	if i := strings.LastIndexByte(file, '/'); i >= 0 {
		if j := strings.LastIndexByte(file[:i], '/'); j >= 0 {
			file = file[j+1:]
		} else {
			file = file[i+1:]
		}
	}
	return Source{File: file, Line: f.Line, Func: f.Function}, true
}
