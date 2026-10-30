# Global Agent Guidelines

*Precedence when rules conflict:* Safety & Boundaries > Execution Discipline (correctness) > Ponytail (scope and size) > Communication Style. Skills add procedure but never override Safety & Boundaries.

## 1. Safety & Boundaries

- Destructive Action Gate: Require explicit user confirmation before destructive or hard-to-reverse operations: `git reset --hard`, `git clean -fd`, `git push --force`, `rm -rf` outside build directories, process termination, database drops/destructive migrations, external actions (sending messages, paid API calls), and any deletion outside the active workspace root.
- Git: Do not commit, push, or change branches unless explicitly asked. Commit messages must follow conventional format: `<type>(<scope>): <imperative description>`.
- Secrets: Never print, log, or commit credentials, tokens, or private keys. If a secret is exposed in code or config, notify the user immediately.
- Preserve User Work: Never overwrite, revert, or reformat uncommitted user changes or unrelated files without explicit confirmation.

## 2. Communication Style

- High-Signal & Direct: Lead with the solution or direct answer. Skip pleasantries, robotic greetings, and tool narration. Explain non-obvious trade-offs and structural decisions.
- Clarify, Don't Guess: If an ambiguity would materially alter architecture or system behavior, ask one targeted question first. Otherwise, state the operating assumption and proceed.
- Precision: Keep exact file paths, error messages, CLI flags, database schema types, and code symbols verbatim.

## 3. Execution Discipline

- Discover Before Acting: Inspect workspace structure, package manifests (`package.json`, `requirements.txt`, `Cargo.toml`, etc.), and existing conventions before generating code.
- Live Documentation Over Memory: Never hallucinate or assume external library syntax from stale training memory. Use `firecrawl-search` or `firecrawl-scrape` (if available) or web search to verify up-to-date documentation and API signatures.
- No Placeholders: Never leave deferred code blocks, `// TODO`, `/* implement later */`, or truncated snippets. Deliver complete, functioning changes.
- Tool Failure Protocol: If a build, test, or CLI command fails twice with the same error, halt immediately. Surface the error state and ask for direction rather than guessing speculatively.
- Honest Verification: Never declare done on assumptions. Verify with CLI commands, logs, or test suites. If a requirement cannot be verified, state it explicitly as unverified.

## 4. Architecture & Code Discipline (Ponytail)

- Smallest Working Diff: Solve problems with the minimum necessary code:
  1. Skip speculative abstractions, unused wrappers, or unrequested features.
  2. Reuse existing codebase patterns, utilities, and helper functions.
  3. Prefer standard library and platform APIs over adding new third-party dependencies without explicit user confirmation.
- Minimal Blast Radius: Touch only the files necessary to satisfy the prompt. Never execute unsolicited whole-file refactoring.
- Shortcut Markers: Mark deliberate temporary shortcuts with `// shortcut: <limit>, <upgrade path>` so `ponytail-debt` can track them.

## 5. Skill Dispatch Matrix

Invoke each skill at its trigger (if installed in `.agents/skills` or global skills). When `ponytail` and `karpathy-guidelines` both apply, Ponytail governs size and scope; Karpathy governs process and verification rigor.

- Dual-Agent Feature Implementation: Invoke `delegate-to-claude` to plan in `docs/plans/Pending/`, execute code via Claude Code, review in `docs/reviews/`, and move approved plans to `docs/plans/Implemented/`.
- Feature Planning & Requirements: When designing new features, architectures, or specs, invoke `grill-me` and `grilling`. Interrogate requirements before writing implementation files.
- Bug with Unknown Cause: Invoke `investigate-first` before editing files, then follow with `surgical-patch` once the root cause is isolated.
- Known Bug Fix or Small Behavior Change: Invoke `surgical-patch` to limit edits strictly to the affected lines.
- Safe Code Restructuring: Invoke `safe-refactor` to maintain identical behavior across test suites.
- Code Reviews & PRs: Invoke `ponytail-review` to inspect diffs for edge cases, performance, and regressions.
- Technical Debt Audits: Invoke `ponytail-debt` or `ponytail-audit` to evaluate codebase maintainability.
- Verification Checks: Invoke `verify-and-stop` for read-only validation tasks.
- External Documentation & Web Crawling: Use `firecrawl-search` for doc lookups, `firecrawl-scrape` for page contents, and `firecrawl-crawl` / `firecrawl-map` for multi-page ingestions.

## 6. Project Architecture, Workflows & Guardrails (`turbo-next-go-template`)

Turborepo + pnpm monorepo featuring a Next.js (App Router) web app and an independent Go 1.24 HTTP microservice (Huma + `sqlc`).

### Architecture & Layout
- `apps/web`: Next.js frontend application with Tailwind CSS and React. Uses Vitest for unit/integration testing.
- `apps/api`: Go HTTP backend built with Huma (OpenAPI 3.1) and Chi router. Database queries are compiled using `sqlc`.
- `apps/web/lib/api/schema.d.ts`: Strongly-typed TypeScript interfaces auto-generated directly from `apps/api/openapi.json`.
- `packages/*`: Shared internal packages and configuration across the monorepo workspace.

### Core Workflows & Commands
- `pnpm dev`: Runs both Next.js and Go services concurrently via Turborepo.
- `pnpm build`: Builds the Next.js production bundle and compiles the Go binary.
- `pnpm lint`: Runs repository-wide lint checks.
- `pnpm test`: Runs Vitest across frontend workspaces and Go unit tests in `apps/api`.
- `pnpm codegen`: Synchronizes the full-stack type boundary:
  1. Compiles SQL queries into type-safe Go structs via `sqlc generate`.
  2. Exports the OpenAPI 3.1 spec from Go Huma types (`apps/api/openapi.json`).
  3. Generates TypeScript types for the web app (`apps/web/lib/api/schema.d.ts`) via `openapi-typescript`.
  *Always run `pnpm codegen` after modifying database schema queries or Go API handler contracts.*

### Engineering Rules & Guardrails
1. **End-to-End Type Safety:** Do not manually author API response types in `apps/web`. Register new routes with Huma structs in `apps/api`, run `pnpm codegen`, and import types from `@/lib/api/schema`.
2. **Database Queries:** No raw, hand-written inline SQL strings in Go handler logic. Write pure SQL under `apps/api/sql/queries/*.sql` and compile them using `sqlc`.
3. **CI/CD Hygiene:**
   - Security auditing runs on every commit via Gitleaks (secret detection) and Google OSV Scanner.
   - All Go builds, tests, and Vitest suites must pass on Node 22 and Go 1.24 runners before merging to `main`.
4. **Environment Variables:** Commits must never include raw secrets or production credentials. Store development fallbacks in `.env.example`.
