STATUS: APPROVED

# Architectural & Code Review — Stage 0 Baseline Cleanup

> **Plan Reviewed:** [`docs/plans/Implemented/stage-0-baseline-cleanup.md`](../plans/Implemented/stage-0-baseline-cleanup.md) (Revision 4)
> **Verification Evidence:** [`docs/verification/stage-0-baseline-cleanup.md`](../verification/stage-0-baseline-cleanup.md)
> **Reviewer:** Gemini (Architect & Reviewer)
> **Date:** 2026-11-01

---

## 1. Executive Summary

The implementation cleanly satisfies all 10 acceptance criteria in Revision 4 of the Stage 0 Baseline Cleanup plan:
- **Zero Premature Database Decisions:** `apps/api/sql/**`, `apps/api/sqlc.yaml`, and `apps/api/internal/repository/db/**` have zero diff (`git diff --exit-code` passed). The database schema and staging documents were moved from `docs/architecture/` to `docs/plans/Pending/database-schema-proposal.md` and `docs/plans/Pending/database-staging-proposal.md` with status updated to `Proposed — Pending approval (not implemented)`.
- **Huma Single Source of Truth (SSOT):** `/api/v1/users/{id}` is registered via `huma.Register` (`get-user`), OpenAPI export runs deterministically before database pool attachment, hand-written frontend DTOs (`apps/web/lib/api/types.ts`) and the client-side Supabase mock (`apps/web/lib/api/supabase.ts`) are deleted, and `apps/web/lib/api/go.ts` derives `User` directly from `@/lib/api/schema`.
- **Fail-Closed Backend Governance:** `PostgresUserRepository.GetByID` returns `domain.ErrDatabaseUnavailable` (`503`) when the database pool is nil or unreachable and `domain.ErrUserNotFound` (`404`) on `pgx.ErrNoRows`, enforcing the 5-second OLTP query timeout without leaking raw connection strings on invalid `DATABASE_URL` parse errors.
- **Bootstrap & Toolchain Alignment:** Go version references across CI, `Dockerfile`, `AGENTS.md`, `README.md`, and ADRs match `apps/api/go.mod` (`1.26`), Tailwind CSS v4 is installed and verified in `apps/web`, TypeScript is unified on `^5.7.0` so `openapi-typescript` runs cleanly under `pnpm codegen`, and `apps/web/lib/supabase.ts` remains untouched.

---

## 2. Guardrail & Invariant Checklist

| Guardrail / Requirement | Status | Evidence |
| :--- | :---: | :--- |
| **No Database Schema/Query Changes** | PASS | `git diff --exit-code -- apps/api/internal/repository/db apps/api/sql apps/api/sqlc.yaml` is empty. |
| **Huma OpenAPI 3.1 Single Source of Truth** | PASS | `apps/api/openapi.json` and `apps/web/lib/api/schema.d.ts` contain `/health` and `/api/v1/users/{id}` (`get-user` + `User` schema), and CI enforces a `codegen-drift` gate. |
| **No Hand-Written Frontend DTOs** | PASS | `apps/web/lib/api/types.ts` and `apps/web/lib/api/supabase.ts` deleted; `apps/web/lib/api/go.ts` imports `paths` from `@/lib/api/schema`. |
| **Fail-Closed Error Handling** | PASS | `"Demo User"` fallback removed; `humatest` unit tests (`user_handler_test.go`) and live curl checks verify `503` on nil DB and `404` on missing user. |
| **Preserve Uncommitted User Work** | PASS | User's edit to `docs/plans/README.md` and `apps/web/.env.local` were preserved without modification. |
| **Full-Stack Lint, Typecheck, Test & Build** | PASS | `pnpm codegen`, `pnpm lint`, `pnpm --filter web check-types`, `pnpm test`, and `pnpm build` all exit `0`. |

---

## 3. Residual Notes for Future Plans

1. **Unlayered CSS Reset in `apps/web/app/globals.css`:** The bootstrap `* { padding: 0; margin: 0; }` reset is currently outside `@layer base`, which overrides Tailwind v4 spacing utilities (`p-*`, `m-*`). When Phase 1 UI work begins, move base resets inside `@layer base` or rely on Tailwind's built-in Preflight reset.
2. **`apps/web/.gitignore` `.env*` Pattern:** If `apps/web/.env.example` needs to be committed in git, add `!.env.example` under `.env*` in `.gitignore` and `apps/web/.gitignore`.

