# Worker lifecycle

Normal work follows `resume -> sync -> checkpoint (milestone only) -> handoff`.
`resume` guides profile creation and ownership acquisition when needed.

```bash
uawp init
uawp resume
uawp sync --context-file context-next.md
uawp checkpoint
uawp handoff --context-file context-final.md
```

Commands default to the current directory. A terminal preview is confirmed in
the same invocation. One physical Workspace permits one ACTIVE writing Session;
same-directory parallel writing is not supported. Git is optional.

## Expert preview/apply

Every mutation previews first and exits with code 5. Review the JSON `planID`, then rerun the identical command with `--approve <planID>`. Any bound-state drift invalidates it.

```sh
uawp sync --workspace ./project --worker-id worker-a --session-id session-a --generation 3 --context-file ./current-context.md --format json
uawp sync --workspace ./project --worker-id worker-a --session-id session-a --generation 3 --context-file ./current-context.md --format json --approve PLAN_ID
```

`resume` reconstructs state before acquisition. `sync` retains ownership.
Checkpoints are explicit and never overwritten. Handoff verifies final context
before release, creates no automatic checkpoint, and clears the local binding
only after verified release.

Exit codes: 0 success, 2 usage, 3 unsafe namespace, 4 invalid state, 5 approval required or rejected, and 10 internal/open-root failure.
