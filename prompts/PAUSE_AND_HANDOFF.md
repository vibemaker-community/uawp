# PAUSE_AND_HANDOFF

UAWP Prompt Version: 2

Purpose: persist final context and release ownership safely.

Prerequisite: the caller holds the exact ACTIVE Worker ID, Session ID, and Ownership Generation in `.uawp/ACTIVE_WORKER.md`.

1. Prepare the complete final `.uawp/CONTEXT.md`.
2. Preview context persistence followed by ownership release.
3. Recheck the complete tuple and drift, then apply after approval and verify context before release.
4. Create no automatic checkpoint.
5. After release, perform no further shared writes. The released Session remains forbidden from writing after any later generation is acquired.

Stop on any drift. Report final context hash, released Worker ID, and next handoff action.
