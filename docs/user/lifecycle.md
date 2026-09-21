# Worker lifecycle

Normal work follows `resume -> acquire -> sync -> checkpoint (milestone only) -> handoff/release`.

Every mutation previews first and exits with code 5. Review the JSON `planID`, then rerun the identical command with `--approve <planID>`. Any bound-state drift invalidates it.

```sh
./uawp acquire --workspace ./project --worker-id worker-a --agent "Agent A" --purpose "implement feature" --format json
./uawp sync --workspace ./project --worker-id worker-a --context-file ./current-context.md --format json
./uawp checkpoint --workspace ./project --worker-id worker-a --milestone-id api-ready --label "API ready" --format json
./uawp handoff --workspace ./project --worker-id worker-a --purpose handoff --context-file ./current-context.md --format json
```

`resume` is read-only. `sync` retains ownership. Checkpoints are explicit and never overwritten. Handoff verifies final context before releasing ownership and creates no automatic checkpoint.

Exit codes: 0 success, 2 usage, 3 unsafe namespace, 4 invalid state, 5 approval required or rejected, and 10 internal/open-root failure.
