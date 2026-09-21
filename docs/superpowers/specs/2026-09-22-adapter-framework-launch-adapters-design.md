# Adapter Framework and Launch Adapters Design

**Status:** Proposed for written review

**Date:** 2026-09-22

**Parent specification:**
`docs/superpowers/specs/2026-09-21-uawp-product-engineering-design.md`

**Launch adapters:** Codex, Claude Code, Tencent WorkBuddy

## 1. Goal

This phase gives UAWP a provider-neutral adapter framework and three officially
verified launch adapters. It makes the canonical UAWP instructions discoverable
by each selected agent without moving UAWP state out of `.uawp/`, taking
ownership of a project-owned native instruction file, or silently changing an
existing project's instruction-loading behavior.

The framework must handle more than the ideal empty-project case. For every
provider it resolves all documented combinations of native entry files,
precedence, co-loading, fallback behavior, version and environment constraints,
configuration overrides, entry shadowing, and later entry drift.

This phase remains subject to the product-wide discovery, preview, explicit
approval, drift revalidation, apply, and verification workflow.

## 2. Decisions added or corrected after design review

The following decisions supersede the earlier simplified launch-adapter table:

1. `.uawp/INSTRUCTIONS.md` is the single canonical, agent-neutral instruction
   source. Agent-native files contain only the minimum bridge needed to load or
   direct the provider to that source.
2. Native files do not synchronize with one another. In particular, UAWP does
   not copy arbitrary user content between `AGENTS.md`, `CLAUDE.md`, and
   `CODEBUDDY.md`.
3. Adapter selection is capability-driven. Providers need not use the same
   integration mode.
4. Tencent WorkBuddy is not advertised as supporting `IMPORT` merely because a
   rule can mention another path. Its initial integration is `MANAGED_BLOCK`.
5. Claude Code's direct `AGENTS.md` support is conditional, not unconditional.
   The resolver must account for `CLAUDE.md`, `CLAUDE.local.md`, user settings,
   product version, and environments where direct support is unavailable.
6. One bridge artifact may serve multiple adapters. UAWP records its configured
   consumers and removes the artifact only after the last consumer is removed.
7. Adapter health is dynamic. `status`, `doctor`, and `upgrade` must resolve the
   currently effective entry again instead of trusting the entry selected at
   installation time.
8. UAWP can report configured consumers, but it cannot infer whether an
   external agent process is currently reading a file or whether an
   unregistered third-party tool also consumes that file.

## 3. Scope

### 3.1 Included

- the provider-neutral adapter contract;
- official-document evidence and capability records;
- effective-entry resolution and diagnostics;
- `DIRECT`, `IMPORT`, and `MANAGED_BLOCK` framework behavior;
- Codex, Claude Code, and Tencent WorkBuddy adapters;
- shared bridge artifacts and consumer registration;
- installation, verification, adapter removal, and entry-drift planning;
- conformance fixtures covering documented file combinations;
- status and doctor reporting for adapter health;
- documentation for adapter authors.

### 3.2 Excluded

- synchronization of arbitrary user-authored native instructions;
- semantic merging of conflicting provider-specific instructions;
- changing user-wide provider configuration without a separate future design;
- claiming to detect live external agent processes;
- supporting undocumented provider behavior;
- generalized schema migration, interrupted-transaction repair, and complete
  product uninstall, which remain in the later upgrade/recovery phase;
- adapters beyond the three explicitly selected launch providers.

## 4. Architectural invariants

1. Core remains agent-neutral. Provider names, native filenames, and provider
   loading rules do not enter Core.
2. UAWP protocol state and shared instructions live only under `.uawp/`.
3. Agent-native files remain project/provider-owned. UAWP may own only a
   precisely identified bridge artifact or managed region created through an
   approved Adapter plan.
4. Existing native files are never silently overwritten, replaced, renamed, or
   semantically merged.
5. Every adapter claim is backed by current official vendor documentation,
   verification date, and a conformance fixture.
6. Unknown capability is not treated as supported capability. An uncertain
   route is reported as uncertain or experimental, not healthy.
7. A new native entry can shadow an older entry. File existence alone is not
   proof that the UAWP bridge is effective.
8. Native bridges are thin. They point to the canonical UAWP instructions and
   do not copy UAWP state, prompts, or lifecycle documents.
9. Native files never synchronize bidirectionally through UAWP.
10. Adapter removal and full UAWP uninstall are different operations.

