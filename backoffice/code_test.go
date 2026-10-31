package backoffice

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// mailbox keeps the codes the server would email; RequestCode sends them from a goroutine.
type mailbox struct {
	mu    sync.Mutex
	codes map[string][]string
}

func (m *mailbox) count(email string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.codes[email])
}

// codeApp is an app signing in with codes, its mail caught in a mailbox; every request comes from a new address so
// the per-IP limits stay out of the way unless a test sets ip.
type codeApp struct {
	*app
	mail *mailbox
	ips  int
}

func newCodeApp(t *testing.T, approval bool) *codeApp {
	a := &codeApp{app: newApp(t, approval), mail: &mailbox{codes: map[string][]string{}}}
	a.bo.codes.send = func(email, code string) {
		a.mail.mu.Lock()
		defer a.mail.mu.Unlock()
		a.mail.codes[email] = append(a.mail.codes[email], code)
	}
	a.mux.HandleFunc("POST /api/v1/sign-in/code", a.bo.RequestCode)
	a.mux.HandleFunc("POST /api/v1/sign-in/verify", a.bo.VerifyCode)
	a.mux.HandleFunc("GET /api/v1/me", a.bo.HandleMe)
	a.mux.HandleFunc("POST /api/v1/sign-out", a.bo.HandleSignOut)
	return a
}

func (a *codeApp) post(path string, body any, ip string) *httptest.ResponseRecorder {
	a.t.Helper()
	if ip == "" {
		a.ips++
		ip = fmt.Sprintf("198.51.100.%d", a.ips)
	}
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(b)))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = ip + ":1234"
	w := httptest.NewRecorder()
	a.mux.ServeHTTP(w, r)
	return w
}

// requestCode asks a code for email and returns the code mailed.
func (a *codeApp) requestCode(email string) string {
	a.t.Helper()
	n := a.mail.count(email)
	if w := a.post("/api/v1/sign-in/code", map[string]string{"email": email}, ""); w.Code != http.StatusNoContent {
		a.t.Fatalf("code request for %s: %d %s", email, w.Code, w.Body)
	}
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(time.Millisecond) {
		a.mail.mu.Lock()
		codes := a.mail.codes[email]
		a.mail.mu.Unlock()
		if len(codes) > n {
			return codes[n]
		}
	}
	a.t.Fatalf("no code mailed to %s", email)
	return ""
}

func (a *codeApp) verify(email, code string) *httptest.ResponseRecorder {
	return a.post("/api/v1/sign-in/verify", map[string]string{"email": email, "code": code}, "")
}

