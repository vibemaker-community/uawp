# ADR 0001: Go Toolchain and Dependency Policy

**Status:** Proposed for implementation-plan approval  
**Date:** 2026-09-21

## Context

UAWP must ship as an installable cross-platform CLI while keeping its Core
agent-neutral. Its highest-risk code performs local filesystem discovery,
planning, preservation, atomic writes, and recovery. The product should be
easy to install from GitHub, straightforward to test with temporary
directories, and maintainable by external contributors.

## Options considered

### Go

Go produces standalone executables, has strong standard-library support for
JSON, paths, files, checksums, and testing, and has conventional multi-platform
release tooling. Its explicit error handling is appropriate for auditable
filesystem operations. The trade-off is more verbose domain modeling than in
TypeScript or Python.

### Rust

Rust offers excellent type and memory safety and strong single-binary
distribution. It has a higher contribution and implementation complexity cost
for the first product version, especially for a protocol whose dominant risks
are filesystem semantics and product-policy mistakes rather than memory
unsafety.

### TypeScript on Node.js

TypeScript offers fast CLI development and a large ecosystem. It requires a
Node runtime or a separate bundling strategy and normally expands the
third-party dependency and supply-chain surface.

### Python

Python is readable and productive but reliable cross-platform CLI packaging
and interpreter/version management add installation variability that conflicts
with the desired GitHub-to-working-tool experience.

## Decision

Use Go for the CLI and Core.

- `go.mod` will declare `go 1.26.0`.
- Development and release CI will test the latest patched Go 1.26 toolchain
  and the current Go 1.27 toolchain.
- The first vertical slice will use only the Go standard library at runtime.
- A third-party dependency may be added only by a separate ADR that documents
  its necessity, license, maintenance status, and security implications.
- Core packages will not import CLI, adapter, or vendor-specific packages.
- Platform-specific filesystem behavior must be isolated behind narrow
  interfaces and exercised on macOS, Linux, and Windows CI before v1.0.

Go's official release history listed Go 1.27.1 as current on 2026-09-21 and
states that a major release remains supported until two newer major releases
exist. Go 1.26 therefore provides a supported compatibility floor while Go
1.27 provides current-toolchain coverage.

## Consequences

Users can receive small, versioned platform binaries without installing a
language runtime. The project can begin without a dependency lockfile beyond
`go.mod`/`go.sum`, and filesystem tests can use `testing.T.TempDir`.

The repository must keep Go package boundaries strict; choosing a compiled
language does not itself provide transactional safety. Preview/apply drift,
symlink escapes, ownership invariants, and interrupted writes remain explicit
application concerns and release gates.

