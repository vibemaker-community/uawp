# UAWP — Universal Agent Workspace Protocol

UAWP is an agent-neutral protocol and CLI for safely persisting workspace
state across agent sessions. It initializes `.uawp/` without
silently changing project-owned or agent-native instruction files.

## Quick Start

Build with Go 1.26 or later. For normal human use, enter the Workspace and run:

```bash
go build -o ./uawp ./cmd/uawp
```

```bash
uawp init
uawp resume
uawp sync --context-file context-next.md
uawp checkpoint
uawp handoff --context-file context-final.md
```

The human surface shows a concise preview and asks for confirmation in the same
invocation. A Workspace may be an ordinary office folder; Git is optional.
UAWP enforces one ACTIVE writing Session, and same-directory parallel writing is
not supported.

## Expert preview/apply

Redirected or JSON output never prompts. Preview exits with exit code `5`; copy
the returned `planID` exactly and approve the same command:

<!-- verify -->
```bash
uawp init --workspace /absolute/project/path --format json
uawp init --workspace /absolute/project/path --approve PLAN_ID --format json
```

Inspect a workspace without modifying it:

<!-- verify -->
```bash
./uawp status --workspace /absolute/project/path --format json
./uawp doctor --workspace /absolute/project/path --format json
```

Read [safe initialization](docs/user/safe-init.md),
[diagnostics](docs/user/diagnostics.md), [worker lifecycle](docs/user/lifecycle.md),
[identity and sessions](docs/user/identity-and-sessions.md),
[agent automation](docs/user/agent-automation.md),
[how Agent integration works](docs/architecture/agent-integration.md),
[stale recovery](docs/user/stale-recovery.md), [agent adapters](docs/user/adapters.md),
and the [v1 state model](docs/protocol/state-v1.md).

In normal Agent use, the Agent runtime loads its native project entry, which
points to `.uawp/INSTRUCTIONS.md`. The LLM combines those instructions with the
user's request and invokes the UAWP CLI through the Agent's terminal capability.
UAWP does not inspect private conversation-memory files.

Maintenance expert commands are also preview-first:

```bash
./uawp upgrade --workspace /absolute/project/path --format json
./uawp repair --workspace /absolute/project/path --format json
./uawp uninstall --workspace /absolute/project/path --format json
./uawp transaction status --workspace /absolute/project/path --format json
```

See [upgrade and repair](docs/user/upgrade-and-repair.md),
[uninstall and verified export](docs/user/uninstall-and-export.md), and
[transaction recovery](docs/user/transaction-recovery.md).

The CLI now includes `resume`, `acquire`, `release`, `sync`, `checkpoint`,
`handoff`, and Human Controller-approved `recover`. The four canonical
agent-neutral prompts live in `prompts/`.

The launch adapters are Codex, Claude Code, and Tencent WorkBuddy. Every native
integration is preview-first, requires exact approval, revalidates drift, and
records configured consumers without claiming ownership of the surrounding
native file:

```bash
./uawp adapter list --workspace /absolute/project/path --format json
./uawp adapter add codex --workspace /absolute/project/path --format json
./uawp adapter remove codex --workspace /absolute/project/path --format json
```
