# UAWP Plan 4.5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a human-friendly UAWP CLI, deterministic agent automation, durable local Worker profiles, and single-ACTIVE-Session fencing without weakening the existing non-destructive, preview/approval, transaction, adapter, or Human Controller guarantees.

**Architecture:** Extend the agent-neutral Core ownership tuple to `(workerID, sessionID, generation)` and migrate released state from `1.0.0` or `1.1.0` to exact state version `1.2.0`. Add a standard-library-only local identity registry outside Workspaces, then route human, agent, and expert CLI presentations through the same typed workspace planners and immutable apply path.

**Tech Stack:** Go 1.26+, Go standard library only, Markdown state documents, JSON local configuration and command output, existing UAWP transaction and migration packages.

**Spec:** `docs/superpowers/specs/2026-09-22-human-friendly-cli-agent-automation-session-design.md`

## Global Constraints

- Target state version is exactly `1.2.0`; `1.0.0` and `1.1.0` are the only supported upgrade sources.
- Runtime code uses only the Go standard library; adding a dependency requires a separate ADR.
- Core packages do not import CLI, adapter, or vendor-specific packages.
- UAWP Workspace state remains under `.uawp/`; local identity configuration uses the OS per-user configuration directory.
- Agent-native files remain project- or provider-owned and are modified only by existing verified adapter plans.
- One physical Workspace permits exactly one ACTIVE `(Worker ID, Session ID, Generation)` tuple.
- Same-directory parallel writes, automatic staleness, TTLs, heartbeats, and silent takeover remain prohibited.
- Human confirmation applies the exact in-memory plan that was previewed; expert and agent apply reconstruct the exact plan bound by `planID`.
- Structured output and redirected execution never prompt or infer approval.
- Existing exit codes retain their broad meanings: `0`, `2`, `3`, `4`, `5`, and `10`.
- Existing project-owned content and bytes outside approved UAWP-owned or adapter-owned regions must remain unchanged.

## Review Focus

- Two conversations reuse one Worker ID: only the exact Session and generation may write; the other receives a stable read-only conflict.
- Interactive detection is ambiguous because one stream is redirected: UAWP must choose non-interactive behavior and never consume a piped `yes` as approval.
- A local identity registry is truncated, duplicated, symlinked, or permission-unsafe: Workspace state must remain untouched and the CLI must report a precise configuration error.
- A legacy Workspace is ACTIVE during `1.1.0 -> 1.2.0`: upgrade must stop and require normal release; it must not invent or bind a Session.
- A previewed Session plan races with handoff and reacquire: changed generation or ownership bytes must invalidate the old approval before mutation.

---

### Task 1: Add Session-aware Core ownership

**Files:**
- Modify: `internal/core/ownership.go`
- Modify: `internal/core/ownership_codec.go`
- Modify: `internal/core/ownership_transition.go`
- Modify: `internal/core/arbitration.go`
- Modify: `internal/core/ownership_test.go`
- Modify: `internal/core/ownership_codec_test.go`
- Modify: `internal/core/ownership_transition_test.go`
- Modify: `internal/core/arbitration_test.go`
- Modify: `templates/state/ACTIVE_WORKER.md`

**Interfaces:**
- Consumes: existing `OwnershipStatus`, timestamp parsing, and domain-error helpers.
- Produces: `core.Actor`, Session-aware `core.Ownership`, `core.ValidateActiveActor`, Session-aware `core.AcquireRequest` and `core.ReleaseRequest` for Tasks 2–9.

- [ ] **Step 1: Write failing ownership validation and codec tests**

Add cases that require an ACTIVE Session and positive generation, permit the bootstrap RELEASED tuple, retain a released Session for audit, reject split empty/non-empty Session fields, and round-trip the new fields:

```go
func TestValidateOwnershipRequiresConsistentSessionTuple(t *testing.T) {
	at := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	released := at.Add(time.Hour)
	tests := []struct {
		name string
		value Ownership
		wantErr bool
	}{
		{"active tuple", Ownership{Status: Active, WorkerID: "worker-a", SessionID: "session-a", Generation: 1, Agent: "Agent A", AcquiredAt: at, Purpose: "work"}, false},
		{"active missing session", Ownership{Status: Active, WorkerID: "worker-a", Generation: 1, Agent: "Agent A", AcquiredAt: at, Purpose: "work"}, true},
		{"active zero generation", Ownership{Status: Active, WorkerID: "worker-a", SessionID: "session-a", Agent: "Agent A", AcquiredAt: at, Purpose: "work"}, true},
		{"bootstrap released", Ownership{Status: Released, WorkerID: "uawp-bootstrap", Agent: "UAWP", AcquiredAt: at, ReleasedAt: &released, Purpose: "Initialize"}, false},
		{"released audit tuple", Ownership{Status: Released, WorkerID: "worker-a", SessionID: "session-a", Generation: 2, Agent: "Agent A", AcquiredAt: at, ReleasedAt: &released, Purpose: "work"}, false},
		{"released split tuple", Ownership{Status: Released, WorkerID: "worker-a", SessionID: "session-a", Agent: "Agent A", AcquiredAt: at, ReleasedAt: &released, Purpose: "work"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOwnership(tt.value)
			if (err != nil) != tt.wantErr { t.Fatalf("error=%v wantErr=%t", err, tt.wantErr) }
		})
	}
}
```

