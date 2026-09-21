# UAWP Workspace Instructions

Before making a shared workspace change, read `.uawp/CONTEXT.md`,
`.uawp/DECISIONS.md`, and `.uawp/ACTIVE_WORKER.md`. Use `RESUME_WORK` to
reconstruct the current state. Do not write unless this worker holds the
ACTIVE ownership claim.

Use the canonical `CONTEXT_SYNC` prompt after material state changes and
`CREATE_CHECKPOINT` only for an explicit milestone. Before pausing or changing
workers, use `PAUSE_AND_HANDOFF`; releasing ownership is the final shared
write.

Never infer that an ACTIVE claim is stale. Only a Human Controller may approve
stale-claim recovery through the protocol's reviewed recovery operation.
