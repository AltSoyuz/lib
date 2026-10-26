package backoffice

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/AltSoyuz/lib/audit"
	"github.com/AltSoyuz/lib/httpserver"
)

const auditPageSize = 50

var (
	errInvalidAction = errors.New("invalid_action")
	errInvalidBefore = errors.New("invalid_before")
)

// LoggedEvent is an audit event with its actor and target named for a person: emails for accounts.
type LoggedEvent struct {
	audit.Event
	Actor       string `json:"actor"`
	TargetLabel string `json:"target_label"`
}

func (b *Backoffice) actions() []string {
	return append(slices.Clone(audit.AccountActions), b.cfg.Actions...)
}

// handleAudit pages through the audit log: ?action= and ?actor= filter it, ?before= is the next_before of the page before.
func (b *Backoffice) handleAudit(w http.ResponseWriter, r *http.Request) {
	if _, ok := b.RequireAdmin(w, r); !ok {
		return
	}
	q := r.URL.Query()
	f := audit.Filter{Action: q.Get("action"), ActorID: q.Get("actor"), Limit: auditPageSize}
	if s := q.Get("before"); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n <= 0 {
			httpserver.WriteError(w, r, http.StatusBadRequest, errInvalidBefore)
			return
		}
		f.Before = n
	}
	actions := b.actions()
	if f.Action != "" && !slices.Contains(actions, f.Action) {
		httpserver.WriteError(w, r, http.StatusBadRequest, errInvalidAction)
		return
	}
	events, next, err := audit.List(r.Context(), b.cfg.DB, f)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	since, err := audit.Since(r.Context(), b.cfg.DB)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	emails, err := b.emails(r.Context())
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	out := make([]LoggedEvent, 0, len(events))
	for _, e := range events {
		ev := LoggedEvent{Event: e, Actor: "System", TargetLabel: e.Target}
		if e.ActorID != "" {
			ev.Actor = cmpOr(emails[e.ActorID], e.ActorID)
		}
		if email, ok := emails[e.Target]; ok {
			ev.TargetLabel = email
		}
		out = append(out, ev)
	}
	w.Header().Set("Cache-Control", "no-store")
	httpserver.WriteJSON(w, r, http.StatusOK, map[string]any{"events": out, "next_before": next, "since": since, "actions": actions})
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (b *Backoffice) emails(ctx context.Context) (map[string]string, error) {
	rows, err := b.cfg.DB.QueryContext(ctx, `SELECT id, email FROM users`)
	if err != nil {
		return nil, fmt.Errorf("cannot list emails: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var id, email string
		if err := rows.Scan(&id, &email); err != nil {
			return nil, fmt.Errorf("cannot read an email: %w", err)
		}
		out[id] = email
	}
	return out, rows.Err()
}

// Stats counts the accounts: by role and access, new and active over the last day and week.
type Stats struct {
	Total      int            `json:"total"`
	ByRole     map[string]int `json:"by_role"`
	Active24h  int            `json:"active_24h"`
	Active7d   int            `json:"active_7d"`
	NeverBack  int            `json:"never_seen_since_sign_up"`
	New7d      int            `json:"new_7d"`
	Pending    int            `json:"pending"`
	Restrained int            `json:"restrained"`
}

// neverBackWithin: an account last seen within this time of its creation never came back after signing up.
const neverBackWithin = time.Hour

// AccountStats counts the accounts at now; last seen leaves out the sessions of a View as.
func (b *Backoffice) AccountStats(ctx context.Context, now time.Time) (Stats, error) {
	rows, err := b.cfg.DB.QueryContext(ctx, `SELECT u.role, u.access_state, COALESCE(u.pending_since, ''), u.created_at,
		COALESCE((SELECT MAX(s.last_seen_at) FROM sessions s WHERE s.user_id = u.id AND s.impersonator_id IS NULL), '')
		FROM users u`)
	if err != nil {
		return Stats{}, fmt.Errorf("cannot count accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	const day = 24 * time.Hour
	out := Stats{ByRole: map[string]int{}}
	for rows.Next() {
		var role, access, pendingSince, createdAt, lastSeenAt string
		if err := rows.Scan(&role, &access, &pendingSince, &createdAt, &lastSeenAt); err != nil {
			return Stats{}, fmt.Errorf("cannot read an account: %w", err)
		}
		out.Total++
		out.ByRole[role]++
		switch {
		case IsPending(access, pendingSince):
			out.Pending++
		case access == AccessRestrained:
			out.Restrained++
		}
		created, _ := time.Parse(time.RFC3339, createdAt)
		if !created.IsZero() && now.Sub(created) <= 7*day {
			out.New7d++
		}
		last, err := time.Parse(time.RFC3339, lastSeenAt)
		seen := err == nil
		if seen && now.Sub(last) <= day {
			out.Active24h++
		}
		if seen && now.Sub(last) <= 7*day {
			out.Active7d++
		}
		if !seen || last.Sub(created) < neverBackWithin {
			out.NeverBack++
		}
	}
	return out, rows.Err()
}

func (b *Backoffice) handleStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := b.RequireAdmin(w, r); !ok {
		return
	}
	stats, err := b.AccountStats(r.Context(), time.Now().UTC())
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpserver.WriteJSON(w, r, http.StatusOK, stats)
}

// Me is what /me adds about access: the API access state, who views the app as this user, whether the user may
// View as, and the announcement to show (only to an active account).
type Me struct {
	Role           string        `json:"role"`
	AccessState    string        `json:"access_state"`
	ImpersonatedBy string        `json:"impersonated_by,omitempty"`
	CanImpersonate bool          `json:"can_impersonate"`
	Announcement   *Announcement `json:"announcement,omitempty"`
}

// MeOf returns the access part of /me for a session from CheckAllowPending.
func (b *Backoffice) MeOf(ctx context.Context, sess Session) (Me, error) {
	out := Me{
		Role:           sess.Role,
		AccessState:    apiAccess(sess.AccessState, sess.PendingSince),
		CanImpersonate: sess.ImpersonatorID == "" && b.IsSuperAdmin(sess.Role, sess.Email),
	}
	if sess.ImpersonatorID != "" {
		admin, err := b.userByID(ctx, b.cfg.DB, sess.ImpersonatorID)
		if err != nil {
			return Me{}, err
		}
		out.ImpersonatedBy = admin.Email
	}
	if out.AccessState == AccessActive {
		a, err := b.ActiveAnnouncement(ctx)
		if err != nil {
			return Me{}, err
		}
		out.Announcement = a
	}
	return out, nil
}
