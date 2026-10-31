package backoffice

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/AltSoyuz/lib/auth/passwords"
	"github.com/AltSoyuz/lib/auth/sessions"
	"github.com/AltSoyuz/lib/httpserver"
	"github.com/AltSoyuz/lib/logger"
	"github.com/AltSoyuz/lib/mailer"
)

// The sign-in by a 6-digit code sent by email. The codes live in lib's magic_links table (auth/schema 002 and the
// attempts column of 004) as an HMAC of the email and the code, under a key made at start and never stored: a copy of
// the database cannot give away an open code, and a restart voids them.
const (
	purposeLoginCode = "login_code"
	// CodeTTL is how long a sign-in code stays valid.
	CodeTTL         = 15 * time.Minute
	maxCodeAttempts = 5
	codesPerEmail   = 5
	codesWindow     = 15 * time.Minute
)

var (
	errInvalidCode     = errors.New("invalid_or_expired_code")
	errTooManyAttempts = errors.New("too_many_attempts")
	errTooManyCodes    = errors.New("too_many_codes")
)

type codeState struct {
	key      []byte
	perEmail *emailLimiter
	requests *httpserver.IPRateLimiter
	verifies *httpserver.IPRateLimiter
	send     func(email, code string)
}

func newCodeState() (*codeState, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("backoffice: cannot make the sign-in code key: %w", err)
	}
	return &codeState{
		key:      key,
		perEmail: newEmailLimiter(codesPerEmail, codesWindow),
		requests: httpserver.NewIPRateLimiter(5, time.Minute),
		verifies: httpserver.NewIPRateLimiter(20, time.Minute),
	}, nil
}

// RequestCode is the handler of POST /api/v1/sign-in/code, body {"email"}: it emails a new sign-in code, which voids
// the previous one, and answers 204. A restrained account gets no code and the same 204. Five codes per address in
// 15 minutes, then 429 too_many_codes; when the mailer is a blackhole the code is logged as "login code (dev)".
func (b *Backoffice) RequestCode(w http.ResponseWriter, r *http.Request) {
	if !b.codes.requests.Check(w, r) {
		return
	}
	in, err := httpserver.DecodeJSON[struct {
		Email string `json:"email"`
	}](r)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, err)
		return
	}
	email, err := passwords.NormalizeEmail(in.Email)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, fmt.Errorf("invalid_email: %w", err))
		return
	}
	ctx := r.Context()
	now := time.Now().UTC()
	// Counted before the restrained check, so hitting the limit says nothing about the account.
	if !b.codes.perEmail.allow(email, now) {
		logger.Info("sign_in_code_denied_too_many", "email", email)
		httpserver.WriteError(w, r, http.StatusTooManyRequests, errTooManyCodes)
		return
	}
	var state, pending string
	err = b.cfg.DB.QueryRowContext(ctx, `SELECT access_state, COALESCE(pending_since, '') FROM users WHERE email = ?`, email).Scan(&state, &pending)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if state == AccessRestrained && pending == "" {
		logger.Info("sign_in_code_denied_restrained", "email", email)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	code, err := b.createCode(ctx, email, now)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	go b.codes.send(email, code)
	w.WriteHeader(http.StatusNoContent)
}

// VerifyCode is the handler of POST /api/v1/sign-in/verify, body {"email","code"}: with the open code of email it
// signs in (SignIn, via "code") and answers 204. A wrong code answers 400 invalid_or_expired_code; the 5th wrong one
// closes the code and answers too_many_attempts; a restrained account gets 403 user_restrained.
func (b *Backoffice) VerifyCode(w http.ResponseWriter, r *http.Request) {
	if !b.codes.verifies.Check(w, r) {
		return
	}
	in, err := httpserver.DecodeJSON[struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}](r)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, err)
		return
	}
	email, err := passwords.NormalizeEmail(in.Email)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, fmt.Errorf("invalid_email: %w", err))
		return
	}
	if !isCodeShaped(in.Code) {
		httpserver.WriteError(w, r, http.StatusBadRequest, errInvalidCode)
		return
	}
	now := time.Now().UTC()
	id, err := b.checkCode(r.Context(), email, in.Code, now)
	if err != nil {
		if errors.Is(err, errInvalidCode) || errors.Is(err, errTooManyAttempts) {
			httpserver.WriteError(w, r, http.StatusBadRequest, err)
			return
		}
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	_, err = b.signIn(w, r, email, "code", now, func(tx *sql.Tx, _ User, _ string) error {
		closed, err := closeCode(r.Context(), tx, id, now)
		if err != nil {
			return err
		}
		if !closed {
			return errInvalidCode
		}
		_, err = tx.ExecContext(r.Context(), `UPDATE users SET email_verified_at = ? WHERE email = ? AND email_verified_at IS NULL`,
			now.Format(time.RFC3339), email)
		return err
	})
	switch {
	case errors.Is(err, ErrRestrained):
		httpserver.WriteError(w, r, http.StatusForbidden, ErrRestrained)
	case errors.Is(err, errInvalidCode):
		httpserver.WriteError(w, r, http.StatusBadRequest, errInvalidCode)
	case err != nil:
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
	default:
		b.codes.perEmail.reset(email)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	}
}

