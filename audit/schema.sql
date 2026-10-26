-- audit_events: who changed what in an app. Copy into the app's migrations; an app that has it already keeps its own.
CREATE TABLE IF NOT EXISTS audit_events (
  id       INTEGER PRIMARY KEY,
  at       TEXT NOT NULL, -- UTC, fixed width (2006-01-02T15:04:05.000Z) so rows compare as strings
  actor_id TEXT,          -- NULL for the system
  action   TEXT NOT NULL, -- stable code: audit.AccountActions and the app's own
  target   TEXT NOT NULL DEFAULT '',
  details  TEXT NOT NULL DEFAULT '{}' -- short JSON object, never a secret, a token or what someone wrote
);
CREATE INDEX IF NOT EXISTS ix_audit_events_action ON audit_events(action, id);
CREATE INDEX IF NOT EXISTS ix_audit_events_actor ON audit_events(actor_id, id);
