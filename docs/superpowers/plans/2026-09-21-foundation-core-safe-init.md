# Foundation, Core, and Safe Init Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a production-quality Go foundation that can inspect, preview,
and safely initialize the UAWP namespace in greenfield and brownfield projects
without changing any project-owned or agent-native file.

**Architecture:** Agent-neutral domain packages define state and immutable
change plans. Application services discover and validate a workspace, then a
filesystem port fingerprints inputs and applies approved plans with atomic
same-directory replacements. A thin standard-library CLI exposes `init`,
`status`, and `doctor`; adapters and lifecycle mutations are later plans.

**Tech Stack:** Go 1.26 compatibility floor; Go 1.26 and 1.27 CI; standard
library only; JSON manifests; Markdown state files; `go test`, fuzz tests,
`go vet`, and `go test -race`.

**Spec:** `docs/superpowers/specs/2026-09-21-uawp-product-engineering-design.md`

## Global Constraints

- Core packages MUST contain no vendor names, native filenames, or CLI imports.
- All UAWP runtime state MUST be rooted at `.uawp/` below a caller-selected
  workspace root.
- Unknown pre-existing `.uawp/` content MUST stop planning without mutation.
- Phase 1 MUST NOT modify any file outside `.uawp/`, including agent-native
  files and a root-level `context.md`.
- Mutation requires a generated plan, visible preview, explicit approval, input
  fingerprint verification, apply, and post-apply verification.
- Paths that traverse or escape through symlinks MUST be rejected.
- State timestamps use RFC 3339/ISO 8601 with explicit timezone.
- Runtime code uses only the Go standard library in this plan.
- Every task follows red-green-refactor and ends in a focused Git commit.
- No advertised agent support is part of this plan.

## Review Focus

- A `.uawp` symlink pointing outside the workspace must fail before planning;
  Task 4 pins this with an integration test.
- A project file changed between preview and apply must reject the stale plan
  without partial writes; Task 6 pins this with a fingerprint test.
- Windows reserved names and path separators must never become generated state
  paths; Task 4 pins cross-platform path validation.
- An interrupted multi-file apply must either roll back or leave a precise
  recovery journal; Task 6 pins failpoint behavior.
- Valid JSON with duplicate keys can hide ownership/version meaning; Task 2
  rejects duplicate manifest keys using streaming token validation.

---

## Planned repository structure

```text
cmd/uawp/main.go                    CLI process entry
internal/core/manifest.go           namespace identity and schema version
internal/core/ownership.go          ACTIVE/RELEASED state and validation
internal/core/errors.go             stable domain error categories
internal/plan/change.go             immutable change-plan model
internal/workspace/boundary.go      root, relative path, and symlink safety
internal/workspace/discover.go      read-only workspace inventory
internal/workspace/init.go          safe-init plan construction
internal/workspace/apply.go         drift checks, journal, atomic apply
internal/workspace/status.go        status and doctor reports
internal/cli/run.go                 argument parsing and exit mapping
templates/state/*.md                canonical initial Markdown state
schemas/manifest.schema.json        published manifest contract
testdata/greenfield/                empty-project fixture metadata
testdata/brownfield/                preservation fixture
docs/                               user and architecture documentation
```

## Task 1: Establish the buildable Go project and quality gate

**Files:**
- Create: `go.mod`
- Create: `cmd/uawp/main.go`
- Create: `internal/cli/run.go`
- Create: `internal/cli/run_test.go`
- Create: `Makefile`
- Create: `.gitignore`
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: none.
- Produces: `cli.Run(args []string, stdout, stderr io.Writer) int` and a buildable
  `uawp` executable for later tasks.

- [ ] **Step 1: Write the failing CLI smoke test**

```go
func TestRunVersion(t *testing.T) {
    var stdout, stderr bytes.Buffer
    code := Run([]string{"version"}, &stdout, &stderr)
    if code != 0 || stdout.String() != "uawp dev\n" || stderr.Len() != 0 {
        t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
    }
}
```

- [ ] **Step 2: Confirm the test fails because the module and `Run` do not exist**

Run: `go test ./internal/cli -run TestRunVersion -v`  
Expected: FAIL with missing `go.mod` or undefined `Run`.

- [ ] **Step 3: Create the minimal module and CLI seam**

Use module path `github.com/uawp/uawp`, declare `go 1.26.0`, implement
`Run` with an exact `version` case, and make `main` call `os.Exit(cli.Run(...))`.
The default/unknown case writes `usage: uawp <command>\n` to stderr and returns
exit code `2`.

