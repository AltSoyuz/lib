// Package sessions provides session token generation, cookie helpers, session
// policy, and request URL/host detection. Pure utility code, no DB.
package sessions

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/AltSoyuz/lib/httpserver"
)

const (
	// TTL is the default session lifetime.
	TTL = 30 * 24 * time.Hour

	cookieDev  = "sid"
	cookieProd = "__Host-session"
)

// NewToken returns a base64url-encoded random token (32 bytes of entropy)
// and the SHA-256 hash of the raw bytes. Store the hash; expose the token.
func NewToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256(b)
	return token, sum[:], nil
}

// HashFromB64 decodes the base64url token and returns the SHA-256 of the raw
// bytes. Same convention as NewToken — used to look up a stored hash.
func HashFromB64(tok string) ([]byte, error) {
	if tok == "" {
		return nil, errors.New("empty token")
	}
	raw, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	return sum[:], nil
}

// HashFromRequest reads the session cookie from r and returns its hash.
func HashFromRequest(r *http.Request) ([]byte, error) {
	val, err := ReadCookie(r)
	if err != nil {
		return nil, err
	}
	return HashFromB64(val)
}

// SetCookie writes a session cookie. The cookie name and Secure flag depend on
// whether the request is HTTPS and not localhost (__Host- prefix in prod).
func SetCookie(w http.ResponseWriter, r *http.Request, token string, exp time.Time) {
	secure := RequestIsHTTPS(r) && !IsLocalhost(RequestHost(r))
	http.SetCookie(w, &http.Cookie{
		Name:     pickCookieName(r),
		Value:    token,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ReadCookie returns the raw session cookie value, falling back across
// dev/prod cookie names.
func ReadCookie(r *http.Request) (string, error) {
	if c, err := r.Cookie(pickCookieName(r)); err == nil && c.Value != "" {
		return c.Value, nil
	}
	if c, err := r.Cookie(cookieDev); err == nil && c.Value != "" {
		return c.Value, nil
	}
	if c, err := r.Cookie(cookieProd); err == nil && c.Value != "" {
		return c.Value, nil
	}
	return "", http.ErrNoCookie
}

// ClearCookie deletes the session cookie under both dev and prod names.
func ClearCookie(w http.ResponseWriter, r *http.Request) {
	secure := RequestIsHTTPS(r) && !IsLocalhost(RequestHost(r))
	clear := func(name string) {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
	}
	clear(cookieDev)
	clear(cookieProd)
}

// SetTransientCookie writes a short-lived HttpOnly cookie under an arbitrary
// name. Used for OAuth state and PKCE verifier cookies (and similar one-shot
// flows). The caller decides Secure based on its own HTTPS detection.
func SetTransientCookie(w http.ResponseWriter, name, value string, exp time.Time, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearTransientCookie deletes a cookie previously set via SetTransientCookie.
func ClearTransientCookie(w http.ResponseWriter, name string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func pickCookieName(r *http.Request) string {
	if RequestIsHTTPS(r) && !IsLocalhost(RequestHost(r)) {
		return cookieProd
	}
	return cookieDev
}

// RequestHost returns the effective host: X-Forwarded-Host (last entry) when
// the peer is trusted, else r.Host.
func RequestHost(r *http.Request) string {
	if httpserver.IsTrustedPeer(r) {
		xfh := r.Header.Get("X-Forwarded-Host")
		if xfh != "" {
			if i := strings.LastIndexByte(xfh, ','); i >= 0 {
				xfh = xfh[i+1:]
			}
			xfh = strings.TrimSpace(xfh)
			if xfh != "" {
				return xfh
			}
		}
	}
	return r.Host
}

// RequestIsHTTPS reports whether the request is HTTPS, honouring
// X-Forwarded-Proto (last entry) when the peer is trusted.
func RequestIsHTTPS(r *http.Request) bool {
	if httpserver.IsTrustedPeer(r) {
		xfp := r.Header.Get("X-Forwarded-Proto")
		if xfp != "" {
			if i := strings.LastIndexByte(xfp, ','); i >= 0 {
				xfp = xfp[i+1:]
			}
			return strings.EqualFold(strings.TrimSpace(xfp), "https")
		}
	}
	return r.TLS != nil
}

// RequestBaseURL derives the public base URL from an inbound request.
func RequestBaseURL(r *http.Request) string {
	scheme := "http"
	if RequestIsHTTPS(r) {
		scheme = "https"
	}
	return scheme + "://" + RequestHost(r)
}

// IsLocalhost reports whether host (which may include :port) is loopback.
func IsLocalhost(h string) bool {
	host := h
	if hh, _, err := net.SplitHostPort(h); err == nil {
		host = hh
	}
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

const (
	DefaultSessionTTL              = 7 * 24 * time.Hour
	DefaultSessionRefreshThreshold = 24 * time.Hour
	DefaultSessionAbsoluteTTL      = 30 * 24 * time.Hour
	minSessionTTL                  = 1 * time.Hour
	minSessionRefreshThreshold     = 5 * time.Minute
)

type SessionPolicy struct {
	TTL              time.Duration
	RefreshThreshold time.Duration
	AbsoluteTTL      time.Duration
}

func NewSessionPolicy(sessionTTL, refreshThreshold, absoluteTTL time.Duration) (SessionPolicy, error) {
	if sessionTTL <= 0 {
		sessionTTL = DefaultSessionTTL
	}
	if refreshThreshold <= 0 {
		refreshThreshold = DefaultSessionRefreshThreshold
	}
	if absoluteTTL <= 0 {
		absoluteTTL = DefaultSessionAbsoluteTTL
	}
	if sessionTTL < minSessionTTL {
		return SessionPolicy{}, fmt.Errorf("sessionTTL_too_short: got=%s min=%s", sessionTTL, minSessionTTL)
	}
	if refreshThreshold < minSessionRefreshThreshold {
		return SessionPolicy{}, fmt.Errorf("sessionRefreshThreshold_too_short: got=%s min=%s", refreshThreshold, minSessionRefreshThreshold)
	}
	if refreshThreshold > sessionTTL {
		return SessionPolicy{}, fmt.Errorf("sessionRefreshThreshold_exceeds_sessionTTL: refresh=%s ttl=%s", refreshThreshold, sessionTTL)
	}
	if absoluteTTL < sessionTTL {
		return SessionPolicy{}, fmt.Errorf("sessionAbsoluteTTL_below_sessionTTL: absolute=%s ttl=%s", absoluteTTL, sessionTTL)
	}
	return SessionPolicy{
		TTL:              sessionTTL,
		RefreshThreshold: refreshThreshold,
		AbsoluteTTL:      absoluteTTL,
	}, nil
}

func NextSessionExpiry(now, createdAt, currentExpiry time.Time, policy SessionPolicy) (time.Time, bool) {
	if currentExpiry.Sub(now) > policy.RefreshThreshold {
		return time.Time{}, false
	}
	nextExpiry := now.Add(policy.TTL)
	absoluteExpiry := createdAt.Add(policy.AbsoluteTTL)
	if nextExpiry.After(absoluteExpiry) {
		nextExpiry = absoluteExpiry
	}
	if !nextExpiry.After(currentExpiry) {
		return time.Time{}, false
	}
	return nextExpiry, true
}

// ParseAllowlistCSV parses a comma-separated list of emails into a lowercase set.
func ParseAllowlistCSV(csv string) map[string]struct{} {
	if csv == "" {
		return nil
	}
	out := make(map[string]struct{})
	for _, p := range strings.Split(csv, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out[p] = struct{}{}
		}
	}
	return out
}
