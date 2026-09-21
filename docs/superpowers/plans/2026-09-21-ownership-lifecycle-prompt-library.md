# Ownership Lifecycle and Prompt Library Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make UAWP's normal worker lifecycle and separately authorized stale-claim recovery executable, auditable, preview-first, and agent-neutral.

**Architecture:** Extend the agent-neutral Core with explicit ownership transitions, add focused workspace planners for normal lifecycle operations and Human Controller recovery, and expose them through thin CLI commands sharing the existing approval-token contract. Versioned Prompt Library assets describe the same operations without depending on any agent vendor or native instruction file.

**Tech Stack:** Go 1.26+, standard library only, Markdown state documents, GitHub Actions, Go unit/e2e/fuzz/race tests.

**Spec:** `docs/superpowers/specs/2026-09-21-ownership-lifecycle-prompt-library-design.md`

## Global Constraints

- Core MUST remain agent-neutral and MUST NOT contain vendor commands, vendor-native filenames, or auto-loading assumptions.
- Runtime writes MUST remain under the canonical project-root `.uawp/` namespace.
- Every mutation MUST use discover, validate, immutable plan, preview, explicit approval, drift revalidation, apply, and verify.
- Only the current ACTIVE Worker may sync, checkpoint, hand off, or normally release.
- Stale ACTIVE recovery MUST require Human Controller approval and MUST NOT automatically acquire a replacement owner.
- Time passing MUST NOT release ownership; no heartbeat, TTL, lease, or automatic liveness inference is permitted.
- Checkpoints MUST be explicit, milestone-driven, immutable, and portable across macOS, Linux, and Windows.
- Standard-library-only dependency policy from `docs/adr/0001-go-toolchain-and-dependency-policy.md` remains binding.

## Review Focus

- Two workers preview acquire from the same RELEASED state: after one succeeds, the other's approval MUST fail before any write.
- A worker previews sync or handoff and ownership changes before apply: the plan MUST be rejected without partial mutation.
- A stale-recovery token generated for worker A is presented after worker B becomes ACTIVE: it MUST fail and MUST NOT release worker B.
- A checkpoint label contains traversal, separators, Windows device aliases, Unicode normalization ambiguity, or an existing name: creation MUST stop without overwrite.
- A multi-file handoff or recovery is interrupted after publication: diagnostics MUST report `RECOVERY_REQUIRED` and project-owned files MUST remain unchanged.

---

### Task 1: Ownership document codec and transition state machine

**Files:**
- Create: `internal/core/ownership_codec.go`
- Create: `internal/core/ownership_transition.go`
- Test: `internal/core/ownership_codec_test.go`
- Test: `internal/core/ownership_transition_test.go`
- Modify: `internal/workspace/status.go`

**Interfaces:**
- Consumes: existing `core.Ownership`, `ValidateOwnership`, `ParseTimestamp`, and `FormatTimestamp`.
- Produces: `DecodeOwnership(io.Reader) (Ownership, error)`, `EncodeOwnership(Ownership) ([]byte, error)`, `Acquire(current Ownership, request AcquireRequest) (Ownership, error)`, `Release(current Ownership, request ReleaseRequest) (Ownership, error)`, and stable errors that workspace planners can classify.

- [ ] **Step 1: Write failing codec tests**

Cover exact round-trip formatting, CRLF input, duplicate recognized fields, missing fields, unknown status, conflicting release time, trailing duplicate headings, and a 1 MiB input limit. Assert that duplicate fields never use last-value-wins behavior.

- [ ] **Step 2: Run the codec tests and verify RED**

Run: `go test ./internal/core -run 'Test(Decode|Encode)Ownership' -v`

Expected: FAIL because the codec functions do not exist.

- [ ] **Step 3: Implement the strict ownership codec**

