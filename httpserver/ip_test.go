package httpserver

import (
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	if err := SetTrustedProxyCIDRs(""); err != nil {
		t.Fatalf("SetTrustedProxyCIDRs default: %v", err)
	}
	t.Cleanup(func() {
		_ = SetTrustedProxyCIDRs("")
	})

	f := func(remoteAddr, xff, want string) {
		t.Helper()

		req := httptest.NewRequest("GET", "http://example.com", nil)
		req.RemoteAddr = remoteAddr
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}

		got := ClientIP(req)
		if got != want {
			t.Fatalf("ClientIP() = %q; want %q (remote=%q, xff=%q)", got, want, remoteAddr, xff)
		}
	}

	t.Run("ignores xff when peer is not loopback", func(t *testing.T) {
		f("203.0.113.10:443", "198.51.100.9", "203.0.113.10")
	})

	t.Run("uses rightmost non-trusted xff when peer is loopback", func(t *testing.T) {
		f("127.0.0.1:8080", "198.51.100.1, 198.51.100.2", "198.51.100.2")
	})

	t.Run("skips invalid xff entries", func(t *testing.T) {
		f("127.0.0.1:8080", "invalid, 198.51.100.20", "198.51.100.20")
	})

	t.Run("falls back to remote when loopback has no xff", func(t *testing.T) {
		f("127.0.0.1:8080", "", "127.0.0.1")
	})

	t.Run("returns empty string on malformed remote addr", func(t *testing.T) {
		f("malformed-remote", "198.51.100.7", "")
	})

	t.Run("uses xff when remote is in configured trusted proxy range", func(t *testing.T) {
		if err := SetTrustedProxyCIDRs("203.0.113.0/24"); err != nil {
			t.Fatalf("SetTrustedProxyCIDRs: %v", err)
		}
		f("203.0.113.10:8080", "198.51.100.55", "198.51.100.55")
	})

	t.Run("strips trusted intermediate proxy from xff in multi-proxy chain", func(t *testing.T) {
		if err := SetTrustedProxyCIDRs("10.0.0.0/8"); err != nil {
			t.Fatalf("SetTrustedProxyCIDRs: %v", err)
		}
		// XFF: client=1.2.3.4, trusted-intermediate=10.0.0.5, direct-peer=10.0.0.1
		// direct peer (10.0.0.1) is trusted, 10.0.0.5 is also trusted → strip both,
		// rightmost non-trusted is 1.2.3.4.
		f("10.0.0.1:8080", "1.2.3.4, 10.0.0.5", "1.2.3.4")
	})

	t.Run("ignores xff when trusted proxies disabled", func(t *testing.T) {
		if err := SetTrustedProxyCIDRs("none"); err != nil {
			t.Fatalf("SetTrustedProxyCIDRs: %v", err)
		}
		f("127.0.0.1:8080", "198.51.100.99", "127.0.0.1")
	})
}

func TestSetTrustedProxyCIDRs(t *testing.T) {
	t.Cleanup(func() {
		_ = SetTrustedProxyCIDRs("")
	})

	t.Run("accepts single IP values", func(t *testing.T) {
		if err := SetTrustedProxyCIDRs("10.0.0.1, ::1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("accepts CIDR values", func(t *testing.T) {
		if err := SetTrustedProxyCIDRs("10.0.0.0/8, 2001:db8::/32"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects invalid values", func(t *testing.T) {
		if err := SetTrustedProxyCIDRs("not-a-cidr"); err == nil {
			t.Fatalf("expected error for invalid CIDR/IP spec")
		}
	})
}
