# Diagnostics

`status` returns the primary workspace condition. `doctor` aggregates stable maintenance findings, including `UPGRADE_AVAILABLE`, `MAINTENANCE_BLOCKED_ACTIVE`, `REPAIR_AVAILABLE`, and `MANUAL_REPAIR_REQUIRED`, without writing any file. Adapter drift and a live transaction remain errors. See [upgrade and repair](upgrade-and-repair.md) and [transaction recovery](transaction-recovery.md).

`uawp status` reports whether a workspace is ready for resume. `uawp doctor`
uses the same read-only integrity checks in this increment. Neither command
creates, repairs, upgrades, or deletes files.

<!-- verify -->
```bash
./uawp status --workspace /absolute/project/path --format json
./uawp doctor --workspace /absolute/project/path --format json
```

The response contains one stable finding code and a safest next action:

- `READY`: valid released ownership; resume context before acquiring work.
- `UNINITIALIZED`: no UAWP namespace; preview `uawp init`.
- `ACTIVE_OWNER`: another persisted worker owns substantive writes; remain
  read-only.
- `UNKNOWN_NAMESPACE`: preserve the existing `.uawp/` and resolve the collision.
- `INVALID_MANIFEST`, `UNSUPPORTED_VERSION`, `INVALID_OWNERSHIP`, or
  `INCOMPLETE_STATE`: repair or restore state before writing.
- `RECOVERY_REQUIRED`: a recovery journal exists; do not mutate until recovery
  is examined.

Human Controller arbitration for stale ownership will arrive with lifecycle
operations in Plan 2. This increment never infers liveness from a timeout.
