package logger

import (
	"io"
	"os"
)

// isTTY reports whether w is an interactive terminal. Zero-dependency check:
// a char device that isn't redirected to a file/pipe. Honors the NO_COLOR
// convention (https://no-color.org).
func isTTY(w io.Writer) bool {
	if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// NewConsoleSink builds a console WriterSink that auto-enables color only when
// w is a real terminal and NO_COLOR is unset — pretty in dev, plain in
// files/CI without any flag.
func NewConsoleSink(w io.Writer, minLevel Level) *WriterSink {
	enc := NewConsoleEncoder()
	enc.Color = isTTY(w)
	return NewWriterSink(w, enc, minLevel)
}
