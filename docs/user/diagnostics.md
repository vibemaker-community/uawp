# Diagnostics

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