- [ ] **Step 4: Add reproducible local and CI checks**

`Makefile` targets:

```make
fmt:
	test -z "$$(gofmt -l .)"
test:
	go test ./...
race:
	go test -race ./...
vet:
	go vet ./...
check: fmt test race vet
```

CI runs `go test ./...`, `go test -race ./...`, and `go vet ./...` on
`ubuntu-latest`, `macos-latest`, and `windows-latest`, with Go `1.26.x` and
`1.27.x`. Document that CI configuration may require updating if GitHub-hosted
images do not yet expose one matrix entry.

- [ ] **Step 5: Run the quality gate**

Run: `make check`  
Expected: all commands exit 0 and `TestRunVersion` passes.

- [ ] **Step 6: Commit the project foundation**

```bash
git add go.mod cmd internal/cli Makefile .gitignore .github/workflows/ci.yml
git commit -m "build: establish Go project and quality gate"
```

## Task 2: Define and validate namespace manifests

**Files:**
- Create: `internal/core/manifest.go`
- Create: `internal/core/manifest_test.go`
- Create: `internal/core/errors.go`
- Create: `schemas/manifest.schema.json`

**Interfaces:**
- Consumes: Go standard library `encoding/json`.
- Produces: `core.Manifest`, `core.DecodeManifest(io.Reader)`,
  `core.ValidateManifest(Manifest)`, and stable `core.ErrorKind` values.

- [ ] **Step 1: Write table tests for accepted and rejected manifests**

```go
func TestDecodeManifest(t *testing.T) {
    tests := []struct{ name, input string; wantErr bool }{
        {"valid", `{"protocol":"UAWP","stateVersion":"1.0.0"}`, false},
        {"wrong owner", `{"protocol":"other","stateVersion":"1.0.0"}`, true},
        {"future major", `{"protocol":"UAWP","stateVersion":"2.0.0"}`, true},
        {"duplicate protocol", `{"protocol":"UAWP","protocol":"other","stateVersion":"1.0.0"}`, true},
        {"unknown field", `{"protocol":"UAWP","stateVersion":"1.0.0","x":1}`, true},
    }
    for _, tt := range tests { /* decode and assert tt.wantErr */ }
}
```

- [ ] **Step 2: Verify the manifest tests fail**

Run: `go test ./internal/core -run TestDecodeManifest -v`  
Expected: FAIL with undefined manifest types/functions.

- [ ] **Step 3: Implement strict streaming decode and validation**

Define:

```go
type Manifest struct {
    Protocol     string `json:"protocol"`
    StateVersion string `json:"stateVersion"`
}

func DecodeManifest(r io.Reader) (Manifest, error)
func ValidateManifest(m Manifest) error
```

Tokenize the top-level object to detect duplicate keys, reject unknown fields,
require exactly one JSON value, require protocol `UAWP`, and accept only state
major version `1`.

- [ ] **Step 4: Publish the matching JSON Schema**

The schema uses draft 2020-12, `additionalProperties: false`, requires both
fields, fixes `protocol` to `UAWP`, and constrains `stateVersion` to
`^1\\.[0-9]+\\.[0-9]+$`.

- [ ] **Step 5: Run domain tests and vet**

Run: `go test ./internal/core -v && go vet ./internal/core`  
Expected: PASS and no vet diagnostics.

- [ ] **Step 6: Commit manifest ownership validation**

```bash
git add internal/core schemas/manifest.schema.json
git commit -m "feat(core): validate UAWP namespace manifests"
```

## Task 3: Define persistent ownership state

**Files:**
- Create: `internal/core/ownership.go`
- Create: `internal/core/ownership_test.go`

**Interfaces:**
- Consumes: `core.ErrorKind` from Task 2.
- Produces: `core.Ownership`, `core.OwnershipStatus`, and
  `core.ValidateOwnership(Ownership) error` for later lifecycle work.

- [ ] **Step 1: Write ownership invariant tests**

```go
func TestValidateOwnership(t *testing.T) {
    acquired := time.Date(2026, 9, 21, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
    released := acquired.Add(time.Hour)
    tests := []struct{ name string; value Ownership; wantErr bool }{
        {"active", Ownership{Status: Active, WorkerID: "worker-a", Agent: "Agent", AcquiredAt: acquired}, false},
        {"active cannot be released", Ownership{Status: Active, WorkerID: "worker-a", Agent: "Agent", AcquiredAt: acquired, ReleasedAt: &released}, true},
        {"released needs time", Ownership{Status: Released, WorkerID: "worker-a", Agent: "Agent", AcquiredAt: acquired}, true},
        {"released", Ownership{Status: Released, WorkerID: "worker-a", Agent: "Agent", AcquiredAt: acquired, ReleasedAt: &released}, false},
    }
    for _, tt := range tests { /* assert validation result */ }
}
```

