# UAWP Product Engineering Kickoff and Handoff Specification

**Document status:** Approved

**Version:** 0.1

**Date:** 2026-09-21

**Product:** UAWP — Universal Agent Workspace Protocol

**Repository phase:** Phase 1 — technical decisions and implementation planning

## 1. Purpose of this document

This document is the authoritative handoff from the completed UAWP protocol
research and dry run into formal product engineering. It fixes the product
intent, architectural boundaries, v1.0 scope, release gates, development
stages, and acceptance criteria before technology selection or product code.

The next phase may refine implementation details, but it must not silently
weaken the invariants in this specification. Any proposed change to a core
invariant requires an explicit architecture decision and Human Controller
approval.

## 2. Product mission

UAWP will be an agent-neutral, installable, verifiable, upgradeable,
non-destructive, and extensible protocol and toolchain for shared project
workspaces.

It enables a developer to install UAWP in a new or existing project and use
compatible agents or fresh sessions to:

- reconstruct the effective workspace state without depending on chat history;
- determine whether substantive shared writes are safe;
- acquire and persist exclusive active-worker ownership;
- synchronize current state without forcing a handoff;
- record durable decisions and milestone checkpoints;
- pause, release ownership, and hand work to another worker safely;
- recover from stale ownership only with human arbitration; and
- add support for future agents through adapters rather than Core changes.

The mature product must be suitable for installation from a public GitHub
repository, open-source collaboration, versioned releases, upgrades, and
third-party adapter development.

## 3. Intended users and success definition

The primary user is a developer or small team using multiple coding agents or
multiple sessions against one local project. The user must not need the
original UAWP research conversation to operate the product correctly.

UAWP succeeds when a first-time user can:

1. obtain and install a versioned release;
2. preview initialization against either a greenfield or brownfield project;
3. understand every proposed filesystem change before applying it;
4. initialize without silent loss or replacement of project-owned content;
5. resume, acquire, work, sync, checkpoint, pause, and hand off by following
   stable protocol operations;
6. diagnose integrity or compatibility problems;
7. upgrade or uninstall without damaging non-UAWP content; and
8. add a new agent adapter using documented contracts and conformance tests.

## 4. Evidence and validated baseline

The preceding dry run completed TEST-01 through TEST-10D-2 and exercised:

- cross-session state recovery;
- cross-agent handoff;
- context lag and synchronization;
- ownership conflict detection;
- read-only behavior for a non-owner;
- stale ACTIVE recovery after Human Controller confirmation; and
- normal release followed by acquisition by a new worker.

The dry run is evidence, not the production repository. Experimental files
must not be copied mechanically into the product. The product must encode the
validated semantics as schemas, operations, tests, and user documentation.

### 4.1 Stable workspace model

| Concern | Meaning | Production location/owner |
|---|---|---|
| HOW | Workspace operating instructions | Protocol plus adapter integration |
| NOW | Current effective workspace state | `.uawp/CONTEXT.md` |
| WHO | Active worker and ownership state | `.uawp/ACTIVE_WORKER.md` |
| WHY | Durable decisions | `.uawp/DECISIONS.md` |
| WHAT | Project artifacts | Project-owned files |
| HISTORY | Milestone snapshots | `.uawp/checkpoints/` |

The identity of UAWP state is determined by its full path and namespace, not
by a filename alone. A project-level `context.md` and
`.uawp/CONTEXT.md` are different objects with different owners.

### 4.2 Stable worker lifecycle

```text
RESUME
  -> CHECK OWNERSHIP
  -> ACQUIRE
  -> WORK
  -> PERSIST
  -> CONTEXT SYNC
  -> CHECKPOINT? (milestone only)
  -> PAUSE & HANDOFF
  -> RELEASE
```

Normal release follows this invariant:

```text
Save -> Context Sync -> Validate -> Release -> No further shared writes
```

## 5. Normative language

The terms **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT**, and **MAY** are
normative requirements. A v1.0 release must satisfy every MUST and MUST NOT.

## 6. Core invariants

