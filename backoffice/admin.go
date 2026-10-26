package backoffice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/AltSoyuz/lib/audit"
	"github.com/AltSoyuz/lib/httpserver"
	"github.com/AltSoyuz/lib/logger"
)

// Mount serves the admin routes; every one needs an admin session.
func (b *Backoffice) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/users", b.handleListUsers(false))
	mux.HandleFunc("GET /api/v1/admin/connected-users", b.handleListUsers(true))
	mux.HandleFunc("POST /api/v1/admin/users/{id}/role", b.handleSetRole)
	mux.HandleFunc("POST /api/v1/admin/users/{id}/restrain", b.handleRestrain)
	mux.HandleFunc("POST /api/v1/admin/users/{id}/unrestrain", b.handleUnrestrain)
	mux.HandleFunc("POST /api/v1/admin/users/{id}/approve", b.handleApprove)
	mux.HandleFunc("POST /api/v1/admin/users/{id}/impersonate", b.handleImpersonate)
	mux.HandleFunc("POST /api/v1/admin/impersonation/stop", b.handleStopImpersonation)
	mux.HandleFunc("PUT /api/v1/admin/announcement", b.handleSetAnnouncement)
	mux.HandleFunc("DELETE /api/v1/admin/announcement", b.handleClearAnnouncement)
	mux.HandleFunc("GET /api/v1/admin/audit", b.handleAudit)
	mux.HandleFunc("GET /api/v1/admin/stats", b.handleStats)
}

// AdminUser is an account in the admin list. Connected means it has a live session of its own (not a View as).
type AdminUser struct {
	User
	LastSeenAt         string `json:"last_seen_at"`
	ActiveSessionCount int64  `json:"active_session_count"`
	Connected          bool   `json:"connected"`
}

// ListUsers returns every account, the pending ones first, then the newest; connectedOnly keeps the connected ones.
func (b *Backoffice) ListUsers(ctx context.Context, connectedOnly bool) ([]AdminUser, error) {
	join := "LEFT JOIN"
	if connectedOnly {
		join = "JOIN"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := b.cfg.DB.QueryContext(ctx, `SELECT u.id, u.email, u.role, u.access_state, COALESCE(u.pending_since, ''), u.created_at,
		COALESCE(MAX(CASE WHEN s.revoked_at IS NULL AND s.impersonator_id IS NULL AND s.expires_at > ? THEN s.last_seen_at END), ''),
		COUNT(CASE WHEN s.revoked_at IS NULL AND s.impersonator_id IS NULL AND s.expires_at > ? THEN 1 END)
		FROM users u `+join+` sessions s ON s.user_id = u.id
		GROUP BY u.id ORDER BY u.created_at DESC`, now, now)
	if err != nil {
		return nil, fmt.Errorf("cannot list accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []AdminUser{}
	for rows.Next() {
		var u AdminUser
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &u.AccessState, &u.PendingSince, &u.CreatedAt, &u.LastSeenAt, &u.ActiveSessionCount); err != nil {
			return nil, fmt.Errorf("cannot read an account: %w", err)
		}
		u.AccessState = apiAccess(u.AccessState, u.PendingSince)
		u.Connected = u.ActiveSessionCount > 0
		if connectedOnly && !u.Connected {
			continue
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("cannot list accounts: %w", err)
	}
	pendingFirst := func(u AdminUser) int {
		if u.AccessState == AccessPending {
			return 0
		}
		return 1
	}
	slices.SortStableFunc(out, func(a, c AdminUser) int { return pendingFirst(a) - pendingFirst(c) })
	return out, nil
}

func (b *Backoffice) handleListUsers(connectedOnly bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := b.RequireAdmin(w, r); !ok {
			return
		}
		users, err := b.ListUsers(r.Context(), connectedOnly)
		if err != nil {
			httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
			return
		}
		httpserver.WriteJSON(w, r, http.StatusOK, map[string]any{"users": users, "roles": b.cfg.Roles})
	}
}

func targetID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		httpserver.WriteError(w, r, http.StatusBadRequest, errMissingUserID)
		return "", false
	}
	return id, true
}

