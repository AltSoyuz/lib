// Package backoffice is the back office the apps share: who may use an app (roles, access states), the session
// check that enforces it on every request, the admin routes over the accounts, View as, the announcement shown to
// everyone, the features an admin keeps to the admins, the audit log and the account stats. schema.sql lists the
// columns and the tables it needs beside lib's auth tables and audit_events.
//
// An app keeps its sign-in flows: they call Admit and AuditSignIn in the transaction that creates the session, and
// every handler that needs a signed-in user calls Check (or RequireAdmin). Mount serves the admin routes under
// /api/v1/admin/ and the features under /api/v1/features; the UI kit (lib's web/admin, web/features.svelte.ts)
// draws them.
package backoffice

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/AltSoyuz/lib/auth/passwords"
	"github.com/AltSoyuz/lib/auth/sessions"
)

// RoleAdmin is the role of the accounts that use the back office; every app has it.
const RoleAdmin = "admin"

// Access states. The database keeps a pending account restrained with pending_since set: AccessPending only
// exists in the API.
const (
	AccessActive     = "active"
	AccessRestrained = "restrained"
	AccessPending    = "pending"
)

var (
	// ErrRestrained refuses the sign-in of an account an admin restrained.
	ErrRestrained = errors.New("user_restrained")

	errForbidden             = errors.New("forbidden")
	errUnauthorized          = errors.New("unauthorized")
	errAccessPending         = errors.New("access_pending")
	errReadOnlyImpersonation = errors.New("read_only_impersonation")
	errUserNotFound          = errors.New("user_not_found")
	errMissingUserID         = errors.New("missing_user_id")
	errInvalidRole           = errors.New("invalid_role")
	errCannotRestrainSelf    = errors.New("cannot_restrain_self")
	errCannotModifyLastAdmin = errors.New("cannot_modify_last_admin")
	errNotPending            = errors.New("not_pending")
	// The super admins come from the app's flag: another admin cannot lock them out of what the flag grants.
	errSuperAdminProtected = errors.New("super_admin_protected")
)

// Config is what an app decides about its accounts.
type Config struct {
	DB *sql.DB
	// Roles an admin can give, RoleAdmin included; MemberRole is the role of a new account that is not an admin.
	Roles      []string
	MemberRole string
	// SuperAdmins are the normalized emails of the app's admin flag: always admins, protected from the other
	// admins, the only ones who can view the app as someone else.
	SuperAdmins map[string]struct{}
	// Approval makes a new account wait for an admin unless Admit lets it in; without it every account is active.
	Approval bool
	// Admit, with Approval, lets a new account in at once and says why ("" keeps it pending); it runs in the
	// sign-in transaction.
	Admit func(ctx context.Context, tx *sql.Tx, email string) (string, error)
	// Actions are the app's own audit codes, listed after audit.AccountActions in the audit log filter.
	Actions []string
	// Sessions is the session lifetime policy the check refreshes sessions with.
	Sessions sessions.SessionPolicy
	// Features are the parts of the app an admin can keep to the admins, in the order the back office lists them.
	Features []Feature
}

// Backoffice serves the back office of one app.
type Backoffice struct {
	cfg Config
}

// New checks the Config.
func New(cfg Config) (*Backoffice, error) {
	switch {
	case cfg.DB == nil:
		return nil, errors.New("backoffice: Config.DB is nil")
	case !slices.Contains(cfg.Roles, RoleAdmin):
		return nil, fmt.Errorf("backoffice: Config.Roles %v lacks %q", cfg.Roles, RoleAdmin)
	case cfg.MemberRole == RoleAdmin || !slices.Contains(cfg.Roles, cfg.MemberRole):
		return nil, fmt.Errorf("backoffice: Config.MemberRole %q is not one of the non-admin roles %v", cfg.MemberRole, cfg.Roles)
	case cfg.Sessions.TTL <= 0:
		return nil, errors.New("backoffice: Config.Sessions has no TTL")
	}
	if err := checkFeatures(cfg.Features); err != nil {
		return nil, err
	}
	if cfg.SuperAdmins == nil {
		cfg.SuperAdmins = map[string]struct{}{}
	}
	return &Backoffice{cfg: cfg}, nil
}

// User is an account as the back office sees it.
type User struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	AccessState  string `json:"access_state"` // the API value: active, restrained or pending
	PendingSince string `json:"-"`
	CreatedAt    string `json:"created_at"`
}

// IsPending reports whether an account signed up and waits for an admin, as opposed to one an admin restrained.
func IsPending(accessState, pendingSince string) bool {
	return accessState == AccessRestrained && pendingSince != ""
}

func apiAccess(accessState, pendingSince string) string {
	if IsPending(accessState, pendingSince) {
		return AccessPending
	}
	return accessState
}

// IsSuperAdmin reports whether an account is an admin whose email is in Config.SuperAdmins.
func (b *Backoffice) IsSuperAdmin(role, email string) bool {
	if role != RoleAdmin {
		return false
	}
	normalized, err := passwords.NormalizeEmail(email)
	if err != nil {
		return false
	}
	_, ok := b.cfg.SuperAdmins[normalized]
	return ok
}

func (b *Backoffice) userByID(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, id string) (User, error) {
	var u User
	err := q.QueryRowContext(ctx, `SELECT id, email, role, access_state, COALESCE(pending_since, ''), created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Email, &u.Role, &u.AccessState, &u.PendingSince, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, errUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("cannot read user %s: %w", id, err)
	}
	return u, nil
}

func (b *Backoffice) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := b.cfg.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("cannot begin a transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("cannot commit: %w", err)
	}
	return nil
}

// newID returns a random UUID (version 4), the form of the ids of lib's auth tables.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
