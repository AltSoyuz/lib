-- users

-- name: CreateUser :one
INSERT INTO users (id, email, created_at) VALUES (?, ?, ?) RETURNING id;

-- name: GetUserByEmail :one
SELECT id, email, email_verified_at, created_at FROM users WHERE email = ?;

-- name: GetUserByID :one
SELECT id, email, email_verified_at, created_at FROM users WHERE id = ?;

-- name: UpdateUserEmail :exec
UPDATE users SET email = ?, email_verified_at = NULL WHERE id = ?;

-- name: VerifyUserEmailByEmail :exec
UPDATE users SET email_verified_at = ? WHERE email = ?;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = ?;

-- password_credentials

-- name: CreatePasswordCredential :exec
INSERT INTO password_credentials (user_id, pass_hash, created_at, updated_at) VALUES (?, ?, ?, ?);

-- name: GetPasswordCredentialByUserID :one
SELECT user_id, pass_hash, created_at, updated_at FROM password_credentials WHERE user_id = ?;

-- name: UpdatePasswordCredential :exec
UPDATE password_credentials SET pass_hash = ?, updated_at = ? WHERE user_id = ?;

-- name: HasPasswordCredential :one
SELECT count(*) > 0 AS has_cred FROM password_credentials WHERE user_id = ?;

-- sessions

-- name: CreateSession :one
INSERT INTO sessions (id, user_id, sid_hash, user_agent, ip, created_at, updated_at, last_seen_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id;

-- name: GetSessionByHash :one
SELECT id, user_id, expires_at FROM sessions WHERE sid_hash = ? AND revoked_at IS NULL AND expires_at > ?;

-- name: GetUserIDBySessionHash :one
SELECT user_id FROM sessions WHERE sid_hash = ? AND revoked_at IS NULL AND expires_at > ?;

-- name: ListSessionsByUserID :many
SELECT id, user_id, user_agent, ip, created_at, updated_at, last_seen_at, expires_at, revoked_at FROM sessions WHERE user_id = ? AND revoked_at IS NULL ORDER BY created_at DESC LIMIT ? OFFSET ?;

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at = ? WHERE id = ? AND user_id = ?;

-- name: RevokeAllSessionsExcept :exec
UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND id != ? AND revoked_at IS NULL;

-- name: RevokeAllUserSessions :exec
UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL;

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = ?, updated_at = ? WHERE id = ?;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < ? OR revoked_at IS NOT NULL;

-- magic_links

-- name: CreateMagicLink :exec
INSERT INTO magic_links (id, email, token_hash, purpose, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?);

-- name: ConsumeMagicLink :one
SELECT id, email, token_hash, purpose, created_at, expires_at, consumed_at FROM magic_links WHERE token_hash = ? AND purpose = ? AND consumed_at IS NULL AND expires_at > ?;

-- name: MarkMagicLinkConsumed :exec
UPDATE magic_links SET consumed_at = ? WHERE id = ?;

-- name: InvalidateOpenMagicLinks :exec
UPDATE magic_links SET consumed_at = ? WHERE email = ? AND purpose = ? AND consumed_at IS NULL;

-- oauth_accounts

-- name: CreateOAuthAccount :one
INSERT INTO oauth_accounts (id, user_id, provider, subject, email, created_at) VALUES (?, ?, ?, ?, ?, ?) RETURNING id;

-- name: GetUserByOAuth :one
SELECT u.id, u.email, u.email_verified_at, u.created_at FROM users u JOIN oauth_accounts oa ON u.id = oa.user_id WHERE oa.provider = ? AND oa.subject = ?;

-- name: DeleteOAuthAccountByUser :exec
DELETE FROM oauth_accounts WHERE user_id = ? AND provider = ?;
