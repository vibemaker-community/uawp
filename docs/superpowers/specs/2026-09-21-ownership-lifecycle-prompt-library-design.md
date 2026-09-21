# Ownership Lifecycle and Prompt Library Design

**Status:** Proposed for written review

**Date:** 2026-09-21

**Parent specification:** `docs/superpowers/specs/2026-09-21-uawp-product-engineering-design.md`

## 1. Goal

This phase makes the validated UAWP worker lifecycle executable and auditable.
It adds normal ownership acquisition and release, context synchronization,
milestone checkpoints, pause and handoff, Human Controller-authorized stale
claim recovery, and the four canonical agent-neutral prompts.

This phase does not implement adapters, modify agent-native files, infer
liveness, or complete general transaction repair and schema migration.

## 2. Separation of normal work and exceptional arbitration

Normal worker operations and Human Controller arbitration are separate paths.
There is no mode switch asking a user to choose between them.

Normal work is performed by a worker:

```text
RESUME
  -> inspect workspace and ownership
  -> acquire if RELEASED
  -> work while still ACTIVE owner
  -> sync context as needed
  -> create checkpoint only at a declared milestone
  -> pause and hand off
  -> release
```

Human arbitration is available only when an existing ACTIVE claim cannot be
released normally:

```text
ACTIVE claim blocks another worker
  -> preview stale-claim recovery
  -> Human Controller reviews claim, reason, and planned changes
  -> Human Controller supplies the one-time approval token
  -> record the arbitration event
  -> convert the old claim to RELEASED
  -> a worker separately performs normal acquire
```

Recovery MUST NOT automatically acquire ownership for a replacement worker.

## 3. Ownership state machine

The normative states remain `ACTIVE` and `RELEASED`.

Allowed transitions are:

| Operation | Before | After | Authorized actor |
|---|---|---|---|
| acquire | RELEASED | ACTIVE | requesting Worker |
| resume-owned | ACTIVE for same Worker ID | unchanged | current owner |
| release | ACTIVE | RELEASED | current owner |
| stale recovery | ACTIVE | RELEASED | Human Controller approval |

An acquire against another worker's ACTIVE claim MUST stop without mutation.
A shared write MUST revalidate the current Worker ID immediately before apply.
Elapsed time MUST NOT change state or authorize a transition. No TTL,
heartbeat, lease expiry, or automatic stale inference is permitted in v1.0.

## 4. Normal operations

### 4.1 Resume

Resume is read-only. It validates the manifest and required namespace,
summarizes context, decisions, ownership, and relevant checkpoints, and reports
one of three outcomes: acquire is available, this worker may continue as the
ACTIVE owner, or another ACTIVE owner blocks shared writes. Resume MUST NOT
silently acquire or release ownership.

### 4.2 Acquire

Acquire changes a valid RELEASED record to ACTIVE for the requesting Worker ID,
Agent label, purpose, and explicit-timezone acquisition timestamp. It clears
the release timestamp. The plan binds the prior ownership fingerprint and is
invalid if ownership changes before apply.

### 4.3 Context sync

Context sync replaces `.uawp/CONTEXT.md` with an explicitly supplied complete
context document. Only the current ACTIVE worker may perform it. Ownership is
unchanged. Partial patch semantics are excluded from this phase so the preview
shows the exact resulting document.

### 4.4 Create checkpoint

Checkpoint creation is explicit and milestone-driven. Only the ACTIVE worker
may create one. Checkpoints are immutable files under `.uawp/checkpoints/` and
contain the effective context, ownership identity, timestamp, milestone label,
and optional decision references. Duplicate paths or identifiers stop safely;
existing checkpoints are never overwritten.

### 4.5 Pause and handoff

Pause and handoff is one reviewed transaction with this invariant:

```text
persist complete context -> validate persistence -> release -> no more shared writes
```

The current ACTIVE worker supplies the final context and handoff purpose. The
operation updates context and ownership from ACTIVE to RELEASED. A checkpoint
is not created automatically. After successful release, the former owner MUST
be treated as non-owner until it performs a new acquire.

### 4.6 Release

A standalone normal release is available for cases in which context is already
current. Only the ACTIVE owner may release. The CLI warns that pause and handoff
is the preferred operation when context requires synchronization.

## 5. Human Controller stale-claim recovery

Recovery uses the existing preview-and-approval model, not an interactive
terminal prompt. The preview identifies the old Worker ID, Agent, acquisition
time, purpose, current ownership hash, recovery reason, Human Controller
identifier, audit entry, and exact RELEASED result.

The one-time approval token binds:

- canonical workspace path;
- operation name;
- full current ownership fingerprint;
- recovery reason and Human Controller identifier;
- all planned file changes; and
- generation timestamp.

