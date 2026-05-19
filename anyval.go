package logger

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// sprintAny renders any value to a string (used by hashing/redaction).
func sprintAny(v any) string { return fmt.Sprint(v) }

// appendAny handles the kindAny escape hatch. Fast-paths the common scalar
// types; falls back to encoding/json (JSON mode) or fmt (text mode) for
// composite values. A marshal failure degrades gracefully to a quoted error
// string rather than panicking the logger.
func appendAny(buf *buffer, v any, jsonMode bool) {
	switch x := v.(type) {
	case nil:
		buf.writeString("null")
		return
	case string:
		if jsonMode {
			buf.writeJSONString(x)
		} else {
			buf.writeString(x)
		}
		return
	case bool:
		buf.writeBool(x)
		return
	case int:
		buf.writeInt(int64(x))
		return
	case int64:
		buf.writeInt(x)
		return
	case uint64:
		buf.writeUint(x)
		return
	case float64:
		buf.writeFloat(x)
		return
	case error:
		s := x.Error()
		if jsonMode {
			buf.writeJSONString(s)
		} else {
			buf.writeString(s)
		}
		return
	case fmt.Stringer:
		s := x.String()
		if jsonMode {
			buf.writeJSONString(s)
		} else {
			buf.writeString(s)
		}
		return
	}

	if jsonMode {
		raw, err := json.Marshal(v)
		if err != nil {
			buf.writeJSONString("!ERR:" + err.Error())
			return
		}
		buf.b = append(buf.b, raw...)
		return
	}
	buf.b = strconv.AppendQuote(buf.b, fmt.Sprint(v))
}
