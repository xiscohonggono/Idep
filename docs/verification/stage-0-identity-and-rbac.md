# Verification Log — Stage 0 Identity & RBAC Foundation

> **Plan:** [`docs/plans/Implemented/stage-0-identity-and-rbac.md`](../plans/Implemented/stage-0-identity-and-rbac.md) (Revision 2)
> **Date:** 2026-11-02
> **Builder:** Claude Code
> **Toolchain:** Go 1.26.8, Node 22.22.1, pnpm 11.25.0, sqlc 1.31.1, openapi-typescript 7.13.0, PostgreSQL 16.15
> **Conditions:** Offline gates ran with `DATABASE_URL` unset. Live gates ran against a throwaway local PostgreSQL 16.15, not `postgres:16-alpine` (see AC8).

## Result

| AC | Check | Result |
| :--- | :--- | :--- |
| 1 | Schema files | PASS |
| 2 | Codegen offline and deterministic | PASS |
| 3 | Generated types and OpenAPI contract | PASS |
| 4 | Offline gate | PASS |
| 5 | Import boundaries | PASS, with one grep false positive |
| 6 | Dependencies | **DEVIATION:** three existing indirect dependencies were also upgraded |
| 7 | Untouched files | PASS |
| 8 | Live DB proof | PASS locally; **GitHub Actions `test-api` not yet run** |

## Deviations from the plan

1. **`TestGetByIDLive` uses two transactions, not one (§2.5.3).** `granted_at` defaults to `NOW()`, which is fixed for a whole transaction. Two grants in one transaction therefore tie on `granted_at`, and the `array_agg(r.role ORDER BY r.granted_at)` order is unspecified, so the "grant order" assertion would not be deterministic. The test commits `CreateUser` + `GrantUserRole('user')`, then `GrantUserRole('editor')` + `InsertAuditEvent` in a second transaction. `GetUserByID` is unchanged, so **roles granted in one transaction still have no defined order**. The future role-admin plan should decide whether to add a tie-breaker.
2. **`go.mod` upgrades three indirect dependencies (AC6).** Goose raised their minimum versions, which AC6 does not list: `puddle/v2` v2.2.2 → v2.2.3, `x/sync` v0.22.0 → v0.24.0, `x/text` v0.40.0 → v0.43.0.
3. **AC5's grep also lists `internal/guardrails_test.go`.** That file only contains `database/sql` as a string in its ban check. It does not import the package, and the plan forbids changing it.
4. **AC2 compares hashes.** The contracts are uncommitted, so `git diff --exit-code` cannot show a clean result. SHA-256 hashes of the generated files were compared across the second codegen run instead.

## Evidence

### AC1 — Schema files

```text
$ ls apps/api/sql/schema
001_identity_and_rbac.sql

$ grep -nE "^-- \+goose (Up|Down)$|^SET |CREATE TRIGGER trg_audit_events_immutable" apps/api/sql/schema/001_identity_and_rbac.sql
1:-- +goose Up
2:SET lock_timeout = '3s';
3:SET statement_timeout = '30s';
67:CREATE TRIGGER trg_audit_events_immutable
71:-- +goose Down
72:SET lock_timeout = '3s';
73:SET statement_timeout = '30s';
```

### AC2 — Codegen

```text
$ env -u DATABASE_URL pnpm codegen
$ pnpm --filter api codegen && pnpm --filter web codegen
$ sqlc generate && go run ./cmd/server --export-openapi
Generated openapi.json successfully
$ openapi-typescript ../api/openapi.json -o lib/api/schema.d.ts
✨ openapi-typescript 7.13.0
exit 0

second codegen exit=0
hash before == hash after: 86d6ef90fbcba1e15b6f620589fa1f6506a6b48571a6d80b622738b0f60ed64f
(apps/api/internal/repository/db, apps/api/openapi.json, apps/web/lib/api/schema.d.ts)
```

### AC3 — Types

