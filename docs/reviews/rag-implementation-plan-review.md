# Review — `rag-implementation.md` (Rev 4) vs. Idep Stack, Dependencies & Business Fit

> **Reviewed:** [`docs/plans/Pending/rag-implementation.md`](../plans/Pending/rag-implementation.md) (Rev 4, 2026-11-19)  
> **Reviewer:** Architect · **Date:** 2026-11-19  
> **Verdict:** `CHANGES REQUESTED` — architecture direction is sound (Postgres-only, pure Go, sqlc, RRF), but the plan has **4 correctness bugs**, **6 unstated dependencies**, and **over-builds relative to what `U-06`/`U-07`/`U-08` need**. Recommend gating on a Phase 3 eval baseline and shipping a smaller "RAG-lite" first.

Evidence base: `apps/api/go.mod`, `apps/api/sqlc.yaml`, `apps/api/sql/schema/001_users.sql`, `.github/workflows/ci.yml`, `apps/api/internal/guardrails_test.go`, `production-deployment-and-observability.md`, `database-schema-proposal.md`, `phase-3-ai-study-sidebar-and-active-recall.md`.

---

## 1. Stack Conformance (Go + PostgreSQL)

| Area | Plan Assumes | Repo Reality (verified) | Status |
| :--- | :--- | :--- | :--- |
| Go version | Go 1.26 | `go 1.26.8` | ✅ |
| DB driver / codegen | `pgx/v5` + `sqlc` | `pgx/v5 v5.11.0`, `sqlc.yaml` v2, `sql_package: pgx/v5`, CI pins `sqlc 1.31.1` | ✅ |
| HTTP contract | Huma OpenAPI 3.1 | `huma/v2 v2.39.1` + chi | ✅ |
| Raw SQL ban | All SQL in `sql/queries/*.sql` | Enforced by `guardrails_test.go` | ✅ plan complies |
| Migrations | goose `-- +goose Up/Down`, `cmd/migrate` | **No goose in `go.mod`, no `cmd/migrate`, only `001_users.sql`** | ⚠️ depends on Stage 0 |
| CI Postgres | "upgrade service image to `pgvector/pgvector:pg16`" | **`ci.yml` has no Postgres service at all** | ⚠️ nothing to upgrade yet |
| OpenAI SDK | `openai-go/v3` | Not in `go.mod` | ⚠️ depends on Phase 3 |
| Prod DB | "existing PostgreSQL 16" | Supabase PG16 via **PgBouncer transaction pooling (6543)** + `PreferSimpleProtocol` | ⚠️ affects `SET LOCAL` and vector encoding |
| Tokenizer | `tiktoken-go` `cl100k_base` "matches `gpt-4.1-mini`" | `gpt-4.1-*` uses **`o200k_base`**; `cl100k_base` only matches `text-embedding-3-small` | ❌ factual error |

**Conclusion:** The plan targets the right stack, but it is written as if Stage 0 → Phase 4 already exist. None of them do in code yet (`git log` head: plan reconciliation only).

---

## 2. Correctness Bugs (must fix before delegation)

### B1 — Lexical leg returns ~nothing for HyDE queries (AND semantics)
`websearch_to_tsquery` / `plainto_tsquery` **AND** every term. A 10–15 word HyDE string (*"commercial kitchen prep, food ingredients delivery, …"*) requires all lexemes in one 150–250 token chunk → empty `sparse_search` → RRF silently degrades to dense-only. Same for 1–3 paragraph academic passages on the curriculum leg.
**Fix:** OR the lexemes: `replace(plainto_tsquery('english', $2)::text, ' & ', ' | ')::tsquery` (still parameterised, sqlc-safe). Rank with `ts_rank_cd` as today.

### B2 — "BM25" is mislabelled
Postgres core has no BM25; `ts_rank_cd` is cover-density ranking without IDF. Fine for RRF (rank-only), but the benchmark/fallback language should say "lexical (`ts_rank_cd`)" so nobody later promises BM25 behaviour.

### B3 — `SET LOCAL hnsw.iterative_scan` under PgBouncer + sqlc
- Must run inside an explicit `pgx.Tx` (transaction pooling makes session `SET` leak across clients; `SET LOCAL` is safe only in a tx).
- Unverified whether sqlc 1.31 parses `SET LOCAL ...` as an `:exec` query; if not, it becomes inline SQL and **fails `guardrails_test.go`**.
- Requires `pgvector >= 0.8.0` on Supabase — **unverified**.
**Fix:** See §5 Rec-2 (exact scan for filtered legs removes the need entirely).

