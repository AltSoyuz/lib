package push

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AltSoyuz/lib/auth/sessions"
	"github.com/AltSoyuz/lib/backoffice"
	_ "github.com/mattn/go-sqlite3"
)

// phone is a browser's subscription keys; open reads a body the way it would.
type phoneKeys struct {
	key    *ecdh.PrivateKey
	secret []byte
}

func newPhone(t *testing.T) phoneKeys {
	t.Helper()
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 16)
	rand.Read(secret)
	return phoneKeys{key: k, secret: secret}
}

func (p phoneKeys) keys() map[string]string {
	return map[string]string{"p256dh": b64.EncodeToString(p.key.PublicKey().Bytes()), "auth": b64.EncodeToString(p.secret)}
}

func (p phoneKeys) open(t *testing.T, body []byte) Message {
	t.Helper()
	salt, rs, idlen := body[:16], binary.BigEndian.Uint32(body[16:20]), int(body[20])
	asRaw := body[21 : 21+idlen]
	if rs != recordSize || idlen != 65 {
		t.Fatalf("header rs=%d idlen=%d", rs, idlen)
	}
	as, err := ecdh.P256().NewPublicKey(asRaw)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := p.key.ECDH(as)
	if err != nil {
		t.Fatal(err)
	}
	uaRaw := p.key.PublicKey().Bytes()
	ikm := expand(extract(p.secret, shared), append(append([]byte("WebPush: info\x00"), uaRaw...), asRaw...), 32)
	prk := extract(salt, ikm)
	block, _ := aes.NewCipher(expand(prk, []byte("Content-Encoding: aes128gcm\x00"), 16))
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, expand(prk, []byte("Content-Encoding: nonce\x00"), 12), body[21+idlen:], nil)
	if err != nil || plain[len(plain)-1] != 0x02 {
		t.Fatalf("cannot open the body: %v", err)
	}
	var m Message
	if err := json.Unmarshal(plain[:len(plain)-1], &m); err != nil {
		t.Fatal(err)
	}
	return m
}

type received struct {
	path   string
	header http.Header
	body   []byte
}

// pushService answers each path with its status, 201 by default, and keeps what it got.
type pushService struct {
	mu     sync.Mutex
	got    []received
	status map[string]int
}