```text
$ grep -rn "pgtype" apps/api/internal/repository/db
(no matches, exit 1)

path param {'description': 'User UUID', 'format': 'uuid', 'type': 'string'}
id         {'format': 'uuid', 'type': 'string'}
roles      {'items': {'enum': ['user', 'editor', 'checker', 'admin'], 'type': 'string'}, 'type': ['array', 'null']}
required   ['id', 'email', 'display_name', 'email_notifications_enabled', 'is_verified_educator', 'roles', 'created_at', 'updated_at']
responses  ['200', '404', '422', '500', '503']
```

`required` excludes `onboarding_completed_at`. The repository always returns a non-nil `roles` slice, although the schema type also allows `null`.

### AC4 — Offline gate

```text
$ env -u DATABASE_URL pnpm lint                  exit 0
$ env -u DATABASE_URL pnpm --filter web check-types   exit 0
$ env -u DATABASE_URL pnpm test                  exit 0  (web: 1 file, 4 tests passed; api: all packages ok)
$ env -u DATABASE_URL pnpm build                 exit 0

$ env -u DATABASE_URL go test -count=1 -v ./...   exit 0
--- PASS: TestNoRawChiRoutes (0.00s)
--- PASS: TestLayerImportsAndSQLBoundaries (0.00s)
    schema_test.go:81: DATABASE_URL not set; live database test skipped
--- SKIP: TestSchemaInvariants (0.00s)
    schema_test.go:142: DATABASE_URL not set; live database test skipped
--- SKIP: TestMigrationRollback (0.00s)
    user_repo_test.go:37: DATABASE_URL not set; live database test skipped
--- SKIP: TestGetByIDLive (0.00s)
ok  	api/cmd/server	0.012s
ok  	api/internal	0.013s
ok  	api/internal/dbtest	0.014s
ok  	api/internal/handler	0.013s
ok  	api/internal/repository	0.009s

$ go build -o bin/migrate ./cmd/migrate   exit 0 (bin/ removed afterwards)
```

### AC5 — Import boundaries

```text
$ grep -rln "pressly/goose\|database/sql" apps/api --include=*.go
apps/api/internal/guardrails_test.go      <- string literal in the ban check, not an import
apps/api/internal/dbtest/dbtest.go
apps/api/cmd/migrate/main.go
```

### AC6 — Dependencies

```diff
 require (
 	github.com/danielgtaylor/huma/v2 v2.39.1
 	github.com/go-chi/chi/v5 v5.5.0
+	github.com/google/uuid v1.6.0
 	github.com/jackc/pgx/v5 v5.11.0
+	github.com/pressly/goose/v3 v3.29.0
 )

 require (
 	github.com/jackc/pgpassfile v1.0.0 // indirect
 	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
-	github.com/jackc/puddle/v2 v2.2.2 // indirect
-	golang.org/x/sync v0.22.0 // indirect
-	golang.org/x/text v0.40.0 // indirect
+	github.com/jackc/puddle/v2 v2.2.3 // indirect
+	github.com/mfridman/interpolate v0.0.2 // indirect
+	github.com/sethvargo/go-retry v0.5.0 // indirect
+	go.uber.org/multierr v1.11.0 // indirect
+	golang.org/x/sync v0.24.0 // indirect
+	golang.org/x/text v0.43.0 // indirect
 )
```

### AC7 — Untouched files

```text
$ git diff --exit-code --stat -- apps/api/internal/guardrails_test.go apps/api/cmd/server/main_test.go apps/web/lib/api/go.ts apps/web/lib/supabase.ts apps/web/.env.local docs/plans/README.md docs/plans/Pending/database-schema-proposal.md .github/workflows/security.yml
exit 0

$ git diff --stat -- docs/plans/Pending/database-staging-proposal.md
 docs/plans/Pending/database-staging-proposal.md | 2 +-
 1 file changed, 1 insertion(+), 1 deletion(-)
-§3.6](file:///home/xiscohonggono/projects/Idep/docs/plans/Pending/database-schema-proposal.md#L622-L654)),
+§3.6](file:///home/xiscohonggono/projects/Idep/docs/plans/Pending/database-schema-proposal.md#L697-L732)),
```

### AC8 — Live database proof

