# RESUME_WORK

UAWP Prompt Version: 1

Purpose: reconstruct the workspace state without changing it.

1. Read `.uawp/manifest.json`, `.uawp/CONTEXT.md`, `.uawp/ACTIVE_WORKER.md`, `.uawp/DECISIONS.md`, and relevant `.uawp/checkpoints/` entries.
2. Validate ownership. If another worker is ACTIVE, remain read-only and stop before shared writes.
3. If ownership is RELEASED, report that acquisition is available but never acquire silently.
4. If this Worker ID is ACTIVE, report that work may resume and ownership must be rechecked before every shared write.

Report the effective stage, owner, blockers, and next safe action.