Update `releasedOwnership` to include:

```text
- Session ID: session-a
- Generation: 2
```

Add malformed codec cases for duplicate, missing, non-numeric, negative, and overflowing generation.

- [ ] **Step 2: Run the Core tests and verify the new cases fail**

Run:

```bash
go test ./internal/core -run 'TestValidateOwnershipRequiresConsistentSessionTuple|TestDecodeOwnership' -count=1
```

Expected: FAIL because `SessionID`, `Generation`, and the new document fields do not exist.

- [ ] **Step 3: Implement the Session tuple and strict codec**

Add these exact public types and fields:

```go
type Actor struct {
	WorkerID  string
	SessionID string
	Generation uint64
}

type Ownership struct {
	Status OwnershipStatus
	WorkerID string
	SessionID string
	Generation uint64
	Agent string
	AcquiredAt time.Time
	ReleasedAt *time.Time
	Purpose string
}

func ValidateActiveActor(current Ownership, actor Actor) error
```

`ValidateActiveActor` must trim caller strings, require ACTIVE state, and compare all three tuple values. Extend the codec field allow-list with `Session ID` and `Generation`; encode an empty Session as `none` and parse generation with `strconv.ParseUint(value, 10, 64)`.

Update `templates/state/ACTIVE_WORKER.md`:

```markdown
# UAWP Active Worker

- Status: RELEASED
- Worker ID: uawp-bootstrap
- Session ID: none
- Generation: 0
- Agent: UAWP
- Acquired At: {{TIMESTAMP}}
- Released At: {{TIMESTAMP}}
- Purpose: Initialize UAWP workspace state
```

- [ ] **Step 4: Write failing transition and fencing tests**

Replace Worker-only requests with:

```go
type AcquireRequest struct {
	WorkerID, SessionID, Agent, Purpose string
	At time.Time
}

type ReleaseRequest struct {
	Actor Actor
	At time.Time
}
```

Test generation increment, exact release tuple, stale generation rejection, same Worker/different Session rejection, and `math.MaxUint64` overflow:

```go
func TestAcquireAdvancesGenerationAndReleaseFencesOldActor(t *testing.T) {
	at := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	released := at.Add(-time.Hour)
	current := Ownership{Status: Released, WorkerID: "old", SessionID: "old-session", Generation: 7, Agent: "old", AcquiredAt: at.Add(-2*time.Hour), ReleasedAt: &released, Purpose: "old"}
	next, err := Acquire(current, AcquireRequest{WorkerID: "worker-a", SessionID: "session-a", Agent: "Agent A", Purpose: "work", At: at})
	if err != nil { t.Fatal(err) }
	if next.Generation != 8 { t.Fatalf("generation=%d", next.Generation) }
	if _, err := Release(next, ReleaseRequest{Actor: Actor{WorkerID: "worker-a", SessionID: "session-b", Generation: 8}, At: at.Add(time.Minute)}); err == nil { t.Fatal("different session released ownership") }
	if _, err := Release(next, ReleaseRequest{Actor: Actor{WorkerID: "worker-a", SessionID: "session-a", Generation: 7}, At: at.Add(time.Minute)}); err == nil { t.Fatal("stale generation released ownership") }
}
```

- [ ] **Step 5: Implement Session-aware acquire, release, and stale recovery**

`Acquire` must reject ACTIVE input, require a non-empty Session ID, reject generation overflow, set `Generation = current.Generation + 1`, and clear `ReleasedAt`. `Release` must call `ValidateActiveActor`, retain the tuple for audit, and set RELEASED time. `RecoverStaleOwnership` must retain the old tuple while changing only status and release time; its audit record must add old Session ID and generation.

- [ ] **Step 6: Run Core tests and commit**

Run:

```bash
gofmt -w internal/core/*.go templates/state/*.go
go test ./internal/core ./templates/state -count=1
```

Expected: PASS.

Commit:

```bash
git add internal/core templates/state
git commit -m "feat: fence ownership by active session"
```

### Task 2: Add the exact `1.2.0` migration path

**Files:**
- Modify: `internal/core/version.go`
- Modify: `internal/core/version_test.go`
- Modify: `internal/migration/registry.go`
- Modify: `internal/migration/v1_0_to_v1_1.go`
- Modify: `internal/migration/v1_0_to_v1_1_test.go`
- Create: `internal/migration/v1_1_to_v1_2.go`
- Create: `internal/migration/v1_1_to_v1_2_test.go`
- Modify: `internal/workspace/upgrade.go`
- Modify: `internal/workspace/upgrade_test.go`
- Modify: `internal/workspace/init_test.go`