## 5. Canonical instruction source

The canonical shared instruction source is:

```text
.uawp/INSTRUCTIONS.md
```

It describes how an agent enters the UAWP lifecycle, which state documents to
read, how ownership is respected, and where the canonical prompts live. It is
agent-neutral and MUST NOT mention a provider command or native filename.

The file is product-owned, versioned, fingerprinted, and upgraded under the
same preview-and-approval rules as other UAWP-owned files. It references rather
than duplicates the four canonical prompts:

- `RESUME_WORK`;
- `CONTEXT_SYNC`;
- `CREATE_CHECKPOINT`;
- `PAUSE_AND_HANDOFF`.

Native entry files contain only enough glue to cause this file to be loaded or
read. `.uawp/INSTRUCTIONS.md` is the authority when native glue and canonical
content disagree.

## 6. Provider-neutral adapter contract

Each adapter consists of a declarative capability/evidence record plus narrow
resolver and planning logic. At minimum it declares:

- provider and agent identity;
- official documentation URLs and verification date;
- verified product versions, when knowable;
- candidate native entries;
- entry selection and precedence rules;
- co-load rules;
- fallback rules;
- supported import syntax and limits;
- version and environment constraints;
- user or organization configuration overrides;
- shadowing and duplicate-load risks;
- supported integration modes;
- validation procedure;
- declared diagnostic and conformance fixtures.

The adapter receives a read-only workspace snapshot and capability evidence. It
returns an entry-resolution result and, when requested, an immutable change
plan. It does not write files directly.

## 7. Effective-entry resolver

Entry resolution is a first-class step:

```text
discover files and observable environment
  -> apply documented provider selection rules
  -> classify every candidate entry
  -> select the least-invasive deterministic route
  -> report uncertainty and semantic side effects
  -> create an immutable plan only when requested
```

Candidate entries use these states:

- `EFFECTIVE`: documented to load in the observed configuration;
- `CO_LOADED`: documented to load alongside another effective entry;
- `FALLBACK_EFFECTIVE`: effective only because a higher-priority entry is
  absent;
- `SHADOWED`: present but not loaded because another entry wins;
- `UNAVAILABLE`: documented as unsupported in the observed version or
  environment;
- `UNKNOWN`: evidence is insufficient to guarantee behavior;
- `MISSING`: the candidate does not exist.

The resolution result also carries a confidence level:

- `VERIFIED`: all facts required for the route are locally observable and
  match official evidence;
- `CONDITIONAL`: the route is documented but depends on a fact UAWP cannot
  observe reliably;
- `UNSUPPORTED`: no released route can be justified.

`CONDITIONAL` is never silently treated as `VERIFIED`. The plan must identify
the missing fact and present a deterministic alternative when one exists.

## 8. Integration modes

Adapters select the least-invasive verified mode in this order:

1. `DIRECT`: the provider loads the canonical UAWP entry without adding or
   changing an agent-native artifact.
2. `IMPORT`: an officially supported deterministic import or transclusion is
   added to an effective native entry.
3. `MANAGED_BLOCK`: UAWP inserts a uniquely marked bridge region in an
   effective native entry.

A sentence asking an agent to read another path is not an `IMPORT` unless the
provider documents deterministic import/transclusion semantics.

Every managed block has a stable artifact ID, schema version, canonical target,
and visible configured consumers. Its content outside the markers remains
byte-for-byte unchanged except for an explicitly previewed newline-boundary
normalization.

Missing, duplicated, nested, reordered, or malformed markers block mutation.
User edits inside the block cause drift and require a reviewed repair plan.

## 9. Integration artifact registry

`.uawp/manifest.json` records native integration artifacts separately from
canonical protocol state. A conceptual record contains:

```json
{
  "id": "uawp-entry-agents-v1",
  "path": "AGENTS.md",
  "kind": "MANAGED_BLOCK",
  "target": ".uawp/INSTRUCTIONS.md",
  "consumers": ["codex", "workbuddy"],
  "createdFile": false,
  "artifactSHA256": "...",
  "outsideContentSHA256": "..."
}
```

The exact persisted schema is fixed during implementation planning, but the
following semantics are normative:

- an artifact may have one or more configured consumers;
- adding an already registered consumer is idempotent;
- removing one consumer does not remove the artifact while another remains;
- removing the last consumer may remove only the owned import or block;
- a whole native file may be deleted only when UAWP created it, its complete
  current fingerprint matches the registered UAWP-created content, and no
  project-owned content has been added;
