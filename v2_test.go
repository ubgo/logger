package logger

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSpanInheritedFieldsAndTree(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithProcessors(NewEnrichProcessor()), WithLevel(LevelTrace))

	ctx, root := l.StartSpan(context.Background(), "request", String("route", "/x"))
	l.InfoContext(ctx, "handling")
	cctx, child := l.StartSpan(ctx, "db_query")
	l.InfoContext(cctx, "querying")
	child.End()
	root.End()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// "handling" inherits root span identity + bound field
	var handling map[string]any
	for _, ln := range lines {
		var m map[string]any
		_ = json.Unmarshal([]byte(ln), &m)
		if m["msg"] == "handling" {
			handling = m
		}
	}
	if handling == nil {
		t.Fatalf("no handling line:\n%s", buf.String())
	}
	if handling["span"] != "request" || handling["route"] != "/x" {
		t.Fatalf("child log did not inherit span context: %v", handling)
	}
	if !strings.Contains(buf.String(), `"span_path":"1.1"`) {
		t.Fatalf("nested span path missing (Eliot task_level):\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), `"msg":"span.end"`) ||
		!strings.Contains(buf.String(), `"ok":true`) {
		t.Fatalf("span.end outcome record missing:\n%s", buf.String())
	}
}

func TestSpanFailMarksError(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithLevel(LevelTrace))
	_, s := l.StartSpan(context.Background(), "op")
	s.Fail(errors.New("boom"))
	s.End()
	s.End() // idempotent
	out := buf.String()
	if !strings.Contains(out, `"level":"error"`) || !strings.Contains(out, `"ok":false`) {
		t.Fatalf("failed span not at error/ok=false: %s", out)
	}
	if strings.Count(out, `"msg":"span.end"`) != 1 {
		t.Fatalf("End not idempotent: %s", out)
	}
}

func TestMessageTemplatePreservesTemplate(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithLevel(LevelTrace))
	l.Infot("processed {count} files for {user}", 12, "ada")

	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &m); err != nil {
		t.Fatalf("bad json: %v\n%s", err, buf.String())
	}
	if m["msg"] != "processed 12 files for ada" {
		t.Fatalf("rendered message wrong: %v", m["msg"])
	}
	if m["msg_template"] != "processed {count} files for {user}" {
		t.Fatalf("template not preserved as stable key: %v", m["msg_template"])
	}
	if m["count"].(float64) != 12 || m["user"] != "ada" {
		t.Fatalf("named holes not structured: %v", m)
	}
}

func TestMessageTemplateEscapes(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithLevel(LevelTrace))
	l.Infot("literal {{braces}} and {x}", 9)
	if !strings.Contains(buf.String(), "literal {braces} and 9") {
		t.Fatalf("escape handling wrong: %s", buf.String())
	}
}
