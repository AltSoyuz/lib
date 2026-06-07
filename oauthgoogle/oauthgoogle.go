package oauthgoogle

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ProviderName is the stable provider string stored in oauth_accounts.provider.
const ProviderName = "google"

const (
	defaultAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultTokenURL = "https://oauth2.googleapis.com/token"
)

// defaultScopes covers Google Sign-In: OpenID Connect identity plus email.
var defaultScopes = []string{"openid", "email"}

// UserInfo describes the authenticated Google account.
type UserInfo struct {
	Provider      string
	Subject       string
	Email         string
	VerifiedEmail bool
}

// Config configures the Google OAuth provider.
type Config struct {
	// ClientID is the OAuth client ID from the Google Cloud console.
	ClientID string
	// ClientSecret is the OAuth client secret from the Google Cloud console.
	ClientSecret string
	// RedirectURL is the callback URL registered for the OAuth client.
	RedirectURL string
	// Scopes overrides the requested scopes. Defaults to "openid email".
	Scopes []string
	// HTTPClient overrides the client used for the token exchange.
	// Defaults to an http.Client with a 10s timeout.
	HTTPClient *http.Client
	// AuthURL and TokenURL override the Google endpoints, for tests.
	AuthURL  string
	TokenURL string
}

// Provider starts and completes Google OAuth flows.
type Provider struct {
	cfg Config
}

var current = &Provider{}

// ErrNotAvailable is returned while the provider is not configured.
var ErrNotAvailable = errors.New("oauth_google_not_available")

// New returns a Provider for the given config. ClientID, ClientSecret and
// RedirectURL are required.
func New(cfg Config) (*Provider, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURL == "" {
		return nil, errors.New("oauth_google_missing_config: ClientID, ClientSecret and RedirectURL are required")
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = defaultScopes
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.AuthURL == "" {
		cfg.AuthURL = defaultAuthURL
	}
	if cfg.TokenURL == "" {
		cfg.TokenURL = defaultTokenURL
	}
	return &Provider{cfg: cfg}, nil
}

// Configure sets the process-wide Google OAuth provider returned by Current.
func Configure(cfg Config) error {
	p, err := New(cfg)
	if err != nil {
		return err
	}
	current = p
	return nil
}

// Current returns the process-wide Google OAuth provider.
func Current() *Provider { return current }

// Available reports whether the Google OAuth provider is configured.
func (p *Provider) Available() bool {
	return p.cfg.ClientID != "" && p.cfg.ClientSecret != "" && p.cfg.RedirectURL != ""
}

// NewState returns a random base64url-encoded state value (32 bytes of
// entropy). Store it in a transient cookie and compare on callback.
func NewState() (string, error) { return randomB64(32) }

// NewVerifier returns a random PKCE code verifier (RFC 7636, 43 chars).
// Store it in a transient cookie and pass it to Start and Callback.
func NewVerifier() (string, error) { return randomB64(32) }

func randomB64(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Start writes the OAuth redirect response sending the user to Google's
// consent screen. state guards against CSRF; verifier is the PKCE code
// verifier whose S256 challenge is sent along.
func (p *Provider) Start(w http.ResponseWriter, state, verifier string) error {
	if !p.Available() {
		return ErrNotAvailable
	}
	if state == "" || verifier == "" {
		return errors.New("oauth_google_missing_state_or_verifier")
	}
	q := url.Values{
		"client_id":             {p.cfg.ClientID},
		"redirect_uri":          {p.cfg.RedirectURL},
		"response_type":         {"code"},
		"scope":                 {strings.Join(p.cfg.Scopes, " ")},
		"state":                 {state},
		"code_challenge":        {s256Challenge(verifier)},
		"code_challenge_method": {"S256"},
	}
	w.Header().Set("Location", p.cfg.AuthURL+"?"+q.Encode())
	w.WriteHeader(http.StatusFound)
	return nil
}

func s256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// Callback exchanges an OAuth authorization code for user information.
// verifier must be the same PKCE code verifier passed to Start.
func (p *Provider) Callback(ctx context.Context, code, verifier string) (*UserInfo, error) {
	if !p.Available() {
		return nil, ErrNotAvailable
	}
	if code == "" || verifier == "" {
		return nil, errors.New("oauth_google_missing_code_or_verifier")
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {p.cfg.ClientID},
		"client_secret": {p.cfg.ClientSecret},
		"redirect_uri":  {p.cfg.RedirectURL},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oauth_google_token_request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("oauth_google_token_response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth_google_token_exchange: status=%d body=%s", resp.StatusCode, truncate(body, 256))
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("oauth_google_token_decode: %w", err)
	}
	if tok.IDToken == "" {
		return nil, errors.New("oauth_google_missing_id_token")
	}
	claims, err := parseIDToken(tok.IDToken)
	if err != nil {
		return nil, err
	}
	if err := claims.validate(p.cfg.ClientID, time.Now()); err != nil {
		return nil, err
	}
	return &UserInfo{
		Provider:      ProviderName,
		Subject:       claims.Subject,
		Email:         claims.Email,
		VerifiedEmail: claims.EmailVerified,
	}, nil
}

type idClaims struct {
	Issuer        string `json:"iss"`
	Audience      string `json:"aud"`
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Expiry        int64  `json:"exp"`
}

// parseIDToken decodes the ID token claims without verifying the signature.
// The token comes straight from Google's token endpoint over TLS, so per
// OpenID Connect Core §3.1.3.7 the signature check may be skipped; the
// issuer, audience and expiry claims are still validated.
func parseIDToken(raw string) (*idClaims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("oauth_google_malformed_id_token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("oauth_google_id_token_decode: %w", err)
	}
	var claims idClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("oauth_google_id_token_claims: %w", err)
	}
	return &claims, nil
}

func (c *idClaims) validate(clientID string, now time.Time) error {
	if c.Issuer != "https://accounts.google.com" && c.Issuer != "accounts.google.com" {
		return fmt.Errorf("oauth_google_bad_issuer: %s", c.Issuer)
	}
	if c.Audience != clientID {
		return errors.New("oauth_google_bad_audience")
	}
	if c.Expiry <= now.Unix() {
		return errors.New("oauth_google_id_token_expired")
	}
	if c.Subject == "" {
		return errors.New("oauth_google_missing_subject")
	}
	return nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
