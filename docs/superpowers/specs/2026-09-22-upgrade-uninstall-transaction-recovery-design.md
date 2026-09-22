# UAWP Upgrade, Uninstall, Repair, and Transaction Recovery Design

**Document status:** Approved

**Version:** 0.1

**Date:** 2026-09-22

**Product phase:** Plan 4 — Upgrade, Uninstall, and Recovery

**Parent specifications:**

- `docs/superpowers/specs/2026-09-21-uawp-product-engineering-design.md`
- `docs/superpowers/specs/2026-09-22-adapter-framework-launch-adapters-design.md`

## 1. Purpose

This document fixes the Plan 4 behavior for version-aware upgrade, drift
repair, full UAWP uninstall, reinstall, and interrupted-transaction recovery.
It closes the recovery gap intentionally left by Plans 1–3 without weakening
their agent-neutral Core, namespace ownership, native-file preservation,
Single Active Worker, or Human Controller invariants.

Plan 4 makes structural maintenance safe enough for existing workspaces. It
does not redesign the normal lifecycle commands or their user experience.
Simplified interactive commands and the agent automation interface remain the
separate Plan 4.5 phase.

## 2. Outcomes and success criteria

After Plan 4, a user can:

1. inspect whether an initialized workspace requires a supported migration;
2. preview and approve every state and integration change made by an upgrade;
3. diagnose drift and repair only damage that UAWP can prove how to restore;
4. detach all registered providers without deleting project-owned content;
5. optionally purge UAWP state only after producing and verifying an export;
6. reinstall after a state-preserving uninstall without duplicating bridges;
7. diagnose an interrupted transaction as rollback-capable,
   roll-forward-capable, or requiring manual recovery; and
8. explicitly approve a safe recovery path whose evidence is still current.

Success requires byte-for-byte preservation of project-owned native content
outside approved UAWP regions, deterministic migration behavior, recovery
after injected interruption at every publication boundary, and refusal to
guess when evidence is incomplete.

## 3. Normative principles

The terms **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT**, and **MAY** are
normative.

All Plan 4 mutations MUST retain the established workflow:

```text
Discover -> Preflight -> Plan -> Preview -> Human approval -> Apply -> Verify
```

In addition:

- Core migration and recovery semantics MUST remain agent-neutral.
- UAWP-owned durable state MUST remain under `.uawp/`.
- Files outside `.uawp/` MUST remain project- or provider-owned even when they
  contain an IMPORT or MANAGED_BLOCK integration.
- No `--force` option may bypass evidence, drift, ownership, export, or
  preservation requirements.
- An approval MUST bind the exact workspace, operation, inputs, versions,
  recovery evidence, and proposed changes. Drift invalidates it.
- Failed verification MUST never be reported as success.

## 4. Scope and boundaries

### 4.1 In scope

- state-schema migration and adapter-integration upgrade;
- migration receipts and supported-version discovery;
- read-only diagnosis plus separately approved repair;
- safe full-provider detach and optional exported state purge;
- state-preserving reinstall;
- durable transaction evidence, backup, rollback, roll-forward, and receipts;
- stable findings and exit behavior for these operations;
- unit, integration, end-to-end, failpoint, and cross-platform safety tests.

### 4.2 Out of scope

- automatic downgrade;
- semantic merging of human-authored instructions;
- reconstructing missing evidence from intuition or provider conventions;
- remote backup, cloud synchronization, or distributed locking;
- automatic stale-worker detection or replacement ownership acquisition;
- simplified prompts, default-current-directory behavior, interactive menus,
  and stdin/stdout agent APIs, which belong to Plan 4.5;
- packaging, signing, public release automation, and public artifact testing,
  which belong to Plan 5.

## 5. Command model and terminology

Plan 4 adds these expert-mode command families:

```text
uawp upgrade
uawp repair
uawp uninstall
uawp transaction status
uawp transaction rollback
uawp transaction continue
```

`uawp recover` remains exclusively the Human Controller-authorized stale
`ACTIVE_WORKER` operation from Plan 2. Transaction recovery uses the
`uawp transaction` namespace so neither users nor automation can confuse the
two forms of recovery.

The terms have these meanings:

- **upgrade**: move recognized state and integrations through an explicit
  supported forward migration path;
- **repair**: restore a damaged current-version artifact only when canonical
  content and ownership are provable;