**Interfaces:**
- Consumes: Session-aware ownership codec from Task 1 and existing migration `Step` registry.
- Produces: `migration.V1_1ToV1_2`, `core.CurrentStateVersion == "1.2.0"`, and composed one-transaction upgrade planning used by all later tests.

- [ ] **Step 1: Write failing version and migration tests**

Update version cases to assert:

```go
tests := map[string]StateCompatibility{
	"1.2.0": StateCurrent,
	"1.1.0": StateUpgradeRequired,
	"1.0.0": StateUpgradeRequired,
	"1.3.0": StateUnsupported,
	"2.0.0": StateFutureMajor,
}
```

Create migration tests with exact legacy RELEASED input and assert one ownership update containing `Session ID: none`, `Generation: 0`, plus a final manifest update to `1.2.0`. Add an ACTIVE legacy fixture and assert planning fails with `upgrade requires RELEASED ownership`.

- [ ] **Step 2: Run migration tests and verify failure**

Run:

```bash
go test ./internal/core ./internal/migration ./internal/workspace -run 'Version|Migration|Upgrade|Init' -count=1
```

Expected: FAIL because `1.2.0` and `V1_1ToV1_2` are absent.

- [ ] **Step 3: Make migration context carry exact file snapshots**

Extend the context without filesystem dependencies:

```go
type Context struct {
	Manifest core.Manifest
	Inputs []plan.Input
	Files map[string][]byte
	GeneratedAt time.Time
}
```

Copy file bytes when constructing a context. Migration steps may read `Files` but must never mutate its byte slices.

- [ ] **Step 4: Implement `V1_1ToV1_2` and single final manifest publication**

Implement:

```go
type V1_1ToV1_2 struct{}
func (V1_1ToV1_2) From() string { return "1.1.0" }
func (V1_1ToV1_2) To() string   { return "1.2.0" }
func (V1_1ToV1_2) Plan(Context) ([]plan.Change, error)
func (V1_1ToV1_2) Verify(Context) error
```

Its private legacy decoder must accept exactly the six `1.1.0` ownership fields, reject duplicate/unknown/missing fields, and require RELEASED ownership. It emits only the ownership update. Refactor `V1_0ToV1_1.Plan` to emit directory changes without publishing an intermediate manifest. `PlanUpgradeAt` resolves:

```go
migration.NewRegistry([]migration.Step{
	migration.V1_0ToV1_1{},
	migration.V1_1ToV1_2{},
})
```

After all steps, append exactly one manifest update from the original manifest hash to `1.2.0` with the highest sequence number. This preserves manifest-last publication and avoids two changes for the same path.

- [ ] **Step 5: Test both supported chains and transactional refusal**

Add workspace cases for `1.0.0 -> 1.2.0`, `1.1.0 -> 1.2.0`, already-current no-op, ACTIVE legacy refusal, unknown `1.0.1`, and injected drift in legacy ownership before apply. Assert migration receipts record the original source and exact `1.2.0` target.

- [ ] **Step 6: Run migration/workspace tests and commit**

Run:

```bash
gofmt -w internal/core/*.go internal/migration/*.go internal/workspace/*.go
go test ./internal/core ./internal/migration ./internal/workspace -run 'Version|Migration|Upgrade|Init' -count=1
```

Expected: PASS.

Commit:

```bash
git add internal/core/version.go internal/core/version_test.go internal/migration internal/workspace/upgrade.go internal/workspace/upgrade_test.go internal/workspace/init_test.go
git commit -m "feat: migrate released workspaces to session state"
```

### Task 3: Enforce the ACTIVE tuple in Workspace operations

**Files:**
- Modify: `internal/plan/change.go`
- Modify: `internal/plan/change_test.go`
- Modify: `internal/workspace/lifecycle.go`
- Modify: `internal/workspace/lifecycle_test.go`
- Modify: `internal/workspace/context.go`
- Modify: `internal/workspace/context_test.go`
- Modify: `internal/workspace/checkpoint.go`
- Modify: `internal/workspace/checkpoint_test.go`
- Modify: `internal/workspace/handoff.go`
- Modify: `internal/workspace/handoff_test.go`
- Modify: `internal/workspace/recover.go`
- Modify: `internal/workspace/recover_test.go`
- Modify: `internal/workspace/status.go`
- Modify: `internal/workspace/status_test.go`
- Modify: `internal/workspace/apply_test.go`

**Interfaces:**
- Consumes: `core.Actor`, `core.ValidateActiveActor`, and state `1.2.0` from Tasks 1–2.
- Produces: Session-aware planners and reports consumed by every CLI surface.

- [ ] **Step 1: Write failing same-Worker/different-Session Workspace tests**

Use this actor throughout tests:

```go
actor := core.Actor{WorkerID: "worker-a", SessionID: "session-a", Generation: 1}
```

Acquire as `session-a`, then assert `session-b` and generation `0` cannot sync, checkpoint, hand off, or release. Preview a sync at generation 1, release and reacquire at generation 2, then assert the old plan fails apply because ownership input drifted.

