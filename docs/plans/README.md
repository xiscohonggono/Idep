# Implementation Ledger (`docs/plans/`)

This directory serves as the project's implementation ledger for tracking project delivery status, evidence, boundaries, and remaining work for the CRM implementation, rather than describing the runtime behavior of the application itself.

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

