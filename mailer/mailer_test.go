package mailer

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestBlackHoleSender(t *testing.T) {
	// ensure we are using blackhole
	UseBlackHole()

	// send should not error
	if err := Send(context.Background(), "a@b.c", "subj", "txt", "<p>html</p>"); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	// addr should report blackhole
	if got := Addr(); got != "blackhole" {
		t.Fatalf("Addr() = %q; want %q", got, "blackhole")
	}

	// last error should contain blackhole message
	if le := LastError(); le == "" {
		t.Fatalf("LastError() = empty; want non-empty")
	}
}

func TestBuildVerifyURL(t *testing.T) {
	f := func(base, email, token, path string, wantErr bool) {
		t.Helper()
		uStr, err := BuildVerifyURL(base, path, token)
		if wantErr {
			if err == nil {
				t.Fatalf("BuildVerifyURL(%q,...) expected error, got none; url=%q", base, uStr)
			}
			return
		}
		if err != nil {
			t.Fatalf("BuildVerifyURL(%q,...) unexpected error: %v", base, err)
		}
		u, err := url.Parse(uStr)
		if err != nil {
			t.Fatalf("parsing result url failed: %v", err)
		}
		// path must match expected
		if u.Path != path {
			t.Fatalf("path = %q; want %q", u.Path, path)
		}
		// query must contain email and token
		q := u.Query()
		if got := q.Get("token"); got != token {
			t.Fatalf("token query = %q; want %q", got, token)
		}
		// ensure host and scheme preserved when provided
		baseURL, _ := url.Parse(base)
		if baseURL.Scheme != "" && baseURL.Host != "" {
			if u.Scheme != baseURL.Scheme {
				t.Fatalf("scheme = %q; want %q", u.Scheme, baseURL.Scheme)
			}
			if u.Host != baseURL.Host {
				t.Fatalf("host = %q; want %q", u.Host, baseURL.Host)
			}
		}
	}

	t.Run("simple", func(t *testing.T) { f("https://example.com", "a@b.c", "tok", "/verify-email", false) })
	t.Run("with path replaced", func(t *testing.T) { f("https://example.com/some/path", "me@x.y", "tkn", "/verify-email", false) })
	t.Run("with port", func(t *testing.T) { f("http://localhost:8080/foo", "u@l", "123", "/verify-email", false) })
	t.Run("invalid base", func(t *testing.T) { f(":", "x", "y", "/verify-email", true) })
}

func TestBuildVerifyURL_QueryStability(t *testing.T) {
	// ensure only the email and token keys are present (no duplicates)
	uStr, err := BuildVerifyURL("https://example.org", "/verify-email", "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	u, err := url.Parse(uStr)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := u.Query()
	if got := q.Get("token"); got != "tok" {
		t.Fatalf("token = %q; want %q", got, "tok")
	}
	if len(q) != 1 {
		t.Fatalf("query has %d keys; want 1", len(q))
	}
}

func TestBuildConfig(t *testing.T) {
	f := func(name, from, smtpAddr, smtpUsername, smtpPassword, region, resendAPIKey, resendAPIBaseURL string, wantMode mode, wantErr string) {
		t.Helper()
		cfg, err := buildConfig(from, smtpAddr, smtpUsername, smtpPassword, region, resendAPIKey, resendAPIBaseURL)
		if wantErr != "" {
			if err == nil {
				t.Fatalf("%s: expected error %q, got nil", name, wantErr)
			}
			if !strings.Contains(err.Error(), wantErr) {
				t.Fatalf("%s: error = %q; want substring %q", name, err.Error(), wantErr)
			}
			return
		}
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}
		if cfg.mode != wantMode {
			t.Fatalf("%s: mode = %d; want %d", name, cfg.mode, wantMode)
		}
	}

	t.Run("blackhole", func(t *testing.T) {
		f("blackhole", "", "", "", "", "", "", "", modeBlackhole, "")
	})
	t.Run("smtp", func(t *testing.T) {
		f("smtp", "noreply@example.com", "127.0.0.1:1025", "", "", "", "", "", modeSMTP, "")
	})
	t.Run("smtp with auth", func(t *testing.T) {
		f("smtp with auth", "noreply@example.com", "smtp.resend.com:587", "resend", "re_test", "", "", "", modeSMTP, "")
	})
	t.Run("ses", func(t *testing.T) {
		f("ses", "noreply@example.com", "", "", "", "eu-west-1", "", "", modeSES, "")
	})
	t.Run("resend", func(t *testing.T) {
		f("resend", "noreply@example.com", "", "", "", "", "re_test", "", modeResend, "")
	})
	t.Run("missing from", func(t *testing.T) {
		f("missing from", "", "smtp.resend.com:587", "resend", "re_test", "", "", "", 0, "from address is required")
	})
	t.Run("multiple modes", func(t *testing.T) {
		f("multiple modes", "noreply@example.com", "127.0.0.1:1025", "", "", "eu-west-1", "", "", 0, "multiple mailer modes configured")
	})
	t.Run("multiple modes with resend", func(t *testing.T) {
		f("multiple modes with resend", "noreply@example.com", "127.0.0.1:1025", "", "", "", "re_test", "", 0, "multiple mailer modes configured")
	})
	t.Run("from without mode", func(t *testing.T) {
		f("from without mode", "noreply@example.com", "", "", "", "", "", "", 0, "mailer mode is missing")
	})
	t.Run("smtp username without password", func(t *testing.T) {
		f("smtp username without password", "noreply@example.com", "smtp.resend.com:587", "resend", "", "", "", "", 0, "smtp password is required")
	})
	t.Run("smtp password without username", func(t *testing.T) {
		f("smtp password without username", "noreply@example.com", "smtp.resend.com:587", "", "re_test", "", "", "", 0, "smtp username is required")
	})
	t.Run("smtp credentials without addr", func(t *testing.T) {
		f("smtp credentials without addr", "noreply@example.com", "", "resend", "re_test", "", "", "", 0, "smtp address is required")
	})
	t.Run("resend base url without key", func(t *testing.T) {
		f("resend base url without key", "noreply@example.com", "", "", "", "", "", "https://api.resend.com", 0, "resend api key is required")
	})
	t.Run("resend invalid base url", func(t *testing.T) {
		f("resend invalid base url", "noreply@example.com", "", "", "", "", "re_test", "://bad", 0, "resend api base url is invalid")
	})
}
