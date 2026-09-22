# UAWP — Universal Agent Workspace Protocol

UAWP is an agent-neutral protocol and CLI for safely persisting workspace
state across agent sessions. It initializes `.uawp/` without
silently changing project-owned or agent-native instruction files.

## Try safe initialization

Build with Go 1.26 or later, then preview a selected project directory. Preview
does not write files; exit code `5` means explicit approval is required.

```bash
go build -o ./uawp ./cmd/uawp
```

<!-- verify -->
```bash
./uawp init --workspace /absolute/project/path --format json
```

Copy the returned `planID` exactly, then apply that exact preview:

<!-- verify -->
```bash
./uawp init --workspace /absolute/project/path --approve PLAN_ID --format json
```

Inspect a workspace without modifying it:

<!-- verify -->
```bash
./uawp status --workspace /absolute/project/path --format json
./uawp doctor --workspace /absolute/project/path --format json
```

Read [safe initialization](docs/user/safe-init.md),
[diagnostics](docs/user/diagnostics.md), [worker lifecycle](docs/user/lifecycle.md),
[stale recovery](docs/user/stale-recovery.md), [agent adapters](docs/user/adapters.md),
and the [v1 state model](docs/protocol/state-v1.md).

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
