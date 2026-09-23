# How Agent-driven UAWP operation works

UAWP does not need to appear as a separately registered function-calling Tool
in v1. The supported flow uses capabilities the Agent already has:

```text
User's natural-language request
              +
Agent automatically loads its native project entry
              ↓
Native entry directs it to .uawp/INSTRUCTIONS.md
              ↓
LLM interprets intent under UAWP's rules
              ↓
Agent uses its terminal capability to execute the UAWP CLI
              ↓
CLI previews, validates, and applies approved workspace state changes
```

## Who triggers instruction loading?

The Agent runtime does. Each provider defines which project-native file or rule
directory it discovers when a conversation works in a folder. A UAWP adapter
uses that documented mechanism to point to the shared
`.uawp/INSTRUCTIONS.md`. The runtime supplies the loaded instructions together
with the user's message to the LLM. UAWP itself does not intercept the message
or inject hidden prompt context.

The LLM then recognizes requests such as “resume this workspace,” “synchronize
the current context,” “create a checkpoint,” or “prepare a handoff.” It asks the
Agent runtime to run the corresponding `uawp` command through the terminal. The
CLI remains the enforcement boundary: natural language never bypasses exact
ownership, preview, approval, drift, or transaction checks.

## Context synchronization is explicit

UAWP does **not** locate or read private provider conversation stores, hidden
memory databases, account data, or files under provider home directories. The
Agent first prepares a complete Markdown file from context it is authorized to
use. It then runs:

```sh
uawp sync --context-file /absolute/path/context-next.md
```

`sync --context-file` means “read this explicitly supplied file and propose it
as the next `.uawp/CONTEXT.md`.” UAWP previews the change, requires approval,
revalidates the bound state, and only then commits it.

## Three interaction surfaces

- **Ordinary user**: natural language or short terminal commands; the CLI shows
  a concise preview and asks for confirmation.
- **Expert**: explicit flags expose Workspace, identity, Session, generation,
  JSON output, and plan approval for diagnosis and controlled automation.
- **Agent automation**: the Agent uses expert flags, structured JSON, and
  non-interactive execution. Preview exit code `5` returns the `planID`; apply
  repeats the exact command with that token.

A future MCP or native Tool may map structured calls onto the same typed Core.
It must not implement a second policy engine or bypass the CLI/Core invariants.

See [architecture](architecture.md), [automation](user/agent-automation.md), and
[adapter evidence](user/adapters.md).
