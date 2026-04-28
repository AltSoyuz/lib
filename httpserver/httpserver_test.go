package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
