# Safe initialization

`uawp init` creates UAWP-owned state only under `.uawp/`. It never silently
overwrites a project root `context.md`, source artifacts, or agent-native files
such as `AGENTS.md` and `CLAUDE.md`.

## Preview, then approve

Build the CLI with `go build -o ./uawp ./cmd/uawp`, then use an absolute
workspace path. The first command returns a plan and exits 5; it does not
mutate the project.

<!-- verify -->
```bash
./uawp init --workspace /absolute/project/path --format json
```

The JSON `planID` contains the exact generated state timestamp plus a hash of
the resulting plan. Pass that unmodified token to apply the same bytes you
previewed.

<!-- verify -->
```bash
./uawp init --workspace /absolute/project/path --approve PLAN_ID --format json
```

If the namespace changes after preview, the approval token does not match the
fresh plan and UAWP refuses to apply it. An existing `.uawp/` with no valid
UAWP manifest is a namespace collision: UAWP stops rather than adopting or
overwriting it.

## Exit categories

| Exit code | Meaning |
|---:|---|
| 0 | Command completed safely. |
| 2 | Command arguments or output format are invalid. |
| 3 | The workspace is unsafe to mutate, such as an unknown `.uawp/`. |
| 4 | Existing UAWP state is invalid or recovery is required. |
| 5 | A preview was produced or approval did not match the current plan. |
| 10 | The selected workspace could not be opened. |

Do not use an approval token from another workspace. Do not edit `.uawp/` by
hand while an approval token is pending.