Any relevant drift invalidates the token. Applying a valid recovery records an
audit event in `.uawp/DECISIONS.md` and converts ownership to RELEASED in one
reviewed transaction. A successfully applied token cannot be replayed because
the ownership fingerprint has changed.

The tool does not decide that a claim is stale. Supplying the approved token is
the Human Controller's explicit authorization to resolve that specific claim.

## 6. Architecture

### 6.1 Core state machine

Core owns vendor-neutral transition validation and document models. It accepts
current state plus an intended actor and returns either a valid next state or a
stable protocol error. It has no filesystem, CLI, or vendor dependencies.

### 6.2 Workspace operations

The workspace layer discovers inputs and builds immutable operation plans. All
mutations follow:

```text
Discover -> Validate -> Plan -> Preview -> Explicit approval
-> Revalidate actor and input drift -> Apply -> Verify
```

Plans bind the workspace, actor Worker ID, current ownership, context,
decisions, affected checkpoint paths, and proposed output. Multi-file failure
leaves an explicit recovery-required state; it does not guess which concurrent
content may be deleted.

### 6.3 CLI

The CLI exposes separate commands for `resume`, `acquire`, `release`, `sync`,
`checkpoint`, `handoff`, and `recover`. Mutating commands preview by default
and require the exact approval token to apply. CLI code formats results and
maps stable errors to exit codes; it does not implement transition policy.

### 6.4 Prompt Library

The canonical prompts are stored as versioned product assets:

- `RESUME_WORK`: reconstruct state, report ownership, and remain read-only if
  another worker is ACTIVE;
- `CONTEXT_SYNC`: produce a complete current context for an ACTIVE owner
  without releasing ownership;
- `CREATE_CHECKPOINT`: create a milestone snapshot only when explicitly
  requested;
- `PAUSE_AND_HANDOFF`: save, sync, validate, release, and prohibit subsequent
  shared writes by the released worker.

Prompts use only Core concepts and `.uawp/` paths. They MUST NOT mention a
vendor command, native instruction filename, or vendor auto-loading behavior.
Adapters in a later phase decide how a concrete agent receives a prompt.

## 7. Audit and file ownership

`.uawp/ACTIVE_WORKER.md` is the sole effective ownership record.
`.uawp/CONTEXT.md` holds the current effective state. Durable Human Controller
recovery events are appended to `.uawp/DECISIONS.md`. Milestone snapshots live
under `.uawp/checkpoints/`.

This phase writes nowhere outside `.uawp/`. It does not create, modify, import,
or manage any agent-native file.

## 8. Error and recovery behavior

- A non-owner shared write returns a stable ownership error and performs zero
  planned mutation.
- An ambiguous, duplicate, malformed, or unsupported ownership document blocks
  all mutation.
- Input drift after preview invalidates approval before any planned write.
- Another ACTIVE owner blocks acquire without suggesting automatic recovery.
- Recovery without the exact Human Controller token is read-only.
- An interrupted multi-file apply leaves `RECOVERY_REQUIRED` diagnostics and
  blocks later mutation until the transaction is resolved by supported
  recovery behavior.

General-purpose transaction repair remains Plan 4 scope. This phase must still
report its own interrupted operations precisely and preserve all inputs.

## 9. Acceptance criteria

The phase is accepted when automated tests prove:

1. RELEASED -> acquire -> ACTIVE succeeds with an explicit approved plan.
2. Competing acquisition permits at most one worker to become ACTIVE.
3. The same ACTIVE worker may resume; another worker is reported read-only.
4. Non-owners cannot sync, checkpoint, hand off, or release.
5. Context sync retains ownership and preserves unrelated UAWP state.
6. Checkpoints are explicit, immutable, portable, and never overwrite.
7. Handoff persists final context before releasing ownership and creates no
   implicit checkpoint.
8. Normal release is allowed only for the current ACTIVE owner.
9. Stale recovery without Human Controller approval performs no mutation.
10. Recovery approval becomes invalid after any bound-state drift and cannot
    be replayed after success.
11. Recovery records an auditable decision and does not acquire a new owner.
12. All four canonical prompts are present, vendor-neutral, and consistent
    with executable operations.
13. Crash and partial-write scenarios report `RECOVERY_REQUIRED` without
    modifying project-owned or agent-native files.
14. The full Phase 1 test suite continues to pass.

## 10. Explicit exclusions

This phase does not implement agent adapters, DIRECT/IMPORT/MANAGED_BLOCK,
agent-native file edits, automatic liveness, schema migrations, uninstall,
packaging, release signing, or a general-purpose recovery command for arbitrary
future transaction types.