- [ ] **Step 2: Verify tests fail before implementation**

Run: `go test ./internal/core -run TestValidateOwnership -v`  
Expected: FAIL with undefined ownership types.

- [ ] **Step 3: Implement the two-state model**

```go
type OwnershipStatus string
const (
    Active OwnershipStatus = "ACTIVE"
    Released OwnershipStatus = "RELEASED"
)
type Ownership struct {
    Status OwnershipStatus
    WorkerID string
    Agent string
    AcquiredAt time.Time
    ReleasedAt *time.Time
    Purpose string
}
```

Validation rejects unknown status, empty identity/purpose, zero or timezone-free
timestamps, release before acquisition, `ReleasedAt` on ACTIVE, and absent
`ReleasedAt` on RELEASED.

- [ ] **Step 4: Add RFC 3339 round-trip tests**

Marshal a `+08:00` acquisition time, parse it back, and assert the instant and
offset are preserved. Reject timestamps without an explicit `Z` or numeric
offset in the state parser introduced here.

- [ ] **Step 5: Run all Core tests**

Run: `go test ./internal/core -v`  
Expected: PASS.

- [ ] **Step 6: Commit ownership state types**

```bash
git add internal/core/ownership.go internal/core/ownership_test.go
git commit -m "feat(core): define persistent ownership state"
```

## Task 4: Enforce workspace and path boundaries

**Files:**
- Create: `internal/workspace/boundary.go`
- Create: `internal/workspace/boundary_test.go`

**Interfaces:**
- Consumes: OS filesystem calls.
- Produces: `workspace.OpenRoot(path string) (Root, error)` and
  `Root.ResolveUAWP(relative string) (string, error)`.

- [ ] **Step 1: Write boundary tests**

Cover an absolute directory, a missing root, a root that is a file, `../escape`,
an absolute child, `.uawp` as a symlink outside the root, an intermediate
symlink, NUL, both slash styles, and Windows reserved basename `CON`.

```go
func TestResolveUAWPRejectsEscape(t *testing.T) {
    root, err := OpenRoot(t.TempDir())
    if err != nil { t.Fatal(err) }
    if _, err := root.ResolveUAWP("../escape"); err == nil {
        t.Fatal("expected traversal rejection")
    }
}
```

- [ ] **Step 2: Verify boundary tests fail**

Run: `go test ./internal/workspace -run 'Test(OpenRoot|ResolveUAWP)' -v`  
Expected: FAIL with undefined `OpenRoot`/`ResolveUAWP`.

- [ ] **Step 3: Implement lexical and physical containment checks**

Canonicalize the root with `filepath.Abs` and `filepath.EvalSymlinks`; require a
directory. Permit only slash-normalized relative paths from a fixed internal
allowlist. Walk existing ancestors with `os.Lstat`; reject every symlink in the
`.uawp` mutation path. Compare `filepath.Rel` results to prevent escape.

- [ ] **Step 4: Add platform-specific reserved-name helpers**

Implement `validPortableSegment(string) bool` so generated segments reject
empty/dot segments, control characters, `<>:"/\\|?*`, trailing dot/space, and
case-insensitive `CON`, `PRN`, `AUX`, `NUL`, `COM1`–`COM9`, `LPT1`–`LPT9`.

- [ ] **Step 5: Run boundary tests on the current platform**

Run: `go test ./internal/workspace -run 'Test(OpenRoot|ResolveUAWP|Portable)' -v`  
Expected: PASS; symlink cases may skip only when the OS denies test symlink
creation, with the skip reason printed.

- [ ] **Step 6: Commit workspace containment**

```bash
git add internal/workspace/boundary.go internal/workspace/boundary_test.go
git commit -m "feat(workspace): enforce mutation boundaries"
```

## Task 5: Model immutable plans and discover workspaces read-only

**Files:**
- Create: `internal/plan/change.go`
- Create: `internal/plan/change_test.go`
- Create: `internal/workspace/discover.go`
- Create: `internal/workspace/discover_test.go`