- **detach**: stop all registered providers from loading UAWP while preserving
  `.uawp/` state;
- **purge**: delete the UAWP namespace after a verified export;
- **rollback**: restore exact pre-transaction bytes from durable evidence;
- **roll forward / continue**: complete the exact approved transaction after
  revalidating every completed, in-progress, and pending action.

Every mutating command first emits an immutable preview and approval token.
`uawp transaction status` is always read-only.

## 6. Version and migration model

### 6.1 Independent versions

The CLI release version and `manifest.json.stateVersion` are independent.
Compatibility code MUST use `stateVersion`; it MUST NOT infer state format
from the executable version.

An older client encountering a newer unsupported state version MUST stop
without mutation. A newer client encountering a recognized older state version
MAY upgrade only through a registered migration chain.

### 6.2 Explicit migration registry

Each migration step declares:

- exact source and target state versions;
- the state and integration facts it consumes;
- a deterministic planner;
- preconditions and unsupported conditions;
- postconditions and verification rules; and
- fixtures for every supported source state.

The resolver MUST find one unambiguous forward-only chain. Missing, branching,
cyclic, skipped, or downgrade paths fail closed. Each step MUST be idempotent:
once its target version and postconditions are present, rerunning upgrade
produces no changes.

### 6.3 Upgrade ordering

A multi-step upgrade is planned as one approved transaction. Its logical order
is:

1. validate namespace identity and current version;
2. validate current Core state and registered integration evidence;
3. resolve the complete migration chain;
4. plan canonical UAWP state changes;
5. re-resolve every configured provider using its verified adapter evidence;
6. plan only the owned integration changes needed for the target state;
7. write the target manifest last; and
8. verify target state, integration health, and absence of duplicate loading.

An ACTIVE ownership record blocks upgrade. Structural maintenance requires a
normal release first; upgrade MUST NOT treat the invoking process as the
active worker or release ownership automatically.

### 6.4 Migration receipts

Successful state transitions append an immutable receipt under:

```text
.uawp/migrations/<UTC timestamp>-<from>-to-<to>.json
```

The receipt records the source and target versions, approved plan identifier,
CLI version, completion time, applied artifact paths, and resulting hashes.
Receipt filenames use a filesystem-safe UTC form and collision-resistant plan
suffix. Receipts contain no native-file content or secrets. They are audit
evidence, not an alternative source of current truth.

## 7. Diagnosis and repair

### 7.1 Read-only diagnosis

`status` remains a concise readiness view. `doctor` performs the deeper
read-only checks required by Plan 4, including:

- namespace, manifest, state version, and required-file integrity;
- ownership validity and whether structural maintenance is blocked;
- configured integration ownership, effective-entry, consumer, marker,
  import, shadowing, and duplicate-load checks;
- live transaction and recovery evidence;
- migration availability; and
- whether a finding is repairable automatically.

Findings MUST carry stable codes, severity, observed evidence, and a next
action. A repairable finding does not authorize mutation.

### 7.2 Repair classes

`uawp repair` MAY plan a repair only when all replacement bytes or structural
actions are derivable from versioned UAWP data and exact ownership evidence.
Examples include:

- recreating a missing canonical UAWP-owned generated file from the exact
  installed state version;
- restoring an unmodified UAWP-created integration file whose manifest record
  proves its full ownership;
- restoring a known IMPORT or MANAGED_BLOCK whose surrounding project bytes
  and ownership metadata still match; and
- reconciling generated registry metadata when the native artifact and its
  consumer set are independently provable.

Repair MUST stop and require manual resolution when:

- namespace ownership or state version is unknown;
- a required user-authored state file has no exact recoverable source;
- managed markers are missing, nested, reordered, or duplicated and the owned
  span cannot be identified exactly;
- content outside an owned region drifted;
- manifest and marker evidence disagree without an independent authority;
- an effective provider route is unsupported or unobservable; or
- any proposed action would require semantic merging.

Repair never edits `CONTEXT.md`, `DECISIONS.md`, checkpoints, or user additions
from a template merely because they look malformed. It reports the problem and
preserves the bytes.

## 8. Uninstall and reinstall

### 8.1 Default uninstall is detach

`uawp uninstall` defaults to a state-preserving detach. It:

1. requires valid RELEASED ownership and no unresolved transaction;
2. re-resolves all configured provider entries;
3. removes all registered UAWP consumers, imports, and managed blocks after
   exact drift checks;
4. deletes a native file created by UAWP only when its complete current bytes
   match the registered UAWP artifact and it contains no user additions;
5. otherwise removes only the provably owned region and preserves the file;
6. updates integration registration atomically; and
7. retains `.uawp/`, including history and migration receipts.

The provider used to run the command is irrelevant. Full detach considers all
registered consumers and never assumes that an `AGENTS.md` bridge belongs to
only Codex, Claude Code, WorkBuddy, or the current process.

### 8.2 Purge requires a verified export

Namespace removal is a separate explicit choice:

```text
uawp uninstall --purge-state --export <path>
```

For v1, `--purge-state` without `--export` is invalid. The export destination
MUST be outside the `.uawp/` subtree and MUST not overwrite an existing path.
Before purge, UAWP creates a gzip-compressed POSIX tar archive (`.tar.gz`)
containing the complete namespace plus an export manifest of relative paths,
modes, sizes, and SHA-256 hashes. The archive contains only relative regular-
file and directory entries, in lexical order, with no links or special files.
It then reopens and verifies every archive entry against that manifest.

Purge is planned only after all provider integrations can be safely detached,
ownership is RELEASED, no transaction is unresolved, and export preflight
succeeds. The detach, export verification, and namespace removal form one
recoverable transaction. UAWP MUST NOT remove `.uawp/` if export creation or
verification fails.

The purge preview lists every removed path and the export destination. It
requires explicit approval distinct from a prior detach approval. v1 has no
unexported or evidence-bypassing purge mode.

### 8.3 Reinstall

After detach, `uawp init` recognizes the retained namespace and proposes only
missing provider integrations; it does not recreate or reset state. After a
purge, reinstall is a new initialization unless the user explicitly restores
the verified export first. Restore-from-export automation is not required in
Plan 4; the export format and documented manual restore procedure MUST make
recovery possible.

## 9. Durable transaction protocol

### 9.1 Recovery layout

The existing live `.uawp/RECOVERY.json` evolves to a versioned transaction
journal. Durable recovery material lives at:

```text
.uawp/recovery/<transaction-id>/
├── journal.json
├── plan.json
├── backups/
└── receipt.json          # written only after resolution
```

`.uawp/RECOVERY.json` remains a small live pointer containing the journal
schema version, transaction ID, operation, plan ID, workspace identity, and
current phase. There may be at most one live mutating transaction per
workspace.

The recovery directory is UAWP-owned implementation state. Native and project
files are copied into it only as exact rollback bytes; paths are encoded as
validated logical paths and cannot escape the recovery directory.

### 9.2 Journal requirements

Before the first externally visible mutation, UAWP MUST durably persist and
sync:

- the canonical approved plan and all bound inputs;
- every ordered action with before and after hashes, type, mode, and sequence;
- exact backups for each update or delete;
- sufficient metadata to reverse newly created files and directories;
- transaction phase and per-action state; and
- the original ownership and manifest evidence.

Per-action state is one of `PENDING`, `IN_PROGRESS`, `APPLIED`, `VERIFIED`,
`ROLLED_BACK`, or `ROLLBACK_VERIFIED`. State transitions are written and
directory-synced before and after publication. Recovery never trusts a status
field alone: it compares the live filesystem with the recorded before and
after fingerprints.

The journal and backup files use restrictive permissions. Reads are bounded;
symlinks, special files, duplicate JSON keys, trailing JSON, unsupported
journal versions, and path traversal make automatic recovery unavailable.

### 9.3 Recovery classification

`uawp transaction status` classifies the live transaction as exactly one of:

- `ROLLBACK_AVAILABLE`: every affected path is either the exact recorded
  before state, exact recorded after state, or safely removable transaction-
  created state, and all required backups verify;
- `CONTINUE_AVAILABLE`: completed actions match their after state, pending
  actions match their before state, the in-progress action can be classified
  exactly, and all original plan inputs not intentionally changed by completed
  actions still match;
- `ROLLBACK_OR_CONTINUE_AVAILABLE`: both proofs hold;
- `MANUAL_RECOVERY_REQUIRED`: neither proof is complete; or
- `TRANSACTION_COMPLETE`: all after states verify but final receipt/cleanup
  was interrupted.