// openCode stores an open code for email, as RequestCode would.
func (a *codeApp) openCode(email, code string, ttl time.Duration) {
	a.t.Helper()
	now := time.Now().UTC()
	if _, err := a.db.Exec(`INSERT INTO magic_links (id, email, token_hash, purpose, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
		newID(), email, a.bo.codeHash(email, code), purposeLoginCode, now.Format(time.RFC3339), now.Add(ttl).Format(time.RFC3339)); err != nil {
		a.t.Fatal(err)
	}
}

// lastCode returns the attempts on the newest code row of email and whether it is still open.
func (a *codeApp) lastCode(email string) (int, bool) {
	a.t.Helper()
	var attempts int
	var open bool
	if err := a.db.QueryRow(`SELECT attempts, consumed_at IS NULL FROM magic_links WHERE email = ? ORDER BY rowid DESC LIMIT 1`, email).
		Scan(&attempts, &open); err != nil {
		a.t.Fatal(err)
	}
	return attempts, open
}

func status(t *testing.T, w *httptest.ResponseRecorder, want int, code string) {
	t.Helper()
	if w.Code != want || errorOf(w) != code {
		t.Fatalf("got %d %q, want %d %q: %s", w.Code, errorOf(w), want, code, w.Body)
	}
}

func sessionCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == "sid" && c.Value != "" {
			return c
		}
	}
	t.Fatalf("no session cookie: %v", w.Result().Cookies())
	return nil
}

func TestACodeSignsInOnce(t *testing.T) {
	a := newCodeApp(t, true)
	a.signIn("first@example.org")
	email := "new.member@example.org"

	code := a.requestCode(email)
	if !regexp.MustCompile(`^[0-9]{6}$`).MatchString(code) {
		t.Fatalf("code %q is not 6 digits", code)
	}
	var stored []byte
	if err := a.db.QueryRow(`SELECT token_hash FROM magic_links WHERE email = ?`, email).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stored), code) {
		t.Fatal("the code is stored in clear")
	}

	w := a.verify(email, code)
	status(t, w, http.StatusNoContent, "")
	var me Me
	if err := json.Unmarshal(a.call("GET", "/api/v1/me", nil, sessionCookie(t, w)).Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if me.Email != email || me.AccessState != AccessPending || me.UserID == "" {
		t.Fatalf("me after a new sign-up under approval: %+v", me)
	}
	status(t, a.verify(email, code), http.StatusBadRequest, "invalid_or_expired_code")

	var details string
	var verified bool
	if err := a.db.QueryRow(`SELECT e.details, u.email_verified_at IS NOT NULL FROM audit_events e JOIN users u ON u.id = e.actor_id
		WHERE e.action = 'signed_in' AND u.email = ?`, email).Scan(&details, &verified); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(details, `"via":"code"`) || strings.Contains(details, code) || !verified {
		t.Fatalf("signed_in details %s, email verified %v", details, verified)
	}
}

func TestALeadingZeroCodeWorks(t *testing.T) {
	a := newCodeApp(t, false)
	a.openCode("zero@example.org", "000731", time.Minute)
	status(t, a.verify("zero@example.org", "731"), http.StatusBadRequest, "invalid_or_expired_code")
	status(t, a.verify("zero@example.org", "000731"), http.StatusNoContent, "")

	digits := regexp.MustCompile(`^[0-9]{6}$`)
	zeros := 0
	for range 3000 {
		code, err := newCode()
		if err != nil {
			t.Fatal(err)
		}
		if !digits.MatchString(code) {
			t.Fatalf("code %q is not 6 digits", code)
		}
		if code[0] == '0' {
			zeros++
		}
	}
	// A tenth of the codes start with 0; none in 3000 draws would mean the zeros are dropped.
	if zeros == 0 {
		t.Fatal("no code starts with 0")
	}
}

func TestTheFifthWrongCodeBurnsTheCode(t *testing.T) {
	a := newCodeApp(t, false)
	email := "typo@example.org"
	code := a.requestCode(email)
	n, err := strconv.Atoi(code)
	if err != nil {
		t.Fatal(err)
	}
	wrong := fmt.Sprintf("%06d", (n+1)%1_000_000)

	for i := 1; i <= 4; i++ {
		status(t, a.verify(email, wrong), http.StatusBadRequest, "invalid_or_expired_code")
		if attempts, open := a.lastCode(email); attempts != i || !open {
			t.Fatalf("after %d wrong codes: attempts %d, open %v", i, attempts, open)
		}
	}
	status(t, a.verify(email, wrong), http.StatusBadRequest, "too_many_attempts")
	if attempts, open := a.lastCode(email); attempts != 5 || open {
		t.Fatalf("after 5 wrong codes: attempts %d, open %v", attempts, open)
	}
	status(t, a.verify(email, code), http.StatusBadRequest, "invalid_or_expired_code")
	status(t, a.verify(email, wrong), http.StatusBadRequest, "invalid_or_expired_code")
	status(t, a.verify(email, a.requestCode(email)), http.StatusNoContent, "")
}

func TestACodeWorksForItsEmailOnly(t *testing.T) {
	a := newCodeApp(t, false)
	a.openCode("a@example.org", "111111", time.Minute)
	a.openCode("b@example.org", "222222", time.Minute)

	status(t, a.verify("b@example.org", "111111"), http.StatusBadRequest, "invalid_or_expired_code")
	status(t, a.verify("c@example.org", "111111"), http.StatusBadRequest, "invalid_or_expired_code")
	if attempts, _ := a.lastCode("a@example.org"); attempts != 0 {
		t.Fatalf("a try on another address counted on a@: %d", attempts)
	}
	status(t, a.verify("A@Example.org", "111111"), http.StatusNoContent, "")
	status(t, a.verify("b@example.org", "222222"), http.StatusNoContent, "")
}

func TestAnExpiredCodeFails(t *testing.T) {
	a := newCodeApp(t, false)
	email := "late@example.org"
	code := a.requestCode(email)
	past := time.Now().UTC().Add(-time.Second).Format(time.RFC3339)
	if _, err := a.db.Exec(`UPDATE magic_links SET expires_at = ? WHERE email = ?`, past, email); err != nil {
		t.Fatal(err)
	}
	status(t, a.verify(email, code), http.StatusBadRequest, "invalid_or_expired_code")
}

func TestANewRequestVoidsThePreviousCode(t *testing.T) {
	a := newCodeApp(t, false)
	email := "twice@example.org"
	first := a.requestCode(email)
	second := a.requestCode(email)

	var open, all int
	if err := a.db.QueryRow(`SELECT count(*) FILTER (WHERE consumed_at IS NULL), count(*) FROM magic_links WHERE email = ?`, email).Scan(&open, &all); err != nil {
		t.Fatal(err)
	}
	if open != 1 || all != 2 {
		t.Fatalf("open codes %d of %d", open, all)
	}
	// Both draws give the same code once in a million.
	if first != second {
		status(t, a.verify(email, first), http.StatusBadRequest, "invalid_or_expired_code")
	}
	status(t, a.verify(email, second), http.StatusNoContent, "")
}

func TestCodeRequestsPerEmailAreCapped(t *testing.T) {
	a := newCodeApp(t, false)
	email := "flood@example.org"
	request := func(email string) *httptest.ResponseRecorder {
		return a.post("/api/v1/sign-in/code", map[string]string{"email": email}, "")
	}

	var last string
	for range 5 {
		last = a.requestCode(email)
	}
	status(t, request(email), http.StatusTooManyRequests, "too_many_codes")
	status(t, request("Flood@Example.org"), http.StatusTooManyRequests, "too_many_codes")
	if got := a.mail.count(email); got != 5 {
		t.Fatalf("codes sent: %d", got)
	}
	a.requestCode("other@example.org")

	// Over the limit nothing changes: the latest code still signs in, and that sign-in starts the count over.
	status(t, a.verify(email, last), http.StatusNoContent, "")
	a.requestCode(email)

	// A restrained account gets no code, but the limit answers the same, so it says nothing about the account.
	a.signIn("gone@example.org")
	if _, err := a.db.Exec(`UPDATE users SET access_state = 'restrained' WHERE email = 'gone@example.org'`); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		status(t, request("gone@example.org"), http.StatusNoContent, "")
	}
	status(t, request("gone@example.org"), http.StatusTooManyRequests, "too_many_codes")
	if got := a.mail.count("gone@example.org"); got != 0 {
		t.Fatalf("codes sent to a restrained account: %d", got)
	}
}

func TestRequestsPerAddressAreCapped(t *testing.T) {
	a := newCodeApp(t, false)
	for i := range 5 {
		status(t, a.post("/api/v1/sign-in/code", map[string]string{"email": fmt.Sprintf("m%d@example.org", i)}, "203.0.113.9"), http.StatusNoContent, "")
	}
	status(t, a.post("/api/v1/sign-in/code", map[string]string{"email": "m5@example.org"}, "203.0.113.9"), http.StatusTooManyRequests, "rate_limit_exceeded")
}

func TestAMalformedCodeTouchesNothing(t *testing.T) {
	a := newCodeApp(t, false)
	a.openCode("old@example.org", "123456", time.Minute)
	for _, code := range []string{"", "12345", "1234567", "12a456", " 123456", "１２３４５６"} {
		status(t, a.verify("old@example.org", code), http.StatusBadRequest, "invalid_or_expired_code")
	}
	if attempts, open := a.lastCode("old@example.org"); attempts != 0 || !open {
		t.Fatalf("malformed codes touched the code: attempts %d, open %v", attempts, open)
	}
	status(t, a.verify("old@example.org", "123456"), http.StatusNoContent, "")
}

func TestARestrainedAccountCannotSignInWithACode(t *testing.T) {
	a := newCodeApp(t, false)
	a.signIn("first@example.org")
	a.signIn("gone@example.org")
	if _, err := a.db.Exec(`UPDATE users SET access_state = 'restrained' WHERE email = 'gone@example.org'`); err != nil {
		t.Fatal(err)
	}
	a.openCode("gone@example.org", "424242", time.Minute)
	w := a.verify("gone@example.org", "424242")
	status(t, w, http.StatusForbidden, "user_restrained")
	if len(w.Result().Cookies()) != 0 {
		t.Fatalf("a restrained account got a cookie: %v", w.Result().Cookies())
	}
	if _, open := a.lastCode("gone@example.org"); !open {
		t.Fatal("a refused sign-in closed the code")
	}
}

func TestSignOutEndsTheSessionOfTheCookie(t *testing.T) {
	a := newCodeApp(t, false)
	a.openCode("me@example.org", "135790", time.Minute)
	cookie := sessionCookie(t, a.verify("me@example.org", "135790"))
	status(t, a.call("GET", "/api/v1/me", nil, cookie), http.StatusOK, "")
	status(t, a.call("POST", "/api/v1/sign-out", nil, cookie), http.StatusNoContent, "")
	status(t, a.call("GET", "/api/v1/me", nil, cookie), http.StatusUnauthorized, "unauthorized")
	status(t, a.call("POST", "/api/v1/sign-out", nil, cookie), http.StatusUnauthorized, "unauthorized")
}

func TestTheEmailCarriesTheCodeAndNoLink(t *testing.T) {
	subject, text, html := englishCodeEmail("Explore")("004217", CodeTTL)
	if subject != "Your Explore sign-in code: 004217" {
		t.Errorf("subject %q", subject)
	}
	for name, body := range map[string]string{"text": text, "html": html} {
		for _, want := range []string{"004217", "expires in 15 minutes", "If you did not ask to sign in, ignore this email."} {
			if !strings.Contains(body, want) {
				t.Errorf("%s lacks %q:\n%s", name, want, body)
			}
		}
	}
	for name, part := range map[string]string{"subject": subject, "text": text, "html": html} {
		for _, banned := range []string{"http", "://", "www.", "href", "/auth", "token", "link"} {
			if strings.Contains(strings.ToLower(part), banned) {
				t.Errorf("%s holds %q:\n%s", name, banned, part)
			}
		}
	}
}
