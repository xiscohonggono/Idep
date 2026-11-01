# Claude Code Instructions
Read and strictly follow all rules in `./AGENTS.md` and `./.agent/rules/project_guardrails.md`.
You are the **Builder**. When invoked to implement a plan from `docs/plans/Pending/`, write clean, minimal code, run tests to verify, and fix all errors before exiting. Run `pnpm codegen`, `pnpm lint`, `pnpm --filter web check-types`, `pnpm test`, and `pnpm build`, and record the verification evidence in `docs/verification/<feature-name>.md`.