### 6.1 Agent-neutral Core

The Core MUST use vendor-neutral concepts: Agent, Worker, Agent Session,
Active Worker, Human Controller, Workspace, Workspace State, Ownership Claim,
Artifact, Decision, and Checkpoint.

The Core MUST NOT depend on a vendor command, a vendor-specific instruction
filename, or a vendor's auto-loading behavior. Vendor behavior belongs only
in adapters. Adding an agent SHOULD add an adapter and conformance fixtures,
not require modification of Core protocol semantics.

### 6.2 Namespace ownership

UAWP-owned runtime state MUST live under the project-root `.uawp/`
namespace. The minimum initialized state is expected to include:

```text
.uawp/
├── manifest.json
├── CONTEXT.md
├── ACTIVE_WORKER.md
├── DECISIONS.md
└── checkpoints/
```

The manifest MUST identify the namespace as UAWP-owned and record a schema or
protocol version sufficient for compatibility and migration checks.

If `.uawp/` already exists without recognizable UAWP ownership metadata,
initialization MUST stop and report a namespace collision. It MUST NOT adopt,
delete, or overwrite the directory.

### 6.3 Ownership of project and agent-native files

Everything outside `.uawp/` is presumed project-owned unless explicitly
created and tracked by a documented UAWP integration transaction.

Agent-native files such as `AGENTS.md`, `CLAUDE.md`, or a future vendor entry
file are not owned by UAWP. UAWP MAY integrate with them through a verified
adapter, but MUST preserve content outside its precisely identified managed
region. UAWP MUST NOT rename, replace, or rewrite an agent-native file as a
whole merely to install its instructions.

### 6.4 Non-destructive integration

Every mutating install, upgrade, repair, adapter integration, and uninstall
operation MUST follow:

```text
Discover -> Preflight -> Plan -> Preview -> Human approval -> Apply -> Verify
```

No mutating command may treat silence or a default timeout as approval.
Uncertain safety MUST produce a stop with an actionable report. There MUST be
no force flag in v1.0 that bypasses preservation guarantees.

Before applying an approved plan, the tool MUST detect whether relevant input
files changed since preview. If they changed, it MUST invalidate the plan and
require a fresh preview rather than applying a stale patch.

### 6.5 Single Active Worker

One workspace MUST have at most one worker with substantive/shared write
authority. Other workers MAY inspect the workspace read-only. A worker MUST
verify ownership immediately before shared writes and MUST stop if another
valid ACTIVE claim exists.

Ownership is persistent workspace state, not session memory. The v1.0 status
domain is:

- `ACTIVE`: the identified worker owns substantive/shared writes.
- `RELEASED`: no active owner; the identified worker is the last owner for
  audit continuity.

Ownership timestamps MUST use ISO 8601 with an explicit timezone, for example
`2026-09-21T22:30:00+08:00`. An ownership record MUST preserve acquisition
and, when released, release time.

### 6.6 Stale ownership and the Human Controller

An ACTIVE claim does not become stale merely because time passes. v1.0 MUST
NOT use a heartbeat, TTL, lease expiry, or automatic liveness inference to
release ownership.

If an apparent owner crashes or disappears, a new worker MUST remain blocked
from substantive/shared writes. Only an explicit Human Controller decision
may authorize force release of the stale claim. The recovery event MUST be
auditable before a new worker acquires ownership.

### 6.7 Context, decisions, and checkpoints

Context sync and handoff are independent operations. A worker MUST be able to
sync `.uawp/CONTEXT.md` while retaining ownership.

Durable decisions belong in `.uawp/DECISIONS.md`; transient observations do
not. Checkpoints are milestone-driven, not session-driven, and MUST NOT be
created automatically for every pause or handoff.

## 7. Product architecture

UAWP is divided into contracts with one-directional dependencies:

```text
CLI / user interface
        |
Application operations and transaction planner
        |
Agent-neutral Core domain and protocol
        |
Filesystem and serialization ports

Adapters -> verified vendor capability records -> integration plans
Prompts  -> Core operation semantics (never vendor behavior)
```

