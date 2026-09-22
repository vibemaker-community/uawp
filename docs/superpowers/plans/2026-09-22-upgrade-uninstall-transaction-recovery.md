# Upgrade, Uninstall, Repair, and Transaction Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make UAWP state and integration maintenance forward-upgradeable, safely repairable, uninstallable with verified export, and recoverable after interruption without guessing or overwriting project-owned content.

**Architecture:** Introduce a strict state-version registry and a provider-neutral transaction package that persists exact plans, backups, action state, and receipts before Workspace publishes mutations. Upgrade, repair, detach, purge, rollback, and continuation remain immutable Workspace plans; CLI only parses and presents them, while adapter code continues to own provider entry resolution.

**Tech Stack:** Go 1.26 standard library only; JSON and JSON Schema; Markdown state; `archive/tar` plus `compress/gzip`; table-driven, fuzz, race, failpoint, and end-to-end tests.

**Spec:** `docs/superpowers/specs/2026-09-22-upgrade-uninstall-transaction-recovery-design.md`

## Global Constraints

- Core MUST remain agent-neutral; provider filenames and precedence stay in adapters.
- Current state becomes `1.1.0`; `1.0.0` is the only v1 upgrade source initially supported.
- CLI release version and manifest `stateVersion` remain independent.
- Every mutation remains previewed, explicitly approved, drift-revalidated, applied, and verified.
- Upgrade, repair, detach, and purge require valid `RELEASED` ownership.
- Transaction recovery and Human Controller stale-ownership recovery remain separate command and token domains.
- Default uninstall preserves `.uawp/`; purge requires a verified external `.tar.gz` export.
- No force flag, semantic merge, automatic downgrade, automatic stale inference, or undocumented provider behavior is permitted.
- Production dependencies remain Go standard-library-only; all paths and formats must work on macOS, Linux, and Windows.
- TDD and one independently reviewable commit per task are mandatory.

## Review Focus

- Crash after native-file publication but before manifest publication must classify truthfully and preserve outside bytes — Tasks 3 and 4.
- Unknown `1.x` minor versions must not be accepted merely because the major is `1` — Tasks 1 and 5.
- A UAWP-created native file with later user additions must survive detach and purge — Tasks 7 and 10.
- Crash after `.uawp` is renamed to a purge tombstone must remain discoverable and cleanup-safe — Tasks 8 and 10.
- Recovery data with path traversal, duplicate JSON keys, symlinks, truncation, or user drift must force manual recovery — Tasks 2, 4, and 10.

---

## File and package map

```text
internal/core/version.go              exact state compatibility policy
internal/migration/registry.go        forward-only migration graph
internal/migration/v1_0_to_v1_1.go    first deterministic state migration
internal/transaction/types.go         journal, action, classification, receipt types
internal/transaction/codec.go         strict bounded canonical persistence
internal/transaction/classify.go      filesystem evidence classification
internal/transaction/recovery.go      rollback/continuation plan construction
internal/workspace/apply.go            transaction-backed normal publication
internal/workspace/upgrade.go          upgrade planner and verifier
internal/workspace/repair.go           evidence-limited repair planner
internal/workspace/uninstall.go        full provider detach planner
internal/workspace/export.go           deterministic verified tar.gz export
internal/workspace/purge.go            tombstone commit and purge recovery
internal/cli/maintenance.go            expert-mode Plan 4 commands
```

Existing `internal/plan` remains the immutable mutation vocabulary. Existing
adapter block/import functions remain the only native integration editors.

### Task 1: Make state compatibility exact and register migrations

**Files:**
- Create: `internal/core/version.go`
- Create: `internal/core/version_test.go`
- Create: `internal/migration/registry.go`
- Create: `internal/migration/registry_test.go`
- Modify: `internal/core/manifest.go`
- Modify: `internal/core/manifest_test.go`
- Modify: `internal/workspace/discover.go`
- Modify: `internal/workspace/discover_test.go`
- Modify: `schemas/manifest.schema.json`

**Interfaces:**
- Produces: `core.CurrentStateVersion`, `core.ClassifyStateVersion(string) StateCompatibility`, and `migration.Registry.Resolve(from, to string) ([]migration.Step, error)`.
- Consumes: existing strict manifest structure and `plan.Change` types.

- [ ] **Step 1: Write failing exact-version compatibility tests**

```go
func TestClassifyStateVersion(t *testing.T) {
    tests := map[string]StateCompatibility{
        "1.1.0": StateCurrent,
        "1.0.0": StateUpgradeRequired,
        "1.0.1": StateUnsupported,
        "1.99.0": StateUnsupported,
        "2.0.0": StateFutureMajor,
        "0.9.0": StateUnsupported,
    }
    for version, want := range tests {
        if got := ClassifyStateVersion(version); got != want {
            t.Fatalf("%s: got %s want %s", version, got, want)
        }
    }
}
```

