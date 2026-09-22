# UAWP state v1

UAWP owns only `.uawp/`:

```text
.uawp/
├── manifest.json
├── CONTEXT.md
├── ACTIVE_WORKER.md
├── DECISIONS.md
├── INSTRUCTIONS.md
├── checkpoints/
├── migrations/
└── recovery/
```

`manifest.json` establishes namespace ownership with `protocol: "UAWP"` and a
exact supported `stateVersion`. Current state is `1.1.0`; `1.0.0` is the only
initial upgrade source. Unknown minor versions, unknown keys, duplicate keys,
trailing JSON, and future major versions are not treated as ready.

`migrations/` contains immutable version-transition receipts. `recovery/`
contains durable transaction plans, journals, exact backups, and completion
receipts. A live `.uawp/RECOVERY.json` pointer blocks ordinary mutation until
evidence-based rollback or continuation completes.

`CONTEXT.md` stores current effective workspace state. `DECISIONS.md` stores
durable decisions. `INSTRUCTIONS.md` is the canonical agent-neutral entry into
the UAWP lifecycle; native provider files may only bridge to this shared
source. `checkpoints/` is reserved for milestone snapshots.

`ACTIVE_WORKER.md` represents persistent ownership. It permits exactly two
states:

- `ACTIVE`: the named worker has substantive shared-write authority and no
  release time.
- `RELEASED`: no worker owns shared writes; the last owner and release time are
  retained for audit continuity.

Acquisition and release timestamps use RFC 3339 with an explicit timezone. A
release cannot precede acquisition. Implemented operations are read-only
resume, acquire, release, context sync, immutable milestone checkpoint,
context-first handoff, and Human Controller-authorized stale recovery.

Recovery never infers staleness or acquires a replacement owner. State v1 has
no TTL, heartbeat, lease expiry, or automatic release.
