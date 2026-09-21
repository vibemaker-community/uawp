# Changelog

## Unreleased

### Added

- Preview-first `.uawp/` initialization.
- Manifest validation, ownership state validation, workspace boundaries, status,
  doctor, and machine-readable CLI output.
- Greenfield and brownfield preservation, race, and fuzz test coverage.

### Safety

- No command in this increment edits agent-native files such as `AGENTS.md` or
  `CLAUDE.md`.
- Initialization requires an exact, explicit plan approval token.
