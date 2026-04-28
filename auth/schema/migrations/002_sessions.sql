-- sessions
CREATE TABLE IF NOT EXISTS sessions (
  id           TEXT PRIMARY KEY NOT NULL,
  user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  sid_hash     BLOB NOT NULL,
  user_agent   TEXT NOT NULL DEFAULT '',
  ip           TEXT NOT NULL DEFAULT '',
  created_at   TEXT NOT NULL,
  updated_at   TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  expires_at   TEXT NOT NULL,
  revoked_at   TEXT
);
CREATE INDEX IF NOT EXISTS ix_sessions_user_id ON sessions(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_sessions_sid_hash ON sessions(sid_hash);

-- magic_links (used for verify_email and reset_password)
CREATE TABLE IF NOT EXISTS magic_links (
  id          TEXT PRIMARY KEY NOT NULL,
  email       TEXT NOT NULL,
  token_hash  BLOB NOT NULL,
  purpose     TEXT NOT NULL,
  created_at  TEXT NOT NULL,
  expires_at  TEXT NOT NULL,
  consumed_at TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_magic_links_token_open ON magic_links(token_hash) WHERE consumed_at IS NULL;
CREATE INDEX IF NOT EXISTS ix_magic_links_email_purpose ON magic_links(email, purpose) WHERE consumed_at IS NULL;
