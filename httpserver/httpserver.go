package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/AltSoyuz/lib/buildinfo"
	"github.com/AltSoyuz/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

// Serve starts HTTP servers on the given addresses with the provided handler.
// It listens for context cancellation to initiate a graceful shutdown.
// It returns an error if any server fails to start or if shutdown is problematic.
func Serve(ctx context.Context, addr string, handler http.Handler) error {
	// Listener with TCP keep-alive configured simply.
	lc := net.ListenConfig{
		KeepAlive: 3 * time.Minute,
	}
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	return ServeWithListener(ctx, ln, handler)
}

// ServeWithListener starts an HTTP server using a provided net.Listener.
// Useful for tests: you can create a listener to retrieve the address and
// control the server's lifecycle from the test.
func ServeWithListener(ctx context.Context, ln net.Listener, handler http.Handler) error {
	logger.InfoSkipframes(2, "listening", "addr", ln.Addr().String())

	srv := &http.Server{
		Handler:           wrapHandlerWithBuiltins(handler),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          logger.StdErrorLogger(),
	}

	return serveWithShutdown(ctx, srv, ln)
}

// serveWithShutdown manages the lifecycle of an HTTP server with graceful shutdown.
func serveWithShutdown(ctx context.Context, srv *http.Server, ln net.Listener) error {
	errCh := make(chan error, 1)

	go func() {
		// Start the server and send errors to the channel if it fails unexpectedly.
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		// Bounded graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		shutdownErr := srv.Shutdown(shutdownCtx) // Capture the error

		// Drain any potential error from Serve
		if err := <-errCh; err != nil {
			return err // Actual server error
		}
		// If the shutdown exceeded the timeout, signal it
		if shutdownErr == context.DeadlineExceeded {
			return shutdownErr
		}
		// Otherwise, normal shutdown → no error
		return nil

	case err := <-errCh:
		// Unexpected failure of Serve
		return err
	}
}

// ErrResponse is the JSON shape written by WriteError.
type ErrResponse struct {
	Error string `json:"error"`
}

// WriteJSON writes a JSON response with the given status code and value.
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("X-Content-Type-Options", "nosniff")

	// Add request ID to the response header if present
	rid := r.Header.Get("X-Request-Id")
	if rid != "" {
		h.Set("X-Request-Id", rid)
	}

	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// WriteError writes an error response with the given status code and error message.
func WriteError(w http.ResponseWriter, r *http.Request, status int, err error) {
	msg := http.StatusText(status)
	if err != nil {
		msg = err.Error()
		args := []any{
			"status", status,
			"method", r.Method,
			"path", r.URL.Path,
			"err", err.Error(), // Flatten error
		}
		if rid := r.Header.Get("X-Request-Id"); rid != "" {
			args = append(args, "rid", rid) // Non-empty
		}
		logger.Error("http error", args...)
	}

	WriteJSON(w, r, status, ErrResponse{Error: msg})
}

// DecodeJSON decodes a JSON request body into the specified type T.
// It ensures that no unknown fields are present and validates the input.
func DecodeJSON[T any](r *http.Request) (T, error) {
	var v T
	defer func() {
		// Ensure the request body is closed after decoding
		if err := r.Body.Close(); err != nil {
			logger.Error("error closing request body", "err", err)
		}
	}()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, err
	}
	// Check for unexpected extra data in the request body
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return v, io.ErrUnexpectedEOF
	}

	return v, nil
}

// ParsePaginationParams extracts offset, perPage, and page from query parameters.
func ParsePaginationParams(r *http.Request) (offset int64, perPage int64, page int64) {
	q := r.URL.Query()
	page = 1
	perPage = 20
	if v := q.Get("page"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			page = n
		}
	}
	if v := q.Get("per_page"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 && n <= 100 {
			perPage = n
		}
	}
	offset = (page - 1) * perPage
	return offset, perPage, page
}

// statusResponseWriter wraps http.ResponseWriter to capture the HTTP status code.
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *statusResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *statusResponseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// wrapHandlerWithBuiltins wraps the provided handler with built-in routes for health checks, version, and metrics.
// It also records per-request HTTP metrics for all non-builtin routes.
func wrapHandlerWithBuiltins(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/healthz":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/version":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(buildinfo.Version))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/metrics":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			metrics.WritePrometheus(w, true)
			return
		}

		// Track request duration and status code for all application routes.
		srw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(srw, r)
		dur := time.Since(start).Seconds()

		path := r.URL.Path
		method := r.Method
		statusCode := strconv.Itoa(srw.statusCode)

		metrics.GetOrCreateCounter(fmt.Sprintf(`app_http_requests_total{method=%q,path=%q,status_code=%q}`, method, path, statusCode)).Inc()
		metrics.GetOrCreateHistogram(fmt.Sprintf(`app_http_request_duration_seconds{method=%q,path=%q}`, method, path)).Update(dur)
	})
}
