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

### Safety

- Adapter commands edit only an approved, exact import or managed block in the
  resolved native entry; initialization still never edits native files.
- Initialization requires an exact, explicit plan approval token.
- Native and manifest drift, unsafe file types, malformed ownership markers,
  and uncertain unacknowledged routes stop mutation.
