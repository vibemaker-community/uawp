# Hosted prepublication evidence

Date: 2026-09-24 (Asia/Shanghai)

This record covers private, non-publishing GitHub-hosted verification of UAWP.
It does not authorize public visibility, a tag, or a GitHub Release.

## Repository and immutable source

| Item | Observed value |
| --- | --- |
| Repository | `vibemaker-community/uawp` |
| Visibility during these runs | Private |
| Branch | `main` |
| Commit | `939b7e61351b5d5f5d0b6c644827d17336cbe22a` |
| Commit subject | `test: finish Windows portability` |

The repository was created under the maintainer's personal GitHub account and
the local `main`, remote `main`, and validation commit were confirmed to match
before the hosted gate was recorded.

## Zero-cost and repository controls

GitHub billing controls were inspected before hosted workflows ran. Actions,
Packages, Codespaces, Git LFS, and AI credits each had a zero-dollar budget
with usage stopped at the limit. The workflows use standard GitHub-hosted
runners and no paid marketplace service.

GitHub Free does not enforce rulesets on this private personal-account
repository. Enforceable `main` protection is therefore a Task 11 publication
gate rather than a claim of this private record. Force pushes and destructive
history changes were not used during private validation.

## Hosted validation

The following runs target immutable commit
`939b7e61351b5d5f5d0b6c644827d17336cbe22a`:

| Gate | Result | Evidence |
| --- | --- | --- |
| Native CI | Passed on Linux, macOS Intel, macOS Arm, and Windows | <https://github.com/vibemaker-community/uawp/actions/runs/35930810378> |
| Go vulnerability check | Passed | <https://github.com/vibemaker-community/uawp/actions/runs/35930810385> |
| Bounded fuzzing | Passed | <https://github.com/vibemaker-community/uawp/actions/runs/35930810350> |
| CodeQL | Skipped while private by the explicit private-repository guard | <https://github.com/vibemaker-community/uawp/actions/runs/35930810460> |

CodeQL is unavailable for this private GitHub Free repository. Its workflow is
configured to become mandatory after public visibility; this record does not
claim private CodeQL coverage. Private security evidence instead includes the
full native suite, `go vet`, `govulncheck`, and bounded fuzzing.

Earlier failed CI runs were diagnostic executions that exposed and then drove
the Windows path, executable-suffix, and short-path portability fixes. They are
superseded by the passing run above and are not represented as release gates.

## Release rehearsal without publication

The read-only `Release Dry Run` workflow builds all release archives, verifies
the archive allowlist, checksums and embedded metadata, and runs the packaged
Linux lifecycle smoke test. It cannot create a tag or GitHub Release and does
not retain release artifacts.

The first rehearsal passed while the portability fixes were being finalized:
<https://github.com/vibemaker-community/uawp/actions/runs/35930505136>.

A second rehearsal was manually dispatched for the immutable commit recorded
above and passed in 1 minute 37 seconds:
<https://github.com/vibemaker-community/uawp/actions/runs/35931911587>.

That successful run emitted two non-fatal setup warnings: the pinned
`actions/setup-go` v5 runtime targeted deprecated Node.js 20, and its cache
logic searched for an absent `go.sum`. The evidence commit upgrades every
workflow reference to immutable `actions/setup-go` v7.0.0, whose Node.js 24
runtime and `go.mod` cache key address both root causes. The workflow contract
now rejects any stale setup-go reference; required hosted checks are rerun on
the evidence commit before Task 10 closes.

## CLA and deferred public-only controls

CLA Assistant is installed with repository access restricted to
`vibemaker-community/uawp`. Its hosted repository picker did not enumerate the
private repository even after installation, refresh, and reauthentication,
while it did enumerate the account's public repository. The CLA Gist link and
harmless pull-request test are therefore deferred to the Task 11 public gate.

No external contribution will be accepted and no release will be published
until all of the following are verified after visibility changes:

1. CLA Assistant links the repository to the canonical individual CLA.
2. A harmless external-style pull request receives and passes the CLA status.
3. `main` protection requires the native, security, and CLA checks.
4. Public CodeQL and dependency-review gates pass.

If CLA Assistant still cannot link the public repository, publication remains
blocked until a repository-local CLA Action replacement is reviewed and
verified.

## Publication boundary

No tag or GitHub Release was created by these runs. Public visibility, branch
protection, public security gates, release-candidate publication, final release
promotion, and post-release installation evidence remain Task 11 gates that
require explicit maintainer confirmation.
