# Implementation Ledger (`docs/plans/`)

This directory serves as the project's implementation ledger for tracking project delivery status, evidence, boundaries, and remaining work, rather than describing the runtime behavior of the application itself.

These plans maintain a clear, authoritative index of:
- What was implemented
- What is still pending
- What was verified
- What remains outside scope or needs external handoff

---

## Directory Structure

### [`Implemented/`](file:///home/xiscohonggono/projects/Idep/docs/plans/Implemented/)
Documents work that is already complete:
- **Recorded Scope**: Preserves the exact boundaries agreed upon during implementation.
- **Verification Evidence**: Retains test results, validation steps, and rollout proofs.
- **Exclusions & Residual Risks**: Explicitly notes items excluded from scope or requiring ongoing attention.
- **Historical Execution Record**: Serves as an immutable audit trail of delivered features.

### [`Pending/`](file:///home/xiscohonggono/projects/Idep/docs/plans/Pending/)
Documents work that is proposed, not started, or still incomplete:
- **Explicit Status Tracking**: Provides clarity on where each initiative stands.
- **State Distinctions**: Clearly differentiates work that is "approved / in progress" from work where "just prerequisites completed".
- **Closure Criteria**: Records exact prerequisites, acceptance criteria, and remaining steps required before closure and migration to `Implemented/`.

### [`Archived/`](file:///home/xiscohonggono/projects/Idep/docs/plans/Archived/)
Documents plans that were superseded or merged during roadmap reconciliation:
- **Historical Reference**: Retains original planning proposals, review logs, and audit trails without cluttering active execution queues.

---

## Active Product Roadmap & Plan Sequence

| Phase / Initiative | Implementation Plan File | Scope & Stories Unlocked | Status |
| :--- | :--- | :--- | :--- |
| **Foundational Contracts** | [`database-schema-proposal.md`](Pending/database-schema-proposal.md)<br>[`database-staging-proposal.md`](Pending/database-staging-proposal.md) | 22-table PostgreSQL schema contract & Stages 0–4 zero-downtime rollout rules. | Approved Specification |
| **Stage 0** *(Identity & RBAC)* | [`stage-0-identity-and-rbac.md`](Pending/stage-0-identity-and-rbac.md) | `001_identity_and_rbac.sql`, Goose CLI runner, `users (id UUID)`, composable roles, and audit trail. | Ready for Execution |
| **Phase 1 MVP** *(Curriculum Reader & Metering)* | [`phase-1-curriculum-reader-and-guest-metering.md`](Pending/phase-1-curriculum-reader-and-guest-metering.md) | `002_work_experience.sql` & `003_curriculum_and_guests.sql`: 4 GED subjects, reader, 10 trades, 3-chapter guest gate (`U-01`, `U-03`, `U-13`). | Ready for Execution |
| **Phase 2 MVP** *(Maker–Checker Governance)* | [`phase-2-maker-checker-editorial-loop.md`](Pending/phase-2-maker-checker-editorial-loop.md) | `004_proposals_and_revisions.sql` & `005_checker_governance_and_notifications.sql`: 48h claims, side-by-side diffs, two-person integrity (`C-06`), approval/rejection (`E-01`, `E-03`, `E-07`, `E-08`, `C-01`–`C-06`, `C-09`, `C-10`). | Ready for Execution |
| **Phase 3 MVP** *(AI Sidebar & Active Recall)* | [`phase-3-ai-study-sidebar-and-active-recall.md`](Pending/phase-3-ai-study-sidebar-and-active-recall.md) | `006_study_highlights_and_ai.sql`: Grounded analogy generation, boundary warnings, elaborations, feedback flywheel, and active recall evaluation (`U-04`–`U-10`). | Ready for Execution |
| **Phase 4** *(Fast-Follow & Editorial Upgrades)* | [`phase-4-editorial-upgrades-and-fast-follow.md`](Pending/phase-4-editorial-upgrades-and-fast-follow.md) | `007_educator_verification.sql` & `008_citations_comments_and_reports.sql`: Educator badges (`E-02`), private drafts (`E-04`), citations (`E-05`), new chapters (`E-06`), line comments (`C-07`), rollbacks (`C-08`), 3-way merge (`E-09`), study history (`U-11`), quality flags (`U-12`). | Ready for Execution |
| **Post-Phase 4 AI Upgrade** | [`rag-implementation.md`](Pending/rag-implementation.md) | `009_rag_knowledge_pipeline.sql`: `pgvector` dual-corpus RAG over O*NET tasks and GED standards. | Scheduled post-Phase 4 |
| **Curriculum Bootstrapping** | [`curriculum-content-bootstrapping.md`](Pending/curriculum-content-bootstrapping.md) | Ingestion and seeding pipeline for foundational GED content across 4 subjects. | Ready for Execution |
| **Production & Cloud Ops** | [`production-deployment-and-observability.md`](Pending/production-deployment-and-observability.md) | Docker packaging, Vercel + Go API topology, PgBouncer pooling, zero-downtime CD rollout, health probes. | Ready for Execution |
| **Roadmap Hardening** | [`plan-reconciliation-and-roadmap-hardening.md`](Implemented/plan-reconciliation-and-roadmap-hardening.md) | Cross-plan consistency, migration numbering alignment, schema uniformity, and blast-radius guard tables. | Implemented & Verified |
| **SDLC & Quality Gates** | [`static-dynamic-analysis-and-sdlc-integration.md`](Pending/static-dynamic-analysis-and-sdlc-integration.md) | SAST (`gosec`, `golangci-lint`), DAST fuzzing, `govulncheck`, and typegen gates. | Ready for Execution |


