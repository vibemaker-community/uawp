# UAWP Human-Friendly CLI, Agent Automation, and Session Fencing Design

**Document status:** Approved

**Version:** 0.1

**Date:** 2026-09-22

**Product phase:** Plan 4.5

**Parent specifications:**

- `docs/superpowers/specs/2026-09-21-uawp-product-engineering-design.md`
- `docs/superpowers/specs/2026-09-21-ownership-lifecycle-prompt-library-design.md`
- `docs/superpowers/specs/2026-09-22-upgrade-uninstall-transaction-recovery-design.md`

## 1. Purpose

Plan 4.5 makes routine UAWP operations concise for people and deterministic
for agents without weakening preview, approval, drift detection, transaction,
preservation, or Human Controller guarantees.

It provides three compatible surfaces over the same Core operations:

1. a default human-friendly surface;
2. a stable non-interactive agent automation surface; and
3. the existing explicit flag-and-plan-ID surface as expert mode.

The design also closes one ownership ambiguity discovered during review. A
long-lived Worker identity may be used by more than one conversation. Therefore
an ACTIVE ownership claim must identify one current Session as well as the
Worker. This is session fencing, not multi-session collaboration: one physical
Workspace still permits only one ACTIVE writing Session at a time.

## 2. Confirmed product decisions

The following decisions are normative:

- UAWP remains Agent-neutral.
- Workspace-owned UAWP state remains under `.uawp/`.
- Existing-project integration remains non-destructive.
- Agent-native files remain provider- or project-owned.
- Adapter behavior remains DIRECT, IMPORT, or MANAGED_BLOCK and continues to
  require current official-documentation evidence.
- Human-friendly commands default to the current directory when `--workspace`
  is omitted.
- An interactive mutation shows a concise preview and asks for confirmation in
  the same invocation.
- Confirmation applies the exact previewed immutable plan; it does not bypass
  drift checks or transaction handling.
- Non-interactive execution never assumes consent and never waits for a prompt.
- Existing explicit commands and approval-token workflows remain supported.
- Worker ID is a long-lived machine identity. Display name is only a human
  label and never participates in authorization or uniqueness.
- Session ID identifies one conversation or execution session.
- One physical Workspace permits exactly one ACTIVE writing Session.
- Multiple conversations may refer to the same Workspace, but all except the
  ACTIVE Session are read-only, paused, completed, or waiting.
- Same-directory parallel writing is outside v1 scope, regardless of whether
  the writers use the same Agent or different Agents.
- Git is not required for a target Workspace. Git worktrees are only an
  optional isolation mechanism for repositories already managed by Git.

## 3. Goals and non-goals

### 3.1 Goals

Plan 4.5 must:

1. make `init`, `resume`, `sync`, `checkpoint`, and `handoff` understandable
   without requiring users to copy opaque plan identifiers;
2. preserve a deterministic, scriptable, non-interactive contract for agents;
3. retain every existing expert command and explicit option unless a separately
   documented incompatibility is approved;
4. create and reuse a stable local Worker identity without repeatedly asking in
   new projects or conversations;
5. fence ACTIVE ownership to one Session so two conversations using the same
   Worker ID cannot both perform protocol-authorized writes;
6. preserve Human Controller arbitration for every stale ACTIVE claim,
   including a stale Session belonging to the same Worker;
7. make errors actionable in human text and stable in structured output; and
8. keep Core policy independent from terminal detection and presentation.

### 3.2 Non-goals

Plan 4.5 does not:

- support multiple ACTIVE writers in one physical Workspace;
- implement file-range or directory-range ownership;
- infer that work is safe to parallelize;
- provide distributed locking, networking, or cloud coordination;
- prevent a non-compliant process from writing directly to the filesystem;
- require Git, create Git repositories in target Workspaces, or automatically
  create Git worktrees;
- infer a user's real name, inspect chat history for identity, or dynamically
  construct a personal display name;
- make display names unique or security-sensitive;
- merge independently edited Word, Excel, image, or other binary files;
- change adapter ownership or native-file preservation rules; or
- replace Plan 5 packaging and public-release work.

## 4. Alternatives considered

### 4.1 Global `--mode human|agent|expert`

Rejected. It adds a concept users must understand and creates invalid
combinations between a selected mode and flags. The surface is instead derived
from explicit machine-oriented flags and terminal capabilities.