- otherwise UAWP removes only its owned region or stops with drift diagnostics;
- manifest registration is the machine authority, while marker metadata makes
  ownership and consumers auditable to a human;
- disagreement between manifest and marker metadata blocks mutation.

This registry describes configured UAWP consumers. It does not prove that an
agent is currently running or that no unregistered external tool reads the
file.

## 10. Codex adapter

### 10.1 Verified behavior

Codex discovers project instructions from the project root toward the current
working directory. In each directory it prefers `AGENTS.override.md`, then
`AGENTS.md`, then configured fallback names, and includes at most one file for
that directory. More specific directories are appended later. The documented
combined size limit is 32 KiB by default.

Official evidence:

- <https://learn.chatgpt.com/docs/agent-configuration/agents-md>

### 10.2 Resolution rules

At the workspace root:

- if `AGENTS.override.md` exists and is non-empty, it is the effective standard
  entry and root `AGENTS.md` is shadowed for Codex;
- otherwise a non-empty `AGENTS.md` is effective;
- configured fallback filenames are reported when observable, but a hidden
  user-level configuration is not guessed;
- if neither standard entry exists, the adapter may propose creating
  `AGENTS.md`.

Nested instruction files remain project-owned and are reported for precedence
awareness. The UAWP root bridge is not copied into every nested directory.

### 10.3 Selected mode

The initial Codex adapter uses `MANAGED_BLOCK` because the verified official
entry mechanism does not provide a deterministic in-file import syntax for an
arbitrary `.uawp/` document. The block directs Codex to read
`.uawp/INSTRUCTIONS.md` before UAWP work.

If a future Codex version documents direct loading of the canonical UAWP path,
the adapter may adopt `DIRECT` only through a reviewed evidence and migration
update.

## 11. Claude Code adapter

### 11.1 Verified behavior

Claude Code supports project `CLAUDE.md`, `.claude/CLAUDE.md`,
`CLAUDE.local.md`, `.claude/rules/`, and documented `@path` imports.

Direct project `AGENTS.md` loading is available from Claude Code v2.1.277, but
the default behavior is conditional:

- with `AGENTS.md` and no project/local `CLAUDE.md` at or above the working
  directory, Claude Code reads `AGENTS.md`;
- when a project/local `CLAUDE.md` is present, the default reads the Claude
  files instead of `AGENTS.md`;
- `CLAUDE.md` may import `AGENTS.md` explicitly;
- the `claude-md-and-agents-md` setting loads both;
- other settings can select Claude-only or managed-only behavior;
- some third-party provider, feature-flag, first-session, telemetry, hook, or
  plugin conditions can make direct `AGENTS.md` support unavailable.

Official evidence:

- <https://code.claude.com/docs/en/memory#agents-md>
- <https://code.claude.com/docs/en/memory#import-additional-files>

### 11.2 Resolution matrix

| Observed project state | Default UAWP route |
|---|---|
| Effective `CLAUDE.md`, no `AGENTS.md` | Add verified UAWP `@path` import to the effective Claude entry |
| Effective `CLAUDE.md` and `AGENTS.md` | Add UAWP import to the effective Claude entry; do not assume `AGENTS.md` loads |
| Only `AGENTS.md`, direct support verified, and an effective shared UAWP bridge already exists | Reuse the registered bridge without duplication |
| Only `AGENTS.md`, direct support verified, but no UAWP bridge exists | Compare reuse via managed block with creating a Claude import; preview the selected least-invasive route |
| Only `AGENTS.md`, direct support unknown or unavailable | Report uncertainty and propose a deterministic Claude entry/import route |
| Neither exists | Propose a minimal `CLAUDE.md` importing `.uawp/INSTRUCTIONS.md` |
| Configuration explicitly co-loads both | Reuse one effective registered bridge where possible and prevent duplicate UAWP loading |
| Configuration excludes project instructions | Report unsupported/blocked rather than claiming healthy integration |

Creating `CLAUDE.md` in a repository that previously relied on Claude Code's
fallback to `AGENTS.md` can shadow the existing instructions. The default
preservation proposal in that case imports both:

```text
@AGENTS.md
@.uawp/INSTRUCTIONS.md
```

This semantic effect must be shown explicitly. UAWP does not silently add the
`@AGENTS.md` import or assume the user intended Claude to consume Codex-specific
instructions.

### 11.3 Selected mode

