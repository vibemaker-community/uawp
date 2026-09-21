# Adapter Framework and Launch Adapters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build safe launch adapters for Codex, Claude Code, and Tencent WorkBuddy, all pointing to `.uawp/INSTRUCTIONS.md`.

**Architecture:** A provider-neutral adapter package resolves effective native entries from documented rules and returns immutable plans. Core stores generic integration artifacts; Workspace alone applies approved changes.

**Tech Stack:** Go 1.26 standard library, JSON Schema, table-driven and end-to-end tests.

**Spec:** `docs/superpowers/specs/2026-09-22-adapter-framework-launch-adapters-design.md`

## Global Constraints

- Core stays agent-neutral; `.uawp/` remains the only UAWP state namespace.
- Native project content is never silently overwritten or semantically merged.
- All writes use preview, approval, drift revalidation, apply, and verification.
- Unknown capability fails closed; no user/global provider settings are written.
- Go 1.26 and standard-library-only production dependencies remain binding.
- TDD and one independently reviewable commit per task are mandatory.

## Review Focus

- Claude entry creation shadowing `AGENTS.md` — Task 6.
- WorkBuddy fallback invalidated by later `CODEBUDDY.md` — Task 7.
- Shared bridge surviving one-consumer removal — Task 8.
- Corrupt markers and user edits stopping mutation — Task 3.
- Unknown provider version/configuration staying conditional — Tasks 4 and 6.

---

### Task 1: Persist generic integration artifacts

**Files:** Modify `internal/core/manifest.go`, `internal/core/manifest_test.go`, `schemas/manifest.schema.json`.

**Interfaces:** Produce `IntegrationMode`, `IntegrationArtifact`, and optional `Manifest.Integrations`; preserve existing two-field manifests.

- [ ] Write failing tests for backward compatibility, canonical round-trip, unknown/duplicate nested fields, clean relative paths, valid hashes, unique sorted consumers, duplicate IDs/paths, and the exact three-mode enum.
- [ ] Run `go test ./internal/core -run 'ManifestIntegration|DecodeManifest' -v`; expect failure because integration types do not exist.
- [ ] Implement:

```go
type IntegrationMode string
const (Direct IntegrationMode = "DIRECT"; Import IntegrationMode = "IMPORT"; ManagedBlock IntegrationMode = "MANAGED_BLOCK")
type IntegrationArtifact struct {
    ID, Path, Target string
    Mode IntegrationMode
    Consumers []string
    CreatedFile bool
    ArtifactSHA256, OutsideContentSHA256 string
}
```

Use explicit JSON tags; reject absolute/traversal/backslash paths, malformed hashes, empty/duplicate consumers, duplicate artifacts; sort copies during encoding.
- [ ] Update Schema with `additionalProperties: false` at every level; run `go test ./internal/core -v` and `git diff --check`.
- [ ] Commit: `git commit -am "feat(core): persist integration artifacts"`.

### Task 2: Add canonical instructions

**Files:** Create `templates/state/INSTRUCTIONS.md`; modify `templates/state/embed.go`, `internal/workspace/init.go`, related init tests, `docs/protocol/state-v1.md`.

**Interfaces:** Produce embedded `state.Instructions` and planned `.uawp/INSTRUCTIONS.md`.

- [ ] Write failing tests proving preview/apply include the file, it names ownership and four prompts, contains no provider/native filename, creates no native entry, and repeated init is idempotent.
- [ ] Run `go test ./internal/workspace ./internal/e2e -run 'Init|Instructions' -v`; expect missing-file failure.
- [ ] Add concise agent-neutral instructions with mode `0600`, published before `manifest.json`.
- [ ] Run focused tests and commit `feat: add canonical workspace instructions`.

### Task 3: Build safe native editing primitives

**Files:** Create `internal/adapter/block.go`, `block_test.go`, `import.go`, `import_test.go`; modify `internal/plan/change.go` and Workspace apply tests/implementation.

**Interfaces:** Produce `UpsertManagedBlock`, `RemoveManagedBlock`, `UpsertImport`, `RemoveImport`, and `plan.DeleteFile`.

- [ ] Write failing byte-exact tests for LF/CRLF, no final newline, idempotence, fenced-code import exclusion, missing/duplicate/nested/reversed markers, edited body, and outside-byte preservation.