Use a deterministic Markdown field order matching `templates/state/ACTIVE_WORKER.md`. Parse only the six recognized list fields, reject duplicates, validate the result with `ValidateOwnership`, and return defensive values. Move `status.go` to this shared decoder so diagnostics and mutation use identical semantics.

- [ ] **Step 4: Write failing transition tests**

Test RELEASED-to-ACTIVE acquisition, same/other ACTIVE conflicts, ACTIVE-owner release, non-owner release, timestamp ordering, trimmed-but-nonempty identifiers, and immutability of the input value.

- [ ] **Step 5: Run transition tests and verify RED**

Run: `go test ./internal/core -run 'Test(Acquire|Release)' -v`

Expected: FAIL because transition functions do not exist.

- [ ] **Step 6: Implement minimal transition functions**

Define:

```go
type AcquireRequest struct {
    WorkerID string
    Agent    string
    Purpose  string
    At       time.Time
}

type ReleaseRequest struct {
    WorkerID string
    At       time.Time
}
```

`Acquire` accepts only RELEASED and clears `ReleasedAt`. `Release` accepts only ACTIVE owned by the exact Worker ID and preserves the owner identity for audit continuity.

- [ ] **Step 7: Run Core and repository tests**

Run: `go test ./internal/core ./internal/workspace ./...`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/core internal/workspace/status.go
git commit -m "feat(core): add ownership codec and transitions"
```

### Task 2: Resume and ownership-aware operation planning

**Files:**
- Create: `internal/workspace/lifecycle.go`
- Create: `internal/workspace/lifecycle_test.go`
- Modify: `internal/plan/change.go`
- Test: `internal/plan/change_test.go`
- Modify: `internal/workspace/apply.go`
- Test: `internal/workspace/apply_test.go`

**Interfaces:**
- Consumes: strict ownership codec, Core transitions, `Root.ResolveUAWP`, and immutable `plan.Plan`.
- Produces: `Resume(root Root, workerID string) (ResumeReport, error)`, `PlanAcquireAt(root Root, request core.AcquireRequest) (plan.Plan, error)`, `PlanReleaseAt(root Root, request core.ReleaseRequest) (plan.Plan, error)`, actor-bound immutable plans, and safe `UPDATE_FILE` application for existing UAWP state.

- [ ] **Step 1: Write failing resume matrix tests**

Exercise RELEASED/acquire-available, ACTIVE/same-worker/continue, ACTIVE/other-worker/read-only, malformed ownership, recovery journal present, missing context, and empty Worker ID. Snapshot the workspace before and after every resume case to prove read-only behavior.

- [ ] **Step 2: Run resume tests and verify RED**

Run: `go test ./internal/workspace -run TestResume -v`

Expected: FAIL because `Resume` and `ResumeReport` do not exist.

- [ ] **Step 3: Implement Resume**

Define a stable outcome domain `ACQUIRE_AVAILABLE`, `OWNED_BY_CALLER`, and `BLOCKED_BY_OTHER`. Include current owner metadata and next action without returning write authority for malformed or recovery-required state.

- [ ] **Step 4: Write failing managed-file update tests**

Add `plan.UpdateFile` and test exact before/after hashes, defensive content copies, plan hashing, and preview output. At the apply layer, test successful replacement of an existing regular UAWP file, before-hash drift, a competing replacement immediately before publication, symlink substitution, interruption after publication, and recovery-required diagnostics. The competing file MUST survive; no ordinary rename may silently replace an unverified path.

- [ ] **Step 5: Run update tests and verify RED**

Run: `go test ./internal/plan ./internal/workspace -run 'Test(UpdateFile|ApplyUpdate)' -v`

Expected: FAIL because `UPDATE_FILE` is not implemented.

- [ ] **Step 6: Implement safe managed-file update**

Add `UpdateFile ChangeKind`. Stage and sync the new file, revalidate the exact before hash and safe parent immediately before publication, record in-progress recovery metadata, then publish through a platform-specific replace primitive whose preconditions are tested. If atomic compare-and-replace cannot be guaranteed on a platform, stop with an unsupported-safe-update error rather than weakening preservation. Never use the create-only hard-link path for updates.

- [ ] **Step 7: Write failing acquire/release planning tests**

Assert exact `.uawp/ACTIVE_WORKER.md` before/after hashes, actor identity binding, deterministic timestamps, rejection of another ACTIVE owner, non-owner release, malformed state, recovery-required state, and unchanged plan IDs for identical inputs.

- [ ] **Step 8: Run planner tests and verify RED**

Run: `go test ./internal/workspace -run 'TestPlan(Acquire|Release)' -v`

Expected: FAIL because the planners do not exist.

- [ ] **Step 9: Extend plan actor binding and implement planners**

Add immutable metadata to `plan.Plan`:

```go
type Metadata struct {
    ActorWorkerID string `json:"actorWorkerID,omitempty"`
    Reason        string `json:"reason,omitempty"`
    ControllerID  string `json:"controllerID,omitempty"`
}
```

Include metadata in canonical plan hashing and defensive copies. Build acquire/release plans with current ownership as a bound input and the full encoded next document as the only change.

- [ ] **Step 10: Prove competing preview safety**

Create two acquire plans from the same RELEASED state, apply the first, then assert the second fails drift verification and leaves the first owner unchanged.

- [ ] **Step 11: Run tests and commit**

Run: `go test ./internal/plan ./internal/workspace ./...`

```bash
git add internal/plan internal/workspace/apply.go internal/workspace/apply_test.go internal/workspace/lifecycle.go internal/workspace/lifecycle_test.go
git commit -m "feat(workspace): plan resume acquire and release"
```

### Task 3: Context synchronization and immutable checkpoints

**Files:**
- Create: `internal/workspace/context.go`
- Create: `internal/workspace/context_test.go`
- Create: `internal/workspace/checkpoint.go`
- Create: `internal/workspace/checkpoint_test.go`
- Modify: `internal/workspace/boundary.go`
- Test: `internal/workspace/boundary_test.go`

**Interfaces:**
- Consumes: actor-bound plan metadata and strict current ownership.
- Produces: `PlanContextSync(root Root, workerID string, content []byte) (plan.Plan, error)` and `PlanCheckpointAt(root Root, request CheckpointRequest) (plan.Plan, error)`.

- [ ] **Step 1: Write failing context-sync tests**

Test ACTIVE owner success, non-owner failure with zero changes, RELEASED failure, empty/oversized context rejection, unchanged-content no-op, ownership drift, context drift, and exact replacement preview.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./internal/workspace -run TestPlanContextSync -v`

