package backoffice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/AltSoyuz/lib/audit"
)

// Why a sign-in created an account, as the signed_up audit row and log line say it.
const (
	JoinedAdminEmail = "admin_email"
	JoinedFirstUser  = "first_user"
	JoinedOpen       = "open"
	JoinedUnknown    = "unknown_email"
)

// Admit returns the account of a sign-in that proved email, in the transaction that creates its session:
//   - a new account is created: an admin when its email is a super admin's or the database has no account yet (or
//     nobody could ever approve anyone), else the member role, active unless Config.Approval keeps it pending;
//   - a super admin is made an admin again at every sign-in;
//   - an account an admin restrained gets ErrRestrained.
//
// joined says why the account was created, "" for an existing one. email must be normalized.
func (b *Backoffice) Admit(ctx context.Context, tx *sql.Tx, email string, now time.Time) (User, string, error) {
	nowStr := now.UTC().Format(time.RFC3339)
	var u User
	err := tx.QueryRowContext(ctx, `SELECT id, email, role, access_state, COALESCE(pending_since, ''), created_at FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.Email, &u.Role, &u.AccessState, &u.PendingSince, &u.CreatedAt)
	joined := ""
	switch {
	case errors.Is(err, sql.ErrNoRows):
		active, reason, err := b.signUpAccess(ctx, tx, email)
		if err != nil {
			return User{}, "", err
		}
		u = User{ID: newID(), Email: email, Role: b.cfg.MemberRole, AccessState: AccessActive, CreatedAt: nowStr}
		if reason == JoinedAdminEmail || reason == JoinedFirstUser {
			u.Role = RoleAdmin
		}
		if !active {
			u.AccessState, u.PendingSince = AccessRestrained, nowStr
		}
		pending := sql.NullString{String: u.PendingSince, Valid: u.PendingSince != ""}
		if _, err := tx.ExecContext(ctx, `INSERT INTO users (id, email, role, access_state, pending_since, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			u.ID, u.Email, u.Role, u.AccessState, pending, u.CreatedAt); err != nil {
			return User{}, "", fmt.Errorf("cannot create the account of %s: %w", email, err)
		}
		joined = reason
	case err != nil:
		return User{}, "", fmt.Errorf("cannot read the account of %s: %w", email, err)
	}
	if _, ok := b.cfg.SuperAdmins[email]; ok && u.Role != RoleAdmin {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET role = ? WHERE id = ?`, RoleAdmin, u.ID); err != nil {
			return User{}, "", fmt.Errorf("cannot make %s an admin: %w", email, err)
		}
		u.Role = RoleAdmin
	}
	if u.AccessState == AccessRestrained && u.PendingSince == "" {
		return User{}, "", ErrRestrained
	}
	u.AccessState = apiAccess(u.AccessState, u.PendingSince)
	return u, joined, nil
}

func (b *Backoffice) signUpAccess(ctx context.Context, tx *sql.Tx, email string) (bool, string, error) {
	if _, ok := b.cfg.SuperAdmins[email]; ok {
		return true, JoinedAdminEmail, nil
	}
	var users int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		return false, "", fmt.Errorf("cannot count accounts: %w", err)
	}
	if users == 0 {
		return true, JoinedFirstUser, nil
	}
	if !b.cfg.Approval {
		return true, JoinedOpen, nil
	}
	if b.cfg.Admit != nil {
		reason, err := b.cfg.Admit(ctx, tx, email)
		if err != nil {
			return false, "", err
		}
		if reason != "" {
			return true, reason, nil
		}
	}
	return false, JoinedUnknown, nil
}

// AuditSignIn writes signed_up first when the sign-in created the account, then signed_in, in the sign-in
// transaction. via names the sign-in method.
func (b *Backoffice) AuditSignIn(ctx context.Context, tx *sql.Tx, u User, joined, via, ip string) error {
	if joined != "" {
		if err := audit.Write(ctx, tx, u.ID, audit.SignedUp, u.ID, map[string]any{"access_state": u.AccessState, "reason": joined}); err != nil {
			return err
		}
	}
	return audit.Write(ctx, tx, u.ID, audit.SignedIn, u.ID, map[string]any{"via": via, "ip": ip})
}
