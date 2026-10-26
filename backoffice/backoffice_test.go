package backoffice

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AltSoyuz/lib/audit"
	"github.com/AltSoyuz/lib/auth/sessions"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schema string

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

type app struct {
	t   *testing.T
	db  *sql.DB
	bo  *Backoffice
	mux *http.ServeMux
}

const superAdmin = "boss@example.com"

func newApp(t *testing.T, approval bool) *app {
	t.Helper()
	db, err := sql.Open("sqlite3", "file::memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, s := range []string{
		readFile(t, "../auth/schema/migrations/001_users.sql"),
		readFile(t, "../auth/schema/migrations/002_sessions.sql"),
		readFile(t, "../audit/schema.sql"),
		schema,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	bo, err := New(Config{
		DB: db, Roles: []string{RoleAdmin, "user"}, MemberRole: "user", Approval: approval,
		SuperAdmins: map[string]struct{}{superAdmin: {}},
		Sessions:    sessions.SessionPolicy{TTL: 7 * 24 * time.Hour, RefreshThreshold: 24 * time.Hour, AbsoluteTTL: 30 * 24 * time.Hour},
	})
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	mux := http.NewServeMux()
	bo.Mount(mux)
	mux.HandleFunc("GET /check", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := bo.Check(w, r); ok {
			w.WriteHeader(http.StatusOK)
		}
	})
	mux.HandleFunc("POST /check", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := bo.Check(w, r); ok {
			w.WriteHeader(http.StatusOK)
		}
	})
	return &app{t: t, db: db, bo: bo, mux: mux}
}

