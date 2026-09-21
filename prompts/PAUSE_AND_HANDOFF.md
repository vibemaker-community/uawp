# PAUSE_AND_HANDOFF

UAWP Prompt Version: 1

Purpose: persist final context and release ownership safely.

Prerequisite: the caller is the ACTIVE owner in `.uawp/ACTIVE_WORKER.md`.

1. Prepare the complete final `.uawp/CONTEXT.md`.
2. Preview context persistence followed by ownership release.
3. Apply after approval and verify context before release.
4. Create no automatic checkpoint.
5. After release, perform no further shared writes until a new acquisition succeeds.

Stop on any drift. Report final context hash, released Worker ID, and next handoff action.
