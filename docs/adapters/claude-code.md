# Claude Code adapter evidence

- Verified: 2026-09-22
- Official sources: <https://code.claude.com/docs/en/memory#agents-md> and
  <https://code.claude.com/docs/en/memory#import-additional-files>
- Native entries: `CLAUDE.md`, `.claude/CLAUDE.md`, `CLAUDE.local.md`, and
  conditional direct `AGENTS.md` support.
- Direct `AGENTS.md`: Claude Code 2.1.277 or newer, subject to documented
  environment, feature, plugin, and instruction-selection restrictions.
- UAWP mode: documented `@path` `IMPORT` where a Claude entry is effective;
  verified fallback reuse may use the shared `AGENTS.md` managed block.

Creating a Claude entry can stop default `AGENTS.md` fallback. UAWP reports
that semantic change and proposes `@AGENTS.md` alongside the UAWP import rather
than silently discarding existing instructions.