### 4.2 Separate human and agent executables

Rejected. Separate binaries would duplicate command semantics, versioning, and
testing. One executable must route all surfaces into the same application
operations.

### 4.3 One command set with capability-driven presentation

Selected. Interactive text execution receives concise prompts. Structured or
non-interactive execution never prompts. Explicit expert flags retain their
current meaning.

### 4.4 Dynamic personal-name detection

Rejected. A display name is not part of machine identity, so operating-system
or Agent-account detection adds privacy and portability costs without improving
correctness. The user supplies an arbitrary human-readable label once.

### 4.5 Same-directory parallel Sessions with a warning

Rejected for v1. A warning cannot enforce path isolation, protect shared
metadata, or prevent indirect conflicts. Supporting this would change UAWP
from a single-writer protocol into a multi-writer coordination system.

### 4.6 Isolated parallel task directories

Permitted but external to the Core. Users may independently choose Git
worktrees, cloned directories, application-native collaboration, or another
isolation system. Every physical directory is treated as a separate UAWP
Workspace and still has one ACTIVE Session.

## 5. Operating-surface selection

There is no persistent global mode switch.

### 5.1 Human-friendly surface

The human-friendly surface is selected only when:

- output format is text;
- standard input and the confirmation terminal are interactive;
- no explicit approval token was supplied; and
- no non-interactive option was supplied.

It may ask for missing presentation-level input, show a concise preview, and
ask `Continue? [y/N]`. EOF, empty input, or any response other than the
documented affirmative responses means no.

### 5.2 Agent automation surface

Agent automation is explicitly non-interactive. It is selected by structured
output or an explicit non-interactive request. It must:

- accept all required intent and actor data explicitly;
- write exactly one structured result to standard output;
- write diagnostics to standard error without corrupting structured output;
- never prompt, open an editor, or depend on terminal state;
- expose the immutable plan and approval requirement;
- retain stable schema, finding codes, and exit codes; and
- permit a caller to preview and later apply the exact approved plan.

Structured output implies non-interactive behavior. Redirected input or output
must never cause an implicit yes.

### 5.3 Expert surface

Existing explicit flags remain the expert surface. Examples include explicit
`--workspace`, `--worker-id`, `--agent`, `--context-file`, `--format`, and
`--approve` usage.

Expert mode is not selected by a global switch. Supplying expert inputs simply
avoids corresponding defaults or prompts. Existing automation that previews,
captures a plan ID, and reruns with `--approve` must remain valid.

## 6. Local Worker identity

### 6.1 Identity record

A local Worker profile contains:

- an opaque, randomly generated Worker ID;
- a user-supplied display name;
- a local profile identifier; and
- format-version metadata.

The Worker ID is the durable identity used by ownership. It must be generated
with sufficient randomness and must not be derived from user name, device name,
Agent name, path, or timestamp alone.

The display name is only for people. It may be any non-empty Unicode string
after trimming, subject to these validation rules:

- maximum 100 Unicode code points;
- no newline or control characters; and
- safe escaping in text, Markdown, and JSON output.

Display names may be duplicated and may have no semantic meaning. They must
never be used in equality, authorization, token generation, plan identity, or
ownership validation.

### 6.2 First-use interaction

When an interactive ownership-bearing workflow has no selected Worker profile,
UAWP asks once:

```text
Name this work identity.
Suggested format: Zhang San's Codex
Work identity name:
```

The example is static guidance, not a default value. UAWP does not detect a
personal name or Agent provider. After valid input, it previews creation of the
local profile and does not ask again while that profile remains available.

Creating `.uawp/` does not itself require a Worker identity. Identity is needed
when a workflow evaluates or changes ownership.

### 6.3 Storage and profiles

Local profiles are UAWP application configuration, not Workspace state. They
must be stored under the operating system's standard per-user configuration
directory with user-only permissions where supported. They must not be written
to the target project's agent-native files.

The local configuration may hold multiple profiles. One profile may be the
human-friendly default. Agent automation must explicitly select a profile or
provide its Worker ID; it must not silently inherit an ambiguous human default.
This prevents different Agent clients on the same operating-system account from
accidentally sharing one Worker identity.

Profile selection and profile management must be possible without modifying a
Workspace. Deleting a local profile does not release any Workspace claim and
must warn that Human Controller recovery may be required.

### 6.4 Security boundary

