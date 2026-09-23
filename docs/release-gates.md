# Release gates and retained evidence

No single test makes a UAWP release complete. The gates below form one chain;
failure at any row blocks the next state.

| Gate | Automation or local command | Evidence |
| --- | --- | --- |
| Source format, tests, race, vet, and workflow contracts | `make check`; CI `native` matrix | GitHub job logs on five runners |
| Immutable workflow dependencies and syntax | `make actionlint` | CI log and pinned SHAs in workflow history |
| Static security analysis | CodeQL `analyze` | GitHub code-scanning result |
| Dependency change risk | Dependency Review `review` | Pull-request check result |
| Reachable Go vulnerabilities | `govulncheck@v1.8.0 ./...`; Go Vulnerability Check `scan` | Workflow log |
| Tag/version/changelog identity | `go run ./cmd/releasecheck --mode prepare ...`; Release `build` | Local approval record and release job log |
| Reproducible archive structure and metadata | `make release-snapshot`; Release `build` | Verified six-asset bundle and build log |
| Exact bundle transfer | Release `build` upload with one-day retention | `release-bundle` artifact and GitHub digest |
| Native checksum and CLI smoke tests | Release `native` five-runner matrix | Five `evidence-<target>` artifacts, one-day retention |
| Aggregate evidence binding | Release `publish` runs `releasecheck --mode publish` | Validated tag/commit/target JSON records |
| Public asset provenance | Release `publish` attestation step | GitHub artifact attestations |
| Human publication authorization | Maintainer creates and pushes a new immutable tag | Signed-in Git actor, tag, and workflow trigger history |
| Public download, checksum, attestation, and execution | Post-release `verify-public-assets` matrix | Five `public-verification-<target>` artifacts retained 30 days |

The public release is not complete until the last row is green. See the
[release procedure](releasing.md), [verification guide](verify-release.md),
and [threat model](security/threat-model.md).
