# UAWP Roadmap

The roadmap communicates direction, not a promise of dates. Safety invariants
remain binding across every phase.

## v1.0 release candidate

- Run CI, security, release, installer, and post-release verification on all
  five supported GitHub-hosted runner targets.
- Publish `v1.0.0-rc.1` only after explicit maintainer approval.
- Collect real user feedback on greenfield and brownfield Workspaces.
- Correct candidate defects with increasing RC numbers; never move a tag.

## v1.0 stable

- Close every mandatory [release gate](docs/release-gates.md).
- Publish a final compatibility and evidence summary.
- Keep state migration and non-destructive uninstall verified from the latest
  supported candidate.

## Later releases

- Add new Agent adapters only from current official vendor evidence.
- Consider MCP or native Tool façades over the same typed Core; the CLI remains
  a stable boundary and no façade may bypass preview, ownership, or approval.
- Evaluate optional isolated parallel-work patterns. Same-directory parallel
  writing is explicitly out of scope until a separately reviewed design can
  preserve UAWP's safety model.
- Extend package-manager distribution only when checksum, provenance, upgrade,
  and uninstall behavior can be verified without weakening current controls.
