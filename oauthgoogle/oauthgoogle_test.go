package oauthgoogle

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func testConfig() Config {
	return Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://example.com/auth/google/callback",
	}
}

// makeIDToken builds an unsigned JWT-shaped ID token from claims.
func makeIDToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func validClaims() map[string]any {
	return map[string]any{
		"iss":            "https://accounts.google.com",
		"aud":            "client-id",
		"sub":            "12345",
		"email":          "user@example.com",
		"email_verified": true,
		"exp":            time.Now().Add(time.Hour).Unix(),
	}
}

func TestNewMissingConfig(t *testing.T) {
	for _, cfg := range []Config{
		{},
		{ClientID: "id", ClientSecret: "secret"},
		{ClientID: "id", RedirectURL: "https://example.com/cb"},
		{ClientSecret: "secret", RedirectURL: "https://example.com/cb"},
	} {
		if _, err := New(cfg); err == nil {
			t.Errorf("New(%+v): want error, got nil", cfg)
		}
	}
}

func TestAvailable(t *testing.T) {
	var zero Provider
	if zero.Available() {
		t.Error("zero Provider: Available() = true, want false")
	}
	p, err := New(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !p.Available() {
		t.Error("configured Provider: Available() = false, want true")
	}
}

func TestStartRedirect(t *testing.T) {
	p, err := New(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	if err := p.Start(w, "state-123", "verifier-123"); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if got := loc.Scheme + "://" + loc.Host + loc.Path; got != defaultAuthURL {
		t.Errorf("auth URL = %s, want %s", got, defaultAuthURL)
	}
	q := loc.Query()
	sum := sha256.Sum256([]byte("verifier-123"))
	wantChallenge := base64.RawURLEncoding.EncodeToString(sum[:])
	for k, want := range map[string]string{
		"client_id":             "client-id",
		"redirect_uri":          "https://example.com/auth/google/callback",
		"response_type":         "code",
		"scope":                 "openid email",
		"state":                 "state-123",
		"code_challenge":        wantChallenge,
		"code_challenge_method": "S256",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("query %s = %q, want %q", k, got, want)
		}
	}
}

func TestStartNotAvailable(t *testing.T) {
	var p Provider
	w := httptest.NewRecorder()
	if err := p.Start(w, "state", "verifier"); !errors.Is(err, ErrNotAvailable) {
		t.Errorf("err = %v, want ErrNotAvailable", err)
	}
}

func TestStartMissingStateOrVerifier(t *testing.T) {
	p, err := New(testConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Start(httptest.NewRecorder(), "", "verifier"); err == nil {
		t.Error("empty state: want error, got nil")
	}
	if err := p.Start(httptest.NewRecorder(), "state", ""); err == nil {
		t.Error("empty verifier: want error, got nil")
	}
}

func TestCallback(t *testing.T) {
	idToken := makeIDToken(t, validClaims())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		for k, want := range map[string]string{
			"grant_type":    "authorization_code",
			"code":          "auth-code",
			"client_id":     "client-id",
			"client_secret": "client-secret",
			"redirect_uri":  "https://example.com/auth/google/callback",
			"code_verifier": "verifier-123",
		} {
			if got := r.PostForm.Get(k); got != want {
				t.Errorf("form %s = %q, want %q", k, got, want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id_token": idToken})
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.TokenURL = srv.URL
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	info, err := p.Callback(context.Background(), "auth-code", "verifier-123")
	if err != nil {
		t.Fatal(err)
	}
	want := UserInfo{Provider: "google", Subject: "12345", Email: "user@example.com", VerifiedEmail: true}
	if *info != want {
		t.Errorf("UserInfo = %+v, want %+v", *info, want)
	}
}

func TestCallbackTokenEndpointError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.TokenURL = srv.URL
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Callback(context.Background(), "bad-code", "verifier"); err == nil {
		t.Error("want error, got nil")
	}
}

func TestCallbackBadClaims(t *testing.T) {
	cases := map[string]func(map[string]any){
		"bad issuer":   func(c map[string]any) { c["iss"] = "https://evil.example.com" },
		"bad audience": func(c map[string]any) { c["aud"] = "other-client" },
		"expired":      func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() },
		"no subject":   func(c map[string]any) { delete(c, "sub") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			claims := validClaims()
			mutate(claims)
			idToken := makeIDToken(t, claims)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]string{"id_token": idToken})
			}))
			defer srv.Close()

			cfg := testConfig()
			cfg.TokenURL = srv.URL
			p, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.Callback(context.Background(), "code", "verifier"); err == nil {
				t.Error("want error, got nil")
			}
		})
	}
}

func TestCallbackNotAvailable(t *testing.T) {
	var p Provider
	if _, err := p.Callback(context.Background(), "code", "verifier"); !errors.Is(err, ErrNotAvailable) {
		t.Errorf("err = %v, want ErrNotAvailable", err)
	}
}

func TestConfigureCurrent(t *testing.T) {
	t.Cleanup(func() { current = &Provider{} })
	if err := Configure(testConfig()); err != nil {
		t.Fatal(err)
	}
	if !Current().Available() {
		t.Error("Current().Available() = false after Configure, want true")
	}
	if err := Configure(Config{}); err == nil {
		t.Error("Configure with empty config: want error, got nil")
	}
}

func TestNewStateAndVerifier(t *testing.T) {
	for name, gen := range map[string]func() (string, error){"state": NewState, "verifier": NewVerifier} {
		a, err := gen()
		if err != nil {
			t.Fatal(err)
		}
		b, err := gen()
		if err != nil {
			t.Fatal(err)
		}
		if a == b {
			t.Errorf("%s: two values are equal", name)
		}
		// RFC 7636 requires the verifier to be 43-128 chars of unreserved
		// characters; base64url of 32 bytes is exactly 43.
		if len(a) != 43 {
			t.Errorf("%s: len = %d, want 43", name, len(a))
		}
		if strings.ContainsAny(a, "+/=") {
			t.Errorf("%s: contains non-base64url characters: %q", name, a)
		}
	}
}