Expected: FAIL because the planner does not exist.

- [ ] **Step 3: Implement context synchronization**

Use a complete-document replacement capped at 1 MiB. Bind both ownership and current context hashes. Return an empty plan only when the supplied content exactly equals the current content and the caller remains ACTIVE owner.

- [ ] **Step 4: Write failing checkpoint tests**

Define and test:

```go
type CheckpointRequest struct {
    WorkerID          string
    MilestoneID       string
    Label             string
    DecisionReferences []string
    At                time.Time
}
```

Cover owner/non-owner, stable encoded snapshot, existing file, traversal, slash/backslash, reserved Windows aliases including superscript digits, control characters, empty label, maximum length, and a concurrent same-name creation.

- [ ] **Step 5: Run and verify RED**

Run: `go test ./internal/workspace -run 'Test(PlanCheckpoint|CheckpointName)' -v`

Expected: FAIL because checkpoint planning is absent.

- [ ] **Step 6: Implement portable immutable checkpoints**

Use a canonical filename `<RFC3339-basic>-<milestone-id>.md` where the ID is restricted to lowercase ASCII letters, digits, and single hyphens, 1–64 characters. The document includes milestone label, created time, Worker ID, Agent, complete context hash and body, plus explicit decision references. Use `CREATE_FILE` with `MISSING` before-state so apply cannot overwrite.

