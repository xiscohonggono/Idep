-- +goose Up
SET lock_timeout = '3s';
SET statement_timeout = '30s';

-- 1. Core Users Table (synchronized with Supabase auth.users id)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL CHECK (char_length(btrim(display_name)) BETWEEN 1 AND 100),
    onboarding_completed_at TIMESTAMPTZ,
    email_notifications_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    is_verified_educator BOOLEAN NOT NULL DEFAULT FALSE,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Composable Multi-Role Grants & Revocations (E-01, C-09, C-10)
CREATE TABLE user_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('user', 'editor', 'checker', 'admin')),
    guidelines_accepted_at TIMESTAMPTZ,
    granted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_by UUID REFERENCES users(id) ON DELETE SET NULL,
    revoked_at TIMESTAMPTZ,
    revocation_reason TEXT,
    CONSTRAINT chk_editor_guidelines CHECK (
        role <> 'editor' OR guidelines_accepted_at IS NOT NULL
    ),
    CONSTRAINT chk_revocation_reason CHECK (
        revoked_at IS NULL OR (revocation_reason IS NOT NULL AND char_length(btrim(revocation_reason)) > 0)
    )
);

-- Enforce at most one active grant per (user_id, role)
CREATE UNIQUE INDEX idx_user_roles_active_unique
    ON user_roles (user_id, role)
    WHERE revoked_at IS NULL;

-- 3. Append-Only Operational & Governance Audit Log
CREATE TABLE audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id UUID NOT NULL,
    reason TEXT,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_events_target
    ON audit_events (target_type, target_id, created_at DESC);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_immutable_table_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Table % is append-only; UPDATE and DELETE are prohibited', TG_TABLE_NAME;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_audit_events_immutable
    BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION prevent_immutable_table_mutation();

-- +goose Down
SET lock_timeout = '3s';
SET statement_timeout = '30s';

DROP TRIGGER IF EXISTS trg_audit_events_immutable ON audit_events;
DROP FUNCTION IF EXISTS prevent_immutable_table_mutation();
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS users;