- [ ] **Step 2: Run Workspace lifecycle tests and verify failure**

Run:

```bash
go test ./internal/workspace -run 'Lifecycle|Context|Checkpoint|Handoff|Apply|Status|Recover' -count=1
```

Expected: FAIL because Workspace planners still accept Worker ID alone.

- [ ] **Step 3: Change all protected planner interfaces to `core.Actor`**

Use these exact signatures:

```go
func Resume(root Root, actor core.Actor) (ResumeReport, error)
func PlanContextSync(root Root, actor core.Actor, content []byte) (plan.Plan, error)

type CheckpointRequest struct {
	Actor core.Actor
	MilestoneID string
	Label string
	DecisionReferences []string
	At time.Time
}

type HandoffRequest struct {
	Actor core.Actor
	FinalContext []byte
	Purpose string
	At time.Time
}
```

`PlanAcquireAt` consumes `AcquireRequest`; `PlanReleaseAt` consumes the new `ReleaseRequest`. Every protected operation must call `ValidateActiveActor` before constructing changes and must bind `.uawp/ACTIVE_WORKER.md` as a plan input.

- [ ] **Step 4: Bind Session evidence into plan metadata and reports**

Extend `plan.Metadata` additively:

```go
ActorSessionID string `json:"actorSessionID,omitempty"`
OwnershipGeneration uint64 `json:"ownershipGeneration,omitempty"`
```

Add `ResumeBlockedBySession` and `ResumeExpiredGeneration` outcomes. `Resume` returns `OWNED_BY_CALLER` only for an exact ACTIVE tuple. Status and doctor text must include Session ID and generation for ACTIVE state without changing existing finding-code meanings.

Update stale-recovery audit text to record:

```text
- Old Session ID: session-a
- Old Generation: 1
```

- [ ] **Step 5: Run Workspace and plan tests and commit**

Run:

```bash
gofmt -w internal/plan/*.go internal/workspace/*.go
go test ./internal/plan ./internal/workspace -count=1
```

Expected: PASS.

Commit:

```bash
git add internal/plan internal/workspace
git commit -m "feat: enforce session tuple on workspace writes"
```

### Task 4: Add durable local identity profiles and human Session bindings

**Files:**
- Create: `internal/identity/profile.go`
- Create: `internal/identity/profile_test.go`
- Create: `internal/identity/store.go`
- Create: `internal/identity/store_test.go`
- Create: `internal/identity/session.go`
- Create: `internal/identity/session_test.go`

**Interfaces:**
- Consumes: Go standard library only.
- Produces: `identity.Registry`, profile creation/selection, Session ID generation, and canonical-Workspace bindings for Tasks 5–7.

- [ ] **Step 1: Write failing profile validation and ID-generation tests**

Define tests for arbitrary Unicode, duplicate display names, trimming, empty/control/newline/101-code-point rejection, deterministic random-reader fixtures, and distinct IDs:

```go
func TestValidateDisplayName(t *testing.T) {
	valid := []string{"Sushi 的 Gemini", "abc123", "🤖"}
	for _, value := range valid {
		if _, err := ValidateDisplayName(value); err != nil { t.Fatalf("%q: %v", value, err) }
	}
	invalid := []string{"", "   ", "line\nbreak", "bad\x00name", strings.Repeat("界", 101)}
	for _, value := range invalid {
		if _, err := ValidateDisplayName(value); err == nil { t.Fatalf("accepted %q", value) }
	}
	}
}
```

- [ ] **Step 2: Implement profile types and random IDs**

Create:

```go
type Profile struct {
	ProfileID string `json:"profileID"`
	WorkerID string `json:"workerID"`
	DisplayName string `json:"displayName"`
}

func ValidateDisplayName(string) (string, error)
func NewProfile(displayName string, random io.Reader) (Profile, error)
func NewSessionID(random io.Reader) (string, error)
```

Read 16 random bytes with `io.ReadFull`. Encode lowercase hexadecimal IDs as `profile-<32 hex>`, `worker-<32 hex>`, and `session-<32 hex>`. Generate profile and Worker IDs from separate random reads.

- [ ] **Step 3: Write failing registry safety and binding tests**

Test new registry save/load, mode `0600`, atomic replacement, default selection, ambiguous selection, duplicate IDs, truncated JSON, unknown JSON fields, trailing JSON, symlinked registry refusal, per-Workspace binding, and removal warnings. Use a registry shaped exactly as:

```go
type Registry struct {
	SchemaVersion string `json:"schemaVersion"`
	DefaultProfileID string `json:"defaultProfileID,omitempty"`
	Profiles []Profile `json:"profiles"`
	Bindings []SessionBinding `json:"bindings,omitempty"`
}

type SessionBinding struct {
	Workspace string `json:"workspace"`
	ProfileID string `json:"profileID"`
	SessionID string `json:"sessionID"`
	Generation uint64 `json:"generation"`
}
```

- [ ] **Step 4: Implement strict atomic registry storage**

Provide:

