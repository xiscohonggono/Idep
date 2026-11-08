# Guardrails & CI Hardening (Items A1–C6)

> **Status:** Implemented & Approved (see [`docs/reviews/guardrails-and-ci-hardening-review.md`](../../reviews/guardrails-and-ci-hardening-review.md) and [`docs/verification/guardrails-and-ci-hardening.md`](../../verification/guardrails-and-ci-hardening.md))
> **Scope:** Align agent rule discovery, decouple unapproved schema identifiers from active guardrails, enforce architectural boundaries via zero-dependency static checks (ESLint AST + Go `go/ast` test), and harden CI/gitignore hygiene.
> **Builder:** Claude Code via `delegate-to-claude`. **Reviewer:** Gemini/Architect → `docs/reviews/guardrails-and-ci-hardening-review.md`.
> **Related Contracts:** [`AGENTS.md`](file:///home/xiscohonggono/projects/Idep/AGENTS.md) · [`.agent/rules/project_guardrails.md`](file:///home/xiscohonggono/projects/Idep/.agent/rules/project_guardrails.md) · [`architectural_decisions.md`](file:///home/xiscohonggono/projects/Idep/docs/architecture/architectural_decisions.md) · [`stage-0-baseline-cleanup.md`](file:///home/xiscohonggono/projects/Idep/docs/plans/Implemented/stage-0-baseline-cleanup.md)

---

## 1. Objective & Guiding Constraints

Close the six gaps identified between Idep's documented engineering guardrails and its automated enforcement without introducing any new third-party dependencies or premature database decisions:

1. **Zero New Dependencies (Ponytail Discipline):** Use built-in ESLint core rules (`no-restricted-syntax`) and Go's standard library (`go/parser`, `go/ast`) rather than adding external linter binaries or npm packages.
2. **Zero Database Changes:** Do not touch `apps/api/sql/**`, `apps/api/sqlc.yaml`, or `apps/api/internal/repository/db/**`.
3. **Preserve Uncommitted User Work:** Do not touch `docs/plans/README.md` or `apps/web/.env.local`.

---

## 2. Work Items (A1–C6)

### A1. Agent Rule Discovery & Verification Ledger Alignment

**Problem:**
- [`CLAUDE.md`](file:///home/xiscohonggono/projects/Idep/CLAUDE.md) and [`GEMINI.md`](file:///home/xiscohonggono/projects/Idep/GEMINI.md) instruct agents to read [`AGENTS.md`](file:///home/xiscohonggono/projects/Idep/AGENTS.md), but [`AGENTS.md`](file:///home/xiscohonggono/projects/Idep/AGENTS.md) never links to [`.agent/rules/project_guardrails.md`](file:///home/xiscohonggono/projects/Idep/.agent/rules/project_guardrails.md) or [`docs/architecture/architectural_decisions.md`](file:///home/xiscohonggono/projects/Idep/docs/architecture/architectural_decisions.md). Because `.agent/rules/` is not automatically loaded by headless `claude -p`, the Builder agent can miss the 14 hard prohibitions and operational timeouts unless repeated in every plan.
- [`.agents/skills/delegate-to-claude/SKILL.md`](file:///home/xiscohonggono/projects/Idep/.agents/skills/delegate-to-claude/SKILL.md) omits recording verification evidence in `docs/verification/<feature-name>.md`, conflicting with [`.agent/rules/project_guardrails.md` §3](file:///home/xiscohonggono/projects/Idep/.agent/rules/project_guardrails.md#L61) (*"7. Verification Proof & Ledger Move (`docs/verification/` -> `docs/plans/Implemented/`)"*).

**Changes:**
1. **[`AGENTS.md`](file:///home/xiscohonggono/projects/Idep/AGENTS.md) (§6 Engineering Rules & Guardrails, L70–L76):**
   - Add Rule 5 pointing explicitly to [`.agent/rules/project_guardrails.md`](file:///home/xiscohonggono/projects/Idep/.agent/rules/project_guardrails.md) and [`docs/architecture/architectural_decisions.md`](file:///home/xiscohonggono/projects/Idep/docs/architecture/architectural_decisions.md) as mandatory reading before planning, building, or reviewing code.
2. **[`CLAUDE.md`](file:///home/xiscohonggono/projects/Idep/CLAUDE.md) & [`GEMINI.md`](file:///home/xiscohonggono/projects/Idep/GEMINI.md):**
   - Update line 2 in both files from:
     `Read and strictly follow all rules in ./AGENTS.md.`
     to:
     `Read and strictly follow all rules in ./AGENTS.md and ./.agent/rules/project_guardrails.md.`
   - In [`CLAUDE.md`](file:///home/xiscohonggono/projects/Idep/CLAUDE.md), require running `pnpm codegen`, `pnpm lint`, `pnpm --filter web check-types`, `pnpm test`, and `pnpm build` and recording verification evidence in `docs/verification/<feature-name>.md`.
3. **[`.agents/skills/delegate-to-claude/SKILL.md`](file:///home/xiscohonggono/projects/Idep/.agents/skills/delegate-to-claude/SKILL.md):**
   - In Step 2 (Delegate prompt), instruct Claude to also read `.agent/rules/project_guardrails.md` and write command verification output to `docs/verification/<feature-name>.md`.
   - In Step 3 (Review), instruct the Reviewer to audit the diff against `.agent/rules/project_guardrails.md` and verify `docs/verification/<feature-name>.md` before approving.

---

### A2. Decouple Unapproved Schema Identifiers in `.agent/rules/project_guardrails.md`

**Problem:**
- [`docs/plans/Pending/database-schema-proposal.md`](file:///home/xiscohonggono/projects/Idep/docs/plans/Pending/database-schema-proposal.md) is **Proposed — Pending approval (not implemented)** per Stage 0, yet [`.agent/rules/project_guardrails.md`](file:///home/xiscohonggono/projects/Idep/.agent/rules/project_guardrails.md) §1 and §2 reference proposed table and column identifiers (`chapter_versions`, `proposal_contributors`, `analogy_limits_json`, `audit_events`, `guidelines_accepted_at`, `claim_expires_at`, `role_title_snapshot`) as if the DDL were already finalized.

**Changes in [`.agent/rules/project_guardrails.md`](file:///home/xiscohonggono/projects/Idep/.agent/rules/project_guardrails.md):**
1. Add a callout banner immediately above §1 (`## 1. Clear "What Is Not Allowed" Rules`):
   > `[!IMPORTANT] Domain Invariants vs. Proposed Schema Identifiers: Domain invariants in §1 and §2 (Maker–Checker separation, append-only version history, mandatory governance reasons, grounded AI selections, private educator proofs, and transactional audit logging) are mandatory now. Specific SQL table and column identifiers shown in backticks (e.g., chapter_versions, audit_events) reflect docs/plans/Pending/database-schema-proposal.md and take effect as each schema stage is approved and implemented.`
2. Keep all 14 domain prohibitions and the 4-step Fail-Closed Governance Checklist intact so no business or security invariant is weakened.

---

### B3. Frontend Static Guardrails via ESLint (`packages/eslint-config` & `apps/web`)

**Problem:**
- [`packages/eslint-config/base.js`](file:///home/xiscohonggono/projects/Idep/packages/eslint-config/base.js#L5) loads `eslint-plugin-only-warn`, which silently downgrades every ESLint rule violation from `error` to `warn`.
- Nothing in ESLint mechanically blocks the two frontend prohibitions in [`.agent/rules/project_guardrails.md` §1](file:///home/xiscohonggono/projects/Idep/.agent/rules/project_guardrails.md#L15-L16):
  1. **Service & Data Boundaries:** Calling Supabase PostgREST/Storage data methods (`.from(...)` or `.rpc(...)`) directly from `apps/web` instead of routing through `apps/api`.
  2. **Type Contracts:** Hand-writing `interface` declarations inside `apps/web/lib/api/` outside the generated `schema.d.ts`.

**Changes:**
1. **[`packages/eslint-config/base.js`](file:///home/xiscohonggono/projects/Idep/packages/eslint-config/base.js):**
   - Remove `import onlyWarn from "eslint-plugin-only-warn";` and the `{ plugins: { onlyWarn } }` block so ESLint reports true errors as errors. (Keep `eslint-plugin-only-warn` in `packages/eslint-config/package.json` if avoiding lockfile churn is preferred, or leave `pnpm-lock.yaml` untouched—do not add or remove packages).
2. **[`apps/web/eslint.config.js`](file:///home/xiscohonggono/projects/Idep/apps/web/eslint.config.js):**
   - Extend `nextJsConfig` with two scoped guardrail blocks using built-in ESLint `no-restricted-syntax`:
     - **Block 1 (All `apps/web/**/*.{js,jsx,ts,tsx}` except `lib/api/schema.d.ts`):** Forbid `CallExpression` on member `.from()` or `.rpc()` targeted at a `supabase` identifier (`CallExpression[callee.type='MemberExpression'][callee.object.name=/supabase/i][callee.property.name=/^(from|rpc)$/]` and `CallExpression[callee.type='MemberExpression'][callee.object.type='CallExpression'][callee.object.callee.name='createClient'][callee.property.name=/^(from|rpc)$/]`) with message: `"Guardrail violation (ADR-001): Do not query PostgreSQL directly from apps/web via Supabase (.from/.rpc). Route all domain reads and mutations through apps/api (Huma OpenAPI)."`.
     - **Block 2 (`apps/web/lib/api/**/*.{ts,tsx}`, ignoring `apps/web/lib/api/schema.d.ts`):** Forbid `TSInterfaceDeclaration` and `TSTypeAliasDeclaration[typeAnnotation.type='TSTypeLiteral']` with message: `"Guardrail violation (Type Contracts): Do not hand-write DTO interfaces or object type literals in apps/web/lib/api/. Register Huma structs in apps/api, run pnpm codegen, and derive types from @/lib/api/schema."`.

---

### B4. Backend Static Guardrails via Go AST Test (`apps/api/internal/guardrails_test.go`)

**Problem:**
- `pnpm --filter api lint` runs `go vet ./...`, which does not check the three backend architectural rules in [`.agent/rules/project_guardrails.md` §1](file:///home/xiscohonggono/projects/Idep/.agent/rules/project_guardrails.md#L16-L18):
  1. **Huma SSOT:** No uncontracted Chi route mounts (`router.Get`, `router.Post`, `router.Put`, `router.Delete`, `router.Patch`, `router.Mount`, `router.Handle`, `router.HandleFunc`) bypassing `huma.Register`.
  2. **Compiled SQL Only (`sqlc`):** No raw SQL execution or `database/sql` usage in `internal/handler` or `internal/service`, and no raw SQL string methods (`Query`, `QueryRow`, `Exec` on `pgxpool.Pool` / `pgx.Conn`) in `internal/repository` outside the `sqlc`-generated `internal/repository/db` package.
  3. **No Direct `pgx` Driver Imports in Handlers/Services:** `internal/handler` and `internal/service` must depend on domain interfaces, never importing `github.com/jackc/pgx/v5*` or `database/sql` directly.

**Changes:**
1. **New [`apps/api/internal/guardrails_test.go`](file:///home/xiscohonggono/projects/Idep/apps/api/internal/guardrails_test.go)** (runs automatically on every `go test ./...` and `pnpm test`, using only Go standard library `go/ast`, `go/parser`, `go/token`, `path/filepath`, `os`, `strings`, `testing`):
   - **`TestNoRawChiRoutes`**: Parses all non-test `.go` files under `apps/api/cmd/` and `apps/api/internal/`. Fails if any `chi.Mux` / `router` variable invokes `.Get`, `.Post`, `.Put`, `.Delete`, `.Patch`, `.Mount`, `.Handle`, or `.HandleFunc`.
   - **`TestLayerImportsAndSQLBoundaries`**:
     - Walks non-test `.go` files in `internal/handler`, `internal/service`, and `internal/domain`: fails if any file imports `"database/sql"` or `"github.com/jackc/pgx/..."`.
     - Walks non-test `.go` files in `internal/repository` (excluding `internal/repository/db`): fails if any file imports `"database/sql"` or calls `.Query(`, `.QueryRow(`, `.Exec(`, or `fmt.Sprintf(` to construct SQL instead of delegating to `internal/repository/db` (`sqlc`).

---

### C5. CI Build Verification & Frozen Lockfile Enforcement (`.github/workflows/ci.yml`)

**Problem:**
- [`AGENTS.md` §6 Rule 3](file:///home/xiscohonggono/projects/Idep/AGENTS.md#L75) requires: *"All Go builds, tests, and Vitest suites must pass on Node 22 and Go 1.26 runners before merging to `main`."*
- Currently, [`.github/workflows/ci.yml`](file:///home/xiscohonggono/projects/Idep/.github/workflows/ci.yml):
  1. Uses `pnpm install --no-frozen-lockfile` in `test-web` (L47) and `codegen-drift` (L85), allowing lockfile drift on PRs.
  2. Never runs `go build` in `test-api` or `pnpm --filter web build` (`next build`) in `test-web`.

**Changes in [`.github/workflows/ci.yml`](file:///home/xiscohonggono/projects/Idep/.github/workflows/ci.yml):**
1. **`test-api` job:** Add `go build -o bin/server ./cmd/server` after `go test -v ./...`.
2. **`test-web` job:**
   - Change `pnpm install --no-frozen-lockfile` → `pnpm install --frozen-lockfile`.
   - Add a `Run Lint (All Workspaces)` step (`pnpm lint` so `@repo/ui` is also linted in CI alongside `web`).
   - Add a `Build Web Bundle` step (`pnpm --filter web build`) after `pnpm --filter web test`.
3. **`codegen-drift` job:**
   - Change `pnpm install --no-frozen-lockfile` → `pnpm install --frozen-lockfile`.

---

### C6. `.env.example` Unignore Rule in `.gitignore` and `apps/web/.gitignore`

**Problem:**
- [`apps/web/.gitignore`](file:///home/xiscohonggono/projects/Idep/apps/web/.gitignore#L29) contains `.env*` without `!.env.example`, which blocks git from staging changes to `apps/web/.env.example` without `git add -f`, conflicting with [`AGENTS.md` §6 Rule 4](file:///home/xiscohonggono/projects/Idep/AGENTS.md#L76) (*"Store development fallbacks in `.env.example`"*).

**Changes:**
1. **[`apps/web/.gitignore`](file:///home/xiscohonggono/projects/Idep/apps/web/.gitignore) (after `.env*` at L29):**
   - Add `!.env.example`.
2. **[`.gitignore`](file:///home/xiscohonggono/projects/Idep/.gitignore) (after `.env.production.local` at L13):**
   - Add `!.env.example` for explicit symmetry across root and workspace gitignores.

---

## 3. Files Touched Summary

| Item | Action | File Path |
| :---: | :--- | :--- |
| **A1** | edit | [`AGENTS.md`](file:///home/xiscohonggono/projects/Idep/AGENTS.md), [`CLAUDE.md`](file:///home/xiscohonggono/projects/Idep/CLAUDE.md), [`GEMINI.md`](file:///home/xiscohonggono/projects/Idep/GEMINI.md), [`.agents/skills/delegate-to-claude/SKILL.md`](file:///home/xiscohonggono/projects/Idep/.agents/skills/delegate-to-claude/SKILL.md) |
| **A2** | edit | [`.agent/rules/project_guardrails.md`](file:///home/xiscohonggono/projects/Idep/.agent/rules/project_guardrails.md) |
| **B3** | edit | [`packages/eslint-config/base.js`](file:///home/xiscohonggono/projects/Idep/packages/eslint-config/base.js), [`apps/web/eslint.config.js`](file:///home/xiscohonggono/projects/Idep/apps/web/eslint.config.js) |
| **B4** | new | `apps/api/internal/guardrails_test.go` |
| **C5** | edit | [`.github/workflows/ci.yml`](file:///home/xiscohonggono/projects/Idep/.github/workflows/ci.yml) |
| **C6** | edit | [`.gitignore`](file:///home/xiscohonggono/projects/Idep/.gitignore), [`apps/web/.gitignore`](file:///home/xiscohonggono/projects/Idep/apps/web/.gitignore) |
| **Guard** | **must NOT change** | `apps/api/sql/**`, `apps/api/sqlc.yaml`, `apps/api/internal/repository/db/**`, `apps/api/go.mod`, `apps/api/go.sum`, `package.json`, `pnpm-lock.yaml`, `docs/plans/README.md`, `apps/web/.env.local` |

---

## 4. Acceptance Criteria (to be verified in `docs/verification/guardrails-and-ci-hardening.md`)

1. **Agent Rule Links (A1):** `grep -rn "project_guardrails.md" AGENTS.md CLAUDE.md GEMINI.md .agents/skills/delegate-to-claude/SKILL.md` returns matches in all 4 files; `SKILL.md` references `docs/verification/`.
2. **Schema Status Clarity (A2):** `.agent/rules/project_guardrails.md` contains the `[!IMPORTANT]` note distinguishing active domain invariants from proposed schema identifiers in `docs/plans/Pending/database-schema-proposal.md`.
3. **ESLint Guardrails Active (B3):**
   - `grep -rn "onlyWarn" packages/eslint-config/base.js` returns no matches.
   - `pnpm lint` exits `0` on the clean repository.
   - Negative test (verified transiently during test run without leaving dirty files): a `TSInterfaceDeclaration` in `apps/web/lib/api/go.ts` or a `supabase.from('users')` call in `apps/web/app/page.tsx` causes `pnpm --filter web lint` to exit non-zero with the custom guardrail error message.
4. **Go AST Guardrails Active (B4):**
   - `go test -v ./internal -run "TestNoRawChiRoutes|TestLayerImportsAndSQLBoundaries"` in `apps/api` passes (`exit 0`).
5. **CI & Lockfile Hardening (C5):**
   - `grep -rn "no-frozen-lockfile" .github/workflows/` returns no matches; `frozen-lockfile`, `go build`, and `pnpm --filter web build` are present in `.github/workflows/ci.yml`.
6. **Gitignore Check (C6):**
   - `git check-ignore -v --no-index apps/web/.env.example apps/api/.env.example` exits `1` (neither file is ignored by git), while `git check-ignore -v --no-index apps/web/.env.local` exits `0` (still ignored).
7. **Full-Stack Regression Gate:**
   - `pnpm codegen`, `pnpm lint`, `pnpm --filter web check-types`, `pnpm test`, and `pnpm build` all exit `0`.
   - `git diff --exit-code -- apps/api/sql apps/api/sqlc.yaml apps/api/internal/repository/db pnpm-lock.yaml` is empty.

---

## 5. Rollback Plan

All changes are documentation, linter configuration, unit test, CI workflow, and `.gitignore` updates with zero database or runtime state impact. Rollback is a single `git revert` of the implementation commit.