- [ ] **Step 7: Run tests and commit**

Run: `go test ./internal/workspace ./...`

```bash
git add internal/workspace/context* internal/workspace/checkpoint* internal/workspace/boundary*
git commit -m "feat(workspace): add context sync and checkpoints"
```

### Task 4: Atomic pause and handoff

**Files:**
- Create: `internal/workspace/handoff.go`
- Create: `internal/workspace/handoff_test.go`
- Modify: `internal/workspace/apply.go`
- Test: `internal/workspace/apply_test.go`

**Interfaces:**
- Consumes: Core release transition, context validation, actor-bound plans, and recovery-journal apply.
- Produces: `PlanHandoffAt(root Root, request HandoffRequest) (plan.Plan, error)` with context-first and ownership-last ordered application.

- [ ] **Step 1: Write failing handoff plan tests**

Define:

```go
type HandoffRequest struct {
    WorkerID    string
    FinalContext []byte
    Purpose     string
    At          time.Time
}
```

Assert two exact changes, owner-only authorization, required final context and purpose, no checkpoint creation, context and ownership drift binding, and deterministic preview.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./internal/workspace -run TestPlanHandoff -v`

Expected: FAIL because handoff planning is absent.

- [ ] **Step 3: Add explicit change ordering**

Replace path-sorted mutation order with a canonical `Sequence` field included in the plan hash. Reject duplicate or nonpositive sequences. Handoff assigns context before ownership; other existing plans preserve their intended deterministic order.

- [ ] **Step 4: Implement handoff planning**

Encode final context replacement as sequence 10 and ACTIVE-to-RELEASED ownership as sequence 20. Bind both input hashes and caller identity. Verify the resulting context before publishing the release action.

- [ ] **Step 5: Test interrupted handoff**

Inject interruption after context publication and before ownership publication. Assert `RECOVERY_REQUIRED`, retained ACTIVE ownership, preserved final context, unchanged project files, and refusal of later lifecycle mutations.

- [ ] **Step 6: Run tests and commit**

Run: `go test -race ./internal/workspace ./...`

```bash
git add internal/plan/change.go internal/plan/change_test.go internal/workspace/apply* internal/workspace/handoff*
git commit -m "feat(workspace): add atomic pause and handoff"
```

### Task 5: Human Controller stale-claim recovery

**Files:**
- Create: `internal/core/arbitration.go`
- Test: `internal/core/arbitration_test.go`
- Create: `internal/workspace/recover.go`
- Test: `internal/workspace/recover_test.go`
- Modify: `internal/workspace/apply.go`

**Interfaces:**
- Consumes: ACTIVE ownership, decisions document, plan metadata, approval tokens, and ordered apply.
- Produces: `PlanStaleRecoveryAt(root Root, request RecoveryRequest) (plan.Plan, error)`; the generic Apply path verifies the exact plan token.

- [ ] **Step 1: Write failing arbitration validation tests**

Define:

```go
type RecoveryRequest struct {
    ControllerID string
    Reason       string
    At           time.Time
}
```

Require nonempty Controller ID and reason, ACTIVE current state, explicit-timezone timestamp, and a RELEASED result retaining the old Worker ID and Agent. Prove the function never returns a new ACTIVE owner.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./internal/core -run TestRecoverStaleOwnership -v`

Expected: FAIL because arbitration validation does not exist.

- [ ] **Step 3: Implement Core arbitration transition**

Return the RELEASED ownership plus an immutable audit value containing controller, reason, old owner identity, old acquisition time, and recovery time. Do not infer staleness.

- [ ] **Step 4: Write failing recovery planner tests**

Assert exact ownership and decisions changes, ownership-last ordering, complete bound metadata, RELEASED-state rejection, missing identity/reason rejection, old-owner drift, decisions drift, workspace mismatch, token replay rejection, and no implicit acquire.

