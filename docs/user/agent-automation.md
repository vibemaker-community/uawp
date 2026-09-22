# Agent automation

Structured automation uses explicit identity data and never prompts. JSON
stdout is exactly one document. A preview exits with exit code `5`, reports
`mutated: false`, and returns a `planID`; apply must repeat the same command with
the exact approval token.

```bash
uawp sync \
  --workspace /absolute/project \
  --worker-id worker-0123 \
  --session-id session-4567 \
  --generation 3 \
  --context-file /absolute/context-next.md \
  --format json \
  --non-interactive
```

Then apply the exact preview:

```bash
uawp sync \
  --workspace /absolute/project \
  --worker-id worker-0123 \
  --session-id session-4567 \
  --generation 3 \
  --context-file /absolute/context-next.md \
  --format json \
  --non-interactive \
  --approve PLAN_ID
```

Changing the Workspace, file content, Worker ID, Session ID, generation, or
bound state invalidates approval. Stable codes distinguish another Session of
the same Worker (`ACTIVE_OTHER_SESSION`) from another Worker
(`ACTIVE_OTHER_WORKER`). Redirected and explicit non-interactive execution use
the same no-prompt contract. Git is optional, but same-directory parallel
writing is not supported.