Also assert that manifest decoding accepts structurally valid `1.0.0` for
upgrade discovery, rejects malformed versions, and does not report an unknown
minor as ready.

- [ ] **Step 2: Run the focused tests and verify failure**

Run: `go test ./internal/core ./internal/workspace -run 'StateVersion|ManifestCompatibility|DiscoverVersion' -v`

Expected: FAIL because exact compatibility types and findings do not exist.

- [ ] **Step 3: Implement exact compatibility without weakening strict JSON**

```go
const CurrentStateVersion = "1.1.0"

type StateCompatibility string
const (
    StateCurrent StateCompatibility = "CURRENT"
    StateUpgradeRequired StateCompatibility = "UPGRADE_REQUIRED"
    StateUnsupported StateCompatibility = "UNSUPPORTED"
    StateFutureMajor StateCompatibility = "FUTURE_MAJOR"
)

func ClassifyStateVersion(v string) StateCompatibility {
    switch v {
    case CurrentStateVersion:
        return StateCurrent
    case "1.0.0":
        return StateUpgradeRequired
    default:
        if strings.HasPrefix(v, "2.") { return StateFutureMajor }
        return StateUnsupported
    }
}
```

Separate structural manifest validation from compatibility classification so
`Discover` can identify an owned old namespace without treating it as ready.
Keep duplicate keys, unknown fields, trailing JSON, and integration validation
strict.

- [ ] **Step 4: Write failing migration graph tests**

Test direct resolution, a synthetic two-step chain, missing path, branch,
cycle, downgrade, duplicate edge, and deterministic ordering. Use:

```go
type Context struct {
    Manifest core.Manifest
    Inputs []plan.Input
    GeneratedAt time.Time
}
type Step interface {
    From() string
    To() string
    Plan(Context) ([]plan.Change, error)
    Verify(Context) error
}
type Registry struct { steps []Step }
func (r Registry) Resolve(from, to string) ([]Step, error)
```

- [ ] **Step 5: Implement the forward-only registry and update schema**

Reject any registry whose source has multiple outgoing edges, whose graph has a
cycle, or whose step does not increase semantic version. Update the schema to
describe current `1.1.0` artifacts while preserving `1.0.0` as an importable
migration fixture rather than broadly accepting `1.*` as ready.

- [ ] **Step 6: Run tests and commit**

Run: `go test ./internal/core ./internal/migration ./internal/workspace -run 'Version|Migration|Manifest' -v && git diff --check`

Expected: PASS.

```bash
git add internal/core/version.go internal/core/version_test.go internal/core/manifest.go internal/core/manifest_test.go internal/migration/registry.go internal/migration/registry_test.go internal/workspace/discover.go internal/workspace/discover_test.go schemas/manifest.schema.json
git commit -m "feat(core): define exact state compatibility"
```

### Task 2: Define strict durable transaction records

**Files:**
- Create: `internal/transaction/types.go`
- Create: `internal/transaction/types_test.go`
- Create: `internal/transaction/codec.go`
- Create: `internal/transaction/codec_test.go`
- Modify: `internal/plan/change.go`
- Modify: `internal/plan/change_test.go`
- Modify: `internal/workspace/boundary.go`
- Modify: `internal/workspace/boundary_test.go`

**Interfaces:**
- Consumes: canonical `plan.Plan`, ordered `plan.Change`, and `plan.Input`.
- Produces: `transaction.Journal`, `transaction.Action`, `transaction.Receipt`, `WriteBundle(root string, p plan.Plan, id string, at time.Time) (Bundle, error)`, and strict `LoadBundle(root, transactionID string) (Bundle, error)`.

- [ ] **Step 1: Write failing round-trip and hostile-input tests**

Cover create directory/file, update, and delete actions; before/after hashes;
modes; sequence; base64 content; full plan inputs and metadata; duplicate keys;
unknown fields; trailing JSON; files over 8 MiB; symlinked recovery paths;
absolute/traversal/backslash paths; unsupported journal schema; duplicate action
paths; and invalid state transitions.