The preferred Claude Code mode is `IMPORT` because `@path` is an officially
documented deterministic import mechanism. Reuse of an already effective
shared `AGENTS.md` bridge is allowed only when direct support and the active
instruction-selection configuration are verified.

## 12. Tencent WorkBuddy adapter

### 12.1 Verified behavior

Tencent's WorkBuddy documentation currently exposes the CodeBuddy project
instruction behavior. `CODEBUDDY.md` is the native root entry. When root
`AGENTS.md` exists and `CODEBUDDY.md` does not, CodeBuddy loads `AGENTS.md` as a
compatibility fallback. The rule system also supports project rules under
`.codebuddy/rules`, but mentioning a path there is not documented as
deterministic transclusion.

Official evidence:

- <https://www.workbuddy.ai/docs/zh/ide/User-guide/Rules>

### 12.2 Resolution matrix

| Observed project state | WorkBuddy route |
|---|---|
| Both `CODEBUDDY.md` and `AGENTS.md` | Treat `CODEBUDDY.md` as the effective WorkBuddy entry; do not rely on fallback |
| Only `CODEBUDDY.md` | Add or reuse its UAWP managed block |
| Only `AGENTS.md` | Use the documented fallback and add or reuse its shared UAWP managed block |
| Neither exists | Propose a minimal `CODEBUDDY.md` containing the UAWP managed block |
| `CODEBUDDY.md` appears after fallback installation | Report entry drift and propose migration to `CODEBUDDY.md` |
| `CODEBUDDY.md` disappears after native installation | Re-resolve fallback; never assume `AGENTS.md` is adequate without a valid bridge |

### 12.3 Selected mode

The initial WorkBuddy mode is `MANAGED_BLOCK`, not `IMPORT`. UAWP does not
create `.codebuddy/rules/uawp/RULE.mdc` by default. This avoids presenting a
path mention as a native import and keeps the bridge visible in the documented
root entry.

When `AGENTS.md` already contains a registered UAWP block for Codex, the same
artifact may add `workbuddy` as a consumer without duplicating the block.

## 13. Shared bridge behavior

A shared `AGENTS.md` bridge is one artifact, not one block per provider:

```text
AGENTS.md UAWP bridge
  consumers: [codex, workbuddy]
  target: .uawp/INSTRUCTIONS.md
```

If Claude Code is also verified to load that entry, it may be registered as a
third consumer. If later `CLAUDE.md` or `CODEBUDDY.md` shadows `AGENTS.md`, the
corresponding consumer becomes unhealthy until a reviewed migration completes.

Consumer changes update both registry and marker metadata in one approved
transaction. A partial consumer update is a recovery-required failure, not a
successful removal.

## 14. Install, update, remove, and uninstall semantics

### 14.1 Add an adapter

```text
discover -> resolve effective entry -> select mode -> preview exact changes
-> approve -> revalidate all inputs -> apply -> verify provider route
```

An already healthy registered adapter is a no-op. A conditional route is not
reported as installed successfully without the user's explicit acceptance of
the documented limitation.

### 14.2 Remove one adapter

Removing an adapter removes it from each artifact's configured consumers. A
shared artifact remains while any consumer remains. The agent used to execute
the command has no special authority and is not evidence of exclusive use.

### 14.3 Full UAWP uninstall

Full uninstall means all registered providers stop using UAWP. It may remove
all UAWP-owned imports and managed blocks after drift checks, regardless of
their former consumer count. It still preserves all content outside owned
regions and does not delete a modified native file.

Complete uninstall sequencing is implemented in the later uninstall/recovery
phase, but this phase must persist enough artifact and consumer information to
make that safe.

## 15. Drift detection and doctor behavior

`status`, `doctor`, and `upgrade` re-run entry resolution and compare current
state with the registered route. Findings include:

- effective entry changed;
- registered entry is now shadowed;
- fallback precondition no longer holds;
- provider version or environment is unsupported or unknown;
- import is missing, duplicated, or points to the wrong canonical target;
- managed markers are missing, malformed, or duplicated;
- content inside an owned artifact drifted;
- manifest and marker consumer sets disagree;
- the same canonical instructions would load more than once;
- a native file created by UAWP now contains project-owned additions;
- an installed adapter is healthy, conditional, degraded, or blocked.

Example report:

```text
Claude Code
  CLAUDE.md: EFFECTIVE
  AGENTS.md: SHADOWED
  UAWP route: CLAUDE.md @path import
  Health: HEALTHY
```