### 7.1 Core domain

The Core defines state schemas, lifecycle transitions, invariants, operation
results, and validation errors. It does not read vendor documentation or know
native filenames. It SHOULD remain usable by a future non-CLI frontend.

### 7.2 Application operations

Application services orchestrate init, status, doctor, upgrade, uninstall,
ownership, sync, checkpoint, and handoff. All filesystem mutations are modeled
as a plan that can be previewed, approved, applied, and verified.

### 7.3 Filesystem safety and transactions

The filesystem layer is responsible for path validation, snapshot/fingerprint
checks, atomic replacement where supported, rollback or recovery reporting,
and exact ownership boundaries. It MUST reject path traversal and symlink
escapes from the selected workspace root.

A failed apply MUST NOT be reported as success. The tool MUST either restore
the pre-apply state or leave an explicit recovery record that identifies
completed and incomplete actions without guessing that rollback succeeded.

### 7.4 Adapter framework

Each adapter is a declarative capability record plus narrowly scoped planning
logic. Before release, every adapter MUST cite current official vendor
documentation and record:

- vendor and agent name;
- official documentation URL;
- verification date;
- native instruction entry point;
- auto-load behavior;
- scope and precedence;
- import/reference support;
- chosen integration mode;
- validation status and supported product versions, when knowable.

Memory, community posts, or behavior inferred from another agent are not
sufficient release evidence. When official behavior cannot be verified, the
adapter MUST remain experimental and MUST NOT be advertised as v1.0-supported.

### 7.5 Adapter integration modes

Adapters MUST select the least invasive supported mode in this order:

1. `DIRECT`: the agent can consume the canonical UAWP instruction/state entry
   without modifying an agent-native file.
2. `IMPORT`: a minimal, officially supported reference is added to the native
   entry file.
3. `MANAGED_BLOCK`: UAWP maintains a uniquely marked region in the native file.

`MANAGED_BLOCK` MUST guarantee:

- markers are unique, detectable, and versioned where migration requires it;
- repeated init does not duplicate the block;
- content outside the markers is byte-for-byte preserved unless the approved
  plan explicitly describes a required newline-boundary normalization;
- missing, nested, reordered, or duplicated markers stop mutation and produce
  a repair report;
- upgrade changes only the owned block;
- uninstall removes only the owned block and preserves surrounding content;
- user edits inside the block are detected and surfaced before replacement.

### 7.6 Prompt Library

The v1.0 Prompt Library MUST include at least:

- `RESUME_WORK`: rebuild effective session context and check ownership;
- `CONTEXT_SYNC`: persist the current effective state without releasing;
- `CREATE_CHECKPOINT`: record a justified milestone snapshot;
- `PAUSE_AND_HANDOFF`: save, sync, validate, communicate handoff state, and
  release ownership as the final shared write.

Prompts MUST be agent-neutral, concise, versioned, and testable against Core
semantics. They MUST reference the persistent protocol rather than duplicate
the full protocol. Adapter-specific glue MUST NOT leak into canonical prompts.

### 7.7 CLI surface

The initial public CLI is expected to cover:

```text
uawp init
uawp status
uawp doctor
uawp upgrade
uawp uninstall
```

Ownership, sync, checkpoint, and handoff command shape will be fixed during
implementation planning. Every mutating command MUST support a clear preview
and explicit approval boundary. Non-interactive automation MUST require an
explicit previously generated plan or equally auditable consent mechanism; it
must never convert a destructive ambiguity into an implicit yes.

The programming language, packaging system, configuration library, and CLI
framework are deliberately undecided at this gate. They will be chosen after
this specification is approved, using portability, atomic filesystem support,
single-binary or simple installation, testability, and release maintenance as
decision criteria.

## 8. State and compatibility model

State formats MUST be independently versioned from the CLI release when their
compatibility can diverge. The manifest MUST provide enough information to:

- reject unsupported future formats safely;
- perform explicit forward migrations;
- prevent an older client from silently corrupting newer state;
- make repeated migrations idempotent; and
- retain evidence of the version transition.