```go
type ActionState string
const (
    Pending ActionState = "PENDING"
    InProgress ActionState = "IN_PROGRESS"
    Applied ActionState = "APPLIED"
    Verified ActionState = "VERIFIED"
    RolledBack ActionState = "ROLLED_BACK"
    RollbackVerified ActionState = "ROLLBACK_VERIFIED"
)

type Action struct {
    Index int `json:"index"`
    Change plan.PersistedChange `json:"change"`
    State ActionState `json:"state"`
    BackupPath string `json:"backupPath,omitempty"`
    BackupSHA256 string `json:"backupSHA256,omitempty"`
}
```

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./internal/plan ./internal/transaction ./internal/workspace -run 'Persisted|Journal|RecoveryBoundary' -v`

Expected: FAIL because persisted changes and transaction codecs are undefined.

- [ ] **Step 3: Add a lossless persisted plan representation**

Add `plan.PersistedChange`, `plan.PersistedPlan`, `Plan.Persisted()`, and
`plan.Restore(PersistedPlan) (Plan, error)`. Restoration recomputes the plan ID
from operation, workspace, changes, inputs, and metadata and rejects a stored
ID mismatch. Content remains defensively copied.

- [ ] **Step 4: Implement strict transaction types and codec**

```go
type Journal struct {
    SchemaVersion string `json:"schemaVersion"`
    TransactionID string `json:"transactionID"`
    Operation string `json:"operation"`
    PlanID string `json:"planID"`
    Workspace string `json:"workspace"`
    Phase string `json:"phase"`
    StartedAt string `json:"startedAt"`
    Actions []Action `json:"actions"`
}

