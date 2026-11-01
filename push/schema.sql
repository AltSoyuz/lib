-- What package push reads and writes; copy into the app's migrations, after lib's auth tables.
-- One row per phone or browser that turned notifications on: its push service endpoint and the keys the payload is encrypted for.
CREATE TABLE IF NOT EXISTS push_subscriptions (
  endpoint   TEXT PRIMARY KEY NOT NULL,
  user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  p256dh     TEXT NOT NULL, -- base64url, the browser's P-256 public key
  auth       TEXT NOT NULL, -- base64url, its 16 byte secret
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS ix_push_subscriptions_user ON push_subscriptions(user_id);