func (s *pushService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.got = append(s.got, received{path: r.URL.Path, header: r.Header.Clone(), body: body})
	if code := s.status[r.URL.Path]; code != 0 {
		w.WriteHeader(code)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

type harness struct {
	t    *testing.T
	db   *sql.DB
	mux  *http.ServeMux
	svc  *Service
	push *pushService
	base string
	ids  map[string]string
}

func newHarness(t *testing.T, on bool) *harness {
	db, err := sql.Open("sqlite3", "file::memory:?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	for _, path := range []string{
		"../auth/schema/migrations/001_users.sql",
		"../auth/schema/migrations/002_sessions.sql",
		"../backoffice/schema.sql",
		"schema.sql",
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			t.Fatalf("schema %s: %v", path, err)
		}
	}
	bo, err := backoffice.New(backoffice.Config{
		DB: db, Roles: []string{backoffice.RoleAdmin, "user"}, MemberRole: "user",
		Sessions: sessions.SessionPolicy{TTL: time.Hour, RefreshThreshold: time.Minute, AbsoluteTTL: 24 * time.Hour},
	})
	if err != nil {
		t.Fatal(err)
	}
	ps := &pushService{status: map[string]int{}}
	ts := httptest.NewTLSServer(ps)
	t.Cleanup(ts.Close)
	var keys Keys
	if on {
		if keys, _, err = NewKeys(); err != nil {
			t.Fatal(err)
		}
	}
	u, _ := url.Parse(ts.URL)
	svc := New(Config{DB: db, Backoffice: bo, Keys: keys, Subject: "mailto:admin@example.org",
		Welcome: Message{Title: "Notifications on", Body: "The app tells you here when something needs you.", URL: "/", Tag: "push-on"}})
	svc.Client, svc.Hosts = ts.Client(), []string{u.Hostname()}
	h := &harness{t: t, db: db, mux: http.NewServeMux(), svc: svc, push: ps, base: ts.URL, ids: map[string]string{}}
	svc.Mount(h.mux)
	for i, email := range []string{"marie@example.org", "lea@example.org"} {
		id := []string{"u-marie", "u-lea"}[i]
		if _, err := db.Exec(`INSERT INTO users (id, email, created_at) VALUES (?, ?, ?)`, id, email, time.Now().UTC().Format(time.RFC3339)); err != nil {
			t.Fatal(err)
		}
		h.ids[email] = id
	}
	return h
}

func (h *harness) login(email string) *http.Cookie {
	h.t.Helper()
	now := time.Now().UTC()
	tok, hash, err := sessions.NewToken()
	if err != nil {
		h.t.Fatal(err)
	}
	if _, err := h.db.Exec(`INSERT INTO sessions (id, user_id, sid_hash, created_at, updated_at, last_seen_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"s-"+email, h.ids[email], hash, now.Format(time.RFC3339), now.Format(time.RFC3339), now.Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)); err != nil {
		h.t.Fatal(err)
	}
	return &http.Cookie{Name: "sid", Value: tok}
}

func (h *harness) call(c *http.Cookie, method, path string, body any) *httptest.ResponseRecorder {
	h.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			h.t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec
}

func (h *harness) subscribe(c *http.Cookie, path string, p phoneKeys) *httptest.ResponseRecorder {
	h.t.Helper()
	// The shape of PushSubscription.toJSON() in Safari and Chrome, expirationTime included.
	return h.call(c, "PUT", "/api/v1/push/subscription", map[string]any{"endpoint": h.base + path, "expirationTime": nil, "keys": p.keys()})
}

func (h *harness) endpoints(email string) []string {
	h.t.Helper()
	phones, err := h.svc.phones(context.Background(), h.ids[email])
	if err != nil {
		h.t.Fatal(err)
	}
	var out []string
	for _, p := range phones {
		out = append(out, strings.TrimPrefix(p.endpoint, h.base))
	}
	return out
}

func status(t *testing.T, rec *httptest.ResponseRecorder, want int, code string) {
	t.Helper()
	if rec.Code != want || (code != "" && !strings.Contains(rec.Body.String(), code)) {
		t.Fatalf("status %d %s, want %d %s", rec.Code, rec.Body.String(), want, code)
	}
}

func TestOffGivesNoKeyAndRefusesSubscriptions(t *testing.T) {
	h := newHarness(t, false)
	marie := h.login("marie@example.org")
	rec := h.call(marie, "GET", "/api/v1/push", nil)
	status(t, rec, http.StatusOK, `"key":""`)
	status(t, h.subscribe(marie, "/a", newPhone(t)), http.StatusServiceUnavailable, "push_not_configured")
	if n, err := h.svc.Send(context.Background(), h.ids["marie@example.org"], Message{Title: "x"}); n != 0 || err != nil {
		t.Fatalf("Send while off = %d, %v", n, err)
	}
	if len(h.push.got) != 0 {
		t.Fatalf("sent while off: %d", len(h.push.got))
	}
}

func TestSubscribeSendsAFirstNotificationTheBrowserCanRead(t *testing.T) {
	h := newHarness(t, true)
	marie := h.login("marie@example.org")
	status(t, h.call(marie, "GET", "/api/v1/push", nil), http.StatusOK, h.svc.Public())

	p := newPhone(t)
	status(t, h.subscribe(marie, "/phone-1", p), http.StatusNoContent, "")
	if got := h.endpoints("marie@example.org"); len(got) != 1 || got[0] != "/phone-1" {
		t.Fatalf("subscriptions = %v", got)
	}
	r := h.push.got[0]
	if r.header.Get("Content-Encoding") != "aes128gcm" || r.header.Get("TTL") != "86400" {
		t.Fatalf("headers = %v", r.header)
	}
	jwt, key, _ := strings.Cut(strings.TrimPrefix(r.header.Get("Authorization"), "vapid t="), ", k=")
	if key != h.svc.Public() || !verifyJWT(key, jwt) {
		t.Fatalf("authorization = %q", r.header.Get("Authorization"))
	}
	if m := p.open(t, r.body); m.Title != "Notifications on" || m.URL != "/" {
		t.Fatalf("first message = %+v", m)
	}
}

func TestSubscribeRefusals(t *testing.T) {
	h := newHarness(t, true)
	marie := h.login("marie@example.org")
	p := newPhone(t)
	for _, endpoint := range []string{
		"http://" + strings.TrimPrefix(h.base, "https://") + "/x", // not https
		"https://169.254.169.254/latest/meta-data",                // not a push service
		"https://fcm.googleapis.com.evil.example/x",
		"https://user@" + strings.TrimPrefix(h.base, "https://") + "/x",
	} {
		rec := h.call(marie, "PUT", "/api/v1/push/subscription", map[string]any{"endpoint": endpoint, "keys": p.keys()})
		status(t, rec, http.StatusBadRequest, "invalid_endpoint")
	}
	bad := p.keys()
	bad["auth"] = "c2hvcnQ"
	rec := h.call(marie, "PUT", "/api/v1/push/subscription", map[string]any{"endpoint": h.base + "/x", "keys": bad})
	status(t, rec, http.StatusBadRequest, "invalid_keys")

	// A push service that already forgot the phone: nothing is kept, the phone hears it failed.
	h.push.status["/forgotten"] = http.StatusGone
	status(t, h.subscribe(marie, "/forgotten", p), http.StatusBadGateway, "push_failed")
	h.push.status["/down"] = http.StatusServiceUnavailable
	status(t, h.subscribe(marie, "/down", p), http.StatusBadGateway, "push_failed")
	if got := h.endpoints("marie@example.org"); len(got) != 0 {
		t.Fatalf("subscriptions kept = %v", got)
	}
	if len(h.push.got) != 2 {
		t.Fatalf("the refused endpoints were posted to: %d requests", len(h.push.got))
	}
}

func TestSendReachesEveryPhoneAndForgetsTheGoneOnes(t *testing.T) {
	h := newHarness(t, true)
	marie, lea := h.login("marie@example.org"), h.login("lea@example.org")
	phones := map[string]phoneKeys{"/phone": newPhone(t), "/tablet": newPhone(t), "/lea": newPhone(t)}
	status(t, h.subscribe(marie, "/phone", phones["/phone"]), http.StatusNoContent, "")
	status(t, h.subscribe(marie, "/tablet", phones["/tablet"]), http.StatusNoContent, "")
	status(t, h.subscribe(lea, "/lea", phones["/lea"]), http.StatusNoContent, "")
	h.push.got = nil

	h.push.status["/tablet"] = http.StatusGone
	m := Message{Title: "Possible duplicate", Body: "Two of your notes look alike.", URL: "/accounts", Tag: "duplicates"}
	n, err := h.svc.Send(context.Background(), h.ids["marie@example.org"], m)
	if n != 1 || err != nil {
		t.Fatalf("Send = %d, %v; want the phone only", n, err)
	}
	if len(h.push.got) != 2 {
		t.Fatalf("requests = %d, want Marie's two phones", len(h.push.got))
	}
	for _, r := range h.push.got {
		if got := phones[r.path].open(t, r.body); got != m {
			t.Fatalf("%s read %+v", r.path, got)
		}
	}
	if got := h.endpoints("marie@example.org"); len(got) != 1 || got[0] != "/phone" {
		t.Fatalf("Marie's subscriptions after a 410 = %v", got)
	}
}

func TestUnsubscribeTakesOnlyYourOwn(t *testing.T) {
	h := newHarness(t, true)
	marie, lea := h.login("marie@example.org"), h.login("lea@example.org")
	status(t, h.subscribe(marie, "/phone", newPhone(t)), http.StatusNoContent, "")
	status(t, h.call(lea, "DELETE", "/api/v1/push/subscription", map[string]string{"endpoint": h.base + "/phone"}), http.StatusNoContent, "")
	if got := h.endpoints("marie@example.org"); len(got) != 1 {
		t.Fatalf("Lea removed Marie's subscription: %v", got)
	}
	// Lea signs in on Marie's phone: the phone's subscription follows the new login.
	status(t, h.subscribe(lea, "/phone", newPhone(t)), http.StatusNoContent, "")
	if m, l := h.endpoints("marie@example.org"), h.endpoints("lea@example.org"); len(m) != 0 || len(l) != 1 {
		t.Fatalf("after the takeover Marie %v, Lea %v", m, l)
	}
	status(t, h.call(lea, "DELETE", "/api/v1/push/subscription", map[string]string{"endpoint": h.base + "/phone"}), http.StatusNoContent, "")
	if got := h.endpoints("lea@example.org"); len(got) != 0 {
		t.Fatalf("Lea's subscription stayed: %v", got)
	}
}
