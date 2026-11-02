STATUS: APPROVED

# Architectural & Code Review — Stage 0: Identity & RBAC Foundation

> **Plan Reviewed:** [`docs/plans/Implemented/stage-0-identity-and-rbac.md`](../plans/Implemented/stage-0-identity-and-rbac.md) (Revision 2)  
> **Verification Evidence:** [`docs/verification/stage-0-identity-and-rbac.md`](../verification/stage-0-identity-and-rbac.md)  
> **Reviewer:** Gemini (Architect & Reviewer)  
> **Date:** 2026-11-02

---

## 1. Executive Summary (`ponytail-review` + Guardrails Audit)

**What this change does:** Replaces the bootstrap `001_users.sql` (`id TEXT`) with the Stage 0 identity, composable multi-role RBAC, and append-only audit schema (`001_identity_and_rbac.sql`), adds a `pressly/goose/v3` migration CLI (`cmd/migrate`) and per-test schema isolation helper (`internal/dbtest`), updates the `GET /api/v1/users/{id}` Huma → OpenAPI 3.1 → Next.js contract to use native `uuid.UUID` and `roles`, and adds a PostgreSQL 16 service container to CI.

All 8 acceptance criteria (**AC1–AC8**) and all guardrails in [`.agent/rules/project_guardrails.md`](../../.agent/rules/project_guardrails.md) pass both offline and against a live PostgreSQL 16.15 instance:

- **Schema & Invariants ([`apps/api/sql/schema/001_identity_and_rbac.sql`](../../apps/api/sql/schema/001_identity_and_rbac.sql)):** Enforces `SET lock_timeout = '3s'; SET statement_timeout = '30s';` in both `-- +goose Up` and `-- +goose Down`, `chk_editor_guidelines`, `chk_revocation_reason`, `idx_user_roles_active_unique`, and `trg_audit_events_immutable` via `prevent_immutable_table_mutation()`.
- **Compiled SQL & Native Type Overrides ([`apps/api/sqlc.yaml`](../../apps/api/sqlc.yaml), [`apps/api/sql/queries/users.sql`](../../apps/api/sql/queries/users.sql)):** `sqlc v1.31.1` compiles `GetUserByID` (single-query `LEFT JOIN` with `COALESCE(array_agg(...), '{}')::text[]`), `CreateUser`, `GrantUserRole`, `RevokeUserRole`, and `InsertAuditEvent` into native `uuid.UUID`, `*uuid.UUID`, `time.Time`, and `*time.Time` types with zero `pgtype` usage.
- **Migration Runner & Parallel Test Isolation ([`apps/api/cmd/migrate/main.go`](../../apps/api/cmd/migrate/main.go), [`apps/api/internal/dbtest/dbtest.go`](../../apps/api/internal/dbtest/dbtest.go)):** `cmd/migrate` applies `up` and one-step `down` via `goose.NewProvider` without leaking `DATABASE_URL` in error logs; `dbtest.New` isolates each live test in its own `t_<12hex>` schema (`CREATE SCHEMA ...` / `DROP SCHEMA ... CASCADE`), passing `go test -count=5` across parallel packages.
- **Fail-Closed Read Path & End-to-End Contract ([`apps/api/internal/handler/user_handler.go`](../../apps/api/internal/handler/user_handler.go), [`apps/api/openapi.json`](../../apps/api/openapi.json), [`apps/web/lib/api/schema.d.ts`](../../apps/web/lib/api/schema.d.ts)):** Rejects malformed UUIDs and `uuid.Nil` with `422 Unprocessable Entity`, missing/soft-deleted users with `404 Not Found`, and nil/unreachable DB with `503 Service Unavailable`.

---

## 2. Guardrail & Acceptance Checklist

| AC | Requirement | Status | Evidence |
| :---: | :--- | :---: | :--- |
| **1** | `001_users.sql` removed; `001_identity_and_rbac.sql` has goose Up/Down, timeout `SET`s, and immutability trigger | PASS | Verified in `apps/api/sql/schema/001_identity_and_rbac.sql:1-79`. |
| **2** | Deterministic offline `pnpm codegen` | PASS | `env -u DATABASE_URL pnpm codegen` exits `0`; second run produces identical SHA-256 across generated files. |
| **3** | No `pgtype`; OpenAPI has `format: "uuid"`, role enum, and optional `onboarding_completed_at` | PASS | Verified in `apps/api/internal/repository/db/models.go`, `apps/api/openapi.json`, and `apps/web/lib/api/schema.d.ts`. |
| **4** | Offline lint, typecheck, test, and build gate | PASS | `pnpm lint`, `pnpm --filter web check-types`, `pnpm test`, `pnpm build`, and `go build ./cmd/migrate` all exit `0` with `DATABASE_URL` unset. |
| **5** | Strict import boundaries (`database/sql` & `goose` isolated to `cmd/migrate` and `internal/dbtest`) | PASS | `TestNoRawChiRoutes` and `TestLayerImportsAndSQLBoundaries` pass. |
| **6** | Direct dependencies limited to `pressly/goose/v3 v3.29.0` and `google/uuid v1.6.0` | PASS | Verified in `apps/api/go.mod` (minor minimum-version bumps to indirect `puddle/v2`, `x/sync`, `x/text` required by `goose/v3`). |
| **7** | Untouched files preserved | PASS | Zero changes to `guardrails_test.go`, `main_test.go`, `go.ts`, `supabase.ts`, `.env.local`, `docs/plans/README.md`, and `security.yml`. |
| **8** | Live PostgreSQL 16 migration, rollback, constraint, and HTTP verification | PASS (Local PG 16.15) | `migrate up → down → up`, `TestSchemaInvariants`, `TestMigrationRollback`, `TestGetByIDLive` (`-count=5`), and live HTTP `200`/`404`/`422` checks all passed against PostgreSQL 16.15. GitHub Actions `test-api` runner execution will occur on push. |

---

## 3. Residual Notes for Future Stages (Non-Blocking)

1. **Deterministic Tie-Breaker for Same-Transaction Role Grants:** In `GetUserByID`, `array_agg(r.role ORDER BY r.granted_at)` orders by `granted_at`, which defaults to `NOW()` (constant within a single PostgreSQL transaction). When the role-admin plan introduces multi-role mutations, changing the `ORDER BY` to `ORDER BY r.granted_at ASC, r.role ASC` will provide a deterministic tie-breaker even when two roles are granted in the same transaction.

**Verdict:** Ship.

