package logger

// JSONEncoder emits one JSON object per line (ndjson) with OTEL-aligned keys.
// Configurable key names keep it compatible with downstream log backends.
type JSONEncoder struct {
	TimeKey  string
	LevelKey string
	MsgKey   string
	// NumericLevel emits the OTEL SeverityNumber instead of the band name.
	NumericLevel bool
}

// NewJSONEncoder returns a JSONEncoder with conventional keys.
func NewJSONEncoder() JSONEncoder {
	return JSONEncoder{TimeKey: "time", LevelKey: "level", MsgKey: "msg"}
}

// Name implements Encoder.
func (JSONEncoder) Name() string { return "json" }

// Encode implements Encoder.
func (e JSONEncoder) Encode(buf *buffer, r *Record) {
	tk, lk, mk := e.TimeKey, e.LevelKey, e.MsgKey
	if tk == "" {
		tk = "time"
	}
	if lk == "" {
		lk = "level"
	}
	if mk == "" {
		mk = "msg"
	}
	buf.writeByte('{')
	first := true
	if !r.Time.IsZero() { // zero time → omit key (slog semantics)
		buf.writeJSONString(tk)
		buf.writeByte(':')
		buf.writeByte('"')
		buf.writeTimeRFC3339(r.Time)
		buf.writeByte('"')
		first = false
	}
	if !first {
		buf.writeByte(',')
	}
	buf.writeJSONString(lk)
	buf.writeByte(':')
	if e.NumericLevel {
		buf.writeInt(int64(r.Level))
	} else {
		b, _ := r.Level.MarshalText()
		buf.writeJSONString(string(b))
	}

	buf.writeByte(',')
	buf.writeJSONString(mk)
	buf.writeByte(':')
	buf.writeJSONString(r.Message)

	if src, ok := r.source(); ok {
		buf.writeByte(',')
		buf.writeJSONString("caller")
		buf.writeByte(':')
		buf.writeByte('"')
		buf.writeString(src.File)
		buf.writeByte(':')
		buf.writeInt(int64(src.Line))
		buf.writeByte('"')
	}

	for i := range r.Fields {
		f := &r.Fields[i]
		buf.writeByte(',')
		buf.writeJSONString(f.Key)
		buf.writeByte(':')
		appendFieldValue(buf, *f, true)
	}
	buf.writeByte('}')
	buf.writeByte('\n')
}
