# Production Deployment, Cloud Infrastructure & Observability Runbook

> **Status:** Proposed — Ready for Review  
> **Scope:** Containerization, cloud hosting topology (Vercel + Go API container on Railway/Fly.io/GCP), production Supabase PostgreSQL connectivity (PgBouncer/connection pooling), zero-downtime migration rollout, production environment configuration, health probes, and structured observability.  
> **Author:** Antigravity (Architect & Reviewer) · **Reviewer:** Human Engineer / Gemini  
> **Related Contracts:** [`architectural_decisions.md`](../../architecture/architectural_decisions.md) · [`database-staging-proposal.md`](database-staging-proposal.md) · [`database-schema-proposal.md`](database-schema-proposal.md) · [`.agent/rules/project_guardrails.md`](../../../.agent/rules/project_guardrails.md)

---

## 0. Revision History

| Rev | Date | Author | Description |
| :--- | :--- | :--- | :--- |
| 1 | 2026-11-09 | Antigravity (Architect) | Initial specification: Defines containerized production build for Go microservice, zero-downtime pre-deploy migration runner, production secret contracts, PgBouncer transaction-mode pool settings, and health/liveness probes. |

---

## 1. Objectives & Architecture Overview

While Idep's application layers (`apps/web` and `apps/api`) are designed for local Turborepo development and GitHub Actions CI, deploying Idep to production requires an explicit, deterministic operational runbook.

```mermaid
flowchart TD
    subgraph Edge["Client & Edge Tier"]
        Learner["Learner / Editor / Checker Browser"]
        VercelEdge["Next.js Web Frontend (Vercel)\n- Edge Routing & ISR\n- Supabase Auth Session Exchange"]
    end

    subgraph Service["Application & API Tier"]
        LB["Cloud Load Balancer / Ingress (HTTPS / TLS 1.3)"]
        GoAPI["Go 1.26 HTTP API Containers (Railway / Fly.io / GCP)\n- Huma OpenAPI 3.1 & Chi Router\n- Structured JSON Logging & OpenTelemetry\n- /healthz, /livez, /readyz Probes"]
    end

    subgraph Data["Persistence & External Services"]
        PgBouncer["Supabase PgBouncer (Port 6543, Transaction Mode)"]
        Postgres["PostgreSQL 16 Engine (Port 5432, Session Mode for DDL)"]
        OpenAI["OpenAI API (gpt-4.1-mini, text-embedding-3-small)"]
        S3Store["Private S3 / Cloudflare R2 Storage (Educator Proofs)"]
    end

    Learner -->|HTTPS| VercelEdge
    Learner -->|API Requests with Bearer JWT| LB
    VercelEdge -->|SSR API Calls| LB
    LB --> GoAPI
    GoAPI -->|Prepared Statements Disabled (Port 6543)| PgBouncer
    PgBouncer --> Postgres
    GoAPI -->|15s Hard Deadline| OpenAI
    GoAPI -->|Presigned URLs (TTL <= 300s)| S3Store
```

---

## 2. Infrastructure & Hosting Topology

### 2.1 Next.js Presentation Tier (`apps/web`)
- **Host:** Vercel (or Node.js 22 container).
- **Environment:**
  - `NEXT_PUBLIC_API_URL`: Public HTTPS URL of the Go backend API.
  - `NEXT_PUBLIC_SUPABASE_URL`: Public Supabase project URL.
  - `NEXT_PUBLIC_SUPABASE_ANON_KEY`: Supabase anonymous public client key.

### 2.2 Go Microservice Tier (`apps/api`)
- **Host:** Railway / Fly.io / GCP Cloud Run.
- **Packaging:** Multi-stage minimal scratch/distroless Docker container (`FROM gcr.io/distroless/static-debian12:nonroot`).
- **Process Management:** Runs non-root (`UID 65532`), gracefully handles `SIGTERM` with 15-second shutdown drain.

### 2.3 PostgreSQL & Migration Execution
- **Host:** Supabase Managed PostgreSQL 16 (`pgvector` enabled for Stage 5).
- **Dual-Port Routing Strategy:**
  1. **Application Runtime (Port 6543):** Connects via PgBouncer in transaction-pooling mode. `apps/api` sets `pgx.PreferSimpleProtocol` to avoid prepared statement naming collisions.
  2. **Migration Runner (Port 5432):** Direct session connection required for Goose DDL execution (`cmd/migrate up`), advisory locking, and table schema alterations.

---

## 3. Zero-Downtime Continuous Deployment (CD) Pipeline

To guarantee that application containers never crash due to schema drift during rollout:

```mermaid
sequenceDiagram
    participant CI as GitHub Actions (CD)
    participant DB as PostgreSQL (Direct Port 5432)
    participant Registry as Container Registry
    participant Deploy as Cloud Deployer (Railway / Fly.io)
    participant API as Running Go API Pods

    CI->>CI: Run lint, tests, and build
    CI->>Registry: Push Docker image tag (commit short-sha)
    Note over CI,DB: Step 1: Pre-Deploy Database Migration
    CI->>DB: Run cmd/migrate up (Backwards-compatible additive DDL)
    DB-->>CI: Goose exit code 0
    Note over CI,Deploy: Step 2: Rolling Container Replacement
    CI->>Deploy: Trigger rolling update with new image tag
    Deploy->>API: Spawn new container; wait for /readyz == 200
    Deploy->>API: Send SIGTERM to old container; drain 15s
    Deploy-->>CI: Deployment healthy
```

