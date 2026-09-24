# Changelog

All notable changes are recorded here. UAWP uses Semantic Versioning; public
tags and assets are immutable.

## 1.0.1 - 2026-09-24

### Fixed

- Corrected the final GitHub release publication path so the publish job has
  repository context before running `gh release create`.
- Preserved the failed `v1.0.0` tag as immutable audit evidence and promoted
  the verified recovery candidate through a new patch release.

## 1.0.0-rc.1 - 2026-09-24

### Added

- Agent-neutral `.uawp/` state, lifecycle, Prompt Library, and provider adapter
  architecture.
- Preview/approve initialization, synchronization, checkpoints, handoff,
  Human Controller recovery, upgrades, repair, and verified-export uninstall.
- Codex, Claude Code, and Tencent WorkBuddy launch adapters using shared UAWP
  instructions without claiming provider-native files.
- Deterministic five-target release archives, checksum-verifying installers,
  provenance attestations, native smoke tests, and public post-release checks.

### Security

- Exact plan binding and drift revalidation prevent stale approvals.
- ACTIVE ownership has no automatic TTL or lease expiry.
- Installers reject checksum ambiguity, traversal, links, duplicate entries,
  and unexpected archive members.
- GitHub Actions use read-only defaults, immutable action commits, bounded
  timeouts, CodeQL, dependency review, and Go vulnerability analysis.

This release candidate is ready for the final human publication gate. It is
not a completed release until its immutable tag, native release workflow,
checksums, attestations, and public installation checks all pass.
