package sessions

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetCookie(t *testing.T) {
	f := func(req *http.Request, wantName string, wantSecure bool) {
		t.Helper()

		rec := httptest.NewRecorder()
		SetCookie(rec, req, "token", time.Now().Add(30*time.Minute))

		cookies := rec.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("expected 1 cookie, got %d", len(cookies))
		}

		c := cookies[0]
		if c.Name != wantName {
			t.Fatalf("cookie name = %q; want %q", c.Name, wantName)
		}
		if c.Secure != wantSecure {
			t.Fatalf("cookie secure = %v; want %v", c.Secure, wantSecure)
		}
		if !c.HttpOnly {
			t.Fatalf("cookie HttpOnly = false; want true")
		}
		if c.Path != "/" {
			t.Fatalf("cookie Path = %q; want %q", c.Path, "/")
		}
		if c.SameSite != http.SameSiteLaxMode {
			t.Fatalf("cookie SameSite = %v; want %v", c.SameSite, http.SameSiteLaxMode)
		}
	}

	mkReq := func(host string, tlsOn bool, xfp, xfh string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		req.Host = host
		req.RemoteAddr = "127.0.0.1:8080"
		if tlsOn {
			req.TLS = &tls.ConnectionState{}
		}
		if xfp != "" {
			req.Header.Set("X-Forwarded-Proto", xfp)
		}
		if xfh != "" {
			req.Header.Set("X-Forwarded-Host", xfh)
		}
		return req
	}

	mkReqFrom := func(remoteAddr, host string, tlsOn bool, xfp, xfh string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		req.Host = host
		req.RemoteAddr = remoteAddr
		if tlsOn {
			req.TLS = &tls.ConnectionState{}
		}
		if xfp != "" {
			req.Header.Set("X-Forwarded-Proto", xfp)
		}
		if xfh != "" {
			req.Header.Set("X-Forwarded-Host", xfh)
		}
		return req
	}

	t.Run("direct https on non-localhost uses host-prefixed secure cookie", func(t *testing.T) {
		f(mkReq("app.example.com", true, "", ""), "__Host-session", true)
	})

	t.Run("direct http uses dev cookie", func(t *testing.T) {
		f(mkReq("app.example.com", false, "", ""), "sid", false)
	})

	t.Run("forwarded https on non-localhost uses host-prefixed secure cookie", func(t *testing.T) {
		f(mkReq("127.0.0.1:8079", false, "https", "app.example.com"), "__Host-session", true)
	})

	t.Run("forwarded https on localhost keeps dev cookie", func(t *testing.T) {
		f(mkReq("127.0.0.1:8079", false, "https", "localhost:5173"), "sid", false)
	})

	t.Run("uses last forwarded values when proxy sends comma-separated headers", func(t *testing.T) {
		f(
			mkReq("127.0.0.1:8079", false, "http, https", "proxy.local, app.example.com"),
			"__Host-session",
			true,
		)
	})

	t.Run("ignores forwarded headers from untrusted peer", func(t *testing.T) {
		f(mkReqFrom("203.0.113.10:8080", "app.example.com", false, "https", "app.example.com"), "sid", false)
	})
}

func TestRequestBaseURL(t *testing.T) {
	f := func(req *http.Request, want string) {
		t.Helper()
		got := RequestBaseURL(req)
		if got != want {
			t.Fatalf("RequestBaseURL() = %q; want %q", got, want)
		}
	}

	mkReq := func(host string, tlsOn bool, xfp, xfh string) *http.Request {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		req.Host = host
		req.RemoteAddr = "127.0.0.1:8080"
		if tlsOn {
			req.TLS = &tls.ConnectionState{}
		}
		if xfp != "" {
			req.Header.Set("X-Forwarded-Proto", xfp)
		}
		if xfh != "" {
			req.Header.Set("X-Forwarded-Host", xfh)
		}
		return req
	}

	t.Run("direct http", func(t *testing.T) {
		f(mkReq("app.example.com", false, "", ""), "http://app.example.com")
	})

	t.Run("direct https", func(t *testing.T) {
		f(mkReq("app.example.com", true, "", ""), "https://app.example.com")
	})

	t.Run("forwarded https and host", func(t *testing.T) {
		f(mkReq("127.0.0.1:8079", false, "https", "app.example.com"), "https://app.example.com")
	})

	t.Run("forwarded comma-separated values uses last entry", func(t *testing.T) {
		f(
			mkReq("127.0.0.1:8079", false, "http, https", "proxy.local, app.example.com"),
			"https://app.example.com",
		)
	})

	t.Run("ignores forwarded headers from untrusted peer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		req.Host = "internal.example.com"
		req.RemoteAddr = "203.0.113.10:8080"
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", "spoofed.example.com")
		f(req, "http://internal.example.com")
	})
}

