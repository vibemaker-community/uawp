# Changelog

## Unreleased

### Added

- Preview-first `.uawp/` initialization.
- Manifest validation, ownership state validation, workspace boundaries, status,
  doctor, and machine-readable CLI output.
- Greenfield and brownfield preservation, race, and fuzz test coverage.
- Ownership acquire/release, read-only resume, context sync, immutable
  checkpoints, ordered handoff, and Human Controller-approved stale recovery.
- Four canonical agent-neutral Prompt Library assets.
- Provider-neutral adapter framework with evidence-backed launch adapters for
  Codex, Claude Code, and Tencent WorkBuddy.
- Preview/approve `adapter list|add|remove`, shared bridge consumers, dynamic
  entry-health diagnostics, and adapter authoring/user documentation.
- Exact `1.0.0` to `1.1.0` state migration with migration receipts.
- Durable transaction journals, backups, classification, rollback, and continuation.
- Evidence-limited repair, full provider detach, and verified-export purge.
- Expert commands: `upgrade`, `repair`, `uninstall`, and `transaction`.
- State `1.2.0`, local Worker profiles, per-conversation Session IDs,
  monotonically increasing ownership generations, and guided human workflows.
- Additive structured automation fields and Prompt Library Version 2.

### Safety

- Adapter commands edit only an approved, exact import or managed block in the
  resolved native entry; initialization still never edits native files.
- Initialization requires an exact, explicit plan approval token.
- Human commands confirm one exact in-memory preview; JSON, redirected, and
  non-interactive commands never prompt.
- Native and manifest drift, unsafe file types, malformed ownership markers,
  and uncertain unacknowledged routes stop mutation.
