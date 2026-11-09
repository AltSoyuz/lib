package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AltSoyuz/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

func TestWrapHandlerWithBuiltins(t *testing.T) {
	h := wrapHandlerWithBuiltins(http.NotFoundHandler())

	t.Run("healthz returns OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		res := w.Result()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("healthz status = %d; want %d", res.StatusCode, http.StatusOK)
		}
		body := w.Body.String()
		if body != "OK" {
			t.Fatalf("healthz body = %q; want %q", body, "OK")
		}
		if ct := res.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
			t.Fatalf("healthz content-type = %q; want %q", ct, "text/plain; charset=utf-8")
		}
	})

	t.Run("metrics returns 200 and content-type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		res := w.Result()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("metrics status = %d; want %d", res.StatusCode, http.StatusOK)
		}
		if ct := res.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
			t.Fatalf("metrics content-type = %q; want %q", ct, "text/plain; charset=utf-8")
		}
	})
}

// metricLines returns the lines of the registered metrics that contain every one of the given substrings.
func metricLines(t *testing.T, contains ...string) []string {
	t.Helper()
	var buf bytes.Buffer
	metrics.WritePrometheus(&buf, false)
	var out []string
	for _, line := range strings.Split(buf.String(), "\n") {
		match := true
		for _, c := range contains {
			match = match && strings.Contains(line, c)
		}
		if match {
			out = append(out, line)
		}
	}
	return out
}

func TestRequestMetricsLabelByRoutePattern(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrictest/files/{id}", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("POST /metrictest/upload/{id}", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/metrictest/any/{id}", func(w http.ResponseWriter, r *http.Request) {})
	h := wrapHandlerWithBuiltins(mux)

	serve := func(method, path string) {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, path, nil))
	}

	t.Run("two ids of one pattern make one series", func(t *testing.T) {
		serve(http.MethodGet, "/metrictest/files/123")
		serve(http.MethodGet, "/metrictest/files/124")

		lines := metricLines(t, "app_http_requests_total{", "metrictest/files")
		want := `app_http_requests_total{method="GET",path="/metrictest/files/{id}",status_code="200"} 2`
		if len(lines) != 1 || lines[0] != want {
			t.Fatalf("request counters = %q; want [%q]", lines, want)
		}
		lines = metricLines(t, "app_http_request_duration_seconds_count{", "metrictest/files")
		want = `app_http_request_duration_seconds_count{method="GET",path="/metrictest/files/{id}"} 2`
		if len(lines) != 1 || lines[0] != want {
			t.Fatalf("duration counts = %q; want [%q]", lines, want)
		}
	})

	t.Run("unknown path is unmatched", func(t *testing.T) {
		serve(http.MethodGet, "/metrictest/unknown/1")
		serve(http.MethodGet, "/metrictest/unknown/2")

		if lines := metricLines(t, "metrictest/unknown"); len(lines) != 0 {
			t.Fatalf("raw path leaked into the metrics: %q", lines)
		}
		lines := metricLines(t, "app_http_requests_total{", `path="unmatched"`, `status_code="404"`)
		if len(lines) == 0 {
			t.Fatal("no unmatched series for the 404")
		}
	})

	t.Run("method prefix is dropped from the label", func(t *testing.T) {
		serve(http.MethodPost, "/metrictest/upload/7")

		lines := metricLines(t, "app_http_requests_total{", "metrictest/upload")
		want := `app_http_requests_total{method="POST",path="/metrictest/upload/{id}",status_code="200"} 1`
		if len(lines) != 1 || lines[0] != want {
			t.Fatalf("request counters = %q; want [%q]", lines, want)
		}
		if lines := metricLines(t, "metrictest", `path="GET `); len(lines) != 0 {
			t.Fatalf("method leaked into the path label: %q", lines)
		}
	})

	t.Run("pattern without method is kept as is", func(t *testing.T) {
		serve(http.MethodPut, "/metrictest/any/9")

		lines := metricLines(t, "app_http_requests_total{", "metrictest/any")
		want := `app_http_requests_total{method="PUT",path="/metrictest/any/{id}",status_code="200"} 1`
		if len(lines) != 1 || lines[0] != want {
			t.Fatalf("request counters = %q; want [%q]", lines, want)
		}
	})
}

