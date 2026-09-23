# UAWP Threat Model

UAWP operates inside existing workspaces, where preserving user files and
maintaining explicit human control are more important than convenience. This
model covers the command-line tool, its adapter integrations, installers, and
the GitHub release path. It does not claim to defend a machine after its local
user account or an already-running Agent has been fully compromised.

| Threat | Control | Verification |
| --- | --- | --- |
| Existing workspace files are silently overwritten | Every mutation is previewed, bound to the observed inputs, and requires explicit approval; UAWP-owned state stays under `.uawp/`. | `go test ./internal/plan ./internal/transaction ./internal/e2e` |
| A project file spoofs or corrupts a UAWP managed block | Managed blocks use validated boundaries, reject malformed or duplicate markers, and preserve content outside the owned block. | `go test ./internal/adapter` |
| A stale owner is taken over automatically or by a worker | ACTIVE ownership has no TTL; recovery requires a separate Human Controller authorization bound to the observed ownership generation and is recorded in the audit history. | `go test ./internal/core ./internal/workspace -run 'Recover|Stale|Ownership'` |
| A release archive writes outside the installation directory | Installers reject absolute paths, parent traversal, links, unexpected members, and unsafe duplicates before extraction. | `go test ./internal/installtest` |
| A substituted binary is installed under a trusted filename | Installers download the versioned checksum manifest separately and require exactly one matching SHA-256 entry before extraction. | `go test ./internal/installtest -run 'Checksum|Installer'` |
| A workflow token is abused by untrusted pull-request content | Workflows reject `pull_request_target`, default to `contents: read`, pin third-party actions by full commit, and reserve release write permissions for the release workflow. | `go test -count=1 ./internal/releasecontract -run 'TestWorkflowSecurityContract|TestSecurityWorkflowsUsePinnedAnalyzers'` |
| A compromised dependency reaches users | Pull requests receive dependency review; CodeQL and `govulncheck` scan pushes and pull requests; release validation invokes the vulnerability workflow again. | `GOPROXY=https://proxy.golang.org,direct go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...` |
| Release assets are changed between platform tests and publication | One reproducible bundle is built, checksummed, transferred to native smoke tests, and published only from that same verified bundle. Public assets receive provenance attestations. | `make release-snapshot` |
| A release is published without the owner's decision | Creating and pushing the immutable release tag is a local Human Controller gate; the hosted workflow cannot invent that authorization and publishes only after all evidence gates pass. | Follow `docs/releasing.md` and inspect the release workflow evidence before creating the tag. |

## Trust boundaries

- Project content is untrusted input. UAWP owns only `.uawp/` and the exact
  adapter block or file recorded in its integration plan.
- Agent-native entry files remain owned by the project and their Agent vendor;
  UAWP edits them only through an approved, previewed adapter plan.
- GitHub-hosted runners and pinned actions are external services. Least-privilege
  workflow permissions limit the effect of an action compromise.
- Checksums detect accidental or malicious substitution relative to the
  published checksum manifest. Provenance attestations additionally bind
  public assets to the repository workflow that produced them.

Security reports should follow [the security policy](../../SECURITY.md).