The report explains the evidence per path. It never chooses an action on the
user's behalf. When both paths are safe, rollback is recommended as the
conservative default, but still requires its own explicit approval.

### 9.4 Rollback

`uawp transaction rollback` produces a recovery plan bound to the current
journal and live fingerprints. It restores update/delete backups, removes only
transaction-created objects that still match their recorded after state, and
processes actions in reverse order. It refuses to replace any third-party
drift.

Rollback completes only after every path verifies against its exact before
state. It then writes a durable rollback receipt and clears the live pointer.
The transaction evidence and receipt remain for audit until a later explicit
retention policy; Plan 4 does not auto-delete them.

### 9.5 Roll forward

`uawp transaction continue` produces a recovery plan bound to the current
journal and live fingerprints. It verifies already completed actions, safely
classifies the in-progress action, and applies only remaining actions from the
original approved plan. It MUST NOT recompute a different upgrade, repair, or
uninstall plan under the old approval.

Continuation completes only after the original postconditions verify. It
writes a completion receipt and clears the live pointer. If current desired
behavior differs, the user must roll back first and generate a fresh normal
plan.

### 9.6 Completed transaction cleanup

If all after states verify but the process stopped before final cleanup,
transaction status reports `TRANSACTION_COMPLETE`. A separately approved
continue operation may write the missing receipt and clear the pointer without
republishing artifacts.

Ordinary mutations remain blocked while the live pointer exists, including
when its contents are malformed. Read-only `status`, `doctor`, `resume`, and
transaction diagnosis remain available where their required reads are safe.

### 9.7 Bootstrap and purge exceptions

Initialization may need recovery before a valid manifest exists, so the live
pointer remains discoverable directly below `.uawp/` and is authoritative
before `manifest.json` publication.

Purge uses an explicit final commit boundary rather than deleting its live
journal in place. After integrations are detached and the external archive is
verified, UAWP atomically renames `.uawp/` on the same filesystem to a unique
reserved sibling tombstone named `.uawp-purge-<transaction-id>/`. The rename is
the purge commit point. Before the rename, normal transaction rollback applies.
After the rename, the verified export is the durable restore source and the
tombstone is cleanup-only: UAWP verifies it, records purge completion in the
export manifest, and removes it safely. A crash after the commit leaves the
tombstone discoverable by `status`, `doctor`, and transaction diagnosis; it is
never treated as project content or silently adopted. Any pre-existing path in
the reserved tombstone family blocks purge during preflight.

The tombstone is the only temporary exception to the normal `.uawp/` namespace
location. It MUST exist only during an approved purge transaction, retain the
original restrictive permissions, and be removed only after the external
archive and committed state verify.

## 10. Ownership and Human Controller interaction

Upgrade, repair, detach, and purge require `ACTIVE_WORKER.md` to be valid and
`RELEASED`. They do not infer staleness, release an owner, or acquire a new
owner. If ownership is unexpectedly ACTIVE, the user follows the normal
release flow or the separate Human Controller stale-recovery flow first.

Transaction rollback or continuation is exceptional maintenance and may be
necessary while the recorded ownership is ACTIVE because the interrupted
transaction itself may include ownership state. It therefore:

- requires explicit human approval and records the supplied controller
  identifier and reason in the recovery receipt;
- may restore or complete only the ownership bytes already bound in the
  original transaction;
- MUST NOT infer that the worker is stale; and
- MUST NOT acquire a replacement worker.

This controller record is an audit assertion, not authentication or an access
control system. It is distinct from the one-time stale-ownership arbitration
token.

## 11. Error handling and stable behavior

Plan 4 extends diagnostics with stable categories for:

- migration available, unsupported path, and downgrade refused;
- structural maintenance blocked by ACTIVE ownership;
- repair available and manual repair required;
- uninstall detach available, purge export required, and native drift blocked;
- transaction rollback available, continue available, both available,
  complete, malformed, and manual recovery required; and
- export creation or verification failure.

CLI text may improve without breaking automation, but JSON field meanings and
exit categories remain stable. Expected safety refusals are not internal
errors. All reports state whether mutation occurred and what the user can do
next.

## 12. Component boundaries

Implementation SHOULD preserve these responsibilities:

- `internal/core`: version compatibility and migration-domain contracts; no
  provider filenames or filesystem mutation;
- `internal/migration`: registered migration graph and deterministic state
  migration planners;
- `internal/transaction`: journal schema, evidence classification, receipts,
  and rollback/continuation planning;
- `internal/workspace`: workspace discovery, structural-operation planners,
  safe filesystem publication, export, and verification;
- `internal/adapter`: provider re-resolution and owned-integration planning;
- `internal/cli`: parsing and presentation only.

Migration, repair, uninstall, and transaction code MUST call shared plan and
boundary primitives rather than create a second unchecked write path.

## 13. Verification strategy

### 13.1 Migration tests

- every supported source version reaches the target through the exact chain;
- unknown future, missing path, ambiguous path, cycle, and downgrade stop;
- repeated upgrade is a no-op;
- each intermediate failpoint is rollback- or continuation-classifiable;
- manifest is published last and receipts match resulting hashes;
- adapter entry drift blocks unsafe integration migration.

### 13.2 Repair tests

- exact generated artifacts can be restored;
- user-authored state and uncertain native regions are never guessed;
- marker corruption, consumer disagreement, shadowing, and outside-region
  drift produce manual findings;
- preview-to-apply drift invalidates approval;
- repeated repair is a no-op.

### 13.3 Uninstall and reinstall tests

- zero, one, and multiple providers detach safely;
- shared bridges remain until full detach removes all registered consumers;
- user additions to UAWP-created native files survive;
- purge cannot run without a non-existing external export destination;
- export contents, modes, sizes, and hashes verify before deletion;
- failure at every detach/export/purge boundary is recoverable;
- state-preserving reinstall does not reset history or duplicate integrations.

### 13.4 Transaction tests

- failpoints before and after every journal, backup, publication, verification,
  receipt, pointer-clear, and directory-sync boundary;
- exact rollback and continuation for create, update, delete, and directory
  operations;
- user drift after interruption forces manual recovery;
- corrupted, truncated, oversized, duplicated-key, symlinked, and future-
  version recovery data fail closed;
- stale recovery approval cannot be reused as transaction approval;
- concurrent recovery and ordinary mutation serialize safely;
- recovery is restart-safe and idempotent on macOS, Linux, and Windows.

Property and fuzz tests cover logical paths, action sequences, journal decode,
and classification. End-to-end snapshots prove that project-owned content is
unchanged outside approved regions.

## 14. Release gates

Plan 4 is complete only when:

1. all supported migrations have fixtures, deterministic planners, and
   idempotence tests;
2. an unsupported or future version cannot be mutated;
3. upgrade, repair, detach, and purge enforce RELEASED ownership;
4. doctor distinguishes repairable from manual-only drift without mutation;
5. default uninstall preserves `.uawp/` and every non-UAWP byte;
6. purge requires and verifies an external export before namespace removal;
7. every planned filesystem action has durable before evidence before
   publication;
8. interruption at every tested boundary yields a truthful recovery class;
9. rollback and continuation refuse all unbound drift;
10. transaction recovery and stale ACTIVE arbitration remain distinct;
11. reinstall is idempotent after detach and documented after purge;
12. unit, race, vet, fuzz/property, integration, and end-to-end suites pass;
13. user and protocol documentation describe exact commands, limits, exports,
    and manual-recovery boundaries; and
14. no Plan 4 command introduces vendor semantics into Core.

## 15. Acceptance criteria

The written design is accepted when the reviewer confirms:

- forward-only explicit migrations and independent state/CLI versions;
- migration receipts under `.uawp/migrations/`;
- read-only doctor and evidence-limited repair;
- state-preserving detach as the default uninstall;
- explicit purge only with a verified external export;
- no deletion of shared native entry files based on the invoking provider;
- a versioned durable transaction journal with exact backups;
- evidence-based rollback, continuation, or manual-recovery classification;
- `uawp recover` reserved for stale ownership and `uawp transaction` reserved
  for interrupted filesystem transactions;
- RELEASED ownership for normal structural maintenance;
- no semantic merge, force bypass, automatic stale inference, or downgrade;
  and
- Plan 4.5 and Plan 5 remain separate follow-on phases.

After written approval, the next step is a detailed test-first implementation
plan. No Plan 4 product code is authorized by approval of this document alone.