### 3.1 Pre-Deploy Migration Invariant
1. All migrations in Stages 0–5 are strictly additive and backward-compatible (governed by [`database-staging-proposal.md`](database-staging-proposal.md)).
2. `cmd/migrate up` runs in a dedicated ephemeral GitHub Actions job or pre-deploy hook before new containers receive traffic.
3. If `cmd/migrate` fails, the CD pipeline halts immediately and cancels the container rollout.

---

## 4. Production Secret & Environment Variable Contract

| Variable Name | Required By | Secret? | Description & Allowed Format |
| :--- | :---: | :---: | :--- |
| `DATABASE_URL` | `apps/api` | Yes | PostgreSQL connection string with TLS enabled (`sslmode=require`). |
| `PORT` | `apps/api` | No | HTTP listener port (default `8080`). |
| `SUPABASE_JWT_SECRET` | `apps/api` | Yes | HMAC-SHA256 secret key from Supabase project for local JWT signature validation without network hops. |
| `OPENAI_API_KEY` | `apps/api` | Yes | OpenAI API key (`sk-...`) for analogy generation and active recall evaluation. |
| `STORAGE_BUCKET_NAME` | `apps/api` | No | Private storage bucket name for educator credential proofs. |
| `STORAGE_ENDPOINT` | `apps/api` | No | S3-compatible API endpoint (e.g., Supabase Storage / Cloudflare R2). |
| `STORAGE_ACCESS_KEY` | `apps/api` | Yes | S3-compatible access key ID. |
| `STORAGE_SECRET_KEY` | `apps/api` | Yes | S3-compatible secret access key. |
| `GUEST_COOKIE_SECRET` | `apps/api` | Yes | 32-byte hex string used to HMAC-sign guest session cookies (`idep_guest_session`). |
| `LOG_LEVEL` | `apps/api` | No | `info`, `warn`, `error` (defaults to `info` in production). |

---

## 5. Runtime Observability & Health Probes

### 5.1 Health Probe Specification
- **`GET /healthz` (Liveness Probe):** Returns `HTTP 200 OK` `{"status":"alive"}` immediately. Indicates process is running.
- **`GET /readyz` (Readiness Probe):** Pings PostgreSQL with a 2-second timeout (`SELECT 1`). Returns `HTTP 200 OK` `{"status":"ready","database":"connected"}` when pool is healthy. If DB ping fails, returns `HTTP 503 Service Unavailable` so the load balancer stops routing traffic to this replica.

### 5.2 Structured Production Logging
- In production (`LOG_LEVEL=info`), all logs emit single-line JSON via standard library `log/slog`.
- Every incoming request generates a unique `request_id` (UUIDv4) via middleware, injected into `context.Context` and included on every log line.
- **PII Scrubbing:** Passwords, JWT tokens, user email addresses, raw educator credential files, and verbatim learner responses are never logged.

---

## 6. Verification Gates & Acceptance Criteria

Execution is complete only when all five gates pass:

1. **Multi-Stage Docker Build Gate:**
   - `docker build -f apps/api/Dockerfile -t idep-api:test .` builds cleanly and produces an image $< 45\text{MB}$.
   - Container executes as non-root user (`UID 65532`).

2. **Health & Readiness Probe Gate:**
   - Calling `/healthz` returns `HTTP 200 OK` within $< 5\text{ms}$.
   - Calling `/readyz` with healthy database returns `HTTP 200 OK`; with disconnected database returns `HTTP 503 Service Unavailable`.

3. **Graceful Shutdown Gate:**
   - Sending `SIGTERM` to the Go API process causes it to stop accepting new requests, finish ongoing HTTP requests within 15 seconds, close database pool connections cleanly, and exit code `0`.

4. **Secret Hygiene Gate:**
   - Attempting to start the API in production mode without `DATABASE_URL`, `SUPABASE_JWT_SECRET`, or `GUEST_COOKIE_SECRET` fails immediately at startup with an explicit descriptive error before binding to the HTTP port.

5. **CI Quality & Monorepo Test Gate:**
   - `pnpm lint`, `pnpm --filter web check-types`, and `pnpm test` pass with 0 errors and 0 warnings.

---

## 7. Files Touched & Blast-Radius Guard

| Action | File Path |
| :--- | :--- |
| **new** | `apps/api/Dockerfile`, `apps/api/.dockerignore`, `.github/workflows/deploy.yml` |
| **new** | `apps/api/internal/handler/health_handler.go`, `apps/api/internal/handler/health_handler_test.go` |
| **edit** | `apps/api/cmd/server/main.go`, `README.md` |
| **must NOT change** | `apps/api/internal/guardrails_test.go`, `apps/api/sql/schema/*.sql`, `apps/web/lib/supabase.ts`, `packages/eslint-config/**` |