### B4 — `vector` type wiring incomplete
`sqlc.yaml` needs an override (`db_type: "vector"` → `github.com/pgvector/pgvector-go.Vector`) **and** the pool needs `pgxvec.RegisterTypes` in `AfterConnect`. With `PreferSimpleProtocol` vectors travel as text — works, but must be tested against PgBouncer, not only the CI container. Also `FULL OUTER JOIN` + `COALESCE` columns generate nullable `pgtype.*` fields, contradicting Phase 3's "no `pgtype` leaks" gate — add explicit casts (`::uuid`, `::text`).

### B5 (minor) — Ad-hoc score mixing
Exemplars use `1/(50+rank)` while vocational uses two-leg RRF `1/(60+r1)+1/(60+r2)` and both are `UNION ALL … ORDER BY rrf_score`. Scores from different formulas are not comparable; ordering is arbitrary. Return separate result sets (or a `slot` column) and let Go take top-N per slot — which is what §4.2 step 5 already describes.

---

## 3. Unstated Dependencies & Constraints

| # | Dependency / Constraint | Where it bites | Required action |
| :--- | :--- | :--- | :--- |
| D1 | **Stage 0** (goose, `cmd/migrate`, CI Postgres service, UUID `users`) | Migration `009` cannot run or be CI-tested | Hard prerequisite; list explicitly |
| D2 | **Phase 1** `002` (`job_fields`) + `003` (`chapters`) | FKs on `knowledge_chunks` | Hard prerequisite |
| D3 | **Phase 3** `openai-go/v3`, `analogy_service.go`, feedback tables | Hook point + exemplar promotion | Hard prerequisite |
| D4 | **Curriculum content bootstrapping** | Golden set for Ragas (100 passages) doesn't exist | Build golden set during Phase 3 |
| D5 | **Re-index on publish** | Plan says "PostgreSQL event trigger" — a trigger can't call OpenAI embeddings, and ADR-001 forbids orchestration in stored procedures | Go-side: on `C-04` publish, write an outbox row; worker embeds the new `chapter_version` |
| D6 | **API contract change** | Phase 3 says RAG needs "no schema/API changes", but Rev 1+ promises *cited source badges* → needs `citations` in the Huma response and a column on `ai_explanations` (e.g. `retrieved_chunk_ids UUID[]`) + `prompt_version` bump | Add to schema + Files Touched + codegen gate |

Other constraints to record:
- **Licensing:** O\*NET is **CC BY 4.0** (not public domain) and OpenStax is CC BY 4.0 → attribution is *mandatory*, so D6 (citations UI) is a legal requirement, not polish. The GED Assessment Guide is GEDTS copyright — "public guidance" ≠ redistributable; get confirmation before chunking/storing it.
- **Paid external calls:** Firecrawl ingestion and OpenAI batch embeddings are paid actions → need explicit approval per `AGENTS.md §1`. O\*NET (CSV) and eCFR/OSHA (has a public API) don't need Firecrawl at all.
- **`tiktoken-go` network fetch:** it downloads the BPE file on first use; offline CI requires `github.com/pkoukk/tiktoken-go-loader` (embedded offline loader).
- **Embedding versioning:** no `embedding_model` column → no safe way to re-embed/migrate models later. Add `embedding_model TEXT NOT NULL`.
- **Denormalised `source_type` drift:** enforce with composite FK `(source_id, source_type) → knowledge_sources(id, source_type)` (add `UNIQUE (id, source_type)`); zero-cost integrity.
- **Self-retrieval:** Idep chapters are in the curriculum corpus, so the highlighted passage will be the #1 hit, wasting a slot. Exclude `chapter_version_id = current` in the curriculum query.
- **"Never 503" wording:** contradicts fail-closed governance if generation itself fails. Scope it to "retrieval failures degrade, never 503"; generation failures still return `503`.
- **Supabase extension schema:** create with `CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA extensions;` and do **not** `DROP EXTENSION` in `Down` (shared, destructive).

---

## 4. Business-Feature Fit

| Story | What the business needs | Does Rev 4 deliver it best? | Better path |
| :--- | :--- | :--- | :--- |
| **U-06** Workplace analogy | Authentic trade workflows, not stereotypes | Partially. Vector RAG over O\*NET is heavy for what is a **small, closed, per-trade corpus** | O\*NET tasks for a job field's SOC codes ≈ 100–300 task statements. Lexical filter (or even direct inclusion of top-N) by `job_field_code` gives authenticity with no embeddings |
| **U-07** "Where this falls short" | Accurate science/math correction | **Not sourced.** Plan claims "misconception inventories" but no listed source provides them | Add a Checker-authored *Common misconceptions* section to chapters (fits Maker–Checker, Phase 2/4), retrieved deterministically by `chapter_id` |
| **U-08** Prerequisites | Valid links to earlier chapters | RAG is the wrong tool — non-deterministic | Use the existing `chapter_prerequisites` table + closed-world filter (already in Phase 3) |
| **U-05** Terms | Plain definitions + chapter links | Fine via closed-world catalog; RAG adds little | Keep Phase 3 approach |
| Exemplars | Reuse great analogies | **Duplicates Phase 3 flywheel**, which already fetches top-3 liked by exact `(chapter_id, job_field_code)` — more precise than cosine ≥ 0.85 | Drop `exemplar` partition from v1; revisit only for cross-chapter reuse |
| Latency p95 ≤ 6.5 s | Learners after shifts | At risk: HyDE (~1–1.5 s) + embed (~0.3 s) + DB + generation with ~4k extra tokens (~4–6 s) | Move HyDE off the request path (Rec-1) |