```go
func DefaultPath(userConfigDir string) string
func Load(path string) (Registry, error)
func Save(path string, value Registry) error
func (r Registry) Selected(profileID string, allowDefault bool) (Profile, error)
func (r *Registry) UpsertBinding(SessionBinding)
func (r *Registry) RemoveBinding(workspace, profileID string)
func (r Registry) Binding(workspace, profileID string) (SessionBinding, bool)
```

`DefaultPath` returns `<user-config-dir>/uawp/identity.json`. `Load` rejects symlinks, non-regular files, duplicate keys via strict token decoding, unknown fields with `DisallowUnknownFields`, and trailing data. `Save` creates the parent with `0700`, writes a same-directory temporary file with `0600`, syncs, renames, and syncs the directory where supported. A failed write must leave the prior registry readable.

- [ ] **Step 5: Run identity tests and commit**

Run:

```bash
gofmt -w internal/identity/*.go
go test ./internal/identity -count=1
```

Expected: PASS.

Commit:

```bash
git add internal/identity
git commit -m "feat: add local worker identity profiles"
```

### Task 5: Introduce a testable CLI runtime and safe surface selection

**Files:**
- Create: `internal/cli/runtime.go`
- Create: `internal/cli/runtime_test.go`
- Create: `internal/cli/confirm.go`
- Create: `internal/cli/confirm_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`
- Modify: `internal/cli/init_test.go`
- Modify: `cmd/uawp/main.go`

**Interfaces:**
- Consumes: existing CLI `Run`, immutable plans, Workspace boundary checks, and identity path from Task 4.
- Produces: `runWithRuntime`, current-directory defaults, deterministic interactivity classification, and same-invocation confirmation used by Tasks 6–8.

- [ ] **Step 1: Write failing runtime classification tests**

Test all stream combinations. Only text mode with both input and confirmation output marked terminal may be interactive:

```go
func TestSurfaceSelectionFailsClosed(t *testing.T) {
	tests := []struct{ name, format string; inTTY, outTTY, nonInteractive, wantInteractive bool }{
		{"human", "text", true, true, false, true},
		{"json", "json", true, true, false, false},
		{"piped input", "text", false, true, false, false},
		{"redirected output", "text", true, false, false, false},
		{"explicit noninteractive", "text", true, true, true, false},
	}
	for _, tt := range tests {
		if got := selectSurface(tt.format, tt.inTTY, tt.outTTY, tt.nonInteractive); (got == surfaceHuman) != tt.wantInteractive { t.Fatalf("%s: %s", tt.name, got) }
	}
}
```

- [ ] **Step 2: Add the injected runtime without changing public `Run`**

Create:

```go
type runtime struct {
	stdin io.Reader
	stdout io.Writer
	stderr io.Writer
	getwd func() (string, error)
	userConfigDir func() (string, error)
	now func() time.Time
	random io.Reader
	stdinTTY bool
	stdoutTTY bool
}

func runWithRuntime(args []string, rt runtime) int
```

`Run(args, stdout, stderr)` remains the compatibility entry point and builds a production runtime from `os.Stdin`, `os.Getwd`, `os.UserConfigDir`, `time.Now`, and `crypto/rand.Reader`. Detect terminal files by `Stat().Mode()&os.ModeCharDevice != 0`; errors mean non-interactive. `cmd/uawp/main.go` keeps calling `cli.Run`.

- [ ] **Step 3: Write failing confirmation tests**

Use a `bufio.Reader` and assert only case-insensitive `y` and `yes` approve; `n`, empty line, EOF, whitespace-only, and arbitrary text cancel. Add a test that piped `yes\n` is never read when the selected surface is non-interactive.

- [ ] **Step 4: Implement current-directory defaults and same-plan confirmation for `init`**

Add `--non-interactive` to common parsing. If Workspace is omitted, call injected `getwd`; always canonicalize through `workspace.OpenRoot`. In human surface:

1. plan once using injected `now`;
2. print canonical Workspace and concise changes;
3. call `confirm`;
4. on yes, pass the same plan ID to `workspace.Apply`;
5. on cancellation, return `0`, report `mutated: false`, and perform no write.

JSON and non-interactive text retain preview + plan ID + exit `5`. Explicit `--approve` retains exact reconstruction behavior.

- [ ] **Step 5: Run CLI initialization tests and commit**

Run:

```bash
gofmt -w internal/cli/*.go cmd/uawp/*.go
go test ./internal/cli -run 'Runtime|Surface|Confirm|Init|Run' -count=1
```

Expected: PASS.

Commit:

```bash
git add internal/cli/runtime.go internal/cli/runtime_test.go internal/cli/confirm.go internal/cli/confirm_test.go internal/cli/run.go internal/cli/run_test.go internal/cli/init_test.go cmd/uawp/main.go
git commit -m "feat: add safe human CLI surface"
```

### Task 6: Add identity/session commands and guided resume

**Files:**
- Create: `internal/cli/identity.go`
- Create: `internal/cli/identity_test.go`
- Create: `internal/cli/session.go`
- Create: `internal/cli/session_test.go`
- Modify: `internal/cli/lifecycle.go`
- Modify: `internal/cli/lifecycle_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/status_test.go`