type LivePointer struct {
    SchemaVersion, TransactionID, Operation, PlanID, Workspace, Phase string
}
```

Write the bundle to `.uawp/recovery/<transaction-id>/` with `0700`
directories and `0600` files. The live `.uawp/RECOVERY.json` contains only the
strict pointer. Use bounded streaming reads and the existing duplicate-key
strategy; never follow links.

- [ ] **Step 5: Expand the UAWP path allowlist narrowly**

Allow `migrations/<receipt>.json`, `recovery/<transaction-id>/{journal.json,plan.json,receipt.json}`, and
`recovery/<transaction-id>/backups/<encoded-name>` only after portable-segment,
depth, and symlink-ancestor validation. Do not make `.uawp/**` generally
writable.

- [ ] **Step 6: Run tests and commit**

Run: `go test ./internal/plan ./internal/transaction ./internal/workspace -run 'Persisted|Journal|RecoveryBoundary' -v && git diff --check`

Expected: PASS.

```bash
git add internal/transaction/types.go internal/transaction/types_test.go internal/transaction/codec.go internal/transaction/codec_test.go internal/plan/change.go internal/plan/change_test.go internal/workspace/boundary.go internal/workspace/boundary_test.go
git commit -m "feat(transaction): define durable recovery records"
```

### Task 3: Replace best-effort apply with durable publication

**Files:**
- Modify: `internal/workspace/apply.go`
- Modify: `internal/workspace/apply_test.go`
- Create: `internal/workspace/transaction_apply.go`
- Create: `internal/workspace/transaction_apply_test.go`
- Modify: `internal/workspace/init.go`
- Modify: `internal/workspace/init_test.go`

**Interfaces:**
- Consumes: `transaction.WriteBundle` and existing `Apply(root, plan, ApplyOptions)` callers.
- Produces: the same public `Apply` contract backed by durable action evidence; normal successful operations leave a completion receipt and no live pointer.

- [ ] **Step 1: Write failpoint tests for every durability boundary**

Create table-driven tests for `before-bundle`, `after-plan-sync`,
`after-backup-sync`, `after-pointer-sync`, `before-publish`, `after-publish`,
`after-action-verify`, `after-receipt-sync`, and `after-pointer-clear`. For each
failpoint snapshot project-owned bytes and assert either no mutation or
`RECOVERY_REQUIRED` with an intact bundle.

```go
for _, stage := range transactionFailpoints {
    t.Run(stage, func(t *testing.T) {
        err := applyWithFailure(t, stage)
        if err == nil { t.Fatal("expected interruption") }
        assertProjectBytesUnchangedOutsidePlan(t)
        assertJournalDecodesAndBackupsVerify(t)
    })
}
```

- [ ] **Step 2: Run focused tests and verify failure**

Run: `go test ./internal/workspace -run 'DurableApply|TransactionFailpoint|InitRecovery' -v`

Expected: FAIL because current backups are temporary and the old journal lacks
the plan and exact before evidence.

- [ ] **Step 3: Stage all backups before the first mutation**

For every update/delete, copy bytes to the transaction backup directory using
bounded regular-file reads, sync the backup, verify its SHA-256 and mode, then
sync the backup directory. Record create actions with exact absence evidence.
Persist and sync the complete plan and journal before publishing the live
pointer; persist and sync the pointer before action zero.

- [ ] **Step 4: Publish actions with journal state transitions**

For each action write `IN_PROGRESS`, publish using the existing safe create or
replace primitive, write `APPLIED`, verify exact after state, then write
`VERIFIED`. Revalidate unaffected plan inputs immediately before each action.
Do not invoke the current destructive best-effort `rollback` from an error
path; leave durable evidence and return recovery-required.

- [ ] **Step 5: Finish successful transactions durably**

Write `receipt.json` containing transaction ID, operation, plan ID, completion
time, result `COMPLETED`, and resulting hashes. Sync it, atomically clear the
live pointer, and sync `.uawp/`. Keep the recovery directory and receipt.
Update initialization so the first namespace creation can safely establish
the recovery bundle before `manifest.json` exists.

- [ ] **Step 6: Run regression tests and commit**

Run: `go test ./internal/workspace ./internal/e2e -run 'Apply|Init|RecoveryRequired' -v`

Expected: PASS, including existing approval, drift, concurrent replacement,
and interruption tests.

```bash
git add internal/workspace/apply.go internal/workspace/apply_test.go internal/workspace/transaction_apply.go internal/workspace/transaction_apply_test.go internal/workspace/init.go internal/workspace/init_test.go
git commit -m "feat(workspace): make apply durably recoverable"
```

### Task 4: Classify and execute transaction recovery

**Files:**
- Create: `internal/transaction/classify.go`
- Create: `internal/transaction/classify_test.go`
- Create: `internal/transaction/recovery.go`
- Create: `internal/transaction/recovery_test.go`
- Create: `internal/workspace/transaction_recovery.go`
- Create: `internal/workspace/transaction_recovery_test.go`
- Modify: `internal/workspace/status.go`
- Modify: `internal/workspace/status_test.go`

**Interfaces:**
- Produces: `transaction.Classify(bundle, observation) Report`, `workspace.TransactionStatus`, `PlanTransactionRollbackAt`, `PlanTransactionContinueAt`, and `ApplyTransactionRecovery`.
- Consumes: durable bundle from Task 3 and exact filesystem observations from Workspace.

- [ ] **Step 1: Write the classification truth table**

```go
type Classification string
const (
    RollbackAvailable Classification = "ROLLBACK_AVAILABLE"
    ContinueAvailable Classification = "CONTINUE_AVAILABLE"
    BothAvailable Classification = "ROLLBACK_OR_CONTINUE_AVAILABLE"
    ManualRequired Classification = "MANUAL_RECOVERY_REQUIRED"
    TransactionComplete Classification = "TRANSACTION_COMPLETE"
)
```

Exercise each action kind with live `before`, `after`, `missing`, and `other`
states; missing/corrupt backup; inconsistent journal state; completed after
state with missing receipt; malformed pointer; and drift in an unrelated bound
input. Assert the report includes per-path evidence and does not mutate files.

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./internal/transaction ./internal/workspace -run 'Classif|TransactionStatus|RecoveryPlan' -v`

Expected: FAIL because classification and recovery planners do not exist.

- [ ] **Step 3: Implement evidence classification**

Observe paths through Workspace boundary functions and pass only typed facts
to `internal/transaction`. Never let the transaction package open provider
paths. A journal label never overrides a live hash. Any special file, link,
permission failure, unbound bytes, or invalid backup makes the affected path
manual-only.

- [ ] **Step 4: Implement rollback and continuation plans**

Rollback restores update/delete backups and removes transaction-created
objects only when their after state still matches, in reverse sequence.
Continuation reconstructs the stored plan, verifies applied after states and
pending before states, and includes only the original remaining actions.
Both plan metadata values bind `transactionID`, `journalSHA256`,
`controllerID`, `reason`, and action:

```go
func PlanTransactionRollbackAt(root Root, controllerID, reason string, at time.Time) (plan.Plan, error)
func PlanTransactionContinueAt(root Root, controllerID, reason string, at time.Time) (plan.Plan, error)
```

Reject empty controller or reason, but do not treat either as authentication.

- [ ] **Step 5: Add the recovery-only apply path**

`ApplyTransactionRecovery` acquires the same transaction lock but accepts only
a plan whose transaction metadata and current journal hash match. It may run
while the live pointer exists; ordinary `Apply` still may not. On verified
completion, write a `ROLLED_BACK` or `COMPLETED` receipt, sync it, and clear the
pointer. Replaying the approval is a no-op refusal, not success.

- [ ] **Step 6: Run tests and commit**

Run: `go test ./internal/transaction ./internal/workspace ./internal/e2e -run 'Transaction|RecoveryRequired' -v`

Expected: PASS.

```bash
git add internal/transaction/classify.go internal/transaction/classify_test.go internal/transaction/recovery.go internal/transaction/recovery_test.go internal/workspace/transaction_recovery.go internal/workspace/transaction_recovery_test.go internal/workspace/status.go internal/workspace/status_test.go
git commit -m "feat(transaction): add evidence-based recovery"
```

### Task 5: Implement the `1.0.0` to `1.1.0` upgrade

**Files:**
- Create: `internal/migration/v1_0_to_v1_1.go`
- Create: `internal/migration/v1_0_to_v1_1_test.go`
- Create: `internal/workspace/upgrade.go`
- Create: `internal/workspace/upgrade_test.go`
- Modify: `internal/workspace/init.go`
- Modify: `internal/workspace/init_test.go`
- Modify: `schemas/manifest.schema.json`
- Create: `testdata/migrations/v1.0.0/manifest.json`

**Interfaces:**
- Consumes: migration registry, adapter `ResolveAdapters`, current manifest integrations, and durable `Apply`.
- Produces: `PlanUpgradeAt(root Root, facts adapter.RuntimeFacts, at time.Time) (plan.Plan, UpgradeReport, error)` and `VerifyUpgrade`.

- [ ] **Step 1: Write failing upgrade matrix tests**

Cover clean `1.0.0`, current `1.1.0` no-op, unknown minor, future major,
downgrade request, ACTIVE ownership, missing/malformed state, healthy adapters,
adapter entry drift, preview drift, repeated upgrade, and receipt collision.

```go
p, report, err := PlanUpgradeAt(root, adapter.RuntimeFacts{}, at)
if err != nil { t.Fatal(err) }
if report.From != "1.0.0" || report.To != "1.1.0" { t.Fatal(report) }
assertLastChange(t, p, ".uawp/manifest.json")
```

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./internal/migration ./internal/workspace -run 'V1_0ToV1_1|Upgrade' -v`

Expected: FAIL because the concrete migration and planner are absent.

- [ ] **Step 3: Implement the migration step**

Create `.uawp/migrations/` and `.uawp/recovery/` at `0700`, retain every
existing state and integration record, bind a migration receipt path plus its
from/to/version fields in plan metadata, and publish a `1.1.0` manifest last.
After all planned changes verify, the transaction finalizer writes the derived
migration receipt with the now-known approved plan ID, CLI version, UTC
completion time, affected paths, and resulting hashes. The preview renders
this derived receipt separately from `plan.Change`; approval binds every
non-derived field through plan metadata, and the receipt writer may fill only
the plan ID and verified result hashes. This avoids self-hashing a receipt that
contains its own plan ID without creating an unapproved general write path.

- [ ] **Step 4: Re-resolve integrations before planning**

For every configured consumer, call the existing adapter resolver with
provided runtime facts. A healthy unchanged route remains unchanged. A safely
migratable owned block/import may be included. Unsupported, shadowed,
duplicated, or drifted routes block upgrade with stable findings; do not repair
them implicitly.

- [ ] **Step 5: Update new initialization to `1.1.0`**

New workspaces create the two directories directly and require no migration
receipt. Existing `1.0.0` fixtures remain decodable only for upgrade. Verify
that `init` on an old workspace reports upgrade required rather than resetting
it.

- [ ] **Step 6: Run tests and commit**

Run: `go test ./internal/migration ./internal/workspace ./internal/e2e -run 'Upgrade|Init' -v`

Expected: PASS.

```bash
git add internal/migration/v1_0_to_v1_1.go internal/migration/v1_0_to_v1_1_test.go internal/workspace/upgrade.go internal/workspace/upgrade_test.go internal/workspace/init.go internal/workspace/init_test.go schemas/manifest.schema.json testdata/migrations/v1.0.0/manifest.json
git commit -m "feat: upgrade workspace state to version 1.1"
```

### Task 6: Expand doctor and add evidence-limited repair

**Files:**
- Modify: `internal/workspace/status.go`
- Modify: `internal/workspace/status_test.go`
- Create: `internal/workspace/repair.go`
- Create: `internal/workspace/repair_test.go`
- Modify: `templates/state/embed.go`

**Interfaces:**
- Produces: multi-finding `DoctorReport`, stable maintenance finding codes, and `PlanRepairAt(root Root, facts adapter.RuntimeFacts, selected []string, at time.Time) (plan.Plan, []StatusReport, error)`.
- Consumes: exact current-version templates, integration evidence, transaction status, and ownership state.

- [ ] **Step 1: Write failing diagnostic matrix tests**

Replace the single-alias doctor model with a report containing all findings.
Cover migration available, ACTIVE maintenance block, missing generated
`INSTRUCTIONS.md`, missing user-authored `CONTEXT.md`, marker corruption,
manifest/marker disagreement, outside drift, unsupported route, live recovery,
and purge tombstone. Snapshot before/after to prove read-only behavior.

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./internal/workspace -run 'DoctorMaintenance|Repair' -v`

Expected: FAIL because multi-finding diagnosis and repair planning are absent.

- [ ] **Step 3: Implement stable finding categories**

Add codes for `UPGRADE_AVAILABLE`, `MAINTENANCE_BLOCKED_ACTIVE`,
`REPAIR_AVAILABLE`, `MANUAL_REPAIR_REQUIRED`, and transaction classifications.
Preserve existing status codes and JSON meanings. `status` stays concise;
`doctor` aggregates findings in deterministic severity/code/path order.

- [ ] **Step 4: Implement repairable cases only**

Permit exact regeneration of a missing current-version generated instruction,
an unchanged fully UAWP-created native file, and a provably owned import/block
whose outside bytes match. Permit registry reconciliation only when artifact
and consumer evidence independently agree. Require RELEASED ownership and no
live transaction. Bind every diagnostic input into the repair plan.

- [ ] **Step 5: Prove manual-only preservation**

Tests must assert zero changes for user-authored state loss, malformed marker
boundaries, outside-content drift, unsupported entries, symlinks, binary
native files, and ambiguous registry evidence. No repair test may call a
semantic merge helper.

- [ ] **Step 6: Run tests and commit**

Run: `go test ./internal/workspace ./internal/adapter -run 'Doctor|Repair|Drift' -v`

Expected: PASS.

```bash
git add internal/workspace/status.go internal/workspace/status_test.go internal/workspace/repair.go internal/workspace/repair_test.go templates/state/embed.go
git commit -m "feat(workspace): diagnose and plan safe repairs"
```

### Task 7: Implement state-preserving full detach

**Files:**
- Create: `internal/workspace/uninstall.go`
- Create: `internal/workspace/uninstall_test.go`
- Modify: `internal/adapter/planner.go`
- Modify: `internal/adapter/planner_test.go`
- Modify: `internal/workspace/adapters.go`

**Interfaces:**
- Produces: `PlanUninstallDetachAt(root Root, facts adapter.RuntimeFacts, at time.Time) (plan.Plan, UninstallReport, error)`.
- Consumes: existing consumer-aware block/import removal and durable Apply.

- [ ] **Step 1: Write failing detach tests**

Cover no adapters, one adapter, all three adapters, a shared `AGENTS.md`
bridge, user-created native file, unchanged UAWP-created native file,
UAWP-created file with user additions, import plus project rules, drifted
markers, changed effective entry, ACTIVE ownership, and repeat detach.

```go
p, report, err := PlanUninstallDetachAt(root, facts, at)
if err != nil { t.Fatal(err) }
if len(report.ConsumersRemoved) != 3 { t.Fatal(report) }
assertNoChangeOutsideOwnedRegions(t, p)
assertNoDeletePrefix(t, p, ".uawp/")
```

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./internal/adapter ./internal/workspace -run 'FullDetach|Uninstall' -v`

Expected: FAIL because full detach planner does not exist.

- [ ] **Step 3: Generalize adapter removal to all consumers**

Add a planner that computes all registered consumer removals in one immutable
transaction. Re-resolve every provider first. Remove only exact imports and
managed blocks. Delete a native file only when `CreatedFile` is true, its full
hash matches, and removal leaves zero bytes; otherwise keep the file and
project additions. Publish the emptied integration registry last.

- [ ] **Step 4: Enforce maintenance preconditions and verification**

Require current state, valid RELEASED ownership, no live transaction, and no
native drift. Post-apply verification proves no configured provider loads a
UAWP bridge and `.uawp/` history is unchanged except manifest integration
registration and transaction receipt.

- [ ] **Step 5: Run tests and commit**

Run: `go test ./internal/adapter ./internal/workspace ./internal/e2e -run 'AdapterRemove|FullDetach|Uninstall' -v`

Expected: PASS.

```bash
git add internal/workspace/uninstall.go internal/workspace/uninstall_test.go internal/adapter/planner.go internal/adapter/planner_test.go internal/workspace/adapters.go
git commit -m "feat(workspace): add state-preserving uninstall"
```

### Task 8: Add verified export and purge tombstone commit

**Files:**
- Create: `internal/workspace/export.go`
- Create: `internal/workspace/export_test.go`
- Create: `internal/workspace/purge.go`
- Create: `internal/workspace/purge_test.go`
- Modify: `internal/workspace/discover.go`
- Modify: `internal/workspace/status.go`
- Modify: `internal/workspace/boundary.go`

**Interfaces:**
- Produces: `PlanPurgeAt(root Root, facts adapter.RuntimeFacts, exportPath string, at time.Time) (plan.Plan, PurgeReport, error)`, `CreateVerifiedExport(root Root, destination string, snapshot NamespaceSnapshot) (ExportManifest, error)`, `ApplyPurge(root Root, p plan.Plan, options PurgeOptions) (PurgeReport, error)`, and tombstone discovery/cleanup.
- Consumes: successful full-detach planning, durable transaction evidence, and exact namespace snapshot.

- [ ] **Step 1: Write deterministic export tests**

Create a namespace with Unicode paths, empty directories, migration receipts,
and recovery receipts. Assert two exports of the same snapshot have identical
entry order and manifest hashes; reject links, special files, unsafe modes,
existing destination, destination inside `.uawp`, and concurrent source drift.

```go
type ExportManifest struct {
    SchemaVersion string `json:"schemaVersion"`
    Protocol string `json:"protocol"`
    StateVersion string `json:"stateVersion"`
    Entries []ExportEntry `json:"entries"`
    Purge *PurgeCommit `json:"purge,omitempty"`
}
```

- [ ] **Step 2: Run export tests and verify failure**

Run: `go test ./internal/workspace -run 'Export|Purge|Tombstone' -v`

Expected: FAIL because archive and purge APIs do not exist.

- [ ] **Step 3: Implement canonical `.tar.gz` export and verification**

Use lexical relative paths, regular files/directories only, normalized tar
headers, preserved permission bits, zeroed volatile owner fields, and an
export manifest containing path, kind, mode, size, and SHA-256. Write to an
exclusive temporary sibling, sync, reopen through gzip/tar, reject duplicate
entries or unsafe names, verify all hashes, then publish destination with
create-if-absent semantics.

- [ ] **Step 4: Implement purge preflight and commit boundary**

`PlanPurgeAt(root, facts, exportPath, at)` requires a non-existing external
`.tar.gz` destination, RELEASED ownership, current state, no recovery pointer,
no `.uawp-purge-*` collision, and a safely detachable provider set. Apply the
detach, create and verify the export, then atomically rename `.uawp` to
`.uawp-purge-<transaction-id>`. The rename is the commit point.

- [ ] **Step 5: Implement tombstone diagnosis and safe cleanup**

On startup, discover reserved tombstones before reporting `UNINITIALIZED`.
Verify their embedded transaction ID and exported archive before cleanup.
Crash before rename follows ordinary rollback; crash after rename reports
`TRANSACTION_COMPLETE` and continuation only records purge completion in the
archive manifest/sidecar receipt and removes the exact verified tombstone.
Never adopt an unrecognized similarly named directory.

- [ ] **Step 6: Run failpoint tests and commit**

Inject failure before export, after export sync, after archive verify, before
rename, after rename, and during tombstone cleanup.

Run: `go test ./internal/workspace ./internal/e2e -run 'Export|Purge|Tombstone' -v`

Expected: PASS with no lost state and no overwritten export.

```bash
git add internal/workspace/export.go internal/workspace/export_test.go internal/workspace/purge.go internal/workspace/purge_test.go internal/workspace/discover.go internal/workspace/status.go internal/workspace/boundary.go
git commit -m "feat(workspace): add verified purge export"
```

### Task 9: Expose expert-mode maintenance commands

**Files:**
- Create: `internal/cli/maintenance.go`
- Create: `internal/cli/maintenance_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`
- Modify: `internal/cli/lifecycle.go`

**Interfaces:**
- Produces: `uawp upgrade`, `repair`, `uninstall`, and `transaction status|rollback|continue`.
- Consumes: all Workspace planners and existing timestamp-bound approval token helpers.

- [ ] **Step 1: Write failing CLI contract tests**

Cover text/JSON output, preview exit `5`, wrong/replayed/drifted approval exit
`5`, unsafe refusal `3`, invalid/recovery state `4`, usage `2`, internal `10`,
read-only transaction status, controller/reason requirements, purge export
requirements, and the unchanged meaning of `uawp recover`.

```go
preview := runCLI(t, 5, "transaction", "rollback", "--workspace", dir,
    "--controller-id", "human-1", "--reason", "confirmed recovery",
    "--format", "json")
runCLI(t, 0, "transaction", "rollback", "--workspace", dir,
    "--controller-id", "human-1", "--reason", "confirmed recovery",
    "--approve", preview.PlanID, "--format", "json")
```

- [ ] **Step 2: Run tests and verify failure**

Run: `go test ./internal/cli -run 'Upgrade|Repair|Uninstall|Transaction' -v`

Expected: FAIL because routing and parsers are absent.

- [ ] **Step 3: Implement thin command routing**

Keep migration, adapter, ownership, export, and classification logic out of
CLI. All mutating commands reconstruct the exact time-bound plan from approval
token time, compare the plan hash, then call normal Apply,
`ApplyTransactionRecovery`, or `ApplyPurge` as appropriate. JSON reports
include schema version, command, workspace, findings, plan ID, changes,
classification, mutation flag, and next action.

- [ ] **Step 4: Preserve stale-recovery separation**

Usage and docs must show `recover` only among lifecycle commands and
`transaction` only among maintenance commands. A stale-recovery token supplied
to transaction rollback and a transaction token supplied to `recover` both
fail without mutation.

- [ ] **Step 5: Run CLI regression tests and commit**

Run: `go test ./internal/cli ./internal/e2e -run 'CLI|Upgrade|Repair|Uninstall|Transaction|StaleRecovery' -v`

Expected: PASS.

```bash
git add internal/cli/maintenance.go internal/cli/maintenance_test.go internal/cli/run.go internal/cli/run_test.go internal/cli/lifecycle.go
git commit -m "feat(cli): expose safe workspace maintenance"
```

### Task 10: Complete end-to-end safety gates and documentation

**Files:**
- Create: `internal/e2e/maintenance_test.go`
- Create: `internal/e2e/transaction_recovery_test.go`
- Create: `internal/transaction/fuzz_test.go`
- Create: `docs/user/upgrade-and-repair.md`
- Create: `docs/user/uninstall-and-export.md`
- Create: `docs/user/transaction-recovery.md`
- Modify: `docs/user/diagnostics.md`
- Modify: `docs/protocol/state-v1.md`
- Modify: `docs/engineering/package-boundaries.md`
- Modify: `docs/engineering/implementation-roadmap.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md`

**Interfaces:**
- Produces: whole-product evidence for every Plan 4 release gate and exact operator guidance.
- Consumes: all Plan 4 public commands and persisted formats.

- [ ] **Step 1: Write failing end-to-end lifecycle tests**

Exercise a real `1.0.0` brownfield workspace with Codex, Claude Code, and
WorkBuddy integrations through upgrade, doctor, safe repair, full detach,
reinstall, exported purge, manual restore, and new init. Snapshot all
project-owned bytes before and after each operation; assert only previewed
managed regions differ.

- [ ] **Step 2: Write exhaustive interruption scenarios**

For create/update/delete/directory, adapter migration, repair, detach, and
purge, interrupt at every named failpoint. Restart through public commands;
assert reported classification, approved rollback or continuation, idempotent
replay refusal, verified receipts, and preserved unrelated bytes.

- [ ] **Step 3: Add fuzz and hostile fixture coverage**

Fuzz strict journal/pointer/receipt decoding, relative paths, classification
action sequences, tar entry names, duplicate keys, and truncated inputs. Seed
with Windows device names, separators, Unicode normalization cases, NUL,
oversized lengths, symlink fixtures, and future schema versions.

- [ ] **Step 4: Write operator documentation with exact examples**

Document preview/apply examples and exit codes for upgrade, repair, default
detach, purge with export, transaction status, rollback, and continue. Explain
that controller ID/reason are audit assertions; differentiate transaction
recovery from stale ownership; document archive verification and manual
restore; state every automatic-repair refusal boundary.

- [ ] **Step 5: Update protocol and roadmap status**

Document state `1.1.0`, migration receipts, recovery directories, purge
tombstones, and transaction classifications. Mark Plan 4 implemented only
after all commands and gates pass. Keep Plan 4.5 next and Plan 5 pending.

- [ ] **Step 6: Run the complete release gates**

```bash
gofmt -w internal
go test ./...
go test -race ./...
go vet ./...
go test ./internal/transaction -run=Fuzz -fuzz=FuzzJournal -fuzztime=10s
go test ./internal/transaction -run=Fuzz -fuzz=FuzzClassification -fuzztime=10s
git diff --check
go build -trimpath -o bin/uawp ./cmd/uawp
./bin/uawp version
```

Expected: every command exits `0`; race and vet report no findings; fuzzing
finds no crash or invariant violation; version prints `uawp dev`; `bin/`
remains ignored.

- [ ] **Step 7: Commit the completed Plan 4 evidence**

```bash
git add internal/e2e/maintenance_test.go internal/e2e/transaction_recovery_test.go internal/transaction/fuzz_test.go docs/user/upgrade-and-repair.md docs/user/uninstall-and-export.md docs/user/transaction-recovery.md docs/user/diagnostics.md docs/protocol/state-v1.md docs/engineering/package-boundaries.md docs/engineering/implementation-roadmap.md README.md CHANGELOG.md
git commit -m "docs: complete upgrade uninstall and recovery gates"
```

## Self-review record

- Spec sections 1–15 map to Tasks 1–10; upgrade, repair, detach, purge,
  reinstall, transaction recovery, ownership, diagnostics, CLI, and release
  gates each have an owning task.
- No task permits semantic merging, force bypass, automatic stale inference,
  downgrade, native-file ownership transfer, or a second unchecked write path.
- Interfaces consistently use immutable `plan.Plan`, exact state versions,
  adapter resolution, Workspace filesystem boundaries, and transaction
  evidence.
- The five Review Focus risks are each pinned by explicit tests in their owning
  tasks.
- Plan 4.5 interaction work and Plan 5 packaging work remain outside this plan.
