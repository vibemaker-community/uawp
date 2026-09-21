# UAWP state v1

UAWP owns only `.uawp/`:

```text
.uawp/
├── manifest.json
├── CONTEXT.md
├── ACTIVE_WORKER.md
├── DECISIONS.md
└── checkpoints/
```

`manifest.json` establishes namespace ownership with `protocol: "UAWP"` and a
supported v1 semantic `stateVersion`. Unknown keys, duplicate keys, trailing
JSON, and unsupported major versions are rejected.

`CONTEXT.md` stores current effective workspace state. `DECISIONS.md` stores
durable decisions. `checkpoints/` is reserved for milestone snapshots.

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
