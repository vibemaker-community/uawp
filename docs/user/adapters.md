# Agent adapters

UAWP keeps its canonical, provider-neutral instructions in
`.uawp/INSTRUCTIONS.md`. Adapters add only a minimal, reviewable bridge to an
agent-native entry. UAWP never owns `AGENTS.md`, `CLAUDE.md`, or
`CODEBUDDY.md`; it owns only the exact import, managed block, or file that an
approved plan records in `.uawp/manifest.json`.

## Inspect before changing anything

```bash
uawp adapter list --workspace /absolute/project/path --format json
uawp status --workspace /absolute/project/path --format json
uawp doctor --workspace /absolute/project/path --format json
```

These commands are read-only. They report the provider, currently effective
entry, selected mode, confidence, health, and next action. `configured
consumers` means only that UAWP registered an adapter; it cannot prove that an
agent process is currently using the file or that an unregistered tool is not.

## Add an adapter

Preview first (exit code `5`):

```bash
uawp adapter add codex --workspace /absolute/project/path --format json
```

Review every native-file and manifest change, then pass back the returned
`planID` exactly:

```bash
uawp adapter add codex --workspace /absolute/project/path --format json --approve PLAN_ID
```

Launch provider IDs are `codex`, `claude-code`, and `workbuddy`. Runtime facts
that affect selection can be supplied with `--provider-version`,
`--instruction-files`, `--direct-agents-support`, and
`--provider-environment`. Codex discovery also accepts the observed
`--fallback-filenames` and `--working-directory`; these files become immutable
preview inputs. If a conditional route reports a warning, review it
and repeat `--acknowledge FINDING_CODE` on both preview and apply. Creating a
Claude entry that would otherwise shadow an existing `AGENTS.md` requires
acknowledging `CLAUDE_CREATION_CHANGES_SELECTION`; its proposal preserves both
the existing instructions and UAWP import.

## Remove one configured consumer

```bash
uawp adapter remove workbuddy --workspace /absolute/project/path --format json
uawp adapter remove workbuddy --workspace /absolute/project/path --format json --approve PLAN_ID
```

Removal never assumes the command's provider is the only reader. A shared
bridge remains until its last registered consumer is removed. User content
outside a UAWP-owned block is byte-preserved. A whole native file is deleted
only when UAWP created it and its complete fingerprint is unchanged.

## Drift and repair

If a higher-priority native entry appears later, diagnostics report
`ENTRY_DRIFT`; for example, `CODEBUDDY.md` appearing after WorkBuddy was
registered through the `AGENTS.md` fallback. Do not hand-edit UAWP markers.
Malformed markers, binary/oversized entries, symlinks, changed preview inputs,
or manifest disagreement stop mutation. Review the reported state and create a
fresh plan. Interrupted publication leaves `RECOVERY_REQUIRED`; recovery is a
separate, explicit workflow.
