# CREATE_CHECKPOINT

UAWP Prompt Version: 2

Purpose: record an explicit milestone snapshot.

Prerequisite: the caller holds the complete ACTIVE tuple (Worker ID, Session ID, and Ownership Generation) and an explicit milestone has been declared.

1. Recheck `.uawp/ACTIVE_WORKER.md` and `.uawp/CONTEXT.md`.
2. Record the complete ACTIVE tuple and preview one new portable file under `.uawp/checkpoints/`.
3. Apply only after approval and never overwrite an existing checkpoint.
4. Recheck drift, verify the immutable snapshot, and retain the exact ownership tuple.

Report milestone ID, checkpoint path, context hash, and decision references.
