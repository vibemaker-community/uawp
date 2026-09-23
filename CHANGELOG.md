# Changelog

All notable changes are recorded here. UAWP uses Semantic Versioning; public
tags and assets are immutable.

## Unreleased

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

The project remains in pre-release hardening until hosted native gates pass and
`v1.0.0-rc.1` is published and verified.
