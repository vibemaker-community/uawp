# Tencent WorkBuddy adapter evidence

- Verified: 2026-09-22
- Official source: <https://www.workbuddy.ai/docs/zh/ide/User-guide/Rules>
- Native entry: root `CODEBUDDY.md`.
- Compatibility fallback: root `AGENTS.md` is loaded only when
  `CODEBUDDY.md` is absent.
- UAWP mode: `MANAGED_BLOCK` targeting `.uawp/INSTRUCTIONS.md`.

The adapter does not create `.codebuddy/rules/uawp/RULE.mdc`: the official
documentation permits mentioning paths in rules but does not describe that as
deterministic import/transclusion. Appearance or disappearance of
`CODEBUDDY.md` after installation is reported as entry drift.