Upgrade MUST preview all state and integration changes. Downgrade is not a
v1.0 promise; attempted incompatible downgrade MUST fail safely with guidance.

## 9. Scope

### 9.1 Required for v1.0

- normative protocol and versioned state schemas;
- `.uawp/` namespace initialization and ownership manifest;
- greenfield and brownfield preflight, preview, apply, and verification;
- Single Active Worker acquisition, release, and read-only conflict behavior;
- Human Controller-authorized stale-claim recovery;
- context sync, durable decisions, milestone checkpoints, and handoff;
- the four canonical operation prompts;
- adapter contract and at least the explicitly selected, officially verified
  launch adapters;
- DIRECT, IMPORT, and MANAGED_BLOCK framework support;
- status and doctor diagnostics;
- idempotent init, upgrade, uninstall, and reinstall behavior;
- package installation and reproducible release instructions;
- user quick start, protocol reference, adapter authoring guide, contribution
  guide, security/safety model, changelog, and license;
- automated unit, integration, conformance, and end-to-end safety tests.

The exact launch-adapter list is an approval-time product choice. No agent is
considered supported until its official behavior is verified and its adapter
passes conformance tests.

### 9.2 Explicitly out of scope for v1.0

- cloud control plane or remote state synchronization;
- team dashboard;
- heartbeat, TTL, lease service, or automatic owner liveness detection;
- distributed locking across filesystems or machines;
- multiple simultaneous substantive writers or automatic merge;
- enterprise RBAC;
- hosted agent marketplace;
- automatic modification based on undocumented vendor behavior;
- arbitrary force-overwrite or force-install escape hatches.

## 10. Public behavior requirements

### 10.1 Initialization

Greenfield initialization creates only approved UAWP-owned state and verified
adapter integrations. Brownfield initialization reports detected project and
native files, namespace status, adapter evidence, exact proposed edits, and
verification steps before approval.

Running init twice against an unchanged workspace MUST result in no additional
changes. A project-owned root `context.md` MUST remain untouched.

### 10.2 Status and doctor

`status` reports protocol/state versions, ownership, configured adapters, and
whether the workspace is ready for resume or blocked. `doctor` performs deeper
integrity and compatibility checks without modifying files by default.

Diagnostics MUST distinguish a normal released state, an active owner,
malformed state, unknown namespace ownership, damaged managed markers,
unsupported versions, and stale-claim suspicion requiring human action.

### 10.3 Upgrade

Upgrade MUST be version-aware, previewable, idempotent, and tested from every
supported prior release. It MUST preserve project content and detect drift in
managed integrations. No migration may assume exclusive ownership merely
because the CLI process is running.

### 10.4 Uninstall

Uninstall MUST remove only UAWP-owned integration content approved in the
plan. Runtime state containing user history MUST require a distinct explicit
choice from removing adapter glue. Default uninstall SHOULD preserve `.uawp/`
state or create a recoverable export; final behavior will be fixed in the
implementation plan and documented before code.

### 10.5 Exit behavior and messages

Commands MUST use stable exit categories for success, unsafe-to-proceed,
invalid workspace/state, approval required, compatibility failure, and
unexpected internal failure. Human-facing messages MUST explain what was
observed, what was or was not changed, and the safest next action.

## 11. Testing strategy

Testing is a product boundary, not a final cleanup phase.

### 11.1 Test layers

- **Schema and domain tests:** valid/invalid state, transitions, timestamps,
  ownership invariants, and migration compatibility.
- **Property and safety tests:** repeated operations, arbitrary pre-existing
  content, marker drift, path handling, and preservation invariants.
- **Adapter conformance tests:** declared vendor behavior, selected mode,
  generated integration, idempotency, upgrade, and uninstall.
- **Filesystem integration tests:** atomicity, permissions, line endings,
  Unicode filenames/content, symlinks, interrupted writes, and concurrent
  modification between preview and apply.
