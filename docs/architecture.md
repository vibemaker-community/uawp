# Architecture

UAWP separates provider discovery from durable workspace coordination.

```text
Agent-native project entry
        ↓ Adapter (DIRECT / IMPORT / MANAGED_BLOCK)
.uawp/INSTRUCTIONS.md
        ↓
Agent-neutral CLI and typed Core
        ↓ preview + exact approval + drift check
.uawp/ state and bounded adapter change
```

## Ownership boundaries

The Core owns `.uawp/`: its manifest, instructions, current context, active
worker record, decisions, checkpoints, receipts, backups, and transaction
journals. A same-named project file outside that directory is unrelated. The
path, not a generic filename such as `CONTEXT.md`, establishes ownership.

Agent-native entries—such as `AGENTS.md`, `CLAUDE.md`, or a vendor rules
directory—remain outside UAWP ownership. An adapter uses only behavior verified
against official vendor documentation and selects one of three integration
modes:

- `DIRECT`: the native entry can consume the shared instruction source without
  a generated bridge.
- `IMPORT`: UAWP creates and records a dedicated provider entry that imports or
  points to the shared source.
- `MANAGED_BLOCK`: UAWP manages one uniquely bounded block inside an existing
  native entry while preserving surrounding content.

Consumer metadata records which configured adapters depend on a shared entry,
so removing one provider cannot delete an entry still used by another.

## State and mutation

Read operations diagnose without ownership. Shared writes require the exact
Worker ID, Session ID, and ownership generation. A mutation is planned against
observed inputs, presented for review, approved by an opaque plan token, then
revalidated immediately before commit. Durable journals allow classification,
continuation, or rollback after interruption.

Single Active Worker is a coordination invariant, not a concurrency engine.
UAWP prevents multiple Sessions from claiming ordinary shared-write authority
for one physical Workspace. It does not merge independent simultaneous edits.

## Exceptional recovery

Time alone never releases ACTIVE ownership. When the owner cannot release
normally, a Human Controller inspects the claim and issues a one-time
authorization bound to that observed ownership state. Any change invalidates
the authorization. Recovery is written to the audit history before another
Session acquires ownership.

See [Agent integration](agent-integration.md), [state protocol](protocol/state-v1.md),
and [package boundaries](engineering/package-boundaries.md).