**Interfaces:**
- Consumes: `workspace.Root`, `core.DecodeManifest`.
- Produces: `plan.Plan`, `plan.Change`, SHA-256 fingerprints, and
  `workspace.Discover(Root) (Inventory, error)`.

- [ ] **Step 1: Test deterministic plan IDs and rendering**

```go
func TestPlanIDIsDeterministic(t *testing.T) {
    changes := []Change{{Kind: CreateFile, Path: ".uawp/manifest.json", AfterSHA256: "abc"}}
    a := New("init", changes)
    b := New("init", changes)
    if a.ID != b.ID { t.Fatalf("%q != %q", a.ID, b.ID) }
}
```

Also assert changes are sorted by portable relative path and preview output
contains action, path, before fingerprint, after fingerprint, and byte count.

- [ ] **Step 2: Verify plan tests fail**

Run: `go test ./internal/plan -v`  
Expected: FAIL with missing package/types.

- [ ] **Step 3: Implement immutable plan values**

Define `ChangeKind` values `CREATE_DIR` and `CREATE_FILE`; `Change` includes
`Path`, `BeforeSHA256`, `AfterSHA256`, `Mode`, and content held privately behind
copy-returning accessors. `New(operation string, changes []Change)` deep-copies,
sorts, and hashes canonical JSON to create the plan ID.

- [ ] **Step 4: Test discovery classifications**

Fixtures must classify: absent `.uawp`, valid owned namespace, unknown
directory, `.uawp` regular file, invalid manifest, and a project root
`context.md` that is reported but never selected for mutation.

- [ ] **Step 5: Implement discovery without writes**

`Discover` uses `Lstat`, reads at most 1 MiB for the manifest, returns immutable
inventory data, and never creates directories or normalizes project files.

- [ ] **Step 6: Run plan and discovery tests**

Run: `go test ./internal/plan ./internal/workspace -v`  
Expected: PASS.

- [ ] **Step 7: Commit planning and discovery**

```bash
git add internal/plan internal/workspace/discover.go internal/workspace/discover_test.go
git commit -m "feat: add immutable plans and workspace discovery"
```

## Task 6: Build safe init plans and transactional application

**Files:**
- Create: `internal/workspace/init.go`
- Create: `internal/workspace/init_test.go`
- Create: `internal/workspace/apply.go`
- Create: `internal/workspace/apply_test.go`
- Create: `templates/state/CONTEXT.md`
- Create: `templates/state/ACTIVE_WORKER.md`
- Create: `templates/state/DECISIONS.md`

**Interfaces:**
- Consumes: Core manifest/ownership, plan model, discovery, and boundary APIs.
- Produces: `workspace.PlanInit(Root) (plan.Plan, error)` and
  `workspace.Apply(Root, plan.Plan, ApplyOptions) (ApplyReport, error)`.

- [ ] **Step 1: Write safe-init plan tests**

Assert an absent namespace plans exactly one directory tree and four files:
manifest, context, released ownership, and decisions. Assert a valid already
initialized namespace returns an empty plan. Assert unknown `.uawp`, root
`context.md`, native files, and arbitrary source files are never included in a
change; the unknown namespace returns an error.

- [ ] **Step 2: Verify init tests fail**

Run: `go test ./internal/workspace -run TestPlanInit -v`  
Expected: FAIL with undefined `PlanInit`.

- [ ] **Step 3: Implement deterministic initial state generation**

Embed canonical templates with `//go:embed`. Generate a manifest with protocol
`UAWP` and state version `1.0.0`. Initial ownership is `RELEASED` with explicit
system bootstrap identity and identical acquired/released RFC 3339 timestamps
provided through an injected clock so tests are deterministic.

- [ ] **Step 4: Write drift and interruption tests before Apply**

Test: successful apply; no approval; plan-root mismatch; file appears after
preview; input fingerprint changes; temp-file write failure; rename failure;
and failure after the second action. In every failure case assert no
project-owned file changes. For partial UAWP writes, assert rollback restores
the pre-apply namespace or `.uawp/RECOVERY.json` precisely lists completed,
rolled-back, and pending actions.

- [ ] **Step 5: Verify apply tests fail**

Run: `go test ./internal/workspace -run TestApply -v`  
Expected: FAIL with undefined `Apply`.

- [ ] **Step 6: Implement approval, drift checks, journal, and atomic writes**

