package logger

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHTTPBatchSinkLokiPayload(t *testing.T) {
	fixedTime(t)
	var got string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = string(b)
		mu.Unlock()
		w.WriteHeader(204)
	}))
	defer srv.Close()

	sink := NewLokiSink(srv.URL, map[string]string{"app": "test"}, LevelInfo)
	sink.MaxBatch = 2
	l := New(WithTransport(NewSyncTransport(sink)), WithLevel(LevelInfo))
	l.Info("a", String("k", "v"))
	l.Info("b")
	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		g := got
		mu.Unlock()
		if g != "" {
			if !strings.Contains(g, `"streams"`) || !strings.Contains(g, `"app":"test"`) {
				t.Fatalf("bad Loki payload: %s", g)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("Loki sink never POSTed the batch")
		case <-time.After(50 * time.Millisecond):
		}
	}
	_ = l.Close()
}

func TestDatadogSinkSendsAPIKey(t *testing.T) {
	fixedTime(t)
	keyCh := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case keyCh <- r.Header.Get("DD-API-KEY"):
		default:
		}
		w.WriteHeader(202)
	}))
	defer srv.Close()
	sink := NewDatadogSink(srv.URL, "secret-key", LevelInfo)
	sink.MaxBatch = 1
	l := New(WithTransport(NewSyncTransport(sink)), WithLevel(LevelInfo))
	l.Info("ship me")
	select {
	case k := <-keyCh:
		if k != "secret-key" {
			t.Fatalf("DD-API-KEY header = %q", k)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Datadog sink never POSTed")
	}
	_ = l.Close()
}

func TestTCPSinkStreams(t *testing.T) {
	fixedTime(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	lineCh := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		line, _ := bufio.NewReader(c).ReadString('\n')
		lineCh <- line
	}()

	sink := NewTCPSink(ln.Addr().String(), NewJSONEncoder(), LevelInfo)
	l := New(WithTransport(NewSyncTransport(sink)), WithLevel(LevelInfo))
	l.Info("over the wire", String("net", "tcp"))
	select {
	case line := <-lineCh:
		if !strings.Contains(line, `"msg":"over the wire"`) || !strings.Contains(line, `"net":"tcp"`) {
			t.Fatalf("TCP sink garbled: %s", line)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("nothing received over TCP")
	}
	_ = l.Close()
}