func TestNewTokenAndHashFromB64(t *testing.T) {
	tok, hash, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if len(tok) == 0 {
		t.Fatalf("token empty")
	}
	if len(hash) != 32 {
		t.Fatalf("hash len = %d; want 32", len(hash))
	}

	got, err := HashFromB64(tok)
	if err != nil {
		t.Fatalf("HashFromB64: %v", err)
	}
	if string(got) != string(hash) {
		t.Fatalf("HashFromB64(token) != hash from NewToken")
	}

	if _, err := HashFromB64(""); err == nil {
		t.Fatalf("HashFromB64(empty): expected error")
	}
	if _, err := HashFromB64("not-base64!@#"); err == nil {
		t.Fatalf("HashFromB64(invalid): expected error")
	}
}

func TestSetTransientCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	exp := time.Now().Add(10 * time.Minute)
	SetTransientCookie(rec, "oauth_state", "abc", exp, true)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Name != "oauth_state" || c.Value != "abc" {
		t.Fatalf("name/value = %q/%q", c.Name, c.Value)
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("flags wrong: %+v", c)
	}
}

func TestClearTransientCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	ClearTransientCookie(rec, "oauth_state", false)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if c.Value != "" {
		t.Fatalf("expected empty value, got %q", c.Value)
	}
	if !c.Expires.Before(time.Now()) {
		t.Fatalf("expected expired cookie, got expires=%v", c.Expires)
	}
}

func TestIsLocalhost(t *testing.T) {
	cases := map[string]bool{
		"localhost":      true,
		"127.0.0.1":      true,
		"127.0.0.1:8080": true,
		"::1":            true,
		"example.com":    false,
		"10.0.0.1":       false,
	}
	for in, want := range cases {
		if got := IsLocalhost(in); got != want {
			t.Errorf("IsLocalhost(%q) = %v; want %v", in, got, want)
		}
	}
}

func TestParseAllowlistCSV(t *testing.T) {
	got := ParseAllowlistCSV(" A@X.COM, b@y.com ,,A@x.com ")
	if len(got) != 2 {
		t.Fatalf("expected 2 unique emails, got %d", len(got))
	}
	if _, ok := got["a@x.com"]; !ok {
		t.Fatalf("expected normalized allowlist to contain a@x.com")
	}
	if _, ok := got["b@y.com"]; !ok {
		t.Fatalf("expected normalized allowlist to contain b@y.com")
	}
}

func TestNewSessionPolicyDefaults(t *testing.T) {
	p, err := NewSessionPolicy(0, 0, 0)
	if err != nil {
		t.Fatalf("expected defaults to be valid, got err: %v", err)
	}
	if p.TTL != DefaultSessionTTL || p.RefreshThreshold != DefaultSessionRefreshThreshold || p.AbsoluteTTL != DefaultSessionAbsoluteTTL {
		t.Fatalf("unexpected defaults: %+v", p)
	}
}

func TestNewSessionPolicyValidation(t *testing.T) {
	if _, err := NewSessionPolicy(30*time.Minute, time.Minute, 24*time.Hour); err == nil {
		t.Fatalf("expected error for short session TTL")
	}
	if _, err := NewSessionPolicy(2*time.Hour, 3*time.Hour, 24*time.Hour); err == nil {
		t.Fatalf("expected error when refresh threshold exceeds TTL")
	}
	if _, err := NewSessionPolicy(8*time.Hour, 2*time.Hour, 6*time.Hour); err == nil {
		t.Fatalf("expected error when absolute TTL is below session TTL")
	}
}

func TestNextSessionExpiry(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	createdAt := now.Add(-2 * time.Hour)
	policy := SessionPolicy{
		TTL:              7 * 24 * time.Hour,
		RefreshThreshold: 24 * time.Hour,
		AbsoluteTTL:      30 * 24 * time.Hour,
	}

	t.Run("no refresh when far from expiry", func(t *testing.T) {
		current := now.Add(48 * time.Hour)
		_, ok := NextSessionExpiry(now, createdAt, current, policy)
		if ok {
			t.Fatalf("expected no refresh")
		}
	})

	t.Run("refresh when near expiry", func(t *testing.T) {
		current := now.Add(3 * time.Hour)
		next, ok := NextSessionExpiry(now, createdAt, current, policy)
		if !ok {
			t.Fatalf("expected refresh")
		}
		if !next.After(current) {
			t.Fatalf("expected next expiry to extend current expiry")
		}
	})

	t.Run("cap at absolute TTL", func(t *testing.T) {
		current := now.Add(2 * time.Hour)
		created := now.Add(-29 * 24 * time.Hour)
		next, ok := NextSessionExpiry(now, created, current, policy)
		if !ok {
			t.Fatalf("expected refresh")
		}
		want := created.Add(policy.AbsoluteTTL)
		if !next.Equal(want) {
			t.Fatalf("expected capped expiry %s, got %s", want, next)
		}
	})

	t.Run("no refresh when cap does not extend", func(t *testing.T) {
		created := now.Add(-29 * 24 * time.Hour)
		current := created.Add(policy.AbsoluteTTL)
		_, ok := NextSessionExpiry(now, created, current, policy)
		if ok {
			t.Fatalf("expected no refresh when capped expiry does not extend")
		}
	})
}
