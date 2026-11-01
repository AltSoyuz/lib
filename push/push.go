// Package push sends Web Push notifications (RFC 8030, 8291 and 8292) to the phones and browsers that turned them on;
// it owns push_subscriptions (schema.sql) and calls the app's backoffice for the session. A notification is a nudge,
// never the only place something is said: the app shows the same thing.
package push

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AltSoyuz/lib/backoffice"
	"github.com/AltSoyuz/lib/httpserver"
	"github.com/AltSoyuz/lib/logger"
)

// Hosts are the push services a subscription may point at, by host or host suffix: the server posts to whatever
// endpoint a phone sends, so no other host is reached.
var Hosts = []string{".push.apple.com", "fcm.googleapis.com", ".push.services.mozilla.com", ".notify.windows.com"}

var (
	errOff         = errors.New("push_not_configured")
	errBadEndpoint = errors.New("invalid_endpoint")
	errBadKeys     = errors.New("invalid_keys")
	errNotReached  = errors.New("push_failed")
	errGone        = errors.New("subscription gone")
)

// Message is what the kit's service worker shows; Tag replaces an earlier notification with the same tag instead of stacking.
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag,omitempty"`
}

// Config is what New needs; the app copies schema.sql into its migrations.
type Config struct {
	DB         *sql.DB
	Backoffice *backoffice.Backoffice
	// Keys is the zero value when notifications are off.
	Keys Keys
	// Subject is the contact the push services may reach the sender at: a mailto: or https: URL.
	Subject string
	// Welcome is sent to a phone as it subscribes; the subscription is kept only once it went through.
	Welcome Message
}

// Service sends the notifications and serves the subscription routes.
type Service struct {
	cfg    Config
	Client *http.Client
	Hosts  []string
}

// New makes the service; Client and Hosts may be replaced before use (tests point them at a local push service).
func New(cfg Config) *Service {
	return &Service{cfg: cfg, Client: &http.Client{Timeout: 10 * time.Second}, Hosts: Hosts}
}

// On reports whether a VAPID key is set: without one, nothing is sent and subscriptions are refused.
func (svc *Service) On() bool { return svc.cfg.Keys.private != nil }

// Public returns the key the phones subscribe with, "" when notifications are off.
func (svc *Service) Public() string { return svc.cfg.Keys.Public }

// Mount adds GET /api/v1/push (the key) and PUT and DELETE /api/v1/push/subscription, behind the session check.
func (svc *Service) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/push", svc.handleKey)
	mux.HandleFunc("PUT /api/v1/push/subscription", svc.handleSubscribe)
	mux.HandleFunc("DELETE /api/v1/push/subscription", svc.handleUnsubscribe)
}

func (svc *Service) handleKey(w http.ResponseWriter, r *http.Request) {
	if _, ok := svc.cfg.Backoffice.Check(w, r); !ok {
		return
	}
	httpserver.WriteJSON(w, r, http.StatusOK, map[string]string{"key": svc.cfg.Keys.Public})
}

// subscription is PushSubscription.toJSON() as browsers send it; the decoder refuses unknown fields.
type subscription struct {
	Endpoint string `json:"endpoint"`
	// Unused: Apple, Google and Mozilla leave it null.
	ExpirationTime *int64 `json:"expirationTime"`
	Keys           struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

type phone struct{ endpoint, p256dh, auth string }

func (svc *Service) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	sess, ok := svc.cfg.Backoffice.Check(w, r)
	if !ok {
		return
	}
	if !svc.On() {
		httpserver.WriteError(w, r, http.StatusServiceUnavailable, errOff)
		return
	}
	in, err := httpserver.DecodeJSON[subscription](r)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, err)
		return
	}
	if !svc.allowed(in.Endpoint) {
		httpserver.WriteError(w, r, http.StatusBadRequest, errBadEndpoint)
		return
	}
	if !validKeys(in.Keys.P256dh, in.Keys.Auth) {
		httpserver.WriteError(w, r, http.StatusBadRequest, errBadKeys)
		return
	}
	p := phone{endpoint: in.Endpoint, p256dh: in.Keys.P256dh, auth: in.Keys.Auth}
	// An endpoint belongs to one phone: a new sign-in on it takes the subscription over.
	if _, err := svc.cfg.DB.ExecContext(r.Context(), `INSERT INTO push_subscriptions (endpoint, user_id, p256dh, auth, created_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (endpoint) DO UPDATE SET user_id = excluded.user_id, p256dh = excluded.p256dh, auth = excluded.auth, created_at = excluded.created_at`,
		p.endpoint, sess.UserID, p.p256dh, p.auth, time.Now().UTC().Format(time.RFC3339)); err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	if err := svc.sendTo(r.Context(), sess.UserID, p, svc.cfg.Welcome); err != nil {
		if err := svc.forget(context.WithoutCancel(r.Context()), p.endpoint); err != nil {
			logger.Error("push_subscription_delete", "user_id", sess.UserID, "err", err)
		}
		httpserver.WriteError(w, r, http.StatusBadGateway, errNotReached)
		return
	}
	logger.Info("push_subscribed", "user_id", sess.UserID, "host", host(p.endpoint))
	w.WriteHeader(http.StatusNoContent)
}

