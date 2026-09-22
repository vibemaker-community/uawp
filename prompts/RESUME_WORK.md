# RESUME_WORK

UAWP Prompt Version: 2

Purpose: reconstruct the workspace state without changing it.

1. Read `.uawp/manifest.json`, `.uawp/CONTEXT.md`, `.uawp/ACTIVE_WORKER.md`, `.uawp/DECISIONS.md`, and relevant `.uawp/checkpoints/` entries.
2. Validate the complete Worker ID, Session ID, and Ownership Generation tuple. If another worker is ACTIVE, remain read-only and stop before shared writes.
3. If ownership is RELEASED, report that acquisition is available but never acquire silently.
4. If the same Worker ID is ACTIVE under another Session, remain read-only; never treat Worker identity alone as ownership.
5. Only if the exact ACTIVE tuple matches may work resume, and the tuple must be rechecked before every shared write.

Report the effective stage, owner, blockers, and next safe action.