```go
after, meta, err := UpsertManagedBlock(before, BlockSpec{
    ArtifactID: "uawp-entry-agents-v1", Consumers: []string{"codex"},
    Body: "Read `.uawp/INSTRUCTIONS.md` before UAWP work.",
})
```

- [ ] Run `go test ./internal/adapter ./internal/plan ./internal/workspace -run 'ManagedBlock|Import|Delete' -v`; expect undefined APIs.
- [ ] Implement strict markers containing artifact ID, schema, target, and sorted consumers. Exact import is standalone `@.uawp/INSTRUCTIONS.md` outside fences.
- [ ] Add `DeleteFile` with before-hash verification, regular-file/no-symlink checks, journaling, and absence verification; only unchanged UAWP-created native files qualify.
- [ ] Run focused tests and commit `feat: add safe native integration edits`.

### Task 4: Define evidence and effective-entry resolution

**Files:** Create `internal/adapter/types.go`, `types_test.go`, `native.go`, `native_test.go`.

**Interfaces:**

```go
type Adapter interface { ID() string; Evidence() Evidence; Resolve(Snapshot) Resolution }
type Snapshot struct {
    Root string
    Files map[string]FileFact
    ProviderVersion string
    Options map[string]string
}
```

- [ ] Write failing tests for missing/regular/symlink/directory/NUL/invalid-UTF8/over-1-MiB files and unobservable options.
- [ ] Run `go test ./internal/adapter -run 'Snapshot|Discovery|Resolution' -v`; expect undefined contract.
- [ ] Implement states `EFFECTIVE`, `CO_LOADED`, `FALLBACK_EFFECTIVE`, `SHADOWED`, `UNAVAILABLE`, `UNKNOWN`, `MISSING`; confidence `VERIFIED`, `CONDITIONAL`, `UNSUPPORTED`; stable findings and deterministic ordering.
- [ ] Run tests and commit `feat(adapter): define effective entry resolution`.

### Task 5: Implement Codex resolution

**Files:** Create `internal/adapter/codex.go`, tests, `docs/adapters/codex.md`, `testdata/adapters/codex/`.

**Interfaces:** Produce `NewCodex() Adapter`.

- [ ] Write failing matrix tests for empty, `AGENTS.md`, `AGENTS.override.md`, both, empty entry, nested entries, observable fallbacks, unknown user config, and size cap.

```go
r := NewCodex().Resolve(snapshot("AGENTS.md", "AGENTS.override.md"))
assertCandidate(t, r, "AGENTS.override.md", Effective)
assertCandidate(t, r, "AGENTS.md", Shadowed)
```

- [ ] Run `go test ./internal/adapter -run Codex -v`; expect missing constructor.
- [ ] Implement documented precedence only, `MANAGED_BLOCK`, dated official evidence, 32-KiB diagnostic, and no nested bridge duplication.
- [ ] Run tests and commit `feat(adapter): resolve Codex instructions`.

### Task 6: Implement Claude Code resolution

**Files:** Create `internal/adapter/claude.go`, tests, `docs/adapters/claude-code.md`, `testdata/adapters/claude/`.

**Interfaces:** Produce `NewClaudeCode() Adapter`; consume options `instructionFiles`, `directAgentsSupport`, `providerEnvironment`.

- [ ] Write failing tests for root/`.claude/CLAUDE.md`, only `AGENTS.md`, both, `CLAUDE.local.md`, existing imports, all selection settings, versions before/after 2.1.277, unknown version, and unavailable environments.
- [ ] Run `go test ./internal/adapter -run Claude -v`; expect missing constructor.
- [ ] Implement preferred `IMPORT`. When only `AGENTS.md` exists and direct support is unverified, propose `CLAUDE.md` containing both imports and finding `CLAUDE_CREATION_CHANGES_SELECTION`; require acknowledgement. `managed-only` is unsupported.
- [ ] Run tests and commit `feat(adapter): resolve Claude Code instructions`.

### Task 7: Implement Tencent WorkBuddy resolution

**Files:** Create `internal/adapter/workbuddy.go`, tests, `docs/adapters/workbuddy.md`, `testdata/adapters/workbuddy/`.

**Interfaces:** Produce `NewWorkBuddy() Adapter`.

