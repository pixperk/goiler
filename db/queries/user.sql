-- name: CreateUser :exec
INSERT INTO users (id, email, name, password_hash, role)
VALUES ($1, $2, $3, $4, $5);

-- name: GetUserByID :one
SELECT id, email, name, password_hash, role, email_verified_at, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, name, password_hash, role, email_verified_at, created_at, updated_at
FROM users
WHERE email = $1;

-- name: UpdateUser :exec
UPDATE users
SET email = $2, name = $3, password_hash = $4
WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2
WHERE id = $1;

-- name: UpdateUserEmail :exec
UPDATE users
SET email = $2
WHERE id = $1;

-- name: VerifyUserEmail :exec
UPDATE users
SET email_verified_at = NOW()
WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;

-- name: ListUsers :many
SELECT id, email, name, password_hash, role, email_verified_at, created_at, updated_at
FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: UserExists :one
SELECT EXISTS(SELECT 1 FROM users WHERE email = $1);

-- Refresh token queries

-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4);

-- name: GetRefreshToken :one
SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
FROM refresh_tokens
WHERE id = $1 AND revoked_at IS NULL AND expires_at > NOW();

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = NOW()
WHERE id = $1;

-- name: RevokeAllUserRefreshTokens :exec
UPDATE refresh_tokens
SET revoked_at = NOW()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteExpiredRefreshTokens :exec
DELETE FROM refresh_tokens
WHERE expires_at < NOW() OR revoked_at IS NOT NULL;

-- Session queries

-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, token_hash, user_agent, ip_address, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetSessionByToken :one
SELECT id, user_id, token_hash, user_agent, ip_address, expires_at, created_at
FROM sessions
WHERE token_hash = $1 AND expires_at > NOW();

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE id = $1;

-- name: DeleteUserSessions :exec
DELETE FROM sessions
WHERE user_id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at < NOW();

-- Audit log queries

-- name: CreateAuditLog :exec
INSERT INTO audit_logs (id, user_id, action, entity_type, entity_id, old_values, new_values, ip_address, user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: GetAuditLogs :many
SELECT id, user_id, action, entity_type, entity_id, old_values, new_values, ip_address, user_agent, created_at
FROM audit_logs
WHERE ($1::uuid IS NULL OR user_id = $1)
  AND ($2::varchar IS NULL OR entity_type = $2)
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;