func (svc *Service) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	sess, ok := svc.cfg.Backoffice.Check(w, r)
	if !ok {
		return
	}
	in, err := httpserver.DecodeJSON[struct {
		Endpoint string `json:"endpoint"`
	}](r)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, err)
		return
	}
	res, err := svc.cfg.DB.ExecContext(r.Context(), `DELETE FROM push_subscriptions WHERE endpoint = ? AND user_id = ?`, in.Endpoint, sess.UserID)
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, fmt.Errorf("internal: %w", err))
		return
	}
	n, _ := res.RowsAffected()
	logger.Info("push_unsubscribed", "user_id", sess.UserID, "removed", n)
	w.WriteHeader(http.StatusNoContent)
}

// Send notifies every phone of a user and returns how many took the message; a failed phone is logged and skipped,
// a phone its push service forgot is deleted.
func (svc *Service) Send(ctx context.Context, userID string, m Message) (int, error) {
	if !svc.On() {
		return 0, nil
	}
	phones, err := svc.phones(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("push subscriptions of %s: %w", userID, err)
	}
	reached := 0
	for _, p := range phones {
		switch err := svc.sendTo(ctx, userID, p, m); {
		case err == nil:
			reached++
		case !errors.Is(err, errGone):
			logger.Warn("push_failed", "user_id", userID, "host", host(p.endpoint), "tag", m.Tag, "err", err)
		}
	}
	return reached, nil
}

func (svc *Service) phones(ctx context.Context, userID string) ([]phone, error) {
	rows, err := svc.cfg.DB.QueryContext(ctx, `SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = ? ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []phone
	for rows.Next() {
		var p phone
		if err := rows.Scan(&p.endpoint, &p.p256dh, &p.auth); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (svc *Service) forget(ctx context.Context, endpoint string) error {
	_, err := svc.cfg.DB.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	return err
}

// sendTo posts one message; a subscription the push service no longer knows (404, 410) is deleted.
func (svc *Service) sendTo(ctx context.Context, userID string, p phone, m Message) error {
	payload, err := json.Marshal(m)
	if err != nil {
		return err
	}
	body, err := encrypt(payload, p.p256dh, p.auth)
	if err != nil {
		return err
	}
	authz, err := svc.cfg.Keys.authorization(p.endpoint, svc.cfg.Subject, time.Now())
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", authz)
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	// A day old, a reminder is stale: the push service may drop it rather than wake the phone.
	req.Header.Set("TTL", "86400")
	req.Header.Set("Urgency", "normal")
	res, err := svc.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	said, _ := io.ReadAll(io.LimitReader(res.Body, 200))
	switch {
	case res.StatusCode >= 200 && res.StatusCode < 300:
		logger.Info("push_sent", "user_id", userID, "host", host(p.endpoint), "tag", m.Tag, "status", res.StatusCode)
		return nil
	case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone:
		logger.Info("push_gone", "user_id", userID, "host", host(p.endpoint), "status", res.StatusCode)
		if err := svc.forget(context.WithoutCancel(ctx), p.endpoint); err != nil {
			return fmt.Errorf("cannot delete a gone subscription: %w", err)
		}
		return errGone
	default:
		return fmt.Errorf("push service answered %d: %s", res.StatusCode, strings.TrimSpace(string(said)))
	}
}

func (svc *Service) allowed(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	h := u.Hostname()
	for _, allowed := range svc.Hosts {
		if h == allowed || (strings.HasPrefix(allowed, ".") && strings.HasSuffix(h, allowed)) {
			return true
		}
	}
	return false
}

func validKeys(p256dh, authSecret string) bool {
	raw, err := b64.DecodeString(p256dh)
	if err != nil {
		return false
	}
	if _, err := ecdh.P256().NewPublicKey(raw); err != nil {
		return false
	}
	secret, err := b64.DecodeString(authSecret)
	return err == nil && len(secret) == 16
}

// host is what the logs say of an endpoint: its path is a capability, enough to notify the phone.
func host(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
