# CREATE_CHECKPOINT

UAWP Prompt Version: 1

Purpose: record an explicit milestone snapshot.

Prerequisite: the caller is the ACTIVE owner and an explicit milestone has been declared.

1. Recheck `.uawp/ACTIVE_WORKER.md` and `.uawp/CONTEXT.md`.
2. Preview one new portable file under `.uawp/checkpoints/`.
3. Apply only after approval and never overwrite an existing checkpoint.
4. Verify the immutable snapshot and retain ownership.

Report milestone ID, checkpoint path, context hash, and decision references.