Worker ID is a protocol coordination identifier, not an authentication secret.
UAWP does not claim to stop a malicious local process from impersonating an ID
or writing directly to files. It guarantees deterministic behavior for
compliant UAWP clients and records actor claims for audit.

## 7. Session fencing and ownership

### 7.1 Identity levels

Ownership is evaluated with three values:

```text
Worker ID + Session ID + Ownership Generation
```

- Worker ID identifies the long-lived work identity.
- Session ID identifies one conversation or execution session.
- Ownership Generation is a monotonically increasing fencing value for that
  Workspace.

Changing a display name changes none of these values.

### 7.2 Single ACTIVE Session invariant

At most one tuple may hold ACTIVE ownership for a physical Workspace:

```text
(workerID, sessionID, generation)
```

All ownership-protected mutations must present and revalidate the complete
ACTIVE tuple immediately before apply. Matching Worker ID alone is
insufficient.

A second Session under the same Worker ID is treated as a different execution
session. It may inspect and plan, but it cannot sync, checkpoint, hand off,
release, repair, upgrade, or otherwise perform an ownership-protected mutation
while another Session is ACTIVE.

### 7.3 Normal Session transition

The normal transition is:

```text
Session A ACTIVE
  -> sync if needed
  -> optional checkpoint
  -> pause and handoff / release
  -> ownership RELEASED
  -> Session B resume and acquire
  -> Session B ACTIVE with a new generation
```

Every successful acquire creates a new ACTIVE Session binding and increments
the Workspace ownership generation. A former Session's tuple becomes invalid
even if its process later resumes.

### 7.4 Abnormal Session transition

Elapsed time, missing heartbeat, closed chat, or process disappearance does not
prove that an ACTIVE Session is stale. If the active Session cannot release
normally, a Human Controller must use the existing stale-claim recovery path.
This rule applies even when the replacement Session has the same Worker ID.

Recovery releases the old tuple and records the arbitration. It never silently
binds the replacement Session. The replacement performs a separate acquire,
which increments the generation.

### 7.5 Read-only conversations

Additional Sessions may read Workspace and UAWP state, run read-only status or
resume inspection, and prepare plans. UAWP does not grant them shared write
ownership. A planned mutation must still pass actor and generation validation
at apply time.

### 7.6 Session identifier handling

Agent automation must supply a stable Session ID for the life of one
conversation or execution session. The identifier may be supplied by the Agent
host or generated through a UAWP session-start operation. It must not be stored
in shared Workspace instructions as a reusable credential.

Human-friendly execution may maintain a local session binding keyed by
canonical Workspace and selected local profile. Starting a replacement local
session must never silently take over an ACTIVE tuple.

## 8. State schema and migration impact

The currently implemented ownership record contains Worker ID but no Session
ID or generation. Plan 4.5 therefore includes one narrowly scoped Core and
state-schema increment in addition to presentation work.

The new exact state version must add:

- Session ID to ACTIVE ownership;
- ownership generation;
- validation rules for ACTIVE and RELEASED records; and
- migration and conformance fixtures.

Migration must use the Plan 4 registry and transaction guarantees. An ACTIVE
legacy ownership record blocks structural migration. The user must first
complete a normal handoff or release under the legacy behavior. A recognized
RELEASED record may migrate without inventing an active Session. Unknown,
ambiguous, malformed, or newer state fails closed.

The implementation plan must assign the exact target state version and update
the roadmap before code changes begin.

## 9. Human-friendly command behavior

### 9.1 Workspace default

When `--workspace` is absent, commands use the canonical current working
directory. The resolved path is always shown before a mutation. Existing
boundary and symlink validation remains unchanged.

### 9.2 Read-only commands

`status`, `doctor`, and a non-acquiring resume inspection run immediately and
do not ask for confirmation. Their concise output leads with current state,
ownership, blocking conditions, and the next safe action.

### 9.3 Mutating commands

An interactive mutation follows:

```text
Discover -> Validate -> Plan -> Concise preview -> Confirm
-> Revalidate actor and drift -> Apply transaction -> Verify
```

The confirmation applies the same in-memory plan shown to the user. A yes does
not regenerate intent, waive validation, or convert an unsafe plan into a safe
one. A no or EOF performs zero mutation and returns the documented cancellation
result.

### 9.4 `uawp init`

