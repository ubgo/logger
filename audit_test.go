package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestEventsNotMessages(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithLevel(LevelTrace))
	l.Event("user.signup", String("plan", "pro"), Int("uid", 7))

	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &m); err != nil {
		t.Fatalf("bad json: %v\n%s", err, buf.String())
	}
	if m["event"] != "user.signup" {
		t.Fatalf("event name not the primary key: %v", m)
	}
	if m["msg"] != "" {
		t.Fatalf("event should have no message, got %q", m["msg"])
	}
	if m["plan"] != "pro" || m["uid"].(float64) != 7 {
		t.Fatalf("event fields lost: %v", m)
	}
}

func TestAuditChainVerifies(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	a := NewAuditSink(&buf, NewJSONEncoder())
	l := New(WithTransport(NewSyncTransport(a)), WithLevel(LevelTrace))
	for i := 0; i < 50; i++ {
		l.Info("audit entry", Int("i", i))
	}
	res := VerifyAudit(bytes.NewReader(buf.Bytes()))
	if !res.OK || res.Records != 50 {
		t.Fatalf("clean chain failed verification: %+v", res)
	}
}

func TestAuditDetectsTampering(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	a := NewAuditSink(&buf, NewJSONEncoder())
	l := New(WithTransport(NewSyncTransport(a)), WithLevel(LevelTrace))
	for i := 0; i < 10; i++ {
		l.Info("entry", Int("i", i))
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// tamper: flip a value in line 4's canonical JSON, keep everything else
	lines[4] = strings.Replace(lines[4], `"i":4`, `"i":999`, 1)
	tampered := strings.Join(lines, "\n")

	res := VerifyAudit(strings.NewReader(tampered))
	if res.OK {
		t.Fatal("verifier failed to detect a tampered record")
	}
	if res.BrokenAtSeq != 4 {
		t.Fatalf("expected break at seq 4, got %d (%s)", res.BrokenAtSeq, res.Reason)
	}
}

func TestAuditDetectsDeletion(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	a := NewAuditSink(&buf, NewJSONEncoder())
	l := New(WithTransport(NewSyncTransport(a)), WithLevel(LevelTrace))
	for i := 0; i < 6; i++ {
		l.Info("e", Int("i", i))
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// delete line 3
	out := append(lines[:3], lines[4:]...)
	res := VerifyAudit(strings.NewReader(strings.Join(out, "\n")))
	if res.OK {
		t.Fatal("verifier failed to detect a deleted record")
	}
}
