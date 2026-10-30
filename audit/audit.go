// Package audit writes who changed what into an app's audit_events table (schema.sql), in the transaction of
// the change, and pages through it for a back office.
//
// The account actions of package backoffice are listed here; an app adds the codes of its own domain.
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/AltSoyuz/lib/logger"
)

// Layout is the fixed-width UTC layout of audit_events.at.
const Layout = "2006-01-02T15:04:05.000Z"

// Codes of the account actions. Rows keep them: never rename one, add a new code.
const (
	SignedIn             = "signed_in"
	SignedUp             = "signed_up"
	SignedOut            = "signed_out"
	RoleChanged          = "role_changed"
	UserRestrained       = "user_restrained"
	UserRestored         = "user_restored"
	UserApproved         = "user_approved"
	ImpersonationStarted = "impersonation_started"
	ImpersonationStopped = "impersonation_stopped"
	AnnouncementSet      = "announcement_set"
	AnnouncementCleared  = "announcement_cleared"
	// FeatureSet stays out of AccountActions: a back office lists it only when the app has features.
	FeatureSet = "feature_set"
)

// AccountActions lists the account codes in the order a back office filter shows them.
var AccountActions = []string{
	SignedIn, SignedUp, SignedOut, RoleChanged, UserRestrained, UserRestored, UserApproved,
	ImpersonationStarted, ImpersonationStopped, AnnouncementSet, AnnouncementCleared,
}

// Execer is a *sql.DB or a *sql.Tx.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Querier is a *sql.DB or a *sql.Tx.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Write writes one audit row through db; given the transaction of the change, both commit or neither does.
func Write(ctx context.Context, db Execer, actorID, action, target string, details map[string]any) error {
	b := []byte("{}")
	if len(details) > 0 {
		var err error
		if b, err = json.Marshal(details); err != nil {
			return fmt.Errorf("cannot encode audit details of %s: %w", action, err)
		}
	}
	actor := sql.NullString{String: actorID, Valid: actorID != ""}
	if _, err := db.ExecContext(ctx, `INSERT INTO audit_events (at, actor_id, action, target, details) VALUES (?, ?, ?, ?, ?)`,
		time.Now().UTC().Format(Layout), actor, action, target, string(b)); err != nil {
		return fmt.Errorf("cannot write audit event %s: %w", action, err)
	}
	return nil
}

// After writes the audit row of a committed change and logs a failure. Never inside a transaction: on a database
// with a single pooled connection, the write would wait for the transaction forever.
func After(ctx context.Context, db Execer, actorID, action, target string, details map[string]any) {
	if err := Write(context.WithoutCancel(ctx), db, actorID, action, target, details); err != nil {
		logger.Error("audit write failed", "action", action, "actor_id", actorID, "target", target, "err", err)
	}
}

// Event is one audit row.
type Event struct {
	ID      int64           `json:"id"`
	At      string          `json:"at"`
	ActorID string          `json:"actor_id"`
	Action  string          `json:"action"`
	Target  string          `json:"target"`
	Details json.RawMessage `json:"details"`
}

// Filter selects a page of events, newest first: those before the event id Before (0 for the newest), of one
// action and one actor when set, Limit at most.
type Filter struct {
	Before  int64
	Action  string
	ActorID string
	Limit   int
}

// List returns a page of events and the id to pass as Before for the next one, 0 after the last page.
func List(ctx context.Context, db Querier, f Filter) ([]Event, int64, error) {
	if f.Limit <= 0 {
		return nil, 0, fmt.Errorf("audit: list needs a positive limit, got %d", f.Limit)
	}
	before := f.Before
	if before <= 0 {
		before = 1<<63 - 1
	}
	rows, err := db.QueryContext(ctx, `SELECT id, at, COALESCE(actor_id, ''), action, target, details FROM audit_events
		WHERE id < ? AND (? = '' OR action = ?) AND (? = '' OR actor_id = ?)
		ORDER BY id DESC LIMIT ?`, before, f.Action, f.Action, f.ActorID, f.ActorID, f.Limit)
	if err != nil {
		return nil, 0, fmt.Errorf("cannot list audit events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	events := []Event{}
	for rows.Next() {
		var e Event
		var details string
		if err := rows.Scan(&e.ID, &e.At, &e.ActorID, &e.Action, &e.Target, &details); err != nil {
			return nil, 0, fmt.Errorf("cannot read an audit event: %w", err)
		}
		e.Details = json.RawMessage(details)
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("cannot list audit events: %w", err)
	}
	var next int64
	if len(events) == f.Limit {
		next = events[len(events)-1].ID
	}
	return events, next, nil
}

// Since returns the time of the first event, "" when there is none.
func Since(ctx context.Context, db Querier) (string, error) {
	var at string
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MIN(at), '') FROM audit_events`).Scan(&at); err != nil {
		return "", fmt.Errorf("cannot read the first audit event: %w", err)
	}
	return at, nil
}
