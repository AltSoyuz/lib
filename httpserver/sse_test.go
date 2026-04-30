package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWriteData(t *testing.T) {
	rec := httptest.NewRecorder()
	flusher, ok := any(rec).(http.Flusher)
	if !ok {
		t.Fatalf("recorder does not implement http.Flusher")
	}

	if err := WriteData(rec, flusher, map[string]string{"delta": "hello"}); err != nil {
		t.Fatalf("WriteData failed: %v", err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `data: {"delta":"hello"}`) {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestHeartbeat(t *testing.T) {
	rec := httptest.NewRecorder()
	flusher, ok := any(rec).(http.Flusher)
	if !ok {
		t.Fatalf("recorder does not implement http.Flusher")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		Heartbeat(ctx, rec, flusher, 5*time.Millisecond)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	if !strings.Contains(rec.Body.String(), ": ping\n\n") {
		t.Fatalf("expected heartbeat ping, got body=%q", rec.Body.String())
	}
}
