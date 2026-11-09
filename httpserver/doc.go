// Package httpserver provides HTTP server helpers for graceful shutdown, JSON
// responses, strict JSON decoding, pagination, trusted-proxy client IP
// detection, rate limiting, built-in metrics routes, and embedded UI assets.
//
// Serve records app_http_requests_total and app_http_request_duration_seconds with the route pattern the mux matched
// as the path label ("unmatched" when none did), so the handler it serves must be, or pass the request unchanged to, an
// [http.ServeMux].
package httpserver