// signIn admits email as a sign-in would and returns its session cookie and account.
func (a *app) signIn(email string) (*http.Cookie, User) {
	a.t.Helper()
	ctx := context.Background()
	token, hash, err := sessions.NewToken()
	if err != nil {
		a.t.Fatal(err)
	}
	var u User
	err = a.bo.inTx(ctx, func(tx *sql.Tx) error {
		var joined string
		var err error
		if u, joined, err = a.bo.Admit(ctx, tx, email, time.Now()); err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339)
		if _, err := tx.ExecContext(ctx, `INSERT INTO sessions (id, user_id, sid_hash, created_at, updated_at, last_seen_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			newID(), u.ID, hash, now, now, now, time.Now().Add(time.Hour).UTC().Format(time.RFC3339)); err != nil {
			return err
		}
		return a.bo.AuditSignIn(ctx, tx, u, joined, "test", "")
	})
	if err != nil {
		a.t.Fatalf("sign in %s: %v", email, err)
	}
	return &http.Cookie{Name: "sid", Value: token}, u
}

func (a *app) call(method, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	a.t.Helper()
	var r *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		r = httptest.NewRequest(method, path, strings.NewReader(string(b)))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	a.mux.ServeHTTP(w, r)
	return w
}

func errorOf(w *httptest.ResponseRecorder) string {
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	return e.Error
}

func (a *app) actions() []string {
	a.t.Helper()
	events, _, err := audit.List(context.Background(), a.db, audit.Filter{Limit: 100})
	if err != nil {
		a.t.Fatal(err)
	}
	var out []string
	for i := len(events) - 1; i >= 0; i-- {
		out = append(out, events[i].Action)
	}
	return out
}

func TestFirstAccountIsAnAdminThenOpenSignUpMakesActiveMembers(t *testing.T) {
	a := newApp(t, false)
	_, first := a.signIn("first@example.com")
	_, second := a.signIn("second@example.com")
	if first.Role != RoleAdmin || second.Role != "user" || second.AccessState != AccessActive {
		t.Fatalf("expected an admin then an active user, got %+v and %+v", first, second)
	}
	if got := strings.Join(a.actions(), ","); got != "signed_up,signed_in,signed_up,signed_in" {
		t.Fatalf("expected sign up and sign in rows, got %s", got)
	}
}

func TestApprovalKeepsNewAccountsPendingUntilAnAdminApproves(t *testing.T) {
	a := newApp(t, true)
	admin, _ := a.signIn(superAdmin)
	waiting, u := a.signIn("new@example.com")
	if u.AccessState != AccessPending {
		t.Fatalf("expected a pending account, got %+v", u)
	}
	if w := a.call("GET", "/check", nil, waiting); w.Code != http.StatusForbidden || errorOf(w) != "access_pending" {
		t.Fatalf("expected 403 access_pending, got %d body=%s", w.Code, w.Body)
	}
	if w := a.call("POST", "/api/v1/admin/users/"+u.ID+"/approve", nil, admin); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body)
	}
	if w := a.call("GET", "/check", nil, waiting); w.Code != http.StatusOK {
		t.Fatalf("expected the approved account through, got %d body=%s", w.Code, w.Body)
	}
	if w := a.call("POST", "/api/v1/admin/users/"+u.ID+"/approve", nil, admin); w.Code != http.StatusConflict {
		t.Fatalf("expected 409 not_pending on a second approval, got %d body=%s", w.Code, w.Body)
	}
}

func TestRestrainRevokesSessionsAndRefusesTheNextSignIn(t *testing.T) {
	a := newApp(t, false)
	admin, _ := a.signIn(superAdmin)
	member, u := a.signIn("member@example.com")
	if w := a.call("POST", "/api/v1/admin/users/"+u.ID+"/restrain", nil, admin); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body)
	}
	if w := a.call("GET", "/check", nil, member); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected the restrained session revoked, got %d", w.Code)
	}
	err := a.bo.inTx(context.Background(), func(tx *sql.Tx) error {
		_, _, err := a.bo.Admit(context.Background(), tx, "member@example.com", time.Now())
		return err
	})
	if err != ErrRestrained {
		t.Fatalf("expected ErrRestrained at the next sign-in, got %v", err)
	}
	if w := a.call("POST", "/api/v1/admin/users/"+u.ID+"/unrestrain", nil, admin); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body)
	}
	if again, _ := a.signIn("member@example.com"); a.call("GET", "/check", nil, again).Code != http.StatusOK {
		t.Fatal("expected a restored account to sign in again")
	}
}

func TestTheLastAdminAndTheSuperAdminsAreProtected(t *testing.T) {
	a := newApp(t, false)
	boss, bossUser := a.signIn(superAdmin)
	other, otherUser := a.signIn("other@example.com")
	if w := a.call("POST", "/api/v1/admin/users/"+otherUser.ID+"/role", map[string]string{"role": "admin"}, boss); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body)
	}
	if w := a.call("POST", "/api/v1/admin/users/"+bossUser.ID+"/role", map[string]string{"role": "user"}, other); w.Code != http.StatusForbidden || errorOf(w) != "super_admin_protected" {
		t.Fatalf("expected 403 super_admin_protected, got %d body=%s", w.Code, w.Body)
	}
	if w := a.call("POST", "/api/v1/admin/users/"+bossUser.ID+"/restrain", nil, other); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on restraining a super admin, got %d body=%s", w.Code, w.Body)
	}
	if w := a.call("POST", "/api/v1/admin/users/"+otherUser.ID+"/role", map[string]string{"role": "user"}, boss); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body)
	}
	if w := a.call("POST", "/api/v1/admin/users/"+bossUser.ID+"/role", map[string]string{"role": "user"}, boss); w.Code != http.StatusConflict || errorOf(w) != "cannot_modify_last_admin" {
		t.Fatalf("expected 409 cannot_modify_last_admin, got %d body=%s", w.Code, w.Body)
	}
	if w := a.call("POST", "/api/v1/admin/users/"+otherUser.ID+"/role", map[string]string{"role": "owner"}, boss); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 invalid_role, got %d", w.Code)
	}
	if w := a.call("GET", "/api/v1/admin/users", nil, other); w.Code != http.StatusForbidden {
		t.Fatalf("expected a member refused the admin list, got %d", w.Code)
	}
}

func TestViewAsIsReadOnlyAndStopRestoresTheAdminSession(t *testing.T) {
	a := newApp(t, false)
	boss, _ := a.signIn(superAdmin)
	_, member := a.signIn("member@example.com")
	w := a.call("POST", "/api/v1/admin/users/"+member.ID+"/impersonate", nil, boss)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body)
	}
	var viewAs, kept *http.Cookie
	for _, c := range w.Result().Cookies() {
		switch c.Name {
		case "sid":
			viewAs = c
		case "admin_sid":
			kept = c
		}
	}
	if viewAs == nil || kept == nil || kept.Value != boss.Value {
		t.Fatalf("expected a View as session and the admin's kept aside, got %v", w.Result().Cookies())
	}
	if w := a.call("GET", "/check", nil, viewAs); w.Code != http.StatusOK {
		t.Fatalf("expected a read through View as, got %d body=%s", w.Code, w.Body)
	}
	if w := a.call("POST", "/check", nil, viewAs); w.Code != http.StatusForbidden || errorOf(w) != "read_only_impersonation" {
		t.Fatalf("expected 403 read_only_impersonation on a write, got %d body=%s", w.Code, w.Body)
	}
	me, err := a.bo.MeOf(context.Background(), func() Session {
		sum, _ := sessions.HashFromB64(viewAs.Value)
		s, _, _ := a.bo.sessionByHash(context.Background(), sum, time.Now().UTC())
		return s
	}())
	if err != nil || me.ImpersonatedBy != superAdmin || me.CanImpersonate {
		t.Fatalf("expected /me to name the super admin, got %+v err=%v", me, err)
	}
	w = a.call("POST", "/api/v1/admin/impersonation/stop", nil, viewAs, kept)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"restored":true`) {
		t.Fatalf("expected the admin session restored, got %d body=%s", w.Code, w.Body)
	}
	if w := a.call("GET", "/check", nil, viewAs); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected the View as session revoked, got %d", w.Code)
	}
	if got := a.actions(); got[len(got)-2] != audit.ImpersonationStarted || got[len(got)-1] != audit.ImpersonationStopped {
		t.Fatalf("expected View as started then stopped in the audit log, got %v", got)
	}

	other, _ := a.signIn("admin2@example.com")
	a.call("POST", "/api/v1/admin/users/"+member.ID+"/role", map[string]string{"role": "user"}, boss)
	if _, err := a.db.Exec(`UPDATE users SET role = 'admin' WHERE email = 'admin2@example.com'`); err != nil {
		t.Fatal(err)
	}
	if w := a.call("POST", "/api/v1/admin/users/"+member.ID+"/impersonate", nil, other); w.Code != http.StatusForbidden {
		t.Fatalf("expected an admin who is not a super admin refused View as, got %d", w.Code)
	}
}

