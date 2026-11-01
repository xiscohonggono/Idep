STATUS: APPROVED

# Architectural & Code Review — Guardrails & CI Hardening (Items A1–C6)

> **Plan Reviewed:** [`docs/plans/Implemented/guardrails-and-ci-hardening.md`](../plans/Implemented/guardrails-and-ci-hardening.md)
> **Verification Evidence:** [`docs/verification/guardrails-and-ci-hardening.md`](../verification/guardrails-and-ci-hardening.md)
> **Reviewer:** Gemini (Architect & Reviewer)
> **Date:** 2026-11-01

---

## 1. Executive Summary

All six items (**A1–C6**) and all seven acceptance criteria are implemented cleanly with zero new third-party dependencies, zero database schema/query changes, and zero modifications to uncommitted user work (`docs/plans/README.md`, `apps/web/.env.local`):

- **A1 (Agent Rule & Ledger Discovery):** [`AGENTS.md`](../../AGENTS.md), [`CLAUDE.md`](../../CLAUDE.md), [`GEMINI.md`](../../GEMINI.md), and [`.agents/skills/delegate-to-claude/SKILL.md`](../../.agents/skills/delegate-to-claude/SKILL.md) explicitly mandate reading [`.agent/rules/project_guardrails.md`](../../.agent/rules/project_guardrails.md) and recording command verification evidence in `docs/verification/<feature-name>.md`.
- **A2 (Schema Identifier Decoupling):** [`.agent/rules/project_guardrails.md`](../../.agent/rules/project_guardrails.md) now includes an explicit `[!IMPORTANT]` boundary note above §1 separating mandatory domain invariants from proposed SQL table/column identifiers in [`docs/plans/Pending/database-schema-proposal.md`](../plans/Pending/database-schema-proposal.md).
- **B3 (Frontend Static Guardrails via ESLint):** [`packages/eslint-config/base.js`](../../packages/eslint-config/base.js) removes `eslint-plugin-only-warn`, and [`apps/web/eslint.config.js`](../../apps/web/eslint.config.js) enforces `no-restricted-syntax` across `.ts`/`.tsx`/`.js`/`.jsx` files—blocking direct Supabase `.from()`/`.rpc()` calls (`ADR-001`) and hand-written `interface` or object-literal DTO `type` aliases inside `apps/web/lib/api/` outside `schema.d.ts`.
- **B4 (Backend Static Guardrails via Go AST):** [`apps/api/internal/guardrails_test.go`](../../apps/api/internal/guardrails_test.go) uses standard-library `go/ast` and `go/parser` to fail `go test ./...` if raw Chi route registration (`Get`, `Post`, `Put`, `Delete`, `Patch`, `Mount`, `Handle`, `HandleFunc`), `database/sql` / `pgx` imports in `handler`/`service`/`domain`, or raw SQL calls (`Query`, `QueryRow`, `Exec`, `fmt.Sprintf`) in `repository` outside `internal/repository/db` are introduced.
- **C5 (CI Build & Lockfile Hardening):** [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml) enforces `pnpm install --frozen-lockfile`, runs `pnpm lint` across all workspaces (with Go setup in `test-web` so `api#lint` succeeds), compiles the Go server binary (`go build -o bin/server ./cmd/server`), and runs `pnpm --filter web build`.
- **C6 (`.env.example` Unignore):** Both [`.gitignore`](../../.gitignore) and [`apps/web/.gitignore`](../../apps/web/.gitignore) include `!.env.example`, allowing `apps/web/.env.example` and `apps/api/.env.example` to be staged cleanly while keeping `.env.local` ignored.

---

## 2. Guardrail & Acceptance Checklist

| AC | Requirement | Status | Evidence |
| :---: | :--- | :---: | :--- |
| **1** | Agent rule links & `docs/verification/` in `SKILL.md` | PASS | Verified in `AGENTS.md:77`, `CLAUDE.md:2-3`, `GEMINI.md:2`, and `.agents/skills/delegate-to-claude/SKILL.md:19,24`. |
| **2** | Schema status clarity in `.agent/rules/project_guardrails.md` | PASS | `[!IMPORTANT]` callout added at `.agent/rules/project_guardrails.md:9-10`. |
| **3** | ESLint guardrails active + negative tests | PASS | `onlyWarn` removed from `base.js`; injected `interface Dto`, `type Dto = { ... }`, `supabase.from(...)`, and `createClient().rpc(...)` all failed `pnpm --filter web lint` with the expected guardrail messages. |
| **4** | Go AST guardrails active + negative tests | PASS | `TestNoRawChiRoutes` and `TestLayerImportsAndSQLBoundaries` pass on clean code and caught injected violations across `cmd/server`, `handler`, and `repository`. |
| **5** | CI build & `--frozen-lockfile` enforcement | PASS | `no-frozen-lockfile` removed; `ci.yml` includes `go build`, `pnpm lint`, and `pnpm --filter web build`. |
| **6** | `.env.example` unignored while `.env.local` stays ignored | PASS | `git check-ignore --no-index apps/web/.env.example apps/api/.env.example` exits `1`; `apps/web/.env.local` exits `0`. |
| **7** | Full-stack regression gate & zero DB/lockfile drift | PASS | `pnpm codegen`, `pnpm lint`, `pnpm --filter web check-types`, `pnpm test`, and `pnpm build` all exit `0` with zero changes to guarded files. |

---

## 3. Residual Notes (Non-Blocking)

1. **`@repo/ui` TypeScript ESLint Coverage:** `packages/eslint-config/react-internal.js` can be given the same `.tsx` JSX parser option in a future UI plan if custom ESLint rules are added for `packages/ui/src/*.tsx` (currently type-checked by `tsc --noEmit`).
2. **Syntax-Only Static Checks:** Both the ESLint `no-restricted-syntax` selectors and `guardrails_test.go` operate on file ASTs rather than full cross-package type resolution, providing fast zero-dependency guardrails complemented by code review.

