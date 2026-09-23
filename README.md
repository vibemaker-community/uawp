# UAWP — Universal Agent Workspace Protocol

UAWP is an Agent-neutral protocol and command-line product for carrying durable
workspace context safely across Agent sessions and providers. It gives Codex,
Claude Code, Tencent WorkBuddy, and future adapters one shared source of truth
without making any provider the owner of that truth.

Repository: <https://github.com/vibemaker-community/uawp>

UAWP solves workspace continuity, explicit write ownership, checkpoints,
handoffs, and non-destructive Agent integration. It does **not** read private
provider conversation memory, merge simultaneous edits, replace Git, or turn a
shared folder into a parallel-write system.

## Safety model

- The Agent-neutral Core owns only the `.uawp/` namespace.
- Existing project files are previewed and preserved by default.
- Agent-native files remain project/provider-owned. Verified adapters use
  `DIRECT`, `IMPORT`, or bounded `MANAGED_BLOCK` integration and record every
  consumer before removal.
- The Single Active Worker invariant permits one writing Session per physical
  Workspace. Reads remain available during conflicts.
- An apparently stale ACTIVE owner never expires automatically. A separate
  Human Controller must inspect and authorize recovery, which is audited.
- Every mutation is preview-first, bound to observed state, and invalidated by
  drift.

See [architecture](docs/architecture.md), [Agent integration](docs/agent-integration.md),
and the [threat model](docs/security/threat-model.md).

## Install

Official release installers verify a versioned SHA-256 manifest and refuse to
overwrite an existing binary unless explicitly forced. See the complete
[installation guide](docs/install.md) and [release verification](docs/verify-release.md).

Until the first public release candidate is published, contributors can build
from a reviewed checkout with Go 1.26 or later:

```sh
go build -o ./uawp ./cmd/uawp
```

<!-- testable-shell -->
```sh
"$UAWP_BIN" version --format json
```

Verified release targets are macOS arm64/amd64, Linux arm64/amd64, and Windows
amd64. The current maturity is **pre-release hardening**: local gates pass, but
hosted native evidence and a public release candidate are still required. See
[supported platforms](docs/supported-platforms.md).

## Quick start

Run normal commands in a terminal, or ask a supported Agent naturally to run
them through its terminal capability:

```sh
uawp init
uawp resume
uawp sync --context-file context-next.md
uawp checkpoint
uawp handoff --context-file context-final.md
```

The ordinary user mode gives concise previews and interactive confirmation.
The expert mode exposes explicit Workspace, identity, generation, JSON, and
approval-token options. Agent automation mode uses the same expert contract
with structured JSON and no prompts. These are interfaces over one Core, not
different safety policies.

For details, read [safe initialization](docs/user/safe-init.md),
[the lifecycle](docs/user/lifecycle.md), [Agent automation](docs/user/agent-automation.md),
and [identity and Sessions](docs/user/identity-and-sessions.md).

## Adapters

Adapters connect the shared `.uawp/INSTRUCTIONS.md` to a provider's documented
native entry. They do not synchronize `AGENTS.md`, `CLAUDE.md`, or other native
files with one another and never claim an entire pre-existing file.

```sh
uawp adapter list --format json
uawp adapter add codex --format json
uawp adapter remove codex --format json
```

Read [adapter usage](docs/user/adapters.md) and the evidence pages for
[Codex](docs/adapters/codex.md), [Claude Code](docs/adapters/claude-code.md),
and [Tencent WorkBuddy](docs/adapters/workbuddy.md).

## Uninstall

`uawp uninstall` first previews removal of only recorded UAWP integrations and
preserves `.uawp/` by default. Purging state is a separate operation requiring a
verified external export. See [uninstall and export](docs/user/uninstall-and-export.md).

## Project and community

- Roadmap: [ROADMAP.md](ROADMAP.md)
- Changelog: [CHANGELOG.md](CHANGELOG.md)
- Release gates: [docs/release-gates.md](docs/release-gates.md)
- Contributing and CLA: [CONTRIBUTING.md](CONTRIBUTING.md), [CLA.md](CLA.md)
- License: [GPL-3.0-only](LICENSE)
- Trademark policy: [TRADEMARKS.md](TRADEMARKS.md)
- Security reports: [SECURITY.md](SECURITY.md)
- Support: [SUPPORT.md](SUPPORT.md)

Copyright © 2026 Li Rui. “vibemaker” is a pending trademark application; see
the trademark policy for permitted descriptive use.
