-- users
CREATE TABLE IF NOT EXISTS users (
  id                TEXT PRIMARY KEY NOT NULL,
  email             TEXT NOT NULL,
  email_verified_at TEXT,
  created_at        TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_users_email ON users(email);

-- password_credentials
CREATE TABLE IF NOT EXISTS password_credentials (
  user_id    TEXT PRIMARY KEY NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  pass_hash  TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
