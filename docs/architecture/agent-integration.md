# How Agent integration works

UAWP separates Agent-specific discovery from Agent-neutral workspace state.
The normal flow is:

```text
Agent-native project entry
  -> .uawp/INSTRUCTIONS.md
  -> LLM interprets the user's request
  -> Agent terminal capability runs uawp
  -> UAWP Core validates and changes .uawp/ state
```

## Startup and instruction loading

Each supported Agent has its own documented project-entry mechanism. A UAWP
Adapter integrates through that native mechanism without claiming ownership of
the surrounding native file. Depending on the Agent runtime, a referenced file
may be expanded automatically or the LLM may be instructed to read it. Either
way, `.uawp/INSTRUCTIONS.md` supplies the Agent-neutral lifecycle rules.

When a user asks naturally to resume work, synchronize context, create a
checkpoint, or hand off, the LLM sees both the request and the loaded UAWP
instructions. It can then run the corresponding CLI command through the
Agent's existing terminal capability. The current release does not require a
separately registered function-calling Tool or MCP server.

## Context synchronization

UAWP does not locate, read, or modify an Agent's private conversation memory,
hidden context database, or account data. The Agent prepares a complete Markdown
context document from information it is authorized to use, then invokes:

```bash
uawp sync --context-file /absolute/path/context-next.md
```

UAWP reads that supplied file, previews replacement of
`.uawp/CONTEXT.md`, requires approval, revalidates bound state, and only then
applies the change. This keeps provider internals outside the protocol boundary.

## CLI today, Tool integration later

The CLI is the stable application boundary today. A future MCP or native Tool
surface may map structured function calls onto the same typed UAWP Core
operations. It must not create a second policy implementation or bypass
preview, ownership, approval, and non-destructive integration rules.