Bare `uawp init` targets the current directory, previews only UAWP-owned paths,
and asks once before creation. Existing `.uawp/` ownership checks and all
non-destructive rules remain unchanged.

### 9.5 `uawp resume`

Resume first reconstructs context, decisions, checkpoints, ownership, and
Session state without mutation.

- If the Workspace is RELEASED, the human surface may offer to acquire it for
  the selected local profile and current Session. Acquisition remains a
  separately previewed mutation within the same guided invocation.
- If the exact tuple is ACTIVE, resume reports that work may continue.
- If another Session or Worker is ACTIVE, resume remains read-only and explains
  normal handoff versus Human Controller recovery.
- It never infers staleness or silently takes over.

The expert resume command retains its existing read-only behavior.

### 9.6 `uawp sync`

Context sync requires complete replacement content. Human-friendly execution
may accept a file or standard input; it must not invent or summarize context.
Interactive omission produces concise guidance instead of silently writing an
empty document. The exact resulting document is previewed before confirmation.

### 9.7 `uawp checkpoint`

Human-friendly checkpoint creation requests only a human-readable Checkpoint
name and automatically generates a unique machine identifier. The generated ID
combines a timestamp, an ASCII-safe label summary, and a random suffix; it is
reported after creation but is not an ordinary-user input. Expert and Agent
automation retain optional `--milestone-id`; omission uses the same automatic
generation, preserved across preview and approval. The checkpoint remains
explicit, immutable, and never overwrites an existing path.

### 9.8 `uawp handoff`

Handoff requires complete final context and a purpose. It previews context
replacement followed by ownership release as one transaction. It does not
create an implicit checkpoint. After success, the former Session cannot perform
ownership-protected writes.

### 9.9 Advanced and exceptional commands

Adapter integration, stale recovery, upgrade, repair, uninstall, purge, and
transaction recovery retain explicit evidence and Human Controller inputs.
Human-friendly formatting may improve, but no concise prompt may conceal or
combine distinct exceptional operations.

## 10. Agent automation contract

### 10.1 Determinism

Agent automation must never rely on locale-sensitive prose, terminal prompts,
ambient editors, or inferred approval. It supplies Workspace, Worker profile or
Worker ID, Session ID, intent fields, and complete content explicitly.

### 10.2 Preview and approval

The first call returns a structured preview, immutable plan ID, mutation flag
set to false, and exit code 5 when approval is required. The apply call must
present the exact plan ID. Any bound-input, ownership, Session, generation, or
filesystem drift rejects approval before planned mutation.

### 10.3 Structured schema

The current output schema remains supported. Additions must be additive or
introduced under a new explicit schema version. Structured results must expose:

- command and canonical Workspace;
- Worker ID, Session ID, and ownership generation where relevant;
- current state classification;
- findings with stable codes and severity;
- exact planned changes;
- plan ID when approval is required;
- whether mutation occurred; and
- an actionable next step.

### 10.4 Input transport

Existing expert flags remain valid. Plan 4.5 may add a structured request input
transport, but it must map to the same typed application request and must not
create a second policy implementation. Secrets or arbitrary shell fragments
must not be accepted as data fields.

## 11. Approval and cancellation semantics

Interactive confirmation and explicit plan-ID approval are two presentations
of the same authorization boundary.

- Interactive yes authorizes only the displayed in-memory plan.
- Explicit `--approve` authorizes only the matching reconstructed plan.
- Cancellation performs no mutation and is not an error requiring recovery.
- Non-interactive absence of approval returns exit code 5.
- Invalid, expired-by-drift, or mismatched approval returns exit code 5 with no
  mutation.
- Unsafe discovery, invalid state, and internal failures retain distinct exit
  classifications.

No `--yes`, environment variable, configuration default, piped input, or
redirected stream may become a general bypass for Human Controller operations.
If a future convenience `--yes` is added for ordinary human mutations, it must
remain interactive-only and must not authorize arbitration, repair, purge, or
transaction recovery.

## 12. Errors and recovery presentation

Human text should lead with the outcome and one safe next action. It may hide
hashes and internal paths by default but must make detailed evidence available.

Structured output must preserve stable machine-readable findings. At minimum,
Plan 4.5 needs distinct codes for:

- identity configuration required;
- Session identifier required;
- same Worker but different ACTIVE Session;
- different ACTIVE Worker;
- stale claim requiring Human Controller decision;
- approval required;
- approval invalidated by drift;
- transaction recovery required;
- unsupported state version; and
- ambiguous or invalid local profile selection.

