package backoffice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/AltSoyuz/lib/audit"
	"github.com/AltSoyuz/lib/auth/sessions"
	"github.com/AltSoyuz/lib/httpserver"
	"github.com/AltSoyuz/lib/logger"
)

// Session is the session of a request, with its account.
type Session struct {
	ID           string
	UserID       string
	Email        string
	Role         string
	AccessState  string // as stored: active or restrained
	PendingSince string
	// ImpersonatorID is the super admin viewing the app as this user, "" for a normal session.
	ImpersonatorID string
	CreatedAt      string
	ExpiresAt      string
	LastSeenAt     string
}

// Pending reports whether the account waits for an admin.
func (s Session) Pending() bool { return IsPending(s.AccessState, s.PendingSince) }

// lastSeenEvery bounds how stale sessions.last_seen_at gets: one write per session per period at most.
const lastSeenEvery = 10 * time.Minute

func (b *Backoffice) sessionByHash(ctx context.Context, sum []byte, now time.Time) (Session, bool, error) {
	var s Session
	err := b.cfg.DB.QueryRowContext(ctx, `SELECT s.id, s.user_id, u.email, u.role, u.access_state, COALESCE(u.pending_since, ''),
		COALESCE(s.impersonator_id, ''), s.created_at, s.expires_at, s.last_seen_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.sid_hash = ? AND s.revoked_at IS NULL AND s.expires_at > ?`, sum, now.Format(time.RFC3339)).
		Scan(&s.ID, &s.UserID, &s.Email, &s.Role, &s.AccessState, &s.PendingSince, &s.ImpersonatorID, &s.CreatedAt, &s.ExpiresAt, &s.LastSeenAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, fmt.Errorf("cannot read session: %w", err)
	}
	return s, true, nil
}

func (b *Backoffice) sessionByToken(ctx context.Context, token string, now time.Time) (Session, bool, error) {
	sum, err := sessions.HashFromB64(token)
	if err != nil {
		return Session{}, false, nil
	}
	return b.sessionByHash(ctx, sum, now)
}

// Check returns the session of an active account, or answers 401 (no valid session), 403 access_pending (an
// account waiting for an admin, who keeps the cookie) or 403 read_only_impersonation (a write while viewing the
// app as someone else) and returns false.
func (b *Backoffice) Check(w http.ResponseWriter, r *http.Request) (Session, bool) {
	sess, ok := b.CheckAllowPending(w, r)
	if ok && sess.Pending() {
		httpserver.WriteError(w, r, http.StatusForbidden, errAccessPending)
		return Session{}, false
	}
	return sess, ok
}

// RequireAdmin is Check for the admins only: anyone else gets 403 forbidden.
func (b *Backoffice) RequireAdmin(w http.ResponseWriter, r *http.Request) (Session, bool) {
	sess, ok := b.Check(w, r)
	if !ok {
		return Session{}, false
	}
	if sess.Role != RoleAdmin {
		httpserver.WriteError(w, r, http.StatusForbidden, errForbidden)
		return Session{}, false
	}
	return sess, true
}