**Interfaces:**
- Consumes: identity registry from Task 4, runtime from Task 5, and Session-aware Workspace lifecycle from Task 3.
- Produces: profile management, `uawp session new`, Session-aware expert flags, and human `uawp resume` acquisition flow.

- [ ] **Step 1: Write failing identity command tests**

Cover:

```text
uawp identity create --name "Sushi 的 Gemini" --format json
uawp identity list --format json
uawp identity use PROFILE_ID --format json
uawp identity show --format json
uawp identity remove PROFILE_ID --format json
uawp session new --format json
```

Assert JSON is one document, Worker and Session IDs match their prefixes, duplicate display names are allowed, ambiguous automation without `--profile` fails usage, and removing a profile with a binding emits a warning without touching the Workspace.

- [ ] **Step 2: Implement identity and Session command routing**

Add `identity` and `session` to `usage`. Identity operations mutate only the local registry and use explicit confirmation when removing a bound profile in the human surface. `session new` returns:

```json
{
  "schemaVersion": "1",
  "command": "session new",
  "sessionID": "session-...",
  "mutated": false,
  "nextAction": "Use this Session ID for one conversation or execution session."
}
```

It must not read or write a Workspace.

- [ ] **Step 3: Add Session-aware lifecycle flags and actor resolution**

Extend lifecycle flags:

```go
profile := set.String("profile", "", "local Worker profile ID")
session := set.String("session-id", "", "conversation or execution Session ID")
generation := set.Uint64("generation", 0, "ownership generation")
nonInteractive := set.Bool("non-interactive", false, "disable prompts")
```

Agent/expert calls resolve either explicit `--worker-id` plus Session fields or an explicit `--profile`; ambiguous combinations fail usage. Human calls may use the default local profile and canonical-Workspace binding. Never choose an ambiguous automation profile.

- [ ] **Step 4: Write failing guided-resume tests**

Test these complete cases with injected runtime:

1. no profile: prompt with static `Zhang San's Codex` format guidance, create profile after a valid label, then offer acquisition;
2. RELEASED: preview acquisition and yes applies it in the same invocation;
3. exact ACTIVE binding: report continuation without mutation;
4. same Worker/different Session: return stable read-only conflict;
5. another Worker: return stable read-only conflict;
6. stale ACTIVE: never infer release and point to Human Controller recovery;
7. JSON resume: never prompt and require explicit actor/session data.

- [ ] **Step 5: Implement guided resume and binding persistence**

Human resume performs read-only reconstruction first. On RELEASED state, create a candidate Session ID, build `AcquireRequest`, show the ownership change, and ask confirmation. Save `SessionBinding{Workspace, ProfileID, SessionID, Generation}` only after verified acquisition. On handoff/release, remove the binding only after verified release. If registry persistence fails after Workspace mutation, report the exact ACTIVE tuple and recovery command; never roll back a verified Workspace transaction by guessing.

- [ ] **Step 6: Run identity/lifecycle CLI tests and commit**

Run:

```bash
gofmt -w internal/cli/*.go
go test ./internal/cli -run 'Identity|Session|Resume|Lifecycle|Status' -count=1
```

Expected: PASS.

Commit:

```bash
git add internal/cli
git commit -m "feat: add worker profiles and guided resume"
```

### Task 7: Make routine human mutations concise and Session-safe

**Files:**
- Modify: `internal/cli/lifecycle.go`
- Modify: `internal/cli/lifecycle_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/e2e/lifecycle_test.go`
- Create: `internal/e2e/human_cli_test.go`

**Interfaces:**
- Consumes: human surface and actor resolution from Tasks 5–6.
- Produces: concise `sync`, `checkpoint`, `handoff`, `acquire`, and `release` workflows over the same Workspace planners.

- [ ] **Step 1: Write failing human routine-flow tests**

Test from a current working directory with no `--workspace`:

```text
uawp sync --context-file final-context.md
uawp checkpoint
uawp handoff --context-file final-context.md
```

For checkpoint, feed `phase-2\nPhase 2 complete\ny\n`; assert the milestone fields and confirmation are consumed in order. For handoff, prompt for missing purpose, preview context before ownership release, apply on yes, and clear the local binding. For sync/handoff with no file and terminal input, report exact guidance and perform zero mutation rather than treating confirmation input as context.

- [ ] **Step 2: Centralize preview/apply without changing Core policy**

Create one CLI helper:

```go
type mutationPresentation struct {
	Command string
	Plan plan.Plan
	Result commandOutput
	Exceptional bool
}

func presentMutation(rt runtime, surface cliSurface, value mutationPresentation, apply func(plan.Plan) (bool, error)) int
```

The helper prints concise human changes, calls `confirm` only for ordinary human mutations, preserves JSON preview/apply, returns exit `5` when non-interactive approval is absent, and never prompts when `Exceptional` is true. Exceptional recovery, purge, repair, and transaction flows keep explicit plan-ID approval.