An error message must not recommend deleting `.uawp/`, editing ownership by
hand, or overwriting an agent-native file.

## 13. Compatibility requirements

Plan 4.5 must retain:

- existing command names and expert flags;
- JSON output schema version 1 fields required by current callers;
- exit codes 0, 2, 3, 4, 5, and 10 with their current broad meanings;
- immutable plan reconstruction and approval-token checks;
- transaction journals, backups, recovery evidence, and verification;
- non-destructive adapter installation and removal;
- configured-consumer tracking for shared native entry files;
- Human Controller-only stale recovery; and
- state-preserving uninstall and verified-export purge.

Where Session fencing makes an old ownership-bearing call incomplete, the call
must fail with a precise migration or Session requirement. It must not silently
reuse another Session. Compatibility means preserving safe workflows, not
preserving an unsafe ambiguity.

## 14. Documentation deliverables

Implementation must update or add:

1. the implementation roadmap and exact Plan 4.5 state-version impact;
2. a quick-start showing the five routine commands;
3. a human CLI reference;
4. an agent automation contract with schemas and exit codes;
5. identity/profile management documentation;
6. Session lifecycle and stale arbitration documentation;
7. migration notes for pre-Session state; and
8. examples for code and non-code Workspaces without implying Git is required.

## 15. Test strategy

Testing must cover unit, integration, end-to-end, and compatibility behavior.

### 15.1 Interaction tests

- TTY text mutation previews and accepts yes.
- TTY text mutation rejects no, empty input, EOF, and unrecognized responses.
- Redirected or structured execution never prompts.
- Current-directory default resolves and displays the canonical path.
- Explicit Workspace retains priority.
- Interactive confirmation applies the exact previewed plan.

### 15.2 Identity tests

- First ownership-bearing interactive use requests one display name.
- Arbitrary valid Unicode labels work and do not affect Worker ID.
- control characters, newline, empty value, and overlength value are rejected;
- later projects and conversations reuse the selected Worker profile;
- duplicate display names produce distinct Worker IDs;
- automation does not silently select an ambiguous profile; and
- deleting or changing a display label does not mutate Workspace ownership.

### 15.3 Session fencing tests

- only the exact ACTIVE tuple may perform protected writes;
- same Worker with a different Session is blocked;
- different Worker is blocked;
- a released Workspace can be acquired by a new Session;
- every acquire advances generation;
- an old Session cannot write after handoff and reacquire;
- drift between preview and apply invalidates approval;
- stale recovery requires Human Controller approval even for the same Worker;
- recovery releases but does not acquire; and
- multiple concurrent acquisition attempts result in at most one ACTIVE tuple.

### 15.4 Migration and regression tests

- recognized RELEASED legacy state migrates transactionally;
- ACTIVE legacy state blocks migration with a safe next action;
- newer or malformed state fails closed;
- all Plan 1–4 tests continue to pass;
- old expert preview/apply flows remain valid where Session identity is not
  relevant; and
- adapters and native files remain byte-for-byte preserved outside approved
  UAWP-owned regions.

## 16. Release gates

Plan 4.5 is complete only when:

1. the exact state migration and compatibility matrix are documented;
2. all three operating surfaces call the same typed application operations;
3. routine human workflows require no copied plan ID;
4. non-interactive runs never prompt or infer consent;
5. no two Sessions can pass protocol authorization as the ACTIVE writer;
6. stale ACTIVE replacement always requires Human Controller arbitration;
7. current-directory defaults never weaken boundary checks;
8. expert commands and stable exit classifications pass regression tests;
9. race, vet, fuzz/property tests selected by the implementation plan pass;
10. independent code review reports no unresolved P0 or P1 issue; and
11. the complete repository test suite passes from a clean checkout.

## 17. Implementation sequencing constraint

No implementation begins until this design is approved and a separate detailed
implementation plan is written and approved.

The implementation plan must split at least these concerns:

1. local identity profiles;
2. Session-aware Core ownership and migration;
3. application-layer surface selection and confirmation;
4. human-friendly command flows;
5. agent automation schemas;
6. compatibility and migration tests; and
7. user and automation documentation.

Each increment must preserve a buildable, testable repository and must not
weaken any prior preservation or recovery guarantee.