```text
WorkBuddy
  CODEBUDDY.md: newly present and EFFECTIVE
  AGENTS.md: previous FALLBACK_EFFECTIVE route is now SHADOWED
  UAWP route: unavailable through effective entry
  Health: ENTRY_DRIFT
  Next action: review migration plan
```

Doctor is read-only. Repairs always produce a new immutable plan and require
explicit approval.

## 16. Safety and failure behavior

- The selected workspace is untrusted input.
- All native paths are validated against traversal, symlink escape, portable
  filename, and workspace-boundary rules.
- Native reads are bounded; binary or undecodable instruction files block
  automatic integration.
- Plans bind full relevant file fingerprints and the resolved-entry facts.
- A provider entry or configuration change after preview invalidates approval.
- No adapter writes user-level or organization-level configuration.
- Unsupported versions and unobservable settings produce diagnostics, not
  optimistic mutations.
- Conflicting instructions are reported; UAWP does not attempt semantic merge.
- Failure to verify the effective post-apply route is not reported as success.
- Recovery records identify completed and incomplete native changes without
  deleting content on inference.

## 17. Conformance test matrix

Framework tests must cover:

- all integration modes and unsupported-mode rejection;
- idempotent add, update, and remove;
- shared artifacts with one, two, and three consumers;
- last-consumer removal;
- marker corruption and registry/marker disagreement;
- user edits inside and outside an owned block;
- file-created-by-UAWP followed by user additions;
- path, symlink, encoding, size, and newline edge cases;
- preview drift between plan and apply;
- interrupted multi-file registry/artifact updates;
- duplicate canonical loading prevention.

Provider fixtures must cover at least:

### Codex

- neither standard entry exists;
- `AGENTS.md` only;
- `AGENTS.override.md` only;
- both standard entries;
- nested entries and size-limit diagnostics;
- observable fallback filename configuration;
- effective entry appearing or disappearing after installation.

### Claude Code

- every row of the resolution matrix in section 11.2;
- versions before and after direct `AGENTS.md` support;
- direct support unavailable because of documented environment constraints;
- default, both-files, Claude-only, and managed-only selection settings;
- `CLAUDE.local.md` shadowing fallback behavior;
- an existing `@AGENTS.md` import;
- an existing UAWP import and duplicate prevention;
- creation of `CLAUDE.md` that would shadow previous fallback;
- `.claude/CLAUDE.md` and ancestor-scope entries.

### Tencent WorkBuddy

- every row of the resolution matrix in section 12.2;
- shared `AGENTS.md` fallback with Codex;
- later creation of `CODEBUDDY.md`;
- later removal of `CODEBUDDY.md`;
- existing project rules without claiming native import support;
- removal of only one consumer from a shared bridge.

## 18. Release gates

No launch adapter is advertised as supported until:

1. its official evidence record is current and reviewable;
2. all documented entry combinations have fixtures;
3. install, status, doctor, update, and removal are idempotent;
4. project-owned native content survives byte-for-byte outside approved owned
   regions;
5. shared-consumer removal is proven safe;
6. shadowing and fallback drift are detected;
7. unsupported or unobservable environments fail closed;
8. cross-platform filesystem tests pass on configured Linux, macOS, and Windows
   runners;
9. user documentation explains effective entries, bridge ownership, and safe
   removal;
10. the adapter authoring guide proves a new provider can be added without a
    Core change.

## 19. Acceptance criteria

This design phase is accepted when the reviewer confirms:

- `.uawp/INSTRUCTIONS.md` as the single canonical shared instruction source;
- no native-file-to-native-file synchronization;
- provider-specific effective-entry resolution instead of filename existence
  checks;
- capability-driven mode selection;
- Codex `MANAGED_BLOCK`, Claude Code preferred `IMPORT`, and WorkBuddy
  `MANAGED_BLOCK` as the initial modes;
- shared bridge artifacts with explicit configured consumers;
- consumer-aware adapter removal and separate full uninstall semantics;
- dynamic shadowing, fallback, and entry-drift detection;
- the three provider resolution matrices and conformance scope;
- no implementation work before this specification and its subsequent
  implementation plan are approved.

After written approval, the next step is a task-level implementation plan. The
implementation plan must define the persisted manifest schema, stable finding
codes, CLI command surface, package boundaries, migration impact on existing
workspaces, and the exact test-first task sequence. Coding begins only after
that plan is separately reviewed and approved.
