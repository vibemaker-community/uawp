# CONTEXT_SYNC

UAWP Prompt Version: 1

Purpose: persist the complete effective state while retaining ownership.

Prerequisite: the caller is the ACTIVE owner in `.uawp/ACTIVE_WORKER.md`.

1. Recheck ownership.
2. Prepare the complete replacement for `.uawp/CONTEXT.md`.
3. Preview and approve the exact change.
4. Apply and verify the new context, then retain ownership unchanged.

Stop on drift or if the caller is not the ACTIVE owner. Report the context hash and continuing owner.
