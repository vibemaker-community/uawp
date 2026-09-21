# UAWP — Universal Agent Workspace Protocol

UAWP is an agent-neutral protocol and CLI for safely persisting workspace
state across agent sessions. This first increment initializes `.uawp/` without
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
[diagnostics](docs/user/diagnostics.md), and the [v1 state model](docs/protocol/state-v1.md).

This increment has no advertised agent adapters yet. Adapter integration and
the worker lifecycle commands are planned for later increments.
