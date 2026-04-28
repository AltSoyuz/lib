package passwords

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHashVerifyRoundtrip(t *testing.T) {
	pw := []byte("correct horse battery staple")
	h, err := Hash(pw)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if err := Verify(h, pw); err != nil {
		t.Fatalf("Verify correct pw: %v", err)
	}
	if err := Verify(h, []byte("wrong")); err == nil {
		t.Fatalf("Verify wrong pw: expected error")
	}
}

func TestVerifyFakeDoesNotPanic(t *testing.T) {
	VerifyFake([]byte("anything"))
}

func TestNormalizeEmail(t *testing.T) {
	cases := []struct {
		in, want string
		wantErr  bool
	}{
		{"alice@example.com", "alice@example.com", false},
		{"  ALICE@Example.COM  ", "alice@example.com", false},
		{"no-at-sign", "", true},
		{"alice@", "", true},
		{"@example.com", "", true},
		{"a@b.c", "a@b.c", false},
		{"unicode@münchen.de", "unicode@xn--mnchen-3ya.de", false},
		{strings.Repeat("a", 260) + "@b.c", "", true},
	}
	for _, c := range cases {
		got, err := NormalizeEmail(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("NormalizeEmail(%q) expected error, got %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeEmail(%q): unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalizeEmail(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestCheckStrengthLength(t *testing.T) {
	ctx := context.Background()
	if err := CheckStrength(ctx, []byte("short")); err == nil {
		t.Fatalf("expected too-short error")
	}
	if err := CheckStrength(ctx, []byte(strings.Repeat("a", 100))); err == nil {
		t.Fatalf("expected too-long error")
	}
}

func TestCheckStrengthHIBPHit(t *testing.T) {
	pw := []byte("password123!")
	sum := sha1.Sum(pw)
	full := strings.ToUpper(hex.EncodeToString(sum[:]))
	suffix := full[5:]

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s:42\r\n", suffix)
	}))
	defer srv.Close()

	old := HIBPEndpoint
	HIBPEndpoint = srv.URL + "/"
	defer func() { HIBPEndpoint = old }()

	if err := CheckStrength(context.Background(), pw); err == nil {
		t.Fatalf("expected breach error")
	}
}

func TestCheckStrengthHIBPMissPasses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:1\r\n"))
	}))
	defer srv.Close()

	old := HIBPEndpoint
	HIBPEndpoint = srv.URL + "/"
	defer func() { HIBPEndpoint = old }()

	if err := CheckStrength(context.Background(), []byte("correct horse battery staple")); err != nil {
		t.Fatalf("expected no error on miss, got %v", err)
	}
}