- [ ] **Step 3: Implement missing-field prompts and input separation**

In human surface only:

- checkpoint asks milestone ID and label when omitted;
- acquire asks purpose when omitted;
- handoff asks purpose when omitted;
- sync and handoff require `--context-file PATH` or `--context-file -`;
- `--context-file -` is rejected when the same stdin would also be needed for interactive confirmation; users must use a file or explicit non-interactive preview/apply.

Validate all collected values before planning. Never place terminal prose into `.uawp/CONTEXT.md`.

- [ ] **Step 4: Add current-directory end-to-end lifecycle coverage**

Initialize a temporary non-Git office-style folder containing `.docx`, `.xlsx`, and arbitrary project files. Run human init, resume/acquire, sync, checkpoint, and handoff. Assert all original file hashes are unchanged, one checkpoint exists, and only `.uawp/` changed.

- [ ] **Step 5: Run routine-flow tests and commit**

Run:

```bash
gofmt -w internal/cli/*.go internal/e2e/*.go
go test ./internal/cli ./internal/e2e -run 'Human|Lifecycle|Checkpoint|Handoff|Sync' -count=1
```

Expected: PASS.

Commit:

```bash
git add internal/cli internal/e2e
git commit -m "feat: simplify routine workspace workflows"
```

### Task 8: Stabilize agent automation and update canonical prompts

**Files:**
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/lifecycle.go`
- Modify: `internal/cli/lifecycle_test.go`
- Modify: `internal/cli/maintenance_test.go`
- Modify: `internal/e2e/lifecycle_test.go`
- Modify: `internal/e2e/recovery_test.go`
- Modify: `internal/core/prompts.go`
- Modify: `internal/core/prompts_test.go`
- Modify: `prompts/RESUME_WORK.md`
- Modify: `prompts/CONTEXT_SYNC.md`
- Modify: `prompts/CREATE_CHECKPOINT.md`
- Modify: `prompts/PAUSE_AND_HANDOFF.md`
- Modify: `templates/state/INSTRUCTIONS.md`

**Interfaces:**
- Consumes: Session-aware lifecycle, stable plan IDs, and surface selection.
- Produces: additive JSON fields, stable Session conflict classifications, and Prompt Library version 2.

- [ ] **Step 1: Write failing non-interactive contract tests**

For every routine mutation, invoke `--format json` with explicit Worker ID and Session data. Assert:

- no prompt text appears;
- stdout decodes as exactly one JSON value;
- preview returns exit `5`, `mutated: false`, and a non-empty `planID`;
- apply with the exact token succeeds;
- changed Session or generation rejects the token with exit `5` and zero mutation;
- structured output exposes `workerID`, `sessionID`, and `ownershipGeneration` where relevant; and
- same Worker/different Session has a stable finding code distinct from another Worker.

Use a decoder followed by a second decode expecting `io.EOF` to prove there is only one JSON document.

- [ ] **Step 2: Add structured actor fields and stable finding codes**

Extend `commandOutput` additively:

```go
WorkerID string `json:"workerID,omitempty"`
SessionID string `json:"sessionID,omitempty"`
OwnershipGeneration uint64 `json:"ownershipGeneration,omitempty"`
Code string `json:"code,omitempty"`
```

Define CLI classification constants:

```go
const (
	codeIdentityRequired = "IDENTITY_CONFIGURATION_REQUIRED"
	codeSessionRequired = "SESSION_ID_REQUIRED"
	codeActiveOtherSession = "ACTIVE_OTHER_SESSION"
	codeActiveOtherWorker = "ACTIVE_OTHER_WORKER"
	codeApprovalRequired = "APPROVAL_REQUIRED"
	codeApprovalDrift = "APPROVAL_INVALIDATED_BY_DRIFT"
	codeProfileAmbiguous = "PROFILE_SELECTION_AMBIGUOUS"
)
```

Map typed outcomes to codes in CLI presentation only; do not parse error prose to implement ownership policy.

- [ ] **Step 3: Update all four prompts to version 2**

Each prompt must require the exact ACTIVE tuple before a protected write. For example, `CONTEXT_SYNC.md` must state:

```markdown
Prerequisite: the caller holds the exact ACTIVE Worker ID, Session ID, and Ownership Generation recorded in `.uawp/ACTIVE_WORKER.md`.

