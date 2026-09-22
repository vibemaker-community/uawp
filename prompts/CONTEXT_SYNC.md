# CONTEXT_SYNC

UAWP Prompt Version: 2

Purpose: persist the complete effective state while retaining ownership.

Prerequisite: the caller holds the exact ACTIVE Worker ID, Session ID, and Ownership Generation recorded in `.uawp/ACTIVE_WORKER.md`.

1. Recheck the complete ACTIVE tuple immediately before planning.
2. Prepare the complete replacement for `.uawp/CONTEXT.md`.
3. Preview and approve the exact change bound to the ownership document.
4. Recheck the tuple and drift immediately before apply.
5. Apply and verify the new context while retaining the tuple unchanged.

Stop on drift or if the caller is not the ACTIVE owner. Report the context hash and continuing owner.
