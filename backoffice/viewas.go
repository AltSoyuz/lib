package backoffice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AltSoyuz/lib/audit"
	"github.com/AltSoyuz/lib/auth/sessions"
	"github.com/AltSoyuz/lib/httpserver"
	"github.com/AltSoyuz/lib/logger"
)

const impersonationTTL = time.Hour

var (
	errNotImpersonating       = errors.New("not_impersonating")
	errCannotImpersonateSelf  = errors.New("cannot_impersonate_self")
	errCannotImpersonateAdmin = errors.New("cannot_impersonate_admin")
)

// adminCookie keeps the admin's own session token aside while they view the app as someone else.
func adminCookie(r *http.Request) (name string, secure bool) {
	if sessions.RequestIsHTTPS(r) && !sessions.IsLocalhost(sessions.RequestHost(r)) {
		return "__Host-admin-session", true
	}
	return "admin_sid", false
}

func clearImpersonationCookies(w http.ResponseWriter, r *http.Request) {
	name, secure := adminCookie(r)
	sessions.ClearTransientCookie(w, name, secure)
	sessions.ClearCookie(w, r)
}

// impersonationLive reports whether a View as session may still be used: its user is an active non-admin and the
// impersonator still an active super admin.
func (b *Backoffice) impersonationLive(ctx context.Context, sess Session) (bool, error) {
	if sess.Role == RoleAdmin || sess.AccessState != AccessActive {
		return false, nil
	}
	admin, err := b.userByID(ctx, b.cfg.DB, sess.ImpersonatorID)
	if errors.Is(err, errUserNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return admin.AccessState == AccessActive && b.IsSuperAdmin(admin.Role, admin.Email), nil
}

func (b *Backoffice) handleImpersonate(w http.ResponseWriter, r *http.Request) {
	adminSession, ok := b.RequireAdmin(w, r)
	if !ok {
		return
	}
	if !b.IsSuperAdmin(adminSession.Role, adminSession.Email) {
		httpserver.WriteError(w, r, http.StatusForbidden, errForbidden)
		return
	}
	targetID := strings.TrimSpace(r.PathValue("id"))
	if targetID == "" {
		httpserver.WriteError(w, r, http.StatusBadRequest, errMissingUserID)
		return
	}
	if targetID == adminSession.UserID {
		httpserver.WriteError(w, r, http.StatusConflict, errCannotImpersonateSelf)
		return
	}
	adminToken, err := sessions.ReadCookie(r)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusUnauthorized, errUnauthorized)
		return
	}
	token, sidHash, err := sessions.NewToken()
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	expires := now.Add(impersonationTTL)
	sessionID := newID()
	txErr := b.inTx(r.Context(), func(tx *sql.Tx) error {
		target, err := b.userByID(r.Context(), tx, targetID)
		if err != nil {
			return err
		}
		if target.Role == RoleAdmin {
			return errCannotImpersonateAdmin
		}
		if target.AccessState != AccessActive {
			return ErrRestrained
		}
		if _, err := tx.ExecContext(r.Context(), `INSERT INTO sessions
			(id, user_id, sid_hash, user_agent, ip, created_at, updated_at, last_seen_at, expires_at, impersonator_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sessionID, target.ID, sidHash, r.UserAgent(), httpserver.ClientIP(r), nowStr, nowStr, nowStr, expires.Format(time.RFC3339), adminSession.UserID); err != nil {
			return fmt.Errorf("cannot create the View as session: %w", err)
		}
		return audit.Write(r.Context(), tx, adminSession.UserID, audit.ImpersonationStarted, target.ID, map[string]any{"expires_at": expires.Format(time.RFC3339)})
	})
	if txErr != nil {
		switch {
		case errors.Is(txErr, errUserNotFound):
			httpserver.WriteError(w, r, http.StatusNotFound, txErr)
		case errors.Is(txErr, errCannotImpersonateAdmin), errors.Is(txErr, ErrRestrained):
			httpserver.WriteError(w, r, http.StatusConflict, txErr)
		default:
			httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", txErr))
		}
		return
	}
	name, secure := adminCookie(r)
	sessions.SetTransientCookie(w, name, adminToken, expires, secure)
	sessions.SetCookie(w, r, token, expires)
	logger.Info("impersonation_started", "admin_id", adminSession.UserID, "target_id", targetID,
		"admin_session_id", adminSession.ID, "session_id", sessionID, "expires_at", expires.Format(time.RFC3339))
	w.Header().Set("Cache-Control", "no-store")
	httpserver.WriteJSON(w, r, http.StatusOK, map[string]bool{"ok": true})
}

func (b *Backoffice) handleStopImpersonation(w http.ResponseWriter, r *http.Request) {
	token, err := sessions.ReadCookie(r)
	if err != nil {
		clearImpersonationCookies(w, r)
		httpserver.WriteError(w, r, http.StatusUnauthorized, errUnauthorized)
		return
	}
	sess, found, err := b.sessionByToken(r.Context(), token, time.Now().UTC())
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	if !found {
		clearImpersonationCookies(w, r)
		httpserver.WriteError(w, r, http.StatusUnauthorized, errUnauthorized)
		return
	}
	if sess.ImpersonatorID == "" {
		httpserver.WriteError(w, r, http.StatusConflict, errNotImpersonating)
		return
	}
	restored, err := b.endImpersonation(w, r, sess)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpserver.WriteJSON(w, r, http.StatusOK, map[string]bool{"restored": restored})
}

// endImpersonation revokes the View as session and puts back the admin session kept aside, when it is still a
// live session of that admin.
func (b *Backoffice) endImpersonation(w http.ResponseWriter, r *http.Request, sess Session) (bool, error) {
	ctx := r.Context()
	now := time.Now().UTC()
	adminID := sess.ImpersonatorID
	name, secure := adminCookie(r)

	var keptID, keptToken string
	var keptExpires time.Time
	if c, err := r.Cookie(name); err == nil {
		kept, found, err := b.sessionByToken(ctx, c.Value, now)
		if err != nil {
			return false, err
		}
		if found && kept.ImpersonatorID == "" && kept.UserID == adminID && kept.Role == RoleAdmin && kept.AccessState == AccessActive {
			if keptExpires, err = time.Parse(time.RFC3339, kept.ExpiresAt); err != nil {
				return false, fmt.Errorf("cannot parse the expiry of session %s: %w", kept.ID, err)
			}
			keptID, keptToken = kept.ID, c.Value
		}
	}
	if _, err := b.cfg.DB.ExecContext(ctx, `UPDATE sessions SET revoked_at = ? WHERE id = ? AND user_id = ?`,
		now.Format(time.RFC3339), sess.ID, sess.UserID); err != nil {
		return false, fmt.Errorf("cannot revoke View as session %s: %w", sess.ID, err)
	}
	restored := keptToken != ""
	sessions.ClearTransientCookie(w, name, secure)
	if restored {
		sessions.SetCookie(w, r, keptToken, keptExpires)
	} else {
		sessions.ClearCookie(w, r)
	}
	logger.Info("impersonation_stopped", "admin_id", adminID, "target_id", sess.UserID,
		"admin_session_id", keptID, "session_id", sess.ID, "restored", restored)
	audit.After(ctx, b.cfg.DB, adminID, audit.ImpersonationStopped, sess.UserID, map[string]any{"restored": restored})
	return restored, nil
}