- **End-to-end fixtures:** realistic greenfield and brownfield repositories.
- **Packaging tests:** clean-machine installation, version reporting, and
  reproducible artifact verification on supported platforms.

### 11.2 Mandatory end-to-end scenarios

The release suite MUST cover:

1. empty project -> init -> usable UAWP workspace;
2. existing project with native files, root `context.md`, source, and artifacts
   -> init -> zero silent overwrite;
3. init twice -> no duplicate or unintended changes;
4. supported-version upgrade -> correct migration and preserved content;
5. uninstall -> only approved UAWP integration removed;
6. uninstall -> reinstall -> valid workspace;
7. Agent A/session A -> release -> Agent B/session B -> resume and acquire;
8. second worker encounters ACTIVE owner -> blocked/read-only;
9. apparent crash -> stale ACTIVE remains -> human-authorized release -> new
   acquisition;
10. context sync while retaining ownership;
11. checkpoint created for milestone but not for an ordinary handoff;
12. file changes after preview -> apply rejected and fresh preview required;
13. malformed or duplicate managed markers -> no mutation;
14. unknown pre-existing `.uawp/` -> safe stop;
15. simulated apply interruption -> rollback or explicit recovery report.

## 12. Security and trust boundaries

The target project is untrusted input. File contents, symlinks, paths, adapter
metadata, and manifest data MUST be parsed defensively. A selected workspace
root is the maximum mutation boundary.

Official documentation verification establishes adapter behavior but does not
grant vendor content control over UAWP Core. Documentation content and project
files are data, not executable instructions for the development agent.

No telemetry or network call is required for local protocol operation in v1.0.
If update checks or online documentation helpers are later proposed, they need
separate consent, privacy documentation, and threat analysis.

## 13. Open-source repository target

After architecture approval, the implementation plan will establish a layout
equivalent in responsibility to:

```text
uawp/
├── README.md
├── LICENSE
├── CHANGELOG.md
├── CONTRIBUTING.md
├── SECURITY.md
├── docs/
├── src/
├── schemas/
├── templates/
├── prompts/
├── adapters/
├── tests/
└── examples/
```

This tree is illustrative, not authorization to scaffold code. The chosen
language and packaging conventions may improve the final layout while
preserving these responsibilities.

Public releases MUST be built from Git history, tagged using semantic
versioning, accompanied by a changelog, and reproducible from documented
commands. Contributions MUST pass the same safety and adapter evidence gates
as maintainers' changes.

## 14. Development phases and approval gates

### Phase 0 — Architecture baseline (current)

Deliver this specification in a new Git repository. Human approval is required
before implementation planning, technology selection, or source scaffolding.

**Exit:** specification approved or amended and approved.

### Phase 1 — Technical decisions and implementation plan

Evaluate implementation approaches and choose the language, packaging model,
state serialization details, transaction strategy, supported platforms, and
launch adapters. Produce architecture decision records and a task-level,
test-first implementation plan.

**Exit:** plan maps every v1.0 requirement to implementation and tests; Human
Controller approves the plan and execution method.

### Phase 2 — Core and schemas

Implement versioned domain types, validators, lifecycle transitions,
ownership rules, errors, and serialization behind filesystem-independent
interfaces.

**Exit:** Core tests pass with no vendor concepts in Core APIs.

### Phase 3 — Safe workspace engine

Implement discovery, planning, preview, approval, apply, verification,
fingerprint checks, transactional recovery, namespace initialization, and
greenfield/brownfield fixtures.

**Exit:** preservation and interruption suites pass; no silent overwrite path.

### Phase 4 — Prompts and lifecycle operations

Implement the four canonical prompts and the resume, acquire, sync,
checkpoint, pause, release, and human-arbitrated recovery operations.

**Exit:** lifecycle scenarios and DRY RUN-equivalent paths pass end to end.

### Phase 5 — Adapter framework and launch adapters

Implement adapter contracts and DIRECT, IMPORT, and MANAGED_BLOCK behavior.
Research launch agents from official current documentation, record evidence,
and add conformance fixtures.