func TestAnnouncementShowsToActiveAccountsUntilCleared(t *testing.T) {
	a := newApp(t, false)
	boss, _ := a.signIn(superAdmin)
	if w := a.call("PUT", "/api/v1/admin/announcement", map[string]string{"text": "  ", "tone": "info"}, boss); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on an empty text, got %d", w.Code)
	}
	if w := a.call("PUT", "/api/v1/admin/announcement", map[string]string{"text": "Bus at 9", "tone": "warning"}, boss); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body)
	}
	if got, _ := a.bo.ActiveAnnouncement(context.Background()); got == nil || got.Text != "Bus at 9" {
		t.Fatalf("expected the announcement, got %+v", got)
	}
	if w := a.call("DELETE", "/api/v1/admin/announcement", nil, boss); w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if got, _ := a.bo.ActiveAnnouncement(context.Background()); got != nil {
		t.Fatalf("expected no announcement, got %+v", got)
	}
}

func TestAuditLogAndStatsNameAccountsByEmail(t *testing.T) {
	a := newApp(t, false)
	boss, _ := a.signIn(superAdmin)
	_, member := a.signIn("member@example.com")
	a.call("POST", "/api/v1/admin/users/"+member.ID+"/restrain", nil, boss)

	w := a.call("GET", "/api/v1/admin/audit?action=user_restrained", nil, boss)
	var page struct {
		Events  []LoggedEvent `json:"events"`
		Actions []string      `json:"actions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || len(page.Events) != 1 {
		t.Fatalf("expected one restrain event, got %s", w.Body)
	}
	if e := page.Events[0]; e.Actor != superAdmin || e.TargetLabel != "member@example.com" {
		t.Fatalf("expected emails for actor and target, got %+v", e)
	}
	if w := a.call("GET", "/api/v1/admin/audit?action=nope", nil, boss); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 invalid_action, got %d", w.Code)
	}

	w = a.call("GET", "/api/v1/admin/stats", nil, boss)
	var s Stats
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil || s.Total != 2 || s.Restrained != 1 || s.ByRole[RoleAdmin] != 1 || s.New7d != 2 {
		t.Fatalf("expected 2 accounts, 1 restrained, 1 admin, 2 new, got %s", w.Body)
	}

	w = a.call("GET", "/api/v1/admin/users", nil, boss)
	var list struct {
		Users []AdminUser `json:"users"`
		Roles []string    `json:"roles"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list.Users) != 2 || len(list.Roles) != 2 {
		t.Fatalf("expected 2 accounts and 2 roles, got %s", w.Body)
	}
}

func TestEndSessionRevokesAndAuditsTheSignOut(t *testing.T) {
	a := newApp(t, false)
	c, _ := a.signIn("member@example.com")
	r := httptest.NewRequest("POST", "/logout", nil)
	r.AddCookie(c)
	found, err := a.bo.EndSession(httptest.NewRecorder(), r)
	if err != nil || !found {
		t.Fatalf("expected the session ended, got found=%v err=%v", found, err)
	}
	if w := a.call("GET", "/check", nil, c); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected the session revoked, got %d", w.Code)
	}
	if got := a.actions(); got[len(got)-1] != audit.SignedOut {
		t.Fatalf("expected signed_out last, got %v", got)
	}
}
