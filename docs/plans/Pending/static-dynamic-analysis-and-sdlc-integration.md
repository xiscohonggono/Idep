# Static Analysis, Dynamic Analysis & SDLC Integration

> **Status:** Proposed — Pending Review & Delegation  
> **Scope:** Embed automated Static Application Security Testing (SAST), Dynamic Analysis (DAST, Fuzzing, Race Detection), and comprehensive SDLC quality gates across the `turbo-next-go-template` monorepo (Go 1.26 + Huma API & Next.js 16 App Router).  
> **Builder:** Claude Code via `delegate-to-claude`. **Reviewer:** Gemini/Architect → `docs/reviews/static-dynamic-analysis-review.md`.  
> **Related Contracts:** [`AGENTS.md`](file:///home/xiscohonggono/projects/Idep/AGENTS.md) · [`.agent/rules/project_guardrails.md`](file:///home/xiscohonggono/projects/Idep/.agent/rules/project_guardrails.md) · [`architectural_decisions.md`](file:///home/xiscohonggono/projects/Idep/docs/architecture/architectural_decisions.md) · [`.github/workflows/ci.yml`](file:///home/xiscohonggono/projects/Idep/.github/workflows/ci.yml) · [`.github/workflows/security.yml`](file:///home/xiscohonggono/projects/Idep/.github/workflows/security.yml)

---

## 1. Objective & Guiding Principles

This plan establishes a comprehensive quality and security testing framework spanning the entire software development lifecycle (SDLC) for this project:

1. **Shift-Left Discipline:** Catch defects at the earliest possible stage (local pre-commit -> PR CI -> staging DAST -> production monitoring).
2. **Minimal Blast Radius & Zero Bloat (Ponytail):** Reuse native compiler capabilities, Go standard library features (`go test -race`, `go test -fuzz`), and the existing Huma OpenAPI 3.1 SSOT rather than accumulating heavy, redundant SaaS wrappers.
3. **Fail-Closed Gateways:** Every stage must have strict exit codes. Warnings cannot be silently ignored or bypassed in CI.
4. **Preserve Monorepo Boundaries:** Respect the clean separation between `apps/web` (Next.js) and `apps/api` (Go 1.26 + Chi + Huma + `sqlc`).

---

## 2. Work Items Breakdown

### Phase 1: Static Application Security Testing (SAST) & Linting

#### S1. Go Backend Deep Linting (`golangci-lint` + `gosec`)
- **Problem:** `pnpm --filter api lint` currently runs only `go vet ./...`, missing security vulnerabilities (unhandled errors, SQL leaks, resource leaks, goroutine leaks).
- **Implementation:**
  1. Add `.golangci.yml` in `apps/api/` configuring high-signal linters:
     - `govet`: Go correctness and shadow variable detection.
     - `staticcheck`: Advanced static analysis for bugs and deprecations.
     - `gosec`: Security vulnerability scanner for Go AST.
     - `bodyclose`: Checks whether HTTP response bodies are properly closed.
     - `errcheck`: Enforces checked error returns.
  2. Add `golangci-lint` step in `.github/workflows/ci.yml` under `test-api` using `golangci/golangci-lint-action@v6`.
  3. Update `apps/api/package.json` lint script or add `lint:strict`.

#### S2. Frontend Security & Strict Type Checking (`apps/web`)
- **Problem:** ESLint in `apps/web` only checks syntax and styling without specialized security rules.
- **Implementation:**
  1. Add `@eslint-community/eslint-plugin-security` to `apps/web/eslint.config.js` to flag vulnerable regexes, insecure object access, and eval patterns (*Note: subject to explicit user confirmation under `AGENTS.md` §4 Ponytail Discipline before adding to `package.json`*).
  2. Maintain zero-warning enforcement (`eslint --max-warnings 0`) in `apps/web/package.json`.
  3. Ensure `check-types` (`next typegen && tsc --noEmit`) runs in parallel via Turborepo task pipeline.

#### S3. Automated Reachable Vulnerability Scanning (`govulncheck`)
- **Problem:** `osv-scanner` flags declared vulnerable dependencies in `go.sum`, but cannot determine whether the vulnerable code paths are actually reachable.
- **Implementation:**
  1. Integrate `golang.org/x/vuln/cmd/govulncheck` in `.github/workflows/security.yml`.
  2. Add a weekly scheduled cron trigger to `.github/workflows/security.yml` to alert on zero-days in existing lockfiles without requiring new commits.

---

### Phase 2: Dynamic Analysis & Fuzzing (DAST)

#### D1. Go Concurrency & Race Detector (`-race`)
- **Problem:** Data races in multi-threaded Go web servers (`Chi` + `pgxpool`) cause intermittent panics and corrupted memory states in production that unit tests without `-race` fail to detect.
- **Implementation:**
  1. Update `.github/workflows/ci.yml` step `Run Go Vet, Test & Build` under `test-api`, **preserving the `postgres:16-alpine` service container, migration verification (`cmd/migrate up -> down -> up`), and `bin/migrate` build**:
     ```yaml
     - name: Run Go Vet, Test & Build
       working-directory: apps/api
       run: |
         go vet ./...
         go test -race -v -coverprofile=coverage.out ./...
         go build -o bin/server ./cmd/server
         go build -o bin/migrate ./cmd/migrate
     ```
  2. Verify that all repository mock and handler tests pass under `-race` without race conditions.

#### D2. Native Go Fuzzing for Input Parsers (`go test -fuzz`)
- **Problem:** Huma models and custom query/body parsers can panic when handling malformed Unicode, edge-case numbers, or unexpected JSON payloads.
- **Implementation:**
  1. Introduce fuzz tests (`*_fuzz_test.go`) in `apps/api/internal/handler/` or `internal/domain/`.
  2. Provide seed corpora for valid requests, testing unmarshaling and validation boundaries.
  3. Add an optional CI fuzzing target running for 30–60 seconds on pull requests.

#### D3. API Contract Property-Based DAST (Schemathesis)
- **Problem:** OpenAPI contracts can drift from actual runtime HTTP responses, and unexpected status codes (e.g. unhandled 500s) can be triggered by edge-case payloads.
- **Implementation:**
  1. Leverage the exported `apps/api/openapi.json` from Huma.
  2. Add a DAST task in `apps/api` using `schemathesis` to automatically fuzz running API endpoints against the OpenAPI 3.1 specification:
     ```bash
     schemathesis run apps/api/openapi.json --url http://127.0.0.1:8080 --checks all
     ```
  3. Add an ephemeral test step in CI or staging verification verifying zero unhandled 500 errors.

#### D4. End-to-End Dynamic Browser Testing (Playwright)
- **Problem:** Vitest unit tests mock the DOM via `jsdom`, failing to verify real browser navigation, CSS rendering, cookies, and client-side hydration.
- **Implementation:**
  1. Configure Playwright in `apps/web/` for critical path testing (login/auth, navigation, data tables) (*Note: subject to explicit user confirmation under `AGENTS.md` §4 Ponytail Discipline before adding to `package.json`*).
  2. Add headless CI execution for PR validation.

---

### Phase 3: SDLC Lifecycle Integration Matrix

| SDLC Phase | Gate | Tooling & Mechanism | Target / Criteria |
| :--- | :--- | :--- | :--- |
| **1. Commit / Local** | Pre-commit hook | `gitleaks protect --staged`, `pnpm lint` | Zero secret leaks, zero lint warnings |
| **2. Pull Request (CI)** | Static Analysis Gate | `golangci-lint`, `eslint --max-warnings 0`, `tsc --noEmit` | Clean AST & strict typing |
| | Dependency & Secret SCA | `gitleaks`, `osv-scanner`, `govulncheck` | No high/critical vulnerabilities |
| | Dynamic Concurrency & Test | `go test -race`, Vitest run | Zero data races, 100% test pass rate |
| | Contract Drift Gate | `pnpm codegen` + `git diff --exit-code` | Go OpenAPI & TS schema in sync |
| **3. Staging / Pre-Release** | DAST & Fuzzing Gate | Schemathesis against live Go server, Playwright E2E | No undocumented 5xx errors, E2E pass |
| **4. Production / Ops** | Scheduled Security Scan | Weekly GitHub Actions cron for OSV/Govulncheck | Continuous vulnerability monitoring |
| | Runtime Observability | Go `net/http/pprof` (guarded behind internal/admin port) | Memory leak and CPU profile inspection |

---

## 3. Verification & Acceptance Criteria

To approve and move this plan to `docs/plans/Implemented/`, the following verification evidence must be recorded in `docs/verification/static-dynamic-analysis.md`:

1. **Static Analysis Passes:**
   - `golangci-lint run ./...` in `apps/api` exits with code 0.
   - `pnpm --filter web lint` and `pnpm --filter web check-types` exit with code 0.
   - `gitleaks` and `osv-scanner` run clean.
2. **Dynamic Analysis Passes:**
   - `go test -race -v ./...` in `apps/api` passes with zero race detections.
   - At least 1 native Go fuzz test passes 10,000 iterations without panic.
   - Schemathesis runs against local Go server with zero contract deviations.
3. **CI Pipeline Green:**
   - Updated `.github/workflows/ci.yml` and `.github/workflows/security.yml` complete successfully in GitHub Actions.

---

## 4. Files Touched & Blast-Radius Guard

| Action | File Path |
| :--- | :--- |
| **new** | `apps/api/.golangci.yml`, `apps/api/internal/handler/*_fuzz_test.go` |
| **edit** | `.github/workflows/ci.yml`, `.github/workflows/security.yml`, `apps/api/package.json`, `apps/web/package.json`, `apps/web/eslint.config.js` |
| **must NOT change** | `apps/api/internal/guardrails_test.go`, `apps/api/sql/**`, `apps/api/sqlc.yaml`, `apps/api/internal/repository/db/**`, `apps/web/lib/supabase.ts` |