**Exit:** each advertised adapter is verified, documented, and conformant.

### Phase 6 — CLI and lifecycle management

Expose init, status, doctor, upgrade, uninstall, and approved lifecycle
commands with stable messages and exit behavior.

**Exit:** clean-machine and realistic project workflows pass.

### Phase 7 — Open-source hardening and release candidate

Complete documentation, licensing, contribution/security policy, packaging,
CI, supported-platform matrix, migration fixtures, and release automation.

**Exit:** all Release Gates pass against a tagged release candidate.

### Phase 8 — v1.0 release

Publish signed or checksummed artifacts as supported by the chosen ecosystem,
tag the Git release, publish the changelog and compatibility matrix, then run
post-release installation verification from public artifacts.

**Exit:** public artifact reproduces the release-candidate results.

## 15. v1.0 Release Gates

All gates are mandatory:

- [ ] Core domain and canonical prompts are agent-neutral.
- [ ] All UAWP-owned runtime state is namespace-isolated under `.uawp/`.
- [ ] Unknown `.uawp/` collisions stop without mutation.
- [ ] No path silently overwrites, deletes, or rewrites project-owned content.
- [ ] Every mutation has preflight, exact preview, explicit approval, apply,
      and verification.
- [ ] Preview/apply drift is detected.
- [ ] Init, managed integration, migrations, and uninstall are idempotent.
- [ ] Single Active Worker is persistently represented and enforced.
- [ ] Conflicting workers stop substantive/shared writes.
- [ ] Stale ACTIVE recovery requires recorded Human Controller authorization.
- [ ] No automatic heartbeat, TTL, lease expiry, or liveness release exists.
- [ ] Cross-session recovery and cross-agent handoff pass end to end.
- [ ] Context sync works independently of handoff.
- [ ] Checkpoints are milestone-only.
- [ ] DIRECT, IMPORT, and MANAGED_BLOCK contracts are tested.
- [ ] Every advertised adapter has current official-document evidence.
- [ ] Greenfield and representative brownfield suites pass.
- [ ] Upgrade, uninstall, reinstall, drift, malformed marker, and interrupted
      apply suites pass.
- [ ] Security review covers paths, symlinks, untrusted contents, mutation
      boundaries, and recovery behavior.
- [ ] Quick start, protocol, adapter authoring, upgrade, uninstall, recovery,
      and troubleshooting documentation are complete.
- [ ] Installation artifacts are reproducible, versioned, and tested from a
      clean environment on every supported platform.
- [ ] Repository has license, contribution policy, security policy, changelog,
      semantic version tag, and passing CI.

Any known silent-overwrite risk is an automatic release blocker.

## 16. Acceptance criteria for the engineering handoff

This kickoff phase is accepted when:

1. the repository is initialized on `main` and this document is committed;
2. the document contains no unresolved placeholder or hidden technology choice;
3. product mission, users, invariants, architecture, v1.0 scope, exclusions,
   safety model, testing, stages, and Release Gates are explicit;
4. every baseline required by the Human Controller is represented;
5. no product source, dependency manifest, adapter claim, or premature
   production skeleton exists before approval; and
6. the Human Controller explicitly approves this specification or requests
   revisions.

## 17. Decisions deferred to Phase 1

The following are deliberately deferred, not forgotten:

- implementation language and minimum runtime;
- package/distribution ecosystem and supported operating systems;
- exact machine-readable schema formats and Markdown front matter, if any;
- transaction journal and rollback implementation;
- exact CLI syntax for ownership and prompt operations;
- launch adapter list, after official-document verification;
- license selection;
- uninstall default for retained `.uawp/` history;
- release signing/checksum mechanism.

Each deferred decision must be resolved before its dependent implementation
task and recorded in an architecture decision record. None may weaken the Core
invariants or Release Gates without explicit Human Controller approval.

## 18. Approval record

The Human Controller approved this specification on 2026-09-21. Product
implementation remains gated on approval of the Phase 1 technical decisions,
implementation plan, and execution method.
