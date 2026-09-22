# Package boundaries

Plan 4 keeps transaction policy provider-neutral: `internal/transaction`
validates and classifies persisted evidence without filesystem access;
`internal/workspace` alone resolves paths, snapshots files, stages backups,
publishes plans, exports archives, and performs purge commits. Migration steps
produce ordinary `plan.Change` values. Adapter packages remain the only place
that knows provider-native filenames, precedence, import syntax, and managed
block structure. The CLI parses and presents these operations but contains no
migration, merge, adapter-selection, recovery-classification, or archive logic.

The dependency direction is:

```text
cli -> workspace -> adapter -> core
                \-> plan/core
```

`internal/core` contains vendor-neutral protocol types and validation.
`internal/plan` contains immutable, previewable changes. `internal/workspace`
owns filesystem discovery, boundaries, apply/recovery, and diagnostics.
`internal/cli` parses commands and renders results.

`internal/adapter` contains provider evidence and pure effective-entry
resolution. It may use agent-neutral Core integration modes, but it does not
write files. Workspace consumes resolutions and remains the sole mutation
authority.

adapter, CLI package, vendor package, native-agent filename, or vendor-specific
instruction behavior.
Core must never import an adapter, CLI package, vendor package, native-agent
filename, or vendor-specific instruction behavior.
adapter, CLI package, vendor package, native-agent filename, or vendor-specific
instruction behavior.

Review dependency direction with:

<!-- verify -->
```bash
go list -deps ./...
```

Any third-party runtime dependency requires a new architecture decision record.
