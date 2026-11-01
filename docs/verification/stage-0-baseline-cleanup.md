# Verification Log — Stage 0 Baseline Cleanup

> **Plan:** [`docs/plans/Pending/stage-0-baseline-cleanup.md`](../plans/Pending/stage-0-baseline-cleanup.md) (Revision 4)
> **Date:** 2026-11-01
> **Builder:** Claude Code
> **Toolchain:** Go 1.26.8, Node 22.22.1, pnpm 11.25.0, sqlc 1.31.1, TypeScript 5.9.3 (resolved from `^5.7.0`), openapi-typescript 7.13.0
> **Conditions:** `DATABASE_URL` unset and API offline for all gates except AC7, which starts the API.

## Result

All 10 acceptance criteria pass locally. See [Not verified](#not-verified) for what a local run cannot prove.

| AC | Check | Result |
| :--- | :--- | :--- |
| 1 | `pnpm codegen` with `DATABASE_URL` unset | PASS |
| 2 | No database changes | PASS |
| 3 | Second `pnpm codegen` is deterministic | PASS |
| 4 | No hand-written API DTOs | PASS |
| 5 | Forbidden strings removed | PASS |
| 6 | Lint, types, tests, build | PASS |
| 7 | Manual API check | PASS |
| 8 | No stale Go version references | PASS |
| 9 | Tailwind utilities in emitted CSS | PASS |
| 10 | Supabase client and `go.mod`/`go.sum` untouched | PASS |

## Evidence

### AC1 — Codegen without `DATABASE_URL`

```text
$ env -u DATABASE_URL pnpm codegen
$ sqlc generate && go run ./cmd/server --export-openapi
Generated openapi.json successfully
$ openapi-typescript ../api/openapi.json -o lib/api/schema.d.ts
✨ openapi-typescript 7.13.0
🚀 ../api/openapi.json → lib/api/schema.d.ts [124.5ms]
exit 0

apps/api/openapi.json:153:    "/api/v1/users/{id}": {
apps/api/openapi.json:226:    "/health": {
apps/web/lib/api/schema.d.ts:15:        get: operations["get-user"];
apps/web/lib/api/schema.d.ts:104:        User: {
apps/web/lib/api/schema.d.ts:124:    "get-user": {
```

### AC2 — No database changes

```text
$ git diff --exit-code -- apps/api/internal/repository/db apps/api/sql apps/api/sqlc.yaml
(no output) exit 0
```

### AC3 — Deterministic regeneration

SHA-256 hashes were taken after the first run and checked after the second.

```text
$ env -u DATABASE_URL pnpm codegen   # exit 0
$ sha256sum -c
apps/api/openapi.json: OK
apps/web/lib/api/schema.d.ts: OK
```

### AC4 — No hand-written API DTOs

```text
$ grep -rn "interface\|type " apps/web/lib/api --include=*.ts | grep -v schema.d.ts
apps/web/lib/api/go.ts:1:import type { paths } from '@/lib/api/schema';
apps/web/lib/api/go.ts:3:export type User =
```

`User` is an alias derived from `paths`.

### AC5 — Forbidden strings removed

```text
$ grep -rIl "Demo User\|Supabase Fast Track\|NEXT_PUBLIC_BACKEND_PROVIDER\|NEXT_PUBLIC_GO_API_URL\|router.Mount" apps/
(no matches, including node_modules, .next and .turbo)
```

### AC6 — Lint, types, tests, build

| Command | Exit | Output |
| :--- | :--- | :--- |
| `pnpm lint` | 0 | 3 tasks successful (`api`, `web`, `@repo/ui`) |
| `pnpm --filter web check-types` | 0 | `next typegen && tsc --noEmit` clean |
| `pnpm --filter @repo/ui check-types` | 0 | Clean (extra check: this package's TypeScript also changed) |
| `pnpm test` | 0 | Web: 4 of 4 tests passed. Go: `api/cmd/server` and `api/internal/handler` ok |
| `go test -count=1 ./...` | 0 | Same packages re-run without the Go test cache |
| `pnpm build` | 0 | 2 tasks successful; `/` is `ƒ (Dynamic)`, so no API response is baked in at build time |

`pnpm build` also printed `WARNING no output files found for task api#build`. The cause is that `turbo.json` declares only `.next/**` outputs, and the plan leaves that file unchanged. The build still succeeds.

### AC7 — Manual API check

The server was built to a temporary binary and started with `DATABASE_URL` unset.

```text
$ curl -i localhost:8080/api/v1/users/101
HTTP/1.1 503 Service Unavailable
Content-Type: application/problem+json
{"$schema":"http://localhost:8080/schemas/ErrorModel.json","title":"Service Unavailable","status":503,"detail":"database unavailable"}

$ curl -i localhost:8080/health
HTTP/1.1 200 OK
Content-Type: application/json
{"$schema":"http://localhost:8080/schemas/HealthOutputBody.json","status":"ok","message":"Go API operational with pgxpool"}
```

Two additional checks:

```text
$ DATABASE_URL='::not-a-url' server
Invalid DATABASE_URL
exit 1

$ next start --port 3100   # with the API running and no database
<pre class="rounded-md bg-gray-100 text-sm text-gray-900">API error: 503 Service Unavailable</pre>
```

### AC8 — No stale Go version references

```text
$ grep -rn "1\.24" --exclude-dir=node_modules --exclude-dir=.next --exclude-dir=.git --exclude-dir=.agents --exclude=*.sum --exclude=pnpm-lock.yaml .
```

Matches are limited to the plan file and `apps/web/public/globe.svg`. The SVG match is path coordinate data, not a Go version.

### AC9 — Tailwind utilities in emitted CSS

```text
apps/web/.next/static/chunks/1oz2fw1-jj68o.css
min-h-screen: found
font-bold: found
text-2xl: found
bg-gray-100: found
rounded-md: found
```

### AC10 — Protected files untouched

```text
$ git diff --stat -- apps/web/lib/supabase.ts apps/api/go.mod apps/api/go.sum
(no output)
```

## §2.8 TypeScript alignment (Option A)

- `apps/web/package.json` and `packages/ui/package.json` now declare `"typescript": "^5.7.0"`.
- `pnpm-workspace.yaml` is back to its bootstrap state: `git status` no longer lists it.
- `pnpm-lock.yaml` resolves `typescript@5.9.3` as the only TypeScript version and `openapi-typescript@7.13.0(typescript@5.9.3)`.

## Fixes made during implementation

None of the gates failed, so no error fixes were needed. One styling constraint shaped `app/page.tsx`: the `* { padding: 0; margin: 0; }` rule in `globals.css` is unlayered, so it overrides Tailwind's layered padding and margin utilities. §2.6 requires that rule to stay, so the page uses only layout, colour and typography utilities.

## Not verified

- **GitHub Actions:** `ci.yml` and `security.yml`, including the new `codegen-drift` job and the OSV `pnpm-lock.yaml` scan, have not run. Their commands were checked locally where possible, but the workflows themselves were not executed.
- **Docker:** `apps/api/Dockerfile` with `golang:1.26-alpine` was not built.
- **Database paths:** the `200` and `404` responses are covered by `humatest` with a fake repository. The real pgx path and the `5s` query timeout were not exercised against PostgreSQL.