1. Recheck the complete ACTIVE tuple immediately before planning.
2. Prepare the complete replacement for `.uawp/CONTEXT.md`.
3. Preview and approve the exact change bound to the ownership document.
4. Recheck the tuple and drift immediately before apply.
5. Apply and verify the new context while retaining the tuple unchanged.
```

`RESUME_WORK` must distinguish another Session under the same Worker. `PAUSE_AND_HANDOFF` must prohibit the released Session from writing after a later generation is acquired. `CREATE_CHECKPOINT` must record the tuple. Update `templates/state/INSTRUCTIONS.md` to reference Prompt Version 2 without mentioning a provider.

- [ ] **Step 4: Run automation, prompt, and stale-recovery tests**

Run:

```bash
gofmt -w internal/cli/*.go internal/core/*.go internal/e2e/*.go
go test ./internal/cli ./internal/core ./internal/e2e -run 'Automation|Prompt|Lifecycle|Recovery|Maintenance' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit automation and prompt changes**

```bash
git add internal/cli internal/core internal/e2e prompts templates/state/INSTRUCTIONS.md
git commit -m "feat: stabilize session-aware agent automation"
```

### Task 9: Complete documentation, compatibility, and release verification

**Files:**
- Modify: `docs/engineering/implementation-roadmap.md`
- Modify: `docs/protocol/state-v1.md`
- Modify: `docs/user/safe-init.md`
- Modify: `docs/user/lifecycle.md`
- Modify: `docs/user/stale-recovery.md`
- Create: `docs/user/identity-and-sessions.md`
- Create: `docs/user/agent-automation.md`
- Modify: `docs/user/upgrade-and-repair.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `internal/e2e/docs_test.go`
- Modify: `.github/workflows/ci.yml` only if the existing matrix does not already execute the required Go 1.26/1.27 checks.

**Interfaces:**
- Consumes: all implemented Plan 4.5 commands and schemas.
- Produces: user-facing contracts, executable documentation examples, and a verified Plan 4.5 increment ready for Plan 5.

- [ ] **Step 1: Write failing documentation contract tests**

Extend `docs_test.go` to assert:

- current state is documented as `1.2.0`;
- Quick Start includes bare `uawp init`, `uawp resume`, `uawp sync`, `uawp checkpoint`, and `uawp handoff`;
- expert preview/apply examples still exist;
- agent automation examples include Worker ID, Session ID, generation, JSON, exit `5`, and exact approval;
- ordinary Workspace examples explicitly say Git is optional; and
- no document claims same-directory parallel writing is supported.

- [ ] **Step 2: Update roadmap and protocol documentation**

Mark Plan 4.5 implemented only after all release gates pass. Before that, write its exact deliverables and state migration. Document the ownership record fields, tuple validation, generation increment, normal release, same-Worker Session conflict, stale arbitration, and `1.0.0/1.1.0 -> 1.2.0` migration restriction.

- [ ] **Step 3: Write the human and automation guides**

`identity-and-sessions.md` must show static display-name guidance, arbitrary label semantics, profile commands, one ACTIVE Session, normal Session switching, and Human Controller stale recovery. `agent-automation.md` must show exact two-call JSON preview/apply examples and explain that structured output never prompts.

Use these human Quick Start commands:

```bash
uawp init
uawp resume
uawp sync --context-file context-next.md
uawp checkpoint
uawp handoff --context-file context-final.md
```

Retain an expert example with explicit `--workspace`, `--worker-id`, `--session-id`, `--generation`, `--format json`, and `--approve`.

- [ ] **Step 4: Run focused and full verification**

Run:

```bash
make fmt
go test ./...
go test -race ./...
go vet ./...
go test ./internal/e2e -run Fuzz -count=1
go build ./cmd/uawp
git diff --check
```

Expected: every command exits `0`; `git diff --check` prints nothing.

- [ ] **Step 5: Perform clean-checkout smoke tests**

From a temporary clone of the current branch, build `uawp`, initialize an empty non-Git directory interactively, run a complete human lifecycle, then run the expert JSON preview/apply flow. Assert the human flow never exposes a required copied plan ID, the expert flow still does, and project files remain byte-for-byte unchanged.

- [ ] **Step 6: Commit documentation and final compatibility updates**

```bash
git add README.md CHANGELOG.md docs internal/e2e/docs_test.go .github/workflows/ci.yml
git commit -m "docs: publish Plan 4.5 workflows"
```

- [ ] **Step 7: Request final whole-branch review before integration**

Review against the approved spec with special attention to the five Review Focus cases, migration safety, non-interactive prompting, native-file preservation, and expert compatibility. Resolve every P0/P1 finding, rerun Step 4, and record the final review result in the handoff.

## Final acceptance checklist

- [ ] State `1.2.0` is exact, validated, and transactionally reachable from released `1.0.0` and `1.1.0` only.
- [ ] ACTIVE ownership requires Worker ID, Session ID, and positive generation.
- [ ] Same Worker/different Session cannot perform a protected write.
- [ ] Human commands default safely to the current directory and apply the exact previewed plan after confirmation.
- [ ] JSON, redirected, and explicit non-interactive execution never prompt.
- [ ] Display names are user-supplied labels only; arbitrary valid strings and duplicates do not affect identity.
- [ ] Local profile failures never mutate a Workspace.
- [ ] Normal handoff/release and Human Controller stale recovery remain distinct.
- [ ] No target Workspace is required to use Git.
- [ ] Existing adapters, transaction recovery, uninstall, repair, and purge guarantees remain green.
- [ ] Full test, race, vet, fuzz/property, build, clean-checkout smoke, and independent review gates pass.
