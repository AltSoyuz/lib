-- oauth_accounts
CREATE TABLE IF NOT EXISTS oauth_accounts (
  id         TEXT PRIMARY KEY NOT NULL,
  user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider   TEXT NOT NULL,
  subject    TEXT NOT NULL,
  email      TEXT,
  created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_oauth_provider_subject ON oauth_accounts(provider, subject);
CREATE UNIQUE INDEX IF NOT EXISTS ux_oauth_user_provider ON oauth_accounts(user_id, provider);