// CheckAllowPending is Check that also lets an account waiting for an admin through: for /me and sign out.
func (b *Backoffice) CheckAllowPending(w http.ResponseWriter, r *http.Request) (Session, bool) {
	unauthorized := func() (Session, bool) {
		sessions.ClearCookie(w, r)
		httpserver.WriteError(w, r, http.StatusUnauthorized, errUnauthorized)
		return Session{}, false
	}
	sum, err := sessions.HashFromRequest(r)
	if err != nil {
		return unauthorized()
	}
	now := time.Now().UTC()
	sess, found, err := b.sessionByHash(r.Context(), sum, now)
	if err != nil {
		logger.Error("session_read_error", "err", err)
		return unauthorized()
	}
	if !found {
		return unauthorized()
	}

	if sess.ImpersonatorID != "" {
		live, err := b.impersonationLive(r.Context(), sess)
		if err != nil {
			httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
			return Session{}, false
		}
		if !live {
			clearImpersonationCookies(w, r)
			httpserver.WriteError(w, r, http.StatusUnauthorized, errUnauthorized)
			return Session{}, false
		}
		// Every write of every package is refused here; stop and sign out end a View as without calling Check.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			httpserver.WriteError(w, r, http.StatusForbidden, errReadOnlyImpersonation)
			return Session{}, false
		}
		return sess, true
	}

	refreshed, err := b.maybeRefresh(w, r, sess, now)
	if err != nil {
		logger.Error("session_refresh_error", "session_id", sess.ID, "err", err)
		return unauthorized()
	}
	if refreshed != "" {
		sess.ExpiresAt = refreshed
	}
	if sess.AccessState != AccessActive && !sess.Pending() {
		return unauthorized()
	}
	if refreshed == "" {
		b.touch(r.Context(), sess, now)
	}
	return sess, true
}

func (b *Backoffice) touch(ctx context.Context, sess Session, now time.Time) {
	if last, err := time.Parse(time.RFC3339, sess.LastSeenAt); err == nil && now.Sub(last) < lastSeenEvery {
		return
	}
	nowStr := now.Format(time.RFC3339)
	if _, err := b.cfg.DB.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ?, updated_at = ? WHERE id = ?`, nowStr, nowStr, sess.ID); err != nil {
		logger.Error("session_touch_error", "session_id", sess.ID, "err", err)
	}
}

func (b *Backoffice) maybeRefresh(w http.ResponseWriter, r *http.Request, sess Session, now time.Time) (string, error) {
	current, err := time.Parse(time.RFC3339, sess.ExpiresAt)
	if err != nil {
		return "", err
	}
	created, err := time.Parse(time.RFC3339, sess.CreatedAt)
	if err != nil {
		return "", err
	}
	next, ok := sessions.NextSessionExpiry(now, created, current, b.cfg.Sessions)
	if !ok {
		return "", nil
	}
	nowStr, nextStr := now.Format(time.RFC3339), next.Format(time.RFC3339)
	if _, err := b.cfg.DB.ExecContext(r.Context(), `UPDATE sessions SET last_seen_at = ?, updated_at = ?, expires_at = ? WHERE id = ?`,
		nowStr, nowStr, nextStr, sess.ID); err != nil {
		return "", err
	}
	token, err := sessions.ReadCookie(r)
	if err != nil {
		return "", err
	}
	sessions.SetCookie(w, r, token, next)
	return nextStr, nil
}

// EndSession signs the request out and clears its cookie: a View as goes back to the admin's own session, any other
// session is revoked with a signed_out audit row. found is false when the request had no valid session.
func (b *Backoffice) EndSession(w http.ResponseWriter, r *http.Request) (found bool, err error) {
	ctx := r.Context()
	now := time.Now().UTC()
	token, err := sessions.ReadCookie(r)
	if err != nil {
		sessions.ClearCookie(w, r)
		return false, nil
	}
	sess, found, err := b.sessionByToken(ctx, token, now)
	if err != nil {
		return false, err
	}
	if !found {
		sessions.ClearCookie(w, r)
		return false, nil
	}
	if sess.ImpersonatorID != "" {
		_, err := b.endImpersonation(w, r, sess)
		return true, err
	}
	if _, err := b.cfg.DB.ExecContext(ctx, `UPDATE sessions SET revoked_at = ? WHERE id = ? AND user_id = ?`,
		now.Format(time.RFC3339), sess.ID, sess.UserID); err != nil {
		return true, fmt.Errorf("cannot revoke session %s: %w", sess.ID, err)
	}
	audit.After(ctx, b.cfg.DB, sess.UserID, audit.SignedOut, sess.UserID, nil)
	sessions.ClearCookie(w, r)
	return true, nil
}
