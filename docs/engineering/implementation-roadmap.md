# UAWP v1.0 Implementation Roadmap

**Status:** Plans 1-3 implemented; Plan 4 design approved and planning; Plans 4-5 remain
**Spec:** `docs/superpowers/specs/2026-09-21-uawp-product-engineering-design.md`

The product specification spans several independently reviewable subsystems.
Implementation is therefore split into five plans. Each plan must leave a
working, testable product increment and pass its own approval gate.

## Plan 1 — Foundation, Core, and Safe Init

Create the Go project, agent-neutral state models, validation, immutable change
plans, workspace boundary checks, drift detection, atomic application, and a
minimal `uawp init|status|doctor` vertical slice. It supports `.uawp/` only and
does not modify agent-native files.

**Deliverable:** greenfield and brownfield projects can preview and initialize
the UAWP namespace without touching project-owned content.

## Plan 2 — Ownership Lifecycle and Prompt Library

Implement acquire, release, context sync, milestone checkpoint, pause/handoff,
the Single Active Worker invariant, and Human Controller-authorized stale
recovery. Add the four canonical prompts and end-to-end
handoff/conflict/crash-recovery scenarios.

The canonical set is `RESUME_WORK`, `CONTEXT_SYNC`, `CREATE_CHECKPOINT`, and
`PAUSE_AND_HANDOFF`.

**Deliverable:** the validated DRY RUN lifecycle is executable and auditable.

## Plan 3 — Adapter Framework and Launch Adapters

Define capability/evidence records and implement DIRECT, IMPORT, and
MANAGED_BLOCK planning. Select launch agents only after reviewing their current
official documentation; add an ADR and conformance fixture for each advertised
adapter.

**Deliverable:** verified adapters integrate idempotently without claiming
ownership of native files.

**Status:** Implemented for Codex, Claude Code, and Tencent WorkBuddy, including
preview/approval, configured-consumer tracking, dynamic health, and safe
single-consumer removal.

## Plan 4 — Upgrade, Uninstall, and Recovery

Implement schema migrations, integration upgrades, state-preserving uninstall,
reinstall, drift repair reports, and interrupted-transaction recovery.

**Deliverable:** supported version transitions and removal paths preserve all
project-owned content.

**Design:**
`docs/superpowers/specs/2026-09-22-upgrade-uninstall-transaction-recovery-design.md`

## Plan 4.5 — Human-friendly CLI and Agent Automation

Add three compatible operating surfaces over the same Core operations:

- a default human-friendly mode with current-directory defaults, concise
  previews, and interactive confirmation;
- a stable non-interactive agent automation mode with structured input/output
  and explicit auditable approval; and
- the existing explicit flag-and-plan-ID surface as expert mode, available
  without a global mode switch.

This phase changes presentation and command ergonomics, not ownership,
transaction, preservation, or adapter semantics.

**Deliverable:** routine `init`, `resume`, `sync`, `checkpoint`, and `handoff`
workflows are concise for people and deterministic for agents, while expert
automation remains backward-compatible.

## Plan 5 — Open-source and Release Hardening

Complete packaging, multi-platform CI, fuzz/property tests, security review,
licenses and policies, contribution documentation, release automation,
checksums/signing decision, and public-artifact smoke tests.

**Deliverable:** every v1.0 Release Gate passes against a reproducible tagged
release candidate.

## Dependency order

Plans execute in numerical order, including Plan 4.5 between Plans 4 and 5. A
later plan may begin design research while an earlier plan is implemented, but
no later plan may weaken or bypass an earlier plan's public contracts.
Interface changes require an ADR and updates to every affected plan and test.
