# Adapter authoring contract

An adapter translates documented provider behavior into a deterministic
`adapter.Resolution`. It does not write files. Workspace planning is the only
layer allowed to turn a resolution into immutable changes, and Workspace apply
is the only layer allowed to publish an approved plan.

## Requirements

An adapter must:

- have a stable lowercase provider ID and dated official-document evidence;
- inspect only declared project candidates and observable runtime facts;
- classify every candidate and select one effective route deterministically;
- fail closed when version, configuration, environment, entry safety, or
  precedence cannot be established;
- select only `DIRECT`, `IMPORT`, or `MANAGED_BLOCK` from Core;
- target `.uawp/INSTRUCTIONS.md`, never duplicate canonical instructions;
- emit stable finding codes and an actionable next step;
- avoid provider filesystem writes, user/global configuration writes, and
  semantic synchronization between native files.

Core must remain provider-neutral: do not add provider IDs, native filenames,
or precedence rules to `internal/core`. Provider resolution belongs in
`internal/adapter`; discovery, plan inputs, registry consumers, approval,
drift checks, atomic publication, verification, and recovery belong in
`internal/workspace`; CLI only parses facts and renders results.

## Conformance evidence

Add table-driven fixtures for every documented combination of candidate files,
including both/neither, empty files, precedence changes, version/configuration
branches, unknown facts, unsafe file types, and a registered route that later
drifts. Tests must also cover idempotent installation, shared-consumer removal,
last-consumer removal, user-byte preservation, corrupt ownership markers,
preview drift, and interrupted publication.

Never claim compatibility from observed convenience alone. A new capability
requires a current official source, a recorded verification date and version
scope, a reviewed resolution matrix, tests, and user documentation.