`ApplyOptions` contains `ApprovedPlanID string`, injected `FS` operations for
failpoint tests, and `Clock`. Reject mismatched approval. Re-discover and verify
every before fingerprint. Write the journal first; for files, create a
same-directory temporary file with `O_CREATE|O_EXCL`, write/sync/chmod/close,
then rename. Sync the containing directory where supported. Update and sync the
journal after each action; remove it only after verification.

- [ ] **Step 7: Run race-enabled workspace tests**

Run: `go test -race ./internal/workspace -v`  
Expected: PASS with all drift and failpoint cases.

- [ ] **Step 8: Commit safe init application**

```bash
git add internal/workspace templates/state
git commit -m "feat: plan and apply safe namespace initialization"
```

## Task 7: Add status and doctor reports

**Files:**
- Create: `internal/workspace/status.go`
- Create: `internal/workspace/status_test.go`

**Interfaces:**
- Consumes: discovery and Core validators.
- Produces: `workspace.Status(Root) StatusReport` and
  `workspace.Doctor(Root) DoctorReport`; both are read-only.

- [ ] **Step 1: Write report matrix tests**

Cover uninitialized, valid RELEASED, valid ACTIVE, unknown namespace, malformed
manifest, missing state file, malformed ownership, unsupported version, and a
recovery journal. Assert a machine-stable code plus a human next action for
every finding.

- [ ] **Step 2: Verify report tests fail**

Run: `go test ./internal/workspace -run 'Test(Status|Doctor)' -v`  
Expected: FAIL with undefined report functions.

- [ ] **Step 3: Implement stable findings**

Use codes `READY`, `UNINITIALIZED`, `ACTIVE_OWNER`, `UNKNOWN_NAMESPACE`,
`INVALID_MANIFEST`, `UNSUPPORTED_VERSION`, `INVALID_OWNERSHIP`,
`INCOMPLETE_STATE`, and `RECOVERY_REQUIRED`. Reports contain severity,
observed evidence, mutation flag fixed to false, and safest next action.

- [ ] **Step 4: Prove diagnostics are read-only**

Snapshot the recursive file list, sizes, modes, and SHA-256 values before and
after every status/doctor fixture and assert equality.

- [ ] **Step 5: Run report tests**

Run: `go test ./internal/workspace -run 'Test(Status|Doctor)' -v`  
Expected: PASS.

- [ ] **Step 6: Commit diagnostics**

```bash
git add internal/workspace/status.go internal/workspace/status_test.go
git commit -m "feat: report workspace status and integrity"
```

## Task 8: Expose preview-first CLI commands

