# Codex adapter evidence

- Verified: 2026-09-22
- Official source: <https://learn.chatgpt.com/docs/agent-configuration/agents-md>
- Native entries: `AGENTS.override.md`, `AGENTS.md`, then configured fallback names.
- Order: project root toward the working directory; the first non-empty file at
  each directory is used and more specific directories are appended later.
- Documented default aggregate limit: 32 KiB.
- UAWP mode: `MANAGED_BLOCK`, targeting `.uawp/INSTRUCTIONS.md`.

Validate by starting a new Codex session and asking it to list its loaded
instruction sources. Configuration that is not locally observable is reported
as conditional rather than assumed.