func TestWriteJSONAndWriteError(t *testing.T) {
	t.Run("WriteJSON sets headers and body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("X-Request-Id", "rid-1")
		w := httptest.NewRecorder()

		WriteJSON(w, req, http.StatusCreated, map[string]string{"a": "b"})

		res := w.Result()
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d; want %d", res.StatusCode, http.StatusCreated)
		}
		if ct := res.Header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
			t.Fatalf("content-type = %q; want %q", ct, "application/json; charset=utf-8")
		}
		if got := res.Header.Get("X-Request-Id"); got != "rid-1" {
			t.Fatalf("X-Request-Id = %q; want %q", got, "rid-1")
		}
		var body map[string]string
		if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if body["a"] != "b" {
			t.Fatalf("body = %v; want map[a:b]", body)
		}
	})

	t.Run("WriteError returns JSON error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		WriteError(w, req, http.StatusBadRequest, errors.New("boom"))
		res := w.Result()
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d; want %d", res.StatusCode, http.StatusBadRequest)
		}
		var er struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(res.Body).Decode(&er); err != nil {
			t.Fatalf("decode error body: %v", err)
		}
		if er.Error != "boom" {
			t.Fatalf("error field = %q; want %q", er.Error, "boom")
		}
	})
}

func TestDecodeJSON(t *testing.T) {
	t.Run("valid JSON decodes", func(t *testing.T) {
		type payload struct {
			Name string `json:"name"`
		}
		body := `{"name":"alice"}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		var got payload
		v, err := DecodeJSON[payload](req)
		got = v
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name != "alice" {
			t.Fatalf("got.Name = %q; want %q", got.Name, "alice")
		}
	})

	t.Run("unknown field returns error", func(t *testing.T) {
		type payload struct {
			Name string `json:"name"`
		}
		body := `{"name":"alice","extra":1}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		_, err := DecodeJSON[payload](req)
		if err == nil {
			t.Fatalf("expected error for unknown field, got nil")
		}
	})

	t.Run("trailing data returns io.ErrUnexpectedEOF", func(t *testing.T) {
		type payload struct {
			Name string `json:"name"`
		}
		// valid object followed by extra token
		body := `{"name":"alice"} 42`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		_, err := DecodeJSON[payload](req)
		if err == nil {
			t.Fatalf("expected error for trailing data, got nil")
		}
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("expected io.ErrUnexpectedEOF, got %v", err)
		}
	})
}

func TestServeGracefulShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	started := make(chan struct{})
	proceed := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// signal that handler started handling the request
		close(started)
		// wait until allowed to proceed (simulates long-running work)
		<-proceed
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("done"))
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ServeWithListener(ctx, ln, handler)
	}()

	// perform a client request; it will block in the handler until we close 'proceed'
	respCh := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get("http://" + ln.Addr().String() + "/")
		if err != nil {
			errCh <- err
			return
		}
		respCh <- resp
	}()

	// wait until handler actually started processing the request
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handler not started")
	}

	// cancel context to trigger graceful shutdown
	cancel()

	// allow handler to finish
	close(proceed)

	// verify client received response
	var resp *http.Response
	select {
	case r := <-respCh:
		resp = r
	case err := <-errCh:
		t.Fatalf("client error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("client did not receive response")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// verify ServeWithListener returned due to context cancellation
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned unexpected error: %v; want nil on graceful shutdown", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serve did not return after cancel")
	}
}

func TestWriteErrorClientCanceled(t *testing.T) {
	var logs strings.Builder
	logger.SetOutput(&logs)
	t.Cleanup(func() { logger.SetOutput(os.Stderr) })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w := httptest.NewRecorder()
	WriteError(w, httptest.NewRequest(http.MethodGet, "/api/v1/guide", nil).WithContext(ctx), http.StatusInternalServerError, fmt.Errorf("internal: %w", context.Canceled))
	if w.Body.Len() != 0 {
		t.Fatalf("a canceled request got a body: %q", w.Body.String())
	}
	if out := logs.String(); strings.Contains(out, " error ") || !strings.Contains(out, " info ") || !strings.Contains(out, "http request canceled by the client") || !strings.Contains(out, "path=/api/v1/guide") {
		t.Fatalf("logs = %q", out)
	}

	// A request that ran out of time is the server's problem: it stays an error.
	logs.Reset()
	ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	w = httptest.NewRecorder()
	WriteError(w, httptest.NewRequest(http.MethodGet, "/slow", nil).WithContext(ctx), http.StatusInternalServerError, context.DeadlineExceeded)
	if w.Code != http.StatusInternalServerError || !strings.Contains(logs.String(), " error ") {
		t.Fatalf("deadline: status %d, logs %q", w.Code, logs.String())
	}
}
