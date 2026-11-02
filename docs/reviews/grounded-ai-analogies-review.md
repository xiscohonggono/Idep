STATUS: APPROVED

# Architectural & Code Review — Grounded AI Profession Analogies, Elaborations & Feedback Flywheel

> **Plan Reviewed:** [`docs/plans/Implemented/grounded-ai-analogies.md`](../plans/Implemented/grounded-ai-analogies.md)  
> **Verification Evidence:** [`docs/verification/grounded-ai-analogies.md`](../verification/grounded-ai-analogies.md)  
> **Reviewer:** Gemini (Architect & Reviewer)  
> **Date:** 2026-11-03

---

## 1. Executive Summary (`ponytail-review` + Guardrails Audit)

**What this change does:** Implements Stage 1 prerequisite migrations (`002_work_experience.sql`, `003_curriculum_and_guests.sql`) and Stage 3 AI migrations (`006_study_highlights_and_ai.sql`), compiles 16 `sqlc` queries (`apps/api/sql/queries/analogies.sql`), adds the OpenAI `gpt-4.1-mini` structured-output adapter (`apps/api/internal/llm/openai.go`) and `AnalogyService` (`apps/api/internal/service/analogy_service.go`), exposes four Huma OpenAPI 3.1 endpoints (`POST /api/v1/chapters/{chapter_id}/explanations`, `GET /api/v1/highlights/{highlight_id}/explanations`, `POST /api/v1/explanations/{explanation_id}/elaborate`, `PUT /api/v1/explanations/{explanation_id}/feedback`), and ships the Next.js `AnalogyStudySidebar` UI (`apps/web/components/AnalogyStudySidebar.tsx`).

All 5 acceptance criteria (**AC1–AC5**) and all guardrails in [`.agent/rules/project_guardrails.md`](../../.agent/rules/project_guardrails.md) pass both offline and against a live PostgreSQL 16.15 instance:

- **Schema & Invariants (`002`, `003`, `006`):** Enforces `SET lock_timeout = '3s'; SET statement_timeout = '30s';` in every `-- +goose Up` and `-- +goose Down` block, seeds the 10 `job_fields` and 4 GED `subjects`, blocks mutation of `chapter_versions` via `trg_chapter_versions_immutable`, enforces paragraph/character bounds (`1..3` paragraphs, `15..3000` chars) on `study_highlights`, requires `jsonb_array_length(analogy_limits_json) >= 1` (`U-07`) and `chk_work_field_or_fallback` on `ai_explanations`, enforces `chk_elaboration_target_index` and `chk_elaboration_content_or_hint` on `ai_elaborations`, and enforces `rating IN ('like', 'dislike')`, `comment` length `1..500`, and `UNIQUE (explanation_id, user_id)` on `ai_explanation_feedback`.
- **Grounded AI Service & Security Hardening ([`apps/api/internal/service/analogy_service.go`](../../apps/api/internal/service/analogy_service.go), [`apps/api/internal/llm/openai.go`](../../apps/api/internal/llm/openai.go)):**
  - Verifies caller existence in `users` (`UserExists`) before any LLM call so an unknown `X-User-ID` returns `401 Unauthorized` without bypassing the `30/24h` quota or burning OpenAI credits.
  - Enforces ownership (`403 Forbidden`) across `ListHighlightExplanations`, `Elaborate`, and `UpsertFeedback`.
  - Strips hallucinated or subsequently archived chapter links from `terms` (`U-05`) and `prerequisites` (`U-08`) on both write and read (`hydrate`).
  - Sanitizes untrusted learner inputs (`role_title`, `user_prompt`, liked exemplar steps/limits, and dislike comments) via `domain.SanitizeUntrusted` (stripping `<`, `>`, and control characters) and wraps them in `<untrusted_learner_profile>`, `<untrusted_community_feedback>`, and `<untrusted_learner_prompt>` delimiters.
  - Enforces the `15s` hard context deadline with `option.WithMaxRetries(0)` on the OpenAI client and at most `1` schema-repair retry (`maxAttempts = 2`).
- **Frontend & End-to-End Type Safety ([`apps/web/lib/api/go.ts`](../../apps/web/lib/api/go.ts), [`apps/web/components/AnalogyStudySidebar.tsx`](../../apps/web/components/AnalogyStudySidebar.tsx)):** Imports all request/response types directly from generated `apps/web/lib/api/schema.d.ts`, routes interactive mutations through Next.js Server Actions in `apps/web/app/page.tsx`, fixes the unlayered CSS reset in `apps/web/app/globals.css` (`@layer base`), and passes 20 Vitest tests across 2 test suites with zero new npm dependencies.

---

## 2. Guardrail & Acceptance Checklist

| AC | Requirement | Status | Evidence |
| :---: | :--- | :---: | :--- |
| **1** | `002`, `003`, and `006` migrations have `-- +goose Up/Down` and timeout `SET`s; clean `up -> down -> up` | PASS | Verified in `apps/api/sql/schema/00[236]*.sql` and live PG 16.15 `migrate up -> down (x4) -> up` in `docs/verification/grounded-ai-analogies.md`. |
| **2** | Deterministic offline `pnpm codegen` with zero `pgtype` leaks | PASS | 3 consecutive `env -u DATABASE_URL -u OPENAI_API_KEY pnpm codegen` runs produced identical SHA-256 (`1a3f3e4fe81029b5...`) and zero `pgtype` matches. |
| **3** | Strict architecture & SQL boundaries (`TestNoRawChiRoutes`, `TestLayerImportsAndSQLBoundaries`) | PASS | Both tests pass; `guardrails_test.go`, `001_identity_and_rbac.sql`, `apps/web/package.json`, and `security.yml` have zero diff. |
| **4** | Offline unit tests covering all domain invariants (`U-04`–`U-08`, best-fit, elaborations, feedback flywheel, rate limit) | PASS | Verified across `analogy_service_test.go`, `openai_test.go`, and `analogy_handler_test.go`. |
| **5** | Full workspace lint, typecheck, test, and build gate | PASS | `pnpm lint`, `pnpm --filter web check-types`, `pnpm test` (92 Go PASS lines + 20 Vitest tests), and `pnpm build` all exit `0`. |

---

## 3. Residual Notes for Future Stages (Non-Blocking)

1. **Migration Numbering Before Stage 2 (`004`, `005`):** Because `006_study_highlights_and_ai.sql` shipped ahead of Stage 2 (`004_proposals_and_revisions.sql`, `005_checker_governance_and_notifications.sql`), when Stage 2 is planned we should either enable `goose.WithAllowOutofOrder(true)` in `cmd/migrate` and `internal/dbtest` or number the Stage 2 migrations `007` and `008`.
2. **Stage 1 JWT Authentication Replacement:** `CallerHeader` (`X-User-ID`) in `apps/api/internal/handler/analogy_handler.go` and `?user=&highlight=` in `apps/web/app/page.tsx` are marked with `// shortcut:` comments and must be replaced by the Supabase Bearer JWT middleware before public deployment.
3. **`job_fields` Deactivation vs. Deletion:** Because `ai_explanations` enforces `chk_work_field_or_fallback` (`is_everyday_fallback = FALSE AND job_field_code IS NOT NULL`), retiring a job field in production must always use `UPDATE job_fields SET is_active = FALSE` rather than `DELETE FROM job_fields`.

**Verdict:** Ship.