- [ ] Write failing tests for neither file, each file alone, both, registered fallback then new `CODEBUDDY.md`, later removal, existing `.codebuddy/rules`, and shared Codex block.
- [ ] Run `go test ./internal/adapter -run WorkBuddy -v`; expect missing constructor.
- [ ] Implement `MANAGED_BLOCK`; never create `.codebuddy/rules/uawp/RULE.mdc`. New `CODEBUDDY.md` makes registered `AGENTS.md` fallback `ENTRY_DRIFT` without deleting a Codex consumer.
- [ ] Run tests and commit `feat(adapter): resolve Tencent WorkBuddy instructions`.

### Task 8: Plan add, migration, and consumer-aware removal

**Files:** Create `internal/adapter/planner.go`, tests, `internal/workspace/adapters.go`, tests.

**Interfaces:**

```go
func ResolveAdapters(Root, []string, adapter.RuntimeFacts) ([]adapter.Resolution, error)
func PlanAdapterAddAt(Root, string, adapter.RuntimeFacts, []string, time.Time) (plan.Plan, adapter.Resolution, error)
func PlanAdapterRemoveAt(Root, string, adapter.RuntimeFacts, time.Time) (plan.Plan, error)
func VerifyAdapterRoute(Root, string, adapter.RuntimeFacts) error
```

- [ ] Write failing tests for idempotent add, exact plan inputs, shared consumer addition, one-consumer removal, last-consumer removal, created-file deletion, modified-created-file preservation, and duplicate-load prevention.
- [ ] Run `go test ./internal/adapter ./internal/workspace -run 'Adapter(Add|Remove|Shared|Drift)' -v`; expect missing planners.
- [ ] Implement native change first and manifest publication last; marker/manifest consumers change together; conditional routes require exact acknowledged finding code.
- [ ] Test native, provider-fact, and manifest drift after preview plus interruption after native publication; require approval invalidation or recovery.
- [ ] Run tests and commit `feat: plan safe adapter lifecycle`.

### Task 9: Add diagnostics and CLI

**Files:** Create `internal/cli/adapters.go`, tests; modify `internal/cli/run.go`, Workspace status and tests.

**Interfaces:** Produce `uawp adapter list|add|remove`; add adapter resolutions to status/doctor output.

- [ ] Write failing CLI tests for JSON/text, preview exit 5, exact approval, usage 2, unsafe 3, invalid state 4, and read-only list/status/doctor.
- [ ] Run `go test ./internal/cli ./internal/workspace -run 'Adapter|Status|Doctor' -v`; expect missing routing.
- [ ] Implement output containing provider, effective entry, mode, confidence, health, and next action; CLI contains no precedence logic.
- [ ] Run tests and commit `feat(cli): manage and diagnose adapters`.

### Task 10: Complete end-to-end and release gates

**Files:** Create `internal/e2e/adapters_test.go`, `docs/adapters/authoring.md`, `docs/user/adapters.md`; modify fuzz tests, README, package boundaries, roadmap, changelog.

**Interfaces:** Produce whole-lifecycle evidence and release documentation.

- [ ] Write failing E2E cases for greenfield/brownfield all-provider installs, shared bridge, Claude import, WorkBuddy drift, single removal, byte preservation, wrong approval, concurrent drift, malformed markers, binary files, Unicode paths, and interruption.

```go
applyAdapter(t, root, "codex")
applyAdapter(t, root, "workbuddy")
removeAdapter(t, root, "workbuddy")
content := readFile(t, root, "AGENTS.md")
if !bytes.Contains(content, []byte("consumers=codex")) { t.Fatal("Codex bridge removed") }
```

- [ ] Run `go test ./internal/e2e -run Adapter -v`; expect incomplete-lifecycle failure.
- [ ] Document evidence, author contract, commands, configured consumers versus live use, drift repair, and safe removal.
- [ ] Run release gates:

```bash
gofmt -w internal
go test ./...
go test -race ./...
go vet ./...
git diff --check
go build -trimpath -o bin/uawp ./cmd/uawp
./bin/uawp version
```

Expected: all exit 0; version prints `uawp dev`; `bin/` stays ignored.
- [ ] Commit `docs: complete launch adapter release gates`.

## Self-review record

- Spec sections 1-19 map to Tasks 1-10; no implementation placeholder remains.
- Interfaces consistently use `Resolution`, generic artifacts, immutable plans, and Workspace apply.
- Existing two-field manifests remain valid; integrations are added only by approved plans.
- Review Focus is pinned by Tasks 3, 4, 6, 7, and 8.

