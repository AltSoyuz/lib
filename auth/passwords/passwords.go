// Package passwords provides bcrypt password hashing, email normalization,
// and breach-check via the HaveIBeenPwned k-anonymity API. Pure utility code,
// no DB or HTTP server dependency.
package passwords

import (
	"bufio"
	"context"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/idna"
)

const (
	// Cost is the bcrypt cost used by Hash.
	Cost = 12

	// FakeHash is a pre-computed bcrypt hash used for constant-time comparison
	// when a user is not found, to prevent timing-based user enumeration.
	FakeHash = "$2a$12$000000000000000000000uGLIFPqxVmExfOiAa9ebvJpSsQd/pxuu"
)

var (
	emailRe    = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	hibpClient = &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns: 100, IdleConnTimeout: 30 * time.Second,
			TLSHandshakeTimeout: 2 * time.Second, ExpectContinueTimeout: 500 * time.Millisecond,
		},
	}

	// HIBPEndpoint is overridable for tests.
	HIBPEndpoint = "https://api.pwnedpasswords.com/range/"
)

// Hash returns a bcrypt hash of pw at the package Cost.
func Hash(pw []byte) (string, error) {
	h, err := bcrypt.GenerateFromPassword(pw, Cost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// Verify reports whether pw matches the stored bcrypt hash.
func Verify(hash string, pw []byte) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), pw)
}

// VerifyFake runs a bcrypt compare against FakeHash. Used to preserve
// constant-time response when a user lookup fails, avoiding user enumeration.
func VerifyFake(pw []byte) {
	_ = bcrypt.CompareHashAndPassword([]byte(FakeHash), pw)
}

// NormalizeEmail lowercases, trims, and IDN-canonicalizes the domain.
// Returns the normalized form or an error if the address is malformed.
func NormalizeEmail(e string) (string, error) {
	e = strings.TrimSpace(strings.ToLower(e))
	if e == "" {
		return "", errors.New("invalid email")
	}
	parts := strings.Split(e, "@")
	if len(parts) != 2 {
		return "", errors.New("invalid email")
	}
	asciiDomain, err := idna.Lookup.ToASCII(parts[1])
	if err != nil {
		return "", errors.New("invalid email")
	}
	e = parts[0] + "@" + asciiDomain
	if len(e) > 254 || !emailRe.MatchString(e) {
		return "", errors.New("invalid email")
	}
	return e, nil
}

// CheckStrength rejects passwords that are too short, too long, or known to be
// compromised per the HaveIBeenPwned breach database. Returns nil on the HIBP
// network error path (failing open) — only a confirmed breach hit blocks.
func CheckStrength(ctx context.Context, pw []byte) error {
	if len(pw) < 12 {
		return errors.New("password too short")
	}
	if len(pw) > 72 {
		return errors.New("password too long")
	}
	ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	comp, err := hibpCompromised(ctx2, pw)
	if err != nil {
		return nil
	}
	if comp {
		return errors.New("password found in breaches")
	}
	return nil
}

func hibpCompromised(ctx context.Context, pw []byte) (bool, error) {
	sum := sha1.Sum(pw)
	full := strings.ToUpper(hex.EncodeToString(sum[:]))
	prefix := full[:5]

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, HIBPEndpoint+prefix, nil)
	req.Header.Set("User-Agent", "app-auth/1.0")
	req.Header.Set("Add-Padding", "true")
	res, err := hibpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return false, fmt.Errorf("hibp %d", res.StatusCode)
	}

	sc := bufio.NewScanner(res.Body)
	want := []byte(full[5:])
	for sc.Scan() {
		line := sc.Text()
		i := strings.IndexByte(line, ':')
		if i <= 0 {
			continue
		}
		suffix := line[:i]
		if len(suffix) == 35 && subtle.ConstantTimeCompare([]byte(suffix), want) == 1 {
			return true, nil
		}
	}
	return false, sc.Err()
}