- [ ] **Step 5: Implement decisions append and recovery planning**

Append a deterministic Markdown section headed by recovery time and old Worker ID. Bind the complete pre-recovery decisions and ownership hashes. Assign decisions sequence 10 and ownership sequence 20.

- [ ] **Step 6: Test one-time token semantics end to end**

Preview recovery for worker A; mutate to worker B ACTIVE and assert rejection. In a fresh fixture apply the original recovery once, assert RELEASED plus audit entry, then replay and assert rejection with zero further change.

- [ ] **Step 7: Run tests and commit**

Run: `go test -race ./internal/core ./internal/workspace ./...`

```bash
git add internal/core/arbitration* internal/workspace/recover* internal/workspace/apply.go
git commit -m "feat(workspace): add human-authorized stale recovery"
```

### Task 6: Lifecycle CLI commands and stable output

**Files:**
- Create: `internal/cli/lifecycle.go`
- Create: `internal/cli/lifecycle_test.go`
- Modify: `internal/cli/run.go`
- Modify: `cmd/uawp/main.go`

**Interfaces:**
- Consumes: all Phase 2 workspace reports and planners plus existing timestamp-bound approval tokens.
- Produces: CLI commands `resume`, `acquire`, `release`, `sync`, `checkpoint`, `handoff`, and `recover` with JSON/text output and stable exit codes.

- [ ] **Step 1: Write failing command-contract tests**

Cover required flags, unknown flags, JSON schema fields, text sections, preview exit 5, blocked ownership exit 6, invalid state exit 4, unsafe input exit 3, successful apply exit 0, and no mutation on parsing or approval errors.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./internal/cli -run 'Test(Resume|Acquire|Release|Sync|Checkpoint|Handoff|Recover)Command' -v`

Expected: FAIL because commands are not routed.

- [ ] **Step 3: Refactor shared mutation command flow**

Extract approval parsing, plan preview, token comparison, Apply, and output formatting into a helper receiving a planner closure. Preserve existing `init` output compatibility and tests.

- [ ] **Step 4: Implement command flags**

Use explicit flags: `--worker-id`, `--agent`, `--purpose`, `--context-file`, `--milestone-id`, `--label`, repeated `--decision-ref`, `--controller-id`, and `--reason`. Context is read from the named file; `-` means stdin. Never accept context as an unescaped positional argument.

- [ ] **Step 5: Add real preview/drift/apply CLI tests**

For every mutating command, preview, mutate a bound input, assert token rejection, preview again, apply, and verify exact state. Include the review-focus worker-A-to-worker-B stale-token case.

- [ ] **Step 6: Run tests and commit**

Run: `go test -race ./internal/cli ./internal/e2e ./...`

```bash
git add internal/cli cmd/uawp
git commit -m "feat(cli): expose ownership lifecycle commands"
```

### Task 7: Canonical Prompt Library

**Files:**
- Create: `prompts/RESUME_WORK.md`
- Create: `prompts/CONTEXT_SYNC.md`
- Create: `prompts/CREATE_CHECKPOINT.md`
- Create: `prompts/PAUSE_AND_HANDOFF.md`
- Create: `prompts/embed.go`
- Create: `internal/core/prompts.go`
- Test: `internal/core/prompts_test.go`

**Interfaces:**
- Consumes: protocol operation names and `.uawp/` paths from the approved spec.
- Produces: `Prompt(name PromptName) ([]byte, error)` and `PromptNames() []PromptName` with immutable returned bytes.

- [ ] **Step 1: Write failing prompt conformance tests**

Require exactly four canonical names, nonempty version headers, correct `.uawp/` paths, required lifecycle verbs, read-only behavior for other ACTIVE owners, no automatic checkpoint on handoff, and no vendor/native-file terms such as `Claude`, `Codex`, `AGENTS.md`, or `CLAUDE.md`.

- [ ] **Step 2: Run and verify RED**

Run: `go test ./internal/core -run TestPrompt -v`

Expected: FAIL because prompt assets/API do not exist.

- [ ] **Step 3: Write the four complete prompts**

Each prompt states purpose, prerequisites, ordered actions, stop conditions, files read/written, ownership effect, and expected completion report. `RESUME_WORK` never silently acquires; `CONTEXT_SYNC` retains ACTIVE; `CREATE_CHECKPOINT` requires an explicit milestone; `PAUSE_AND_HANDOFF` releases only after persistence verification.

- [ ] **Step 4: Embed and expose immutable assets**

Use `//go:embed *.md`, validate names through a closed enum, return defensive copies, and keep prompt text free of vendor behavior.

