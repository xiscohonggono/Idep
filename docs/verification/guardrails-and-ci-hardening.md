# Verification Log — Guardrails & CI Hardening

> **Plan:** [`docs/plans/Implemented/guardrails-and-ci-hardening.md`](../plans/Implemented/guardrails-and-ci-hardening.md)
> **Date:** 2026-11-01
> **Builder:** Claude Code
> **Toolchain:** Go 1.26.8, Node 22.22.1, pnpm 11.25.0, sqlc 1.31.1
> **Conditions:** `DATABASE_URL` unset, API offline.

## Result

All 7 acceptance criteria pass, with two AC commands corrected because they could not report the intended result (AC6, AC7). See [Deviations](#deviations-from-the-plan) and [Not verified](#not-verified).

| AC | Check | Result |
| :--- | :--- | :--- |
| 1 | Agent rule links | PASS |
| 2 | Schema status banner | PASS |
| 3 | ESLint guardrails active | PASS |
| 4 | Go AST guardrails active | PASS |
| 5 | CI and lockfile hardening | PASS (static check only) |
| 6 | Gitignore check | PASS (command corrected) |
| 7 | Full-stack regression gate | PASS (command corrected) |

## Evidence

### AC1 — Agent rule links

`grep -rn "project_guardrails.md"` matches `AGENTS.md:77`, `CLAUDE.md:2`, `GEMINI.md:2`, and `.agents/skills/delegate-to-claude/SKILL.md:19,24`. `SKILL.md` references `docs/verification/` on 2 lines.

### AC2 — Schema status

`.agent/rules/project_guardrails.md:9` holds the `[!IMPORTANT]` banner, directly above §1. The 14 prohibitions and the 4-step checklist are unchanged.

### AC3 — ESLint guardrails

`grep -rn "onlyWarn" packages/eslint-config/base.js` returns no matches. `pnpm lint` exits 0.

Negative tests were applied transiently; `go.ts` and `page.tsx` were restored and compared byte-for-byte with `cmp`.

| Injected code | File | `pnpm --filter web lint` |
| :--- | :--- | :--- |
| `export interface Dto` | `lib/api/go.ts` | exit 1, Type Contracts message |
| `export type Dto = { id: string }` | `lib/api/go.ts` | exit 1, Type Contracts message |
| `supabase.from("users")` | `lib/api/go.ts` | exit 1, ADR-001 message |
| `supabase.from("users")` | `app/page.tsx` | exit 1, ADR-001 message |
| `createClient().rpc("x")` | `app/page.tsx` | exit 1, ADR-001 message |
| `Array.from([1])` (must stay allowed) | `app/page.tsx` | exit 0 |

### AC4 — Go AST guardrails

```text
$ go test -v ./internal -run "TestNoRawChiRoutes|TestLayerImportsAndSQLBoundaries"
--- PASS: TestNoRawChiRoutes (0.00s)
--- PASS: TestLayerImportsAndSQLBoundaries (0.00s)
ok  	api/internal	0.009s
```

Negative tests used transient `zz_guard.go` files, since removed. Each run failed `api/internal`:

```text
../cmd/server/zz_guard.go:3: raw chi route .Get bypasses huma.Register
../cmd/server/zz_guard.go:4: raw chi route .Post bypasses huma.Register
../cmd/server/zz_guard.go:5: raw chi route .Put bypasses huma.Register
handler/zz_guard.go:2: handler must not import github.com/jackc/pgx/v5/pgxpool; depend on domain interfaces
handler/zz_guard.go:3: handler must not import database/sql; depend on domain interfaces
repository/zz_guard.go:3: raw SQL call .Exec; delegate to internal/repository/db (sqlc)
repository/zz_guard.go:3: fmt.Sprintf is banned in repositories; write SQL in sql/queries
repository/zz_guard.go:3: raw SQL call .QueryRow; delegate to internal/repository/db (sqlc)
```

The same chi fixture also contained `newRouter().Mount("/y", nil)`, which was **not** reported. See [Known limits](#known-limits).

### AC5 — CI

`grep -rn "no-frozen-lockfile" .github/workflows/` returns no matches. `ci.yml` holds `go build -o bin/server ./cmd/server` (L30), `pnpm install --frozen-lockfile` (L54, L95), `pnpm lint` (L57), and `pnpm --filter web build` (L66). The file parses as YAML. `pnpm install --frozen-lockfile --offline` exits 0 locally.

### AC6 — Gitignore

```text
$ git check-ignore --no-index apps/web/.env.example apps/api/.env.example
exit 1
$ git check-ignore --no-index apps/web/.env.local
apps/web/.env.local
exit 0
```

### AC7 — Regression gate

`pnpm codegen`, `pnpm lint`, `pnpm --filter web check-types`, `pnpm test`, and `pnpm build` all exit 0. Vitest: 4 passed. Go: `api/cmd/server`, `api/internal`, and `api/internal/handler` pass.

`sha256sum -c` against hashes taken before any edit reports no change for `apps/api/sql/**`, `apps/api/sqlc.yaml`, `apps/api/internal/repository/db/**`, `apps/api/go.mod`, `apps/api/go.sum`, `package.json`, `pnpm-lock.yaml`, `docs/plans/README.md`, `apps/web/.env.local`, `apps/api/openapi.json`, and `apps/web/lib/api/schema.d.ts`.

## Deviations from the Plan

1. **AC6 command.** `git check-ignore -v` exits 0 for a file matched by a negation rule, so the planned command can never exit 1 here. The check was run without `-v`.
2. **AC7 command.** `pnpm-lock.yaml` already had uncommitted changes, so the planned `git diff --exit-code` cannot be empty. Before/after hashes were used instead.
3. **ESLint globs.** The plan's `apps/web/**` globs would match nothing, because flat-config paths are relative to `apps/web`. They are written as `**/*` and `lib/api/**/*`.
4. **`no-restricted-syntax` in Block 2.** Flat config replaces rule options, so Block 2 repeats the Supabase selectors. Otherwise the ban would be lost inside `lib/api/`.
5. **Added ESLint blocks.** `.ts` and `.tsx` were not linted before this change; the new globs brought them in. That needed a `.tsx`-only `jsx` parser plugin, plus `no-undef` and `no-unused-vars` off for TypeScript, because the Babel parser does not record TypeScript types in scope. `tsc` remains the check for undefined names.
6. **`test-web` CI.** `pnpm lint` also runs `go vet` for `api`, so the job gained a `Setup Go` step. The `Run Lint` step was replaced by `Run Lint (All Workspaces)` rather than kept alongside it, because it would repeat the web lint.

## Known Limits

- **`@repo/ui` TypeScript is still not linted.** `eslint .` in `packages/ui` checks only `eslint.config.mjs`, not the `src` files. Plan item C5 expects `pnpm lint` to cover `@repo/ui`. Fixing it needs the same parser work in `packages/eslint-config/react-internal.js` or `packages/ui`, which the plan does not list.
- **The ESLint Supabase selectors match by name.** They miss an alias (`const db = createClient(); db.from(...)`), `supabase.storage.from(...)`, and `(await createClient()).from(...)`.
- **The chi check is syntax-only.** It recognises identifiers named `router`, and names bound to chi constructors or chi-typed declarations. It misses a router returned from a helper under another name or called directly, such as `newRouter().Mount(...)`. It also covers only the plan's eight methods, so `.Method`, `.MethodFunc`, `.Head`, `.Options`, `.Connect`, and `.Trace` are not checked.

## Not Verified

- **GitHub Actions.** `ci.yml` was not run on a runner.
- **`next build` without `apps/web/.env.local`.** The file exists locally and the plan forbids touching it, so a CI-like build was not reproduced. `lib/supabase.ts` has no importers in `app/`, `lib/`, or `__tests__/`, so the build is not expected to need those variables.