Docker has no WSL integration here, so PostgreSQL 16.15 ran from the `@embedded-postgres/linux-x64@16.15.0-beta.19` binaries unpacked in `/tmp`, on port `55432`. The binaries were not added to the repository or to any manifest. The GitHub Actions `test-api` job has not run because nothing has been pushed, so its `postgres:16-alpine` run remains **UNVERIFIED**.

```text
$ go run ./cmd/migrate down     (fresh database)
nothing to roll back
exit=0
$ go run ./cmd/migrate up
OK    up 001_identity_and_rbac.sql (20.6ms)
exit=0
$ go run ./cmd/migrate down
OK    down 001_identity_and_rbac.sql (14.31ms)
exit=0
$ go run ./cmd/migrate up
OK    up 001_identity_and_rbac.sql (17.31ms)
exit=0

$ ./bin/migrate sideways
usage: migrate up|down
exit 2
$ DATABASE_URL=<unreachable URL with password> ./bin/migrate up
Failed to reach the database at DATABASE_URL     (URL not printed)
exit 1
$ env -u DATABASE_URL ./bin/migrate up
DATABASE_URL is not set
exit 1

$ DATABASE_URL=... go test -count=1 -v ./...     exit 0
--- PASS: TestSchemaInvariants (0.10s)
    --- PASS: TestSchemaInvariants/tables_exist (0.01s)
    --- PASS: TestSchemaInvariants/editor_grant_requires_accepted_guidelines (0.00s)
    --- PASS: TestSchemaInvariants/revocation_requires_a_non-blank_reason (0.00s)
    --- PASS: TestSchemaInvariants/one_active_grant_per_user_and_role (0.00s)
    --- PASS: TestSchemaInvariants/audit_events_are_append-only (0.00s)
    --- PASS: TestSchemaInvariants/display_name_cannot_be_blank (0.00s)
--- PASS: TestMigrationRollback (0.12s)
--- PASS: TestGetUser (0.01s)
--- PASS: TestGetByIDNilPool (0.00s)
--- PASS: TestGetByIDLive (0.08s)
ok  	api/cmd/server	0.011s
ok  	api/internal	0.013s
ok  	api/internal/dbtest	0.237s
ok  	api/internal/handler	0.020s
ok  	api/internal/repository	0.087s

$ DATABASE_URL=... go test -count=5 ./internal/dbtest ./internal/repository
ok  	api/internal/dbtest	1.131s
ok  	api/internal/repository	0.513s
```

### Extra — HTTP check against the live database

`./bin/server` ran against the same database with one seeded user and an active `user` role.

```text
$ curl /api/v1/users/00000000-0000-0000-0000-000000000101
{"$schema":"http://localhost:8080/schemas/User.json","id":"00000000-0000-0000-0000-000000000101","email":"a@example.com","display_name":"Ada","email_notifications_enabled":false,"is_verified_educator":false,"roles":["user"],"created_at":"2026-11-02T14:18:50.515392+08:00","updated_at":"2026-11-02T14:18:50.515392+08:00"}
HTTP 200

$ curl /api/v1/users/00000000-0000-0000-0000-000000000999
{"$schema":"http://localhost:8080/schemas/ErrorModel.json","title":"Not Found","status":404,"detail":"user not found"}
HTTP 404

$ curl /api/v1/users/not-a-uuid
{"$schema":"http://localhost:8080/schemas/ErrorModel.json","title":"Unprocessable Entity","status":422,"detail":"validation failed","errors":[{"message":"invalid value: invalid UUID length: 10","location":"path.id","value":"not-a-uuid"}]}
HTTP 422

$ curl /api/v1/users/00000000-0000-0000-0000-000000000000
{"$schema":"http://localhost:8080/schemas/ErrorModel.json","title":"Unprocessable Entity","status":422,"detail":"invalid user ID"}
HTTP 422
```

## Not verified

- The `test-api` job on GitHub Actions, including its `postgres:16-alpine` service.
- The `codegen-drift` job on GitHub Actions.
- `ci.yml` was only parsed as YAML (jobs: `test-api`, `test-web`, `codegen-drift`); it has not been executed.