**Key insight:** highlights are always 1–3 paragraphs of an **immutable, published `chapter_version`** across **10 fixed job fields**. That makes query-side work precomputable — the plan treats it as open-ended search.

---

## 5. Recommendations (ordered by value / effort)

**Rec-0 — Gate on evidence.** During Phase 3, record the baseline: dislike rate per `(chapter, job_field)`, a 50–100 passage golden set, and LLM-judged vocational authenticity. Build RAG only if the baseline misses the target (≥ 0.88). Zero-shot `gpt-4.1-mini` already knows kitchens and warehouses; the marginal gain is unproven.

**Rec-1 — Precompute HyDE per paragraph × job field (removes 2–4 s from request path).** At publish time (`C-04` outbox worker), for each paragraph of the new `chapter_version`, generate the trade-agnostic mechanism once and the 10 trade queries + embeddings; store keyed by `(chapter_version_id, paragraph_index, job_field_code)`. Versions are append-only, so the cache never goes stale. Runtime = lookup the paragraphs overlapping `start_offset..end_offset`. Cost estimate: ~200 chapters × ~30 paragraphs × 10 trades ≈ 60k small calls (one-off, approve before running).

**Rec-2 — Right-size the vector indexes.** Vocational is filtered to one of 10 trades (~10% selectivity) and exemplars are tiny. At < 100k rows, an **exact scan** (`WHERE source_type='vocational' AND job_field_code=$2 ORDER BY embedding <=> $1 LIMIT 20`, served by the B-tree on `(source_type, job_field_code)`) gives 100% recall in single-digit ms and needs no `iterative_scan`/`SET LOCAL` (fixes B3). Keep partial HNSW only for `curriculum` (OpenStax volume justifies it). Add per-trade HNSW later only if `EXPLAIN ANALYZE` shows > 50 ms.

**Rec-3 — Fix B1/B4/B5 and add D6 citations, `embedding_model`, composite FK, self-retrieval exclusion.**

**Rec-4 — Ship "RAG-lite" first (smallest diff that serves U-06/U-07):**
1. `knowledge_chunks` with `tsvector` only for O\*NET tasks + Checker-authored misconceptions (no embeddings, no `pgvector`, no CI image change).
2. Lexical OR-query by `job_field_code` + `chapter_id`.
3. Citations in the response (licence compliance).
4. Measure against the Rec-0 baseline; add dense vectors + RRF (the current Rev 4 design, with fixes) only if lexical recall is the measured bottleneck.

---

## 6. What Rev 4 Gets Right (keep)

- Single PostgreSQL instance, no vector SaaS, no LangChain — matches ADR-001 and Ponytail.
- All retrieval in `sql/queries/rag.sql` via sqlc — passes `guardrails_test.go`.
- RRF $k=60$ as the fusion method (rank-based, scale-free).
- Parent–child chunking with token bounds (150–250 / 600–800) and table-header repetition.
- Explicit sub-timeouts inside the 15 s ceiling, retrieval-failure degradation.
- Offline-only Firecrawl; no request-time crawling.

---

## 7. Required Plan Edits Before `delegate-to-claude`

1. Prerequisites block: Stage 0, Phase 1 (`002`, `003`), Phase 3, curriculum bootstrapping, golden set.
2. Correct tokenizer note (`cl100k_base` = embeddings only; `o200k_base` for `gpt-4.1-mini` prompt budgeting) + offline loader.
3. Replace AND tsquery with OR tsquery (B1); rename "BM25" (B2).
4. Exact scan for vocational/exemplar legs; drop `SET LOCAL iterative_scan` (B3).
5. sqlc override + `pgxvec.RegisterTypes` + explicit casts (B4); per-slot results (B5).
6. Schema: `embedding_model`, composite FK for `source_type`, `ai_explanations.retrieved_chunk_ids`, extension in `extensions` schema, no `DROP EXTENSION`.
7. Replace "event trigger" with Go outbox worker on publish.
8. Decide: exemplar partition (drop vs. keep) and precomputed HyDE (Rec-1).
9. Licensing confirmations (GED guide) and paid-call approvals (Firecrawl, embeddings).