- [ ] **Step 5: Run tests and commit**

Run: `go test ./internal/core ./...`

```bash
git add prompts internal/core/prompts*
git commit -m "feat: add canonical agent-neutral prompt library"
```

### Task 8: End-to-end lifecycle, documentation, and release gate

**Files:**
- Create: `internal/e2e/lifecycle_test.go`
- Create: `internal/e2e/recovery_test.go`
- Create: `docs/user/lifecycle.md`
- Create: `docs/user/stale-recovery.md`
- Modify: `docs/protocol/state-v1.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `internal/e2e/docs_test.go`

**Interfaces:**
- Consumes: complete Phase 2 CLI and Prompt Library.
- Produces: executable user journey and green Phase 2 quality gate.

- [ ] **Step 1: Write failing normal-lifecycle e2e test**

Initialize a brownfield fixture, acquire worker A, resume as A and B, sync context, create one milestone checkpoint, hand off without an implicit checkpoint, acquire worker B, and assert every project-owned fixture hash and mode is preserved.

- [ ] **Step 2: Write failing conflict/recovery e2e test**

Exercise concurrent acquire previews, non-owner write attempts, stale recovery without approval, recovery-token drift, successful Human Controller recovery, token replay, separate replacement acquire, and an interrupted handoff reporting `RECOVERY_REQUIRED`.

- [ ] **Step 3: Run e2e tests and verify RED**

Run: `go test ./internal/e2e -run 'Test(Lifecycle|StaleRecovery)' -v`

Expected: FAIL until all command wiring and fixtures are correct.

- [ ] **Step 4: Complete user and protocol documentation**

Document exact preview/apply examples, normal versus exceptional paths, stable exit codes, the fact that a recovery token approves a plan rather than authenticating a person, and the explicit absence of automatic stale detection. Update the protocol document from “Plan 2 work” to normative implemented behavior.

- [ ] **Step 5: Expand documentation verification**

Make `docs_test.go` read every documented `<!-- verify -->` command block, run it in an isolated temporary workspace, and validate local Markdown links. Exclude network, installation, and platform-specific examples through explicit non-executable markers.

- [ ] **Step 6: Run the full quality gate**

Run:

```bash
make check
go test ./internal/e2e -run '^$' -fuzz '^FuzzDecodeManifest$' -fuzztime 10s
go test ./internal/e2e -run '^$' -fuzz '^FuzzResolveUAWP$' -fuzztime 10s
git diff --check -- . ':(exclude)testdata/brownfield/CLAUDE.md'
```

Expected: formatting, unit, e2e, race, vet, fuzz, documentation, and preservation checks all PASS.

- [ ] **Step 7: Commit**

```bash
git add README.md CHANGELOG.md docs internal/e2e
git commit -m "docs: publish ownership lifecycle workflow"
```

- [ ] **Step 8: Whole-branch review**

Review from the Phase 2 merge base through HEAD with special attention to the five Review Focus cases. Fix Critical and Important findings with regression tests; record Minor findings for later work. Do not claim Windows release validation without a real Windows runner result.