**Files:**
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`
- Create: `internal/cli/init_test.go`
- Create: `internal/cli/status_test.go`

**Interfaces:**
- Consumes: `workspace.PlanInit`, `workspace.Apply`, `workspace.Status`, and
  `workspace.Doctor`.
- Produces: `uawp init --workspace PATH [--approve PLAN_ID]`, `uawp status`, and
  `uawp doctor` with stable exit categories.

- [ ] **Step 1: Write CLI behavior tests**

Use temp workspaces and assert: init without approval prints an exact plan ID
and changes but writes nothing; init with matching approval applies; wrong or
stale ID fails; status/doctor never mutate; unknown command exits 2; unsafe
workspace exits 3; invalid state exits 4; approval required exits 5; unexpected
internal failure exits 10.

- [ ] **Step 2: Verify CLI tests fail**

Run: `go test ./internal/cli -run 'Test(Init|Status|Doctor|Exit)' -v`  
Expected: FAIL because commands are not registered.

- [ ] **Step 3: Implement standard-library flag parsing per command**

Require an explicit `--workspace`; resolve it through `OpenRoot`. `init` always
rebuilds the plan immediately before apply and accepts approval only when the
fresh plan ID equals `--approve`. Print structured plain text with `observed`,
`planned`, `changed`, and `next` sections. Never prompt when stdin is not a TTY;
this plan uses explicit plan IDs for auditable consent.

- [ ] **Step 4: Add JSON output for automation**

Support `--format text|json`; JSON objects include `schemaVersion`, `command`,
`workspace`, `planID`, `findings`, `changes`, `mutated`, and `nextAction`.
Reject unknown formats before workspace access.

- [ ] **Step 5: Run CLI and full tests**

Run: `go test ./internal/cli -v && go test ./...`  
Expected: PASS.

- [ ] **Step 6: Commit the CLI vertical slice**

```bash
git add internal/cli
git commit -m "feat(cli): add preview-first init status and doctor"
```

## Task 9: Add realistic preservation and end-to-end fixtures

**Files:**
- Create: `testdata/brownfield/AGENTS.md`
- Create: `testdata/brownfield/CLAUDE.md`
- Create: `testdata/brownfield/context.md`
- Create: `testdata/brownfield/src/example.txt`
- Create: `testdata/brownfield/.fixture-manifest.json`
- Create: `internal/e2e/init_test.go`
- Create: `internal/e2e/fuzz_test.go`

**Interfaces:**
- Consumes: compiled CLI behavior from Task 8.
- Produces: preservation evidence for greenfield/brownfield initialization.

- [ ] **Step 1: Create a fixture manifest of protected content**

Record SHA-256 and mode for every brownfield file. Include LF and CRLF text,
Unicode content and filenames, a zero-byte file, a read-only file, and native
instruction files with content that resembles UAWP markers but lies outside
`.uawp/`.

- [ ] **Step 2: Write end-to-end init tests**

Copy the fixture to a temp directory, run preview, assert no mutation, apply
the approved plan, verify `.uawp/`, then compare every protected hash and mode.
Run init preview again and assert zero changes. Add greenfield and unknown
namespace cases.

- [ ] **Step 3: Verify the end-to-end test catches a deliberate hash mismatch**

Temporarily change the expected digest in the test fixture, run
`go test ./internal/e2e -run TestBrownfieldInit -v`, observe FAIL, then restore
the correct digest and rerun to PASS.

- [ ] **Step 4: Add fuzz targets for manifest and relative paths**

`FuzzDecodeManifest` asserts no panic and strict trailing-data rejection.
`FuzzResolveUAWP` seeds traversal, separators, NUL, reserved names, Unicode,
and long segments; every accepted result must remain beneath the canonical
workspace root.

- [ ] **Step 5: Run preservation, race, and fuzz smoke checks**

Run: `go test -race ./... && go test ./internal/e2e -run '^$' -fuzz Fuzz -fuzztime 10s`  
Expected: PASS with no protected-file mismatch, race, or panic.

- [ ] **Step 6: Commit end-to-end safety evidence**

```bash
git add testdata internal/e2e
git commit -m "test: prove greenfield and brownfield init safety"
```

## Task 10: Document the first usable increment

**Files:**
- Modify: `README.md`
- Create: `docs/protocol/state-v1.md`
- Create: `docs/user/safe-init.md`
- Create: `docs/user/diagnostics.md`
- Create: `docs/engineering/package-boundaries.md`
- Create: `CHANGELOG.md`

**Interfaces:**
- Consumes: exact CLI and state behavior from Tasks 1–9.
- Produces: user-operable documentation and contributor boundaries.

- [ ] **Step 1: Write command examples from executable tests**

Document the two-step preview/apply flow using a captured plan ID, exact exit
categories, status/doctor examples, unknown namespace behavior, and the promise
that this increment never edits agent-native files.

- [ ] **Step 2: Document state schemas and ownership semantics**

Describe manifest v1, Markdown file responsibilities, ACTIVE/RELEASED meaning,
timestamps, and the fact that lifecycle mutations arrive in Plan 2. Do not
claim launch-agent compatibility.

- [ ] **Step 3: Document package dependency rules**

State the allowed direction `cli -> workspace -> plan/core`; adapters may
depend on Core contracts but Core may never import adapters, CLI, or vendors.
Include the command `go list -deps` used in review to detect boundary drift.

- [ ] **Step 4: Add documentation assertions**

Create a table-driven test in `internal/e2e/docs_test.go` that runs every shell
command marked `<!-- verify -->` in temporary workspaces and compares expected
exit codes. Assert every linked local document exists.

- [ ] **Step 5: Run the complete Phase 1 gate**

Run: `make check`  
Run: `go test ./internal/e2e -run TestDocumentationExamples -v`  
Run: `git diff --check`  
Expected: all exit 0.

- [ ] **Step 6: Commit the usable increment documentation**

```bash
git add README.md CHANGELOG.md docs internal/e2e/docs_test.go
git commit -m "docs: publish safe-init vertical slice guidance"
```

## Phase 1 completion review

Before marking this plan complete:

1. Run `make check` from a clean checkout.
2. Run the end-to-end suite on macOS, Linux, and Windows CI.
3. Confirm `git status --short` is empty.
4. Map each Global Constraint and Review Focus item to its passing test.
5. Review the branch specifically for whole-file rewrites, symlink escapes,
   stale-plan application, error paths that claim success, and vendor leakage.
6. Do not begin Plan 2 until the branch review is accepted.
