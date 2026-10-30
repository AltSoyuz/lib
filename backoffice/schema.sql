-- What package backoffice reads and writes beside lib's auth tables (auth/schema) and audit_events (audit/schema.sql).
-- Copy into the app's migrations, with the app's own role list in the CHECK and its member role as the default.
ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user'));
ALTER TABLE users ADD COLUMN access_state TEXT NOT NULL DEFAULT 'active' CHECK (access_state IN ('active', 'restrained'));
-- Set while a new account waits for an admin (access_state stays restrained), NULL otherwise.
ALTER TABLE users ADD COLUMN pending_since TEXT;
CREATE INDEX IF NOT EXISTS ix_users_role ON users(role);
CREATE INDEX IF NOT EXISTS ix_users_access_state ON users(access_state);

-- The super admin who opened this session to view the app as its user, NULL for a normal session.
ALTER TABLE sessions ADD COLUMN impersonator_id TEXT REFERENCES users(id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS announcement (
  id         INTEGER PRIMARY KEY CHECK (id = 1),
  text       TEXT NOT NULL,
  tone       TEXT NOT NULL CHECK (tone IN ('info', 'warning')),
  active     INTEGER NOT NULL CHECK (active IN (0, 1)),
  updated_at TEXT NOT NULL, -- fixed width UTC: a phone keeps the one it closed, so each change gives a new value
  updated_by TEXT REFERENCES users(id) ON DELETE SET NULL
);

-- The audience an admin chose for a feature of Config.Features; a feature without a row is for everyone.
CREATE TABLE IF NOT EXISTS features (
  name       TEXT PRIMARY KEY,
  audience   TEXT NOT NULL CHECK (audience IN ('everyone', 'admins')),
  updated_at TEXT NOT NULL,
  updated_by TEXT REFERENCES users(id) ON DELETE SET NULL
);
