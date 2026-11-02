-- name: GetUserByID :one
SELECT u.id, u.email, u.display_name, u.onboarding_completed_at,
       u.email_notifications_enabled, u.is_verified_educator,
       u.created_at, u.updated_at,
       COALESCE(array_agg(r.role ORDER BY r.granted_at) FILTER (WHERE r.role IS NOT NULL), '{}')::text[] AS roles
FROM users u
LEFT JOIN user_roles r ON r.user_id = u.id AND r.revoked_at IS NULL
WHERE u.id = $1 AND u.deleted_at IS NULL
GROUP BY u.id;

-- name: CreateUser :one
INSERT INTO users (id, email, display_name)
VALUES ($1, $2, $3)
RETURNING id, email, display_name, onboarding_completed_at, email_notifications_enabled, is_verified_educator, deleted_at, created_at, updated_at;

-- name: GrantUserRole :one
INSERT INTO user_roles (user_id, role, guidelines_accepted_at, granted_by)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, role, guidelines_accepted_at, granted_by, granted_at, revoked_by, revoked_at, revocation_reason;

-- name: RevokeUserRole :one
UPDATE user_roles
SET revoked_by = $3, revoked_at = NOW(), revocation_reason = $4
WHERE user_id = $1 AND role = $2 AND revoked_at IS NULL
RETURNING id, user_id, role, guidelines_accepted_at, granted_by, granted_at, revoked_by, revoked_at, revocation_reason;

-- name: InsertAuditEvent :one
INSERT INTO audit_events (actor_user_id, event_type, target_type, target_id, reason, metadata_json, request_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, actor_user_id, event_type, target_type, target_id, reason, metadata_json, request_id, created_at;
