# Stage 0 — Identity & RBAC Foundation

> **Status:** Implemented & Approved (see [`docs/reviews/stage-0-identity-and-rbac-review.md`](../../reviews/stage-0-identity-and-rbac-review.md) and [`docs/verification/stage-0-identity-and-rbac.md`](../../verification/stage-0-identity-and-rbac.md))  
> **Scope:** Stage 0 of [`database-staging-proposal.md`](file:///home/xiscohonggono/projects/Idep/docs/plans/Pending/database-staging-proposal.md): `001_identity_and_rbac.sql`, `sqlc.yaml` overrides, Stage 0 SQL queries, `pressly/goose/v3` migration CLI, read-path contract update (`GET /api/v1/users/{id}`), and PostgreSQL 16 CI verification.  
> **Builder:** Claude Code via `delegate-to-claude` · **Reviewer:** Gemini/Architect → `docs/reviews/stage-0-identity-and-rbac-review.md`  
> **Related Contracts:** [`architectural_decisions.md`](file:///home/xiscohonggono/projects/Idep/docs/architecture/architectural_decisions.md) · [`database-schema-proposal.md`](file:///home/xiscohonggono/projects/Idep/docs/plans/Pending/database-schema-proposal.md) · [`database-staging-proposal.md`](file:///home/xiscohonggono/projects/Idep/docs/plans/Pending/database-staging-proposal.md) · [`.agent/rules/project_guardrails.md`](file:///home/xiscohonggono/projects/Idep/.agent/rules/project_guardrails.md)

---

## 0. Revision History

| Rev | Change |
| :--- | :--- |
| 1 | Initial draft: custom zero-dependency runner, Go write methods (`CreateUser`/`GrantRole`/`RevokeRole`), doc status edits by Builder. |
| 2 | **D1:** replace custom runner with `pressly/goose/v3` (version tracking, one-step `down`; matches `[ASSUMPTION-M2]`). **D2:** defer Go write methods to the auth/role-admin plan; Stage 0 ships SQL queries + live DB tests only. **D3:** include `README.md`, `AGENTS.md`, and a stale anchor fix in `database-staging-proposal.md`. Fixes: per-test schema isolation (parallel `go test` packages shared one DB), single-query role aggregation, `format:"uuid"`/`enum` tags, non-NULL `metadata_json`, CI-only live verification stated honestly, proposal status edits moved to post-approval ledger step. |

---

## 1. Objective & Constraints

Replace the bootstrap `001_users.sql` (`id TEXT`) with the Stage 0 identity, RBAC, and append-only audit schema; propagate the new `User` read contract through `sqlc` → Huma → OpenAPI 3.1 → `schema.d.ts`; and verify migrations and DB invariants against a real PostgreSQL 16 in CI.

```mermaid
flowchart LR
    M["sql/schema/001_identity_and_rbac.sql\n(goose Up/Down)"] -->|"sqlc generate"| DB["internal/repository/db\n(uuid.UUID, time.Time, pointers)"]
    DB --> R["PostgresUserRepository.GetByID\n(user + active roles, 1 query)"]
    R --> H["Huma GET /api/v1/users/{id}"]
    H -->|"pnpm codegen"| OA["openapi.json -> schema.d.ts"]
    M -->|"cmd/migrate (goose) + CI postgres:16"| PG[("PostgreSQL 16\nup -> down -> up")]
```

### Decisions (resolved 2026-11-02)

| ID | Decision |
| :--- | :--- |
| **D1** | Use `github.com/pressly/goose/v3` **v3.29.0** (requires Go ≥ 1.26.0; `go.mod` is `go 1.26.8`) via `goose.NewProvider(goose.DialectPostgres, db, os.DirFS(...))` and `database/sql` + `github.com/jackc/pgx/v5/stdlib` (same `pgx/v5` module, no new driver). `goose_db_version` tracks applied versions; `Provider.Down` rolls back **one** migration, satisfying [`database-staging-proposal.md` §5.3](file:///home/xiscohonggono/projects/Idep/docs/plans/Pending/database-staging-proposal.md#L142-L143). API verified via Context7 `/pressly/goose`. |
| **D2** | No Go write methods in Stage 0. `CreateUser`, `GrantUserRole`, `RevokeUserRole`, `InsertAuditEvent` are compiled by `sqlc` and exercised only by live DB tests. Their transactional Go wrappers (with `X-Request-ID` middleware and `C-10` claim release) belong to the auth/role-admin plan. |
| **D3** | Builder updates `README.md`, `AGENTS.md`, and one stale anchor in `database-staging-proposal.md` (§2.7). Proposal **status banners** are updated by the Reviewer at ledger move, not by the Builder. |

### Constraints
1. Greenfield: no non-test environment holds `users` rows ([`[ASSUMPTION-M1]`](file:///home/xiscohonggono/projects/Idep/docs/plans/Pending/database-staging-proposal.md#L179)), so `001_users.sql` is replaced directly.
2. Only new module: `github.com/pressly/goose/v3` (and its transitive deps). `github.com/google/uuid v1.6.0` is already in `go.sum`; add it to the direct `require` block.
3. `database/sql` and goose imports are allowed **only** in `cmd/migrate` and `internal/dbtest`. [`guardrails_test.go`](file:///home/xiscohonggono/projects/Idep/apps/api/internal/guardrails_test.go#L152-L191) bans them in `handler`, `service`, `domain`, and `repository` — do not modify that test.
4. Offline (no `DATABASE_URL`): `pnpm codegen`, `pnpm lint`, `pnpm --filter web check-types`, `pnpm test`, `pnpm build` exit `0`; live tests `t.Skip`.

---

## 2. Work Items

### 2.1 Schema migration — `apps/api/sql/schema/001_identity_and_rbac.sql`

1. Delete `apps/api/sql/schema/001_users.sql`.
2. Create `001_identity_and_rbac.sql` with exactly this content (DDL copied from [`database-schema-proposal.md` §3.1, §3.4, §3.5](file:///home/xiscohonggono/projects/Idep/docs/plans/Pending/database-schema-proposal.md#L58-L94)):

```sql
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
```

> [!NOTE]
> goose runs each migration in a transaction by default, so a failed `001` leaves no partial schema. The session-level `SET`s persist on that connection after commit; this is harmless because the `*sql.DB` in `cmd/migrate` is used only for migrations and closed on exit. Do not run migrations through the API server's pool.

---

### 2.2 `sqlc` config, queries, and `go.mod`

1. **[`apps/api/sqlc.yaml`](file:///home/xiscohonggono/projects/Idep/apps/api/sqlc.yaml)** — replace with [`database-schema-proposal.md` §3.6](file:///home/xiscohonggono/projects/Idep/docs/plans/Pending/database-schema-proposal.md#L700-L732) (compiled against `sqlc v1.31.1` during planning; output has no `pgtype`):

```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "sql/schema"
    queries: "sql/queries"
    gen:
      go:
        package: "db"
        out: "internal/repository/db"
        sql_package: "pgx/v5"
        emit_json_tags: true
        emit_pointers_for_null_types: true
        overrides:
          - db_type: "uuid"
            go_type:
              import: "github.com/google/uuid"
              type: "UUID"
          - db_type: "uuid"
            nullable: true
            go_type:
              import: "github.com/google/uuid"
              type: "UUID"
              pointer: true
          - db_type: "timestamptz"
            go_type: "time.Time"
          - db_type: "timestamptz"
            nullable: true
            go_type:
              type: "time.Time"
              pointer: true
```

2. **[`apps/api/sql/queries/users.sql`](file:///home/xiscohonggono/projects/Idep/apps/api/sql/queries/users.sql)** — replace with (compiled during planning; `GetUserByID` yields `Roles []string`):

```sql
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
```

> [!IMPORTANT]
> `metadata_json` maps to `[]byte`. A nil slice inserts SQL `NULL` and violates `NOT NULL` (the column default applies only when the column is omitted). Every caller, including tests, must pass `[]byte("{}")` or a valid JSON object.

3. **`apps/api/go.mod` / `go.sum`** (run inside `apps/api`):
   - `go get github.com/pressly/goose/v3@v3.29.0`
   - Add `github.com/google/uuid v1.6.0` to the direct `require` block.
   - `go mod tidy`. Record the resulting `go.mod` diff in the verification log.

---

### 2.3 Migration CLI — `apps/api/cmd/migrate/main.go` (new)

- Usage, run from `apps/api`: `go run ./cmd/migrate up` | `go run ./cmd/migrate down`.
- Read `DATABASE_URL`; if empty or `sql.Open("pgx", ...)`/`db.PingContext` fails → `log.Fatalf` without printing the URL.
- `p, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("sql/schema"))`; `defer p.Close()`.
- `up` → `p.Up(ctx)`; print each `MigrationResult`. `down` → `p.Down(ctx)` (one step); treat `goose.ErrNoNextVersion` as "nothing to roll back" (exit `0`).
- Any other argument → usage message, exit `2`.
- Import `_ "github.com/jackc/pgx/v5/stdlib"`. No other files import goose except `internal/dbtest`.

---

### 2.4 Live-DB test isolation & schema invariant tests — `apps/api/internal/dbtest/` (new)

`go test ./...` runs packages in parallel, so live tests must never share a schema.

1. **`dbtest.go`** — `func New(t *testing.T) string`:
   - If `DATABASE_URL == ""` → `t.Skip("DATABASE_URL not set; live database test skipped")`.
   - Connect with `pgx.Connect`, `CREATE SCHEMA` named `t_` + 12 random lowercase hex chars, close.
   - Build a DSN equal to `DATABASE_URL` plus `search_path=<schema>` (use `net/url` to set the query param; support only URL-form DSNs and fail the test otherwise).
   - Apply migrations with goose `Provider.Up` on `os.DirFS(<abs path to apps/api/sql/schema>)` — resolve the path from the package dir via `runtime.Caller(0)`, not the caller's working dir.
   - `t.Cleanup`: close resources, then `DROP SCHEMA <schema> CASCADE` on a fresh connection.
   - Return the schema-scoped DSN.
2. **`schema_test.go`** (live; uses `dbtest.New`), each case in its own transaction/rows:
   - Tables `users`, `user_roles`, `audit_events` exist in the test schema.
   - `user_roles` editor grant with `guidelines_accepted_at IS NULL` fails with constraint `chk_editor_guidelines` (assert `*pgconn.PgError.ConstraintName`).
   - Revoking with `revocation_reason = '   '` fails `chk_revocation_reason`.
   - A second active `(user_id, role)` grant fails `idx_user_roles_active_unique`; after revoking the first with a reason, a new grant succeeds.
   - `UPDATE audit_events ...` and `DELETE FROM audit_events ...` each fail with message containing `Table audit_events is append-only`.
   - `users.display_name = '  '` fails the `display_name` check.
   - Rollback proof: goose `Provider.Down` once → the three tables and `prevent_immutable_table_mutation()` no longer exist in the schema; `Provider.Up` again → they exist.

Raw SQL in these test files is allowed (`_test.go` is exempt from `guardrails_test.go`, and `internal/dbtest` is outside its checked layers).

---

### 2.5 Backend read path (`apps/api/internal/...`, `apps/api/cmd/server/main.go`)

1. **[`internal/domain/user.go`](file:///home/xiscohonggono/projects/Idep/apps/api/internal/domain/user.go):**
   ```go
   type Role string

   const (
       RoleUser    Role = "user"
       RoleEditor  Role = "editor"
       RoleChecker Role = "checker"
       RoleAdmin   Role = "admin"
   )

   var (
       ErrUserNotFound        = errors.New("user not found")
       ErrDatabaseUnavailable = errors.New("database unavailable")
       ErrInvalidInput        = errors.New("invalid input")
   )

   type User struct {
       ID                        uuid.UUID  `json:"id" format:"uuid"`
       Email                     string     `json:"email" format:"email"`
       DisplayName               string     `json:"display_name"`
       OnboardingCompletedAt     *time.Time `json:"onboarding_completed_at,omitempty"`
       EmailNotificationsEnabled bool       `json:"email_notifications_enabled"`
       IsVerifiedEducator        bool       `json:"is_verified_educator"`
       Roles                     []Role     `json:"roles" enum:"user,editor,checker,admin"`
       CreatedAt                 time.Time  `json:"created_at"`
       UpdatedAt                 time.Time  `json:"updated_at"`
   }

   type UserRepository interface {
       GetByID(ctx context.Context, id uuid.UUID) (*User, error)
   }
   ```
   `format:"uuid"` is required: Huma maps `uuid.UUID` (a `TextUnmarshaler`) to a plain `string` schema and adds a format only from the tag.
2. **[`internal/repository/user_repo.go`](file:///home/xiscohonggono/projects/Idep/apps/api/internal/repository/user_repo.go):** `GetByID(ctx, id uuid.UUID)` keeps the existing nil-queries → `ErrDatabaseUnavailable`, `5s` timeout, and `pgx.ErrNoRows` → `ErrUserNotFound` behavior; calls `r.queries.GetUserByID`; converts `row.Roles` into a **non-nil** `[]domain.Role`. Only `r.queries` calls are allowed (no `.Query`/`.QueryRow`/`.Exec`/`fmt.Sprintf`).
3. **`internal/repository/user_repo_test.go` (new):**
   - Offline: nil pool → `ErrDatabaseUnavailable`.
   - Live (`dbtest.New`): open a `pgxpool` on the returned DSN; via generated `db.Queries` in one transaction run `CreateUser`, `GrantUserRole('user')`, `GrantUserRole('editor', guidelines_accepted_at=now)`, `InsertAuditEvent(metadata_json=[]byte("{}"))`, commit; then `GetByID` returns both roles in grant order. `RevokeUserRole('editor', reason)` → `GetByID` returns only `user`. Unknown UUID → `ErrUserNotFound`. A user with `deleted_at` set → `ErrUserNotFound`.
4. **[`internal/service/user_service.go`](file:///home/xiscohonggono/projects/Idep/apps/api/internal/service/user_service.go):** `GetUserProfile(ctx, id uuid.UUID)`; `id == uuid.Nil` → `domain.ErrInvalidInput`.
5. **[`internal/handler/user_handler.go`](file:///home/xiscohonggono/projects/Idep/apps/api/internal/handler/user_handler.go):**
   - `GetUserInput.ID uuid.UUID` with tags `path:"id" format:"uuid" doc:"User UUID"` (remove `minLength`/`maxLength`). Verified in Huma v2.39.1 source: path params fall back to `encoding.TextUnmarshaler`, so a malformed UUID returns `422`.
   - Error mapping: `ErrInvalidInput` → `huma.Error422UnprocessableEntity("invalid user ID")`; `ErrUserNotFound` → `404`; anything else → logged + `503`. Add `http.StatusUnprocessableEntity` to `Errors`.
6. **[`internal/handler/user_handler_test.go`](file:///home/xiscohonggono/projects/Idep/apps/api/internal/handler/user_handler_test.go):** use `00000000-0000-0000-0000-000000000101`; cases: nil-pool repo → `503`, unavailable → `503`, other error → `503`, not found → `404`, `/api/v1/users/not-a-uuid` → `422`, nil UUID → `422`, found → `200` with JSON equal to the fake user (compare decoded `domain.User`; roles `["user"]`).
7. **[`cmd/server/main.go`](file:///home/xiscohonggono/projects/Idep/apps/api/cmd/server/main.go)** `openPool`: set `MaxConns=25`, `MinConns=2`, `MaxConnLifetime=30m`, `MaxConnIdleTime=5m`, `HealthCheckPeriod=1m` ([`database-schema-proposal.md` §4](file:///home/xiscohonggono/projects/Idep/docs/plans/Pending/database-schema-proposal.md#L750-L751)). [`cmd/server/main_test.go`](file:///home/xiscohonggono/projects/Idep/apps/api/cmd/server/main_test.go) stays unchanged and must pass.

---

### 2.6 Frontend sync (`apps/web`)

1. Run `pnpm codegen` (regenerates `internal/repository/db/*`, `apps/api/openapi.json`, `apps/web/lib/api/schema.d.ts`). Never hand-edit these.
2. [`apps/web/app/page.tsx`](file:///home/xiscohonggono/projects/Idep/apps/web/app/page.tsx): `getUser('101')` → `getUser('00000000-0000-0000-0000-000000000101')`.
3. [`apps/web/__tests__/smoke.test.ts`](file:///home/xiscohonggono/projects/Idep/apps/web/__tests__/smoke.test.ts): fixture becomes `{ id: '00000000-0000-0000-0000-000000000101', email: 'a@example.com', display_name: 'Ada', email_notifications_enabled: false, is_verified_educator: false, roles: ['user'], created_at: '2026-11-02T00:00:00Z', updated_at: '2026-11-02T00:00:00Z' }` typed as `User` from `../lib/api/go` so drift is a type error. Keep the URL-encoding test.
4. No change to [`apps/web/lib/api/go.ts`](file:///home/xiscohonggono/projects/Idep/apps/web/lib/api/go.ts) — its `User` alias derives from `paths`.

---

### 2.7 CI and documentation

1. **[`.github/workflows/ci.yml`](file:///home/xiscohonggono/projects/Idep/.github/workflows/ci.yml)** — replace only the `test-api` job:

```yaml
  test-api:
    name: Go API (Migrate, Test & Build)
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: idep_test
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 5s
          --health-timeout 3s
          --health-retries 10
    env:
      # Ephemeral CI-only service credentials; not a secret.
      DATABASE_URL: postgres://postgres:postgres@localhost:5432/idep_test?sslmode=disable
    steps:
      - name: Checkout Code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: apps/api/go.mod
          cache-dependency-path: apps/api/go.sum

      - name: Verify Migrations (up -> down -> up)
        working-directory: apps/api
        run: |
          go run ./cmd/migrate up
          go run ./cmd/migrate down
          go run ./cmd/migrate up

      - name: Run Go Vet, Test & Build
        working-directory: apps/api
        run: |
          go vet ./...
          go test -v ./...
          go build -o bin/server ./cmd/server
          go build -o bin/migrate ./cmd/migrate
```

   Live tests use their own `t_*` schemas, so they don't collide with the `public` schema migrated above. `codegen-drift`, `test-web`, and `security.yml` are unchanged (OSV already scans `apps/api/go.sum`, so it will cover goose's dependency tree).

2. **[`README.md`](file:///home/xiscohonggono/projects/Idep/README.md):**
   - L218: `# PostgreSQL DDL consumed by sqlc (bootstrap: 001_users.sql)` → `# goose migrations, also consumed by sqlc (001_identity_and_rbac.sql)`.
   - Add to the repository tree under `apps/api/`: `cmd/migrate/` — `# goose migration CLI (up / down one step)`.
   - Commands table (near L255): add a row `` `go run ./cmd/migrate up` / `down` (in `apps/api`) `` — "Applies pending migrations / rolls back the latest one against `DATABASE_URL`. `down` is destructive; require explicit confirmation outside local/test databases."
3. **[`AGENTS.md` §6 Core Workflows](file:///home/xiscohonggono/projects/Idep/AGENTS.md#L59-L68):** add one bullet after `pnpm codegen`: `` `go run ./cmd/migrate up|down` (from `apps/api`): Applies or rolls back one goose migration in `apps/api/sql/schema/` against `DATABASE_URL`. `down` falls under the Destructive Action Gate. ``
4. **[`database-staging-proposal.md` L65](file:///home/xiscohonggono/projects/Idep/docs/plans/Pending/database-staging-proposal.md#L65):** fix stale anchor `#L622-L654` → `#L697-L732` (points at §3.6). No other edits to either proposal.

---

## 3. Files Touched

| Action | Path |
| :--- | :--- |
| delete | `apps/api/sql/schema/001_users.sql` |
| new | `apps/api/sql/schema/001_identity_and_rbac.sql`, `apps/api/cmd/migrate/main.go`, `apps/api/internal/dbtest/dbtest.go`, `apps/api/internal/dbtest/schema_test.go`, `apps/api/internal/repository/user_repo_test.go` |
| edit | `apps/api/sqlc.yaml`, `apps/api/sql/queries/users.sql`, `apps/api/go.mod`, `apps/api/go.sum` |
| edit | `apps/api/internal/domain/user.go`, `apps/api/internal/repository/user_repo.go`, `apps/api/internal/service/user_service.go`, `apps/api/internal/handler/user_handler.go`, `apps/api/internal/handler/user_handler_test.go`, `apps/api/cmd/server/main.go` |
| regenerated (`pnpm codegen` only) | `apps/api/internal/repository/db/*`, `apps/api/openapi.json`, `apps/web/lib/api/schema.d.ts` |
| edit | `apps/web/app/page.tsx`, `apps/web/__tests__/smoke.test.ts` |
| edit | `.github/workflows/ci.yml`, `README.md`, `AGENTS.md`, `docs/plans/Pending/database-staging-proposal.md` (anchor only) |
| **must NOT change** | `apps/api/internal/guardrails_test.go`, `apps/api/cmd/server/main_test.go`, `apps/web/lib/api/go.ts`, `apps/web/lib/supabase.ts`, `apps/web/.env.local`, `docs/plans/README.md`, `docs/plans/Pending/database-schema-proposal.md`, `.github/workflows/security.yml` |

---

## 4. Acceptance Criteria (record in `docs/verification/stage-0-identity-and-rbac.md`)

1. **Schema files:** `001_users.sql` absent; `001_identity_and_rbac.sql` contains `-- +goose Up`, `-- +goose Down`, both `SET` guards in each section, and `trg_audit_events_immutable`.
2. **Codegen:** with `DATABASE_URL` unset, `pnpm codegen` exits `0`; a second run leaves `git diff --exit-code -- apps/api/internal/repository/db apps/api/openapi.json apps/web/lib/api/schema.d.ts` empty.
3. **Types:** `grep -rn "pgtype" apps/api/internal/repository/db` → no matches. `openapi.json` has `"format": "uuid"` on the `id` path param and on `User.id`, `User.roles.items.enum` = `["user","editor","checker","admin"]`, and `User.required` excludes `onboarding_completed_at`.
4. **Offline gate:** `pnpm lint`, `pnpm --filter web check-types`, `pnpm test`, `pnpm build` exit `0`; `go test -v ./...` output shows `TestNoRawChiRoutes`, `TestLayerImportsAndSQLBoundaries` PASS and live tests SKIP with the `DATABASE_URL not set` message. `go build ./cmd/migrate` succeeds (remove `bin/` afterwards).
5. **Import boundaries:** `grep -rln "pressly/goose\|database/sql" apps/api --include=*.go` lists only files under `cmd/migrate/` and `internal/dbtest/`.
6. **Dependencies:** `go.mod` diff adds only `github.com/pressly/goose/v3 v3.29.0`, `github.com/google/uuid v1.6.0` (direct), and transitive `// indirect` lines.
7. **Untouched files:** `git diff --exit-code` on every "must NOT change" path except `database-staging-proposal.md` (anchor line only; show the one-line diff).
8. **Live DB proof — CI only:** this workstation has no PostgreSQL and Docker Desktop has no WSL integration, so record `migrate up → down → up` and live `go test` results as **"UNVERIFIED locally — verified by GitHub Actions `test-api` run <link>"**. If the Builder can reach a Postgres 16 via `DATABASE_URL`, run them and paste the output instead.

---

## 5. Rollback

1. **Database:** from `apps/api`, `go run ./cmd/migrate down` rolls back `001` (drops trigger, function, `audit_events`, `user_roles`, `users`). Destructive — requires explicit confirmation on any non-test database.
2. **Code:** `git revert` the implementation commit restores `001_users.sql`, `sqlc.yaml`, the generated contracts, `go.mod`/`go.sum`, and docs.

## 6. Post-Approval Ledger Step (Reviewer, not Builder)

After `STATUS: APPROVED`: update the status banners of `database-schema-proposal.md` and `database-staging-proposal.md` to "Stage 0 implemented; Stages 1–4 pending", then move this plan to `docs/plans/Implemented/`.
