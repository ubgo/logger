package logger

// Encoder serializes a Record into a buffer. It must not retain the Record.
// Encoders are values (cheap to copy) and safe for concurrent use.
type Encoder interface {
	// Encode writes one fully-formed log line (including trailing newline)
	// into buf.
	Encode(buf *buffer, r *Record)
	// Name identifies the encoder for diagnostics/config.
	Name() string
}

// appendField dispatches a Field to the right buffer writer without
// reflecting on common scalar types.
func appendFieldValue(buf *buffer, f Field, jsonMode bool) {
	switch f.knd {
	case kindString:
		if jsonMode {
			buf.writeJSONString(f.string())
		} else {
			buf.writeString(f.string())
		}
	case kindInt64:
		buf.writeInt(f.int64())
	case kindUint64:
		buf.writeUint(f.uint64())
	case kindFloat64:
		buf.writeFloat(f.float64())
	case kindBool:
		buf.writeBool(f.bool())
	case kindDuration:
		if jsonMode {
			buf.writeJSONString(f.duration().String())
		} else {
			buf.writeString(f.duration().String())
		}
	case kindTime:
		if jsonMode {
			buf.writeByte('"')
			buf.writeTimeRFC3339(f.time())
			buf.writeByte('"')
		} else {
			buf.writeTimeRFC3339(f.time())
		}
	case kindError:
		e := f.errVal()
		s := "null"
		if e != nil {
			s = e.Error()
		}
		if jsonMode && e != nil {
			buf.writeJSONString(s)
		} else if jsonMode {
			buf.writeString("null")
		} else {
			buf.writeString(s)
		}
	default: // kindAny
		appendAny(buf, f.any, jsonMode)
	}
}