func countAdmins(ctx context.Context, tx *sql.Tx) (int, error) {
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = ?`, RoleAdmin).Scan(&n); err != nil {
		return 0, fmt.Errorf("cannot count admins: %w", err)
	}
	return n, nil
}

func writeAdminError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errUserNotFound):
		httpserver.WriteError(w, r, http.StatusNotFound, err)
	case errors.Is(err, errCannotModifyLastAdmin), errors.Is(err, errNotPending), errors.Is(err, errCannotRestrainSelf):
		httpserver.WriteError(w, r, http.StatusConflict, err)
	case errors.Is(err, errSuperAdminProtected):
		httpserver.WriteError(w, r, http.StatusForbidden, err)
	default:
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
	}
}

func (b *Backoffice) handleSetRole(w http.ResponseWriter, r *http.Request) {
	admin, ok := b.RequireAdmin(w, r)
	if !ok {
		return
	}
	id, ok := targetID(w, r)
	if !ok {
		return
	}
	in, err := httpserver.DecodeJSON[struct {
		Role string `json:"role"`
	}](r)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, err)
		return
	}
	role := strings.ToLower(strings.TrimSpace(in.Role))
	if !slices.Contains(b.cfg.Roles, role) {
		httpserver.WriteError(w, r, http.StatusBadRequest, errInvalidRole)
		return
	}
	err = b.inTx(r.Context(), func(tx *sql.Tx) error {
		u, err := b.userByID(r.Context(), tx, id)
		if err != nil {
			return err
		}
		if role != RoleAdmin && id != admin.UserID && b.IsSuperAdmin(u.Role, u.Email) {
			return errSuperAdminProtected
		}
		if u.Role == RoleAdmin && role != RoleAdmin {
			n, err := countAdmins(r.Context(), tx)
			if err != nil {
				return err
			}
			if n <= 1 {
				return errCannotModifyLastAdmin
			}
		}
		if u.Role == role {
			return nil
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE users SET role = ? WHERE id = ?`, role, id); err != nil {
			return fmt.Errorf("cannot change the role of %s: %w", id, err)
		}
		return audit.Write(r.Context(), tx, admin.UserID, audit.RoleChanged, id, map[string]any{"from": u.Role, "to": role})
	})
	if err != nil {
		writeAdminError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, map[string]bool{"ok": true})
}

// handleRestrain locks an account out at once: its sessions are revoked with it.
func (b *Backoffice) handleRestrain(w http.ResponseWriter, r *http.Request) {
	admin, ok := b.RequireAdmin(w, r)
	if !ok {
		return
	}
	id, ok := targetID(w, r)
	if !ok {
		return
	}
	if id == admin.UserID {
		writeAdminError(w, r, errCannotRestrainSelf)
		return
	}
	nowStr := time.Now().UTC().Format(time.RFC3339)
	err := b.inTx(r.Context(), func(tx *sql.Tx) error {
		u, err := b.userByID(r.Context(), tx, id)
		if err != nil {
			return err
		}
		if b.IsSuperAdmin(u.Role, u.Email) {
			return errSuperAdminProtected
		}
		if u.Role == RoleAdmin {
			n, err := countAdmins(r.Context(), tx)
			if err != nil {
				return err
			}
			if n <= 1 {
				return errCannotModifyLastAdmin
			}
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE users SET access_state = ? WHERE id = ?`, AccessRestrained, id); err != nil {
			return fmt.Errorf("cannot restrain %s: %w", id, err)
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`, nowStr, id); err != nil {
			return fmt.Errorf("cannot revoke the sessions of %s: %w", id, err)
		}
		return audit.Write(r.Context(), tx, admin.UserID, audit.UserRestrained, id, nil)
	})
	if err != nil {
		writeAdminError(w, r, err)
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, map[string]bool{"ok": true})
}

// handleUnrestrain makes an account active again; for a pending one it is an approval.
func (b *Backoffice) handleUnrestrain(w http.ResponseWriter, r *http.Request) {
	b.activate(w, r, false)
}

// handleApprove lets a pending account in; any other answers 409 not_pending.
func (b *Backoffice) handleApprove(w http.ResponseWriter, r *http.Request) {
	b.activate(w, r, true)
}

func (b *Backoffice) activate(w http.ResponseWriter, r *http.Request, pendingOnly bool) {
	admin, ok := b.RequireAdmin(w, r)
	if !ok {
		return
	}
	id, ok := targetID(w, r)
	if !ok {
		return
	}
	how := "restore"
	if pendingOnly {
		how = "approve"
	}
	wasPending := false
	err := b.inTx(r.Context(), func(tx *sql.Tx) error {
		u, err := b.userByID(r.Context(), tx, id)
		if err != nil {
			return err
		}
		wasPending = u.AccessState == AccessRestrained && u.PendingSince != ""
		if pendingOnly && !wasPending {
			return errNotPending
		}
		if u.AccessState == AccessActive {
			return nil
		}
		if _, err := tx.ExecContext(r.Context(), `UPDATE users SET access_state = ? WHERE id = ?`, AccessActive, id); err != nil {
			return fmt.Errorf("cannot make %s active: %w", id, err)
		}
		if wasPending {
			return audit.Write(r.Context(), tx, admin.UserID, audit.UserApproved, id, map[string]any{"how": how})
		}
		return audit.Write(r.Context(), tx, admin.UserID, audit.UserRestored, id, nil)
	})
	if err != nil {
		writeAdminError(w, r, err)
		return
	}
	if wasPending {
		logger.Info("user_approved", "user_id", id, "by", admin.UserID, "how", how)
	}
	httpserver.WriteJSON(w, r, http.StatusOK, map[string]bool{"ok": true})
}
