# UAWP Cross-Account Engineering Handoff

**Date:** 2026-09-21

**Reason:** Continue implementation in another Codex account before the current
account reaches its usage limit.

**Status:** Task 6 complete; Task 7 is the next unstarted task.

## 1. Exact workspace to open

Continue in this linked Git worktree, not the main checkout:

```text
/Users/rock-2024-macbook/Documents/UAWP v1.0/.worktrees/foundation-safe-init
```

Repository and branch state at handoff:

```text
Main checkout: /Users/rock-2024-macbook/Documents/UAWP v1.0
Worktree:      /Users/rock-2024-macbook/Documents/UAWP v1.0/.worktrees/foundation-safe-init
Branch:        feature/foundation-safe-init
Merge base:    11445ee
HEAD:          0317a42
```

Do not start implementation in the main checkout. Do not delete the linked
worktree. Do not reset, recreate, or squash the completed task commits.

## 2. Authoritative documents

Read these before changing code:

1. Product specification:
   `docs/superpowers/specs/2026-09-21-uawp-product-engineering-design.md`
2. Technical decision:
   `docs/adr/0001-go-toolchain-and-dependency-policy.md`
3. Product roadmap:
   `docs/engineering/implementation-roadmap.md`
4. Current implementation plan:
   `docs/superpowers/plans/2026-09-21-foundation-core-safe-init.md`
5. Execution ledger, which is intentionally Git-ignored:
   `.superpowers/sdd/2026-09-21-foundation-core-safe-init/progress.md`
6. This handoff:
   `docs/handoffs/2026-09-21-cross-account-handoff.md`

The specification is binding. The implementation plan is approved. Execution
mode is Native/inline using `superpowers:executing-plans`, with strict
`superpowers:test-driven-development` for every remaining task.

## 3. Product invariants that must not change

- Core remains Agent-neutral; vendor names and native instruction behavior do
  not enter Core.
- UAWP runtime state is owned only under `.uawp/`.
- Existing projects are integrated non-destructively.
- Agent-native files are project/vendor-owned and are not modified in this
  first plan.
- Every mutation follows discovery, plan, preview, explicit approval, drift
  check, apply, and verification.
- Single Active Worker and persistent ownership remain protocol invariants.
- A stale ACTIVE claim requires Human Controller arbitration; v1.0 has no TTL,
  heartbeat, lease expiry, or automatic liveness release.
- Adapter integration modes remain DIRECT, IMPORT, and MANAGED_BLOCK, but
  adapters are outside this first implementation plan.
- The canonical Prompt Library must eventually contain `RESUME_WORK`,
  `CONTEXT_SYNC`, `CREATE_CHECKPOINT`, and `PAUSE_AND_HANDOFF`; that is Plan 2,
  not the current foundation plan.

## 4. Completed implementation tasks

### Task 1 — Go project and quality gate

Commit: `d838326 build: establish Go project and quality gate`

Delivered:

- Go module `github.com/uawp/uawp`, compatibility floor Go 1.26;
- CLI seam `cli.Run` and `uawp version`;
- Makefile formatting, test, race, and vet gates;
- GitHub Actions matrix for macOS/Linux/Windows and Go 1.26/1.27.

### Task 2 — Manifest validation

Commit: `f79076c feat(core): validate UAWP namespace manifests`

Delivered:

- strict manifest decoding;
- duplicate and unknown key rejection;
- UAWP namespace ownership verification;
- state major-version compatibility validation;
- published JSON Schema.

### Task 3 — Persistent ownership model

Commit: `7a96ea7 feat(core): define persistent ownership state`

Delivered:

- `ACTIVE` and `RELEASED` states;
- worker, agent, purpose, acquisition, and release invariants;
- RFC 3339 timestamp parsing with explicit timezone preservation.

### Task 4 — Workspace boundary enforcement

Commit: `eb8879b feat(workspace): enforce mutation boundaries`

Delivered:

- canonical workspace roots;
- `.uawp/` mutation allowlist;
- traversal, absolute path, NUL, separator, and portable-name rejection;
- namespace and intermediate symlink escape rejection.

### Task 5 — Immutable plans and read-only discovery

Commit: `310ef3a feat: add immutable plans and workspace discovery`

Delivered:

- deterministically hashed immutable plans and copied content;
- human-readable exact previews;
- `.uawp` classification as absent, owned, unknown, or invalid;
- manifest reads capped at 1 MiB;
- root `context.md` and native-file reporting without mutation.

### Task 6 — Safe init and transactional apply

Commit: `0317a42 feat: plan and apply safe namespace initialization`

Delivered:

- deterministic initialization plans bound to a canonical workspace root;
- `.uawp/`, checkpoints, manifest, context, ownership, and decisions state;
- embedded canonical templates;
- explicit plan-ID approval;
- pre-apply drift and fingerprint detection;
- atomic same-directory file replacement;
- recovery journal, verification, and rollback;
- failpoint tests for interruption, temporary-write failure, and rename failure;
- brownfield file-preservation tests at the unit level.

## 5. Verification evidence at pause

Immediately before committing Task 6, these commands passed:

```text
go test -race ./internal/workspace -v
go test ./...
git diff --check
```

The workspace package passed approval, plan-root mismatch, namespace drift,
input fingerprint drift, temporary-write failure, rename failure, interrupted
apply, rollback, path/symlink boundary, discovery, idempotent init, and
brownfield-preservation cases.

Go was installed using Homebrew:

```text
/opt/homebrew/opt/go@1.26/bin/go
go version go1.26.8 darwin/arm64
```

Because the desktop sandbox may block the default Go cache directory, local
commands used:

```bash
export PATH="/opt/homebrew/opt/go@1.26/bin:$PATH"
export GOCACHE=/private/tmp/uawp-go-cache
export GOMODCACHE=/private/tmp/uawp-go-mod-cache
```

Remote GitHub CI has not run because no remote or pull request has been
created. Go 1.27 and non-macOS execution are therefore configured but not yet
observed.

## 6. Recorded implementation rulings

1. `.gitignore` existed from worktree setup, so Task 1 extended it instead of
   recreating it. If wrong, generated artifacts or worktree content could
   become trackable.
2. `templates/state/embed.go` was added because Go embed directives must live
   with the embedded template files. If wrong, packaged templates could drift
   from repository templates.
3. Task 6 uses a narrow stage/index failpoint hook instead of a broad injected
   filesystem interface. This keeps the production API smaller while testing
   interruption stages. If wrong, an OS-specific filesystem failure might not
   be represented by the current failpoints.

Do not silently reverse these rulings. If changing one, record a new ruling or
ADR and add a failing test first.

## 7. Exact next task

Start **Task 7 — Add status and doctor reports**. No Task 7 production or test
file has been created.

Task 7 must create:

```text
internal/workspace/status.go
internal/workspace/status_test.go
```

It consumes discovery and Core validators, and produces read-only:

```go
workspace.Status(Root) StatusReport
workspace.Doctor(Root) DoctorReport
```

Required finding codes:

```text
READY
UNINITIALIZED
ACTIVE_OWNER
UNKNOWN_NAMESPACE
INVALID_MANIFEST
UNSUPPORTED_VERSION
INVALID_OWNERSHIP
INCOMPLETE_STATE
RECOVERY_REQUIRED
```

Use TDD exactly:

1. Generate/read the Task 7 brief.
2. Write the report matrix tests first.
3. Run them and observe failure due to missing report APIs.
4. Implement the smallest read-only report behavior.
5. Snapshot paths, modes, sizes, and SHA-256 values before/after diagnostics to
   prove zero mutation.
6. Run the Task 7 tests and full suite.
7. Commit with `feat: report workspace status and integrity`.
8. Append RED, rulings, GREEN, commit, and verification to the execution ledger.

After Task 7, continue Tasks 8–10 in the approved plan without pausing between
tasks unless an irreversible/destructive action, security-sensitive action,
external side effect, or fundamentally broken plan requires Human Controller
input.

## 8. Resume checks

Run these before Task 7:

```bash
cd '/Users/rock-2024-macbook/Documents/UAWP v1.0/.worktrees/foundation-safe-init'
git branch --show-current
git status --short
git log -7 --oneline
export PATH="/opt/homebrew/opt/go@1.26/bin:$PATH"
export GOCACHE=/private/tmp/uawp-go-cache
export GOMODCACHE=/private/tmp/uawp-go-mod-cache
go test ./...
```

Expected branch: `feature/foundation-safe-init`.

Expected implementation HEAD before the handoff-document commit: `0317a42`.
The handoff itself will add one later documentation commit; inspect `git log`
rather than resetting to `0317a42`.

Expected working tree after the handoff commit: clean.

If any expectation differs, stop and diagnose before editing. Do not use
`git reset --hard`, do not recreate the branch, and do not work from `main`.

## 9. Remaining plan sequence

- Task 7: status and doctor reports.
- Task 8: preview-first `init`, `status`, and `doctor` CLI commands.
- Task 9: realistic greenfield/brownfield fixtures, preservation E2E, fuzzing.
- Task 10: user/protocol/diagnostic/package-boundary documentation and final
  Phase 1 gate.
- Final whole-branch review by a fresh reviewer.
- One fix pass for Critical/Important findings using RED→GREEN tests.
- Run `superpowers:finishing-a-development-branch`; do not merge or push
  without Human Controller authorization.
