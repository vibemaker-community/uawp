# Package boundaries

The dependency direction is:

```text
cli -> workspace -> plan/core
```

`internal/core` contains vendor-neutral protocol types and validation.
`internal/plan` contains immutable, previewable changes. `internal/workspace`
owns filesystem discovery, boundaries, apply/recovery, and diagnostics.
`internal/cli` parses commands and renders results.

Future adapters may depend on Core contracts, but Core must never import an
adapter, CLI package, vendor package, native-agent filename, or vendor-specific
instruction behavior.

Review dependency direction with:

<!-- verify -->
```bash
go list -deps ./...
```

Any third-party runtime dependency requires a new architecture decision record.