// SignIn signs the request in as email, an address the app has proven (by a password, a provider): in one
// transaction it admits the account (Admit), runs Config.OnSignIn then also (when not nil) with the account and why
// it was created ("" for an existing one), opens a session and writes the audit rows (AuditSignIn, with via), then
// sets the session cookie. A restrained account gets ErrRestrained and no cookie; an error of also or OnSignIn
// rolls the sign-in back and is returned as is.
func (b *Backoffice) SignIn(w http.ResponseWriter, r *http.Request, email, via string, also func(tx *sql.Tx, u User, joined string) error) (User, error) {
	return b.signIn(w, r, email, via, time.Now().UTC(), also)
}

func (b *Backoffice) signIn(w http.ResponseWriter, r *http.Request, email, via string, now time.Time, also func(tx *sql.Tx, u User, joined string) error) (User, error) {
	ctx := r.Context()
	token, hash, err := sessions.NewToken()
	if err != nil {
		return User{}, err
	}
	nowStr := now.Format(time.RFC3339)
	expires := now.Add(b.cfg.Sessions.TTL)
	ip := httpserver.ClientIP(r)
	var u User
	var joined string
	err = b.inTx(ctx, func(tx *sql.Tx) error {
		var err error
		if u, joined, err = b.Admit(ctx, tx, email, now); err != nil {
			return err
		}
		if b.cfg.OnSignIn != nil {
			if err := b.cfg.OnSignIn(ctx, tx, u, joined, via); err != nil {
				return err
			}
		}
		if also != nil {
			if err := also(tx, u, joined); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO sessions (id, user_id, sid_hash, user_agent, ip, created_at, updated_at, last_seen_at, expires_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, newID(), u.ID, hash, r.UserAgent(), ip, nowStr, nowStr, nowStr, expires.Format(time.RFC3339)); err != nil {
			return fmt.Errorf("cannot create the session of %s: %w", email, err)
		}
		return b.AuditSignIn(ctx, tx, u, joined, via, ip)
	})
	if err != nil {
		return User{}, err
	}
	if joined != "" {
		logger.Info("user_signed_up", "user_id", u.ID, "email", u.Email, "access_state", u.AccessState, "reason", joined)
	}
	sessions.SetCookie(w, r, token, expires)
	return u, nil
}

// createCode closes the open code of email and stores a new one, as a hash only.
func (b *Backoffice) createCode(ctx context.Context, email string, now time.Time) (string, error) {
	code, err := newCode()
	if err != nil {
		return "", err
	}
	nowStr := now.Format(time.RFC3339)
	err = b.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE magic_links SET consumed_at = ? WHERE email = ? AND purpose = ? AND consumed_at IS NULL`,
			nowStr, email, purposeLoginCode); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO magic_links (id, email, token_hash, purpose, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
			newID(), email, b.codeHash(email, code), purposeLoginCode, nowStr, now.Add(CodeTTL).Format(time.RFC3339))
		return err
	})
	return code, err
}

// newCode is uniform over 000000 to 999999.
func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// checkCode returns the id of the open code of email when code matches it; a wrong code counts a try, and the 5th
// closes the code.
func (b *Backoffice) checkCode(ctx context.Context, email, code string, now time.Time) (string, error) {
	var id string
	var hash []byte
	err := b.cfg.DB.QueryRowContext(ctx, `SELECT id, token_hash FROM magic_links
		WHERE email = ? AND purpose = ? AND consumed_at IS NULL AND expires_at > ? ORDER BY created_at DESC LIMIT 1`,
		email, purposeLoginCode, now.Format(time.RFC3339)).Scan(&id, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errInvalidCode
	}
	if err != nil {
		return "", err
	}
	if subtle.ConstantTimeCompare(hash, b.codeHash(email, code)) == 1 {
		return id, nil
	}
	var attempts int64
	err = b.inTx(ctx, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx, `UPDATE magic_links SET attempts = attempts + 1 WHERE id = ? AND consumed_at IS NULL RETURNING attempts`, id).
			Scan(&attempts); err != nil {
			return err
		}
		if attempts < maxCodeAttempts {
			return nil
		}
		_, err := closeCode(ctx, tx, id, now)
		return err
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", errInvalidCode
	}
	if err != nil {
		return "", err
	}
	logger.Info("sign_in_wrong_code", "email", email, "attempts", attempts)
	if attempts >= maxCodeAttempts {
		return "", errTooManyAttempts
	}
	return "", errInvalidCode
}

func closeCode(ctx context.Context, tx *sql.Tx, id string, now time.Time) (bool, error) {
	res, err := tx.ExecContext(ctx, `UPDATE magic_links SET consumed_at = ? WHERE id = ? AND consumed_at IS NULL`, now.Format(time.RFC3339), id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// codeHash binds the code to the email, so a code never matches the row of another address.
func (b *Backoffice) codeHash(email, code string) []byte {
	m := hmac.New(sha256.New, b.codes.key)
	m.Write([]byte(email))
	m.Write([]byte{0})
	m.Write([]byte(code))
	return m.Sum(nil)
}

func isCodeShaped(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (b *Backoffice) mailCode(email, code string) {
	if mailer.IsBlackhole() {
		logger.Info("login code (dev)", "email", email, "code", code)
	}
	write := b.cfg.CodeEmail
	if write == nil {
		write = englishCodeEmail(b.cfg.AppName)
	}
	subject, text, html := write(code, CodeTTL)
	if err := mailer.Send(context.Background(), email, subject, text, html); err != nil {
		logger.Error("sign_in_code_send_error", "email", email, "err", err)
	}
}

// englishCodeEmail carries no link: a home screen app has its own cookies, so a link would sign in Safari instead.
func englishCodeEmail(app string) func(code string, ttl time.Duration) (string, string, string) {
	return func(code string, ttl time.Duration) (string, string, string) {
		minutes := int(ttl / time.Minute)
		subject := fmt.Sprintf("Your %s sign-in code: %s", app, code)
		text := fmt.Sprintf("Your %s sign-in code is:\n\n%s\n\nIt expires in %d minutes. If you did not ask to sign in, ignore this email.\n", app, code, minutes)
		html := fmt.Sprintf(`<p>Your %s sign-in code is:</p>`+
			`<p style="font-size:28px;font-weight:bold;letter-spacing:4px">%s</p>`+
			`<p>It expires in %d minutes. If you did not ask to sign in, ignore this email.</p>`, app, code, minutes)
		return subject, text, html
	}
}

// emailLimiter caps the code emails per address in a fixed window; a sign-in with a code starts the count over.
type emailLimiter struct {
	mu        sync.Mutex
	max       int
	window    time.Duration
	entries   map[string]*emailWindow
	lastSweep time.Time
}

type emailWindow struct {
	count int
	end   time.Time
}

func newEmailLimiter(max int, window time.Duration) *emailLimiter {
	return &emailLimiter{max: max, window: window, entries: map[string]*emailWindow{}}
}

func (l *emailLimiter) allow(email string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastSweep) >= l.window {
		for k, e := range l.entries {
			if now.After(e.end) {
				delete(l.entries, k)
			}
		}
		l.lastSweep = now
	}
	e, ok := l.entries[email]
	if !ok || now.After(e.end) {
		l.entries[email] = &emailWindow{count: 1, end: now.Add(l.window)}
		return true
	}
	e.count++
	return e.count <= l.max
}

func (l *emailLimiter) reset(email string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, email)
}
