# UAWP Plan 5: Open-source and Release Hardening Design

**Status:** Human Controller review
**Date:** 2026-09-23
**Product:** UAWP — Universal Agent Workspace Protocol
**Publisher:** Vibemaker™
**Copyright owner:** Li Rui（李锐）
**Target repository:** `https://github.com/vibemaker-community/uawp`

## 1. Purpose

Plan 5 turns the tested local UAWP product into a reproducible, installable,
auditable open-source release. It completes repository governance,
cross-platform verification, supply-chain controls, packaging, installation,
release automation, and post-release validation without weakening any safety
contract established by Plans 1 through 4.5.

The release candidate is acceptable only when every mandatory v1.0 Release
Gate passes against artifacts built from the exact tagged commit. A supported
platform is a tested claim, not merely a cross-compilation target.

## 2. Binding decisions

Plan 5 implements these Human Controller decisions:

- public source license: GNU General Public License v3.0 only,
  SPDX identifier `GPL-3.0-only`;
- external contributions: a Harmony-derived Contributor License Agreement
  accepted electronically through CLA Assistant;
- contributor copyright remains with the contributor, while Li Rui receives
  a perpetual, worldwide, irrevocable, transferable, sublicensable license
  broad enough for open-source, dual-license, and proprietary releases;
- commercial closed-source distribution may later be offered under a separate
  Vibemaker commercial license;
- Vibemaker™ is a trademark of Li Rui（李锐）; registration is pending, so the
  registered mark symbol MUST NOT be used;
- repository owner: `vibemaker-community`;
- repository name: `uawp`;
- Go module path: `github.com/vibemaker-community/uawp`;
- release automation: GitHub Actions plus pinned GoReleaser Community;
- GitHub Actions spending MUST remain zero;
- final public release requires an explicit Human Controller confirmation.

## 3. Scope

### 3.1 Included

- repository and module identity migration;
- licensing, trademark, contribution, conduct, security, and support files;
- protected contribution and release workflows;
- native operating-system CI and bounded fuzz/property testing;
- reproducible multi-platform archives and SHA-256 checksums;
- GitHub artifact provenance for public release artifacts;
- safe macOS/Linux and Windows installation helpers;
- clean-environment artifact smoke tests;
- version, commit, and deterministic build metadata;
- release-candidate, final-release, and post-release procedures;
- a compatibility matrix and a completed Release Gate report.

### 3.2 Excluded

- paid runners, paid signing services, or paid package registries;
- Homebrew Tap, Scoop Bucket, WinGet, apt, rpm, or container publishing in
  v1.0;
- automatic telemetry, installation reporting, or user tracking;
- a hosted UAWP service;
- a formal MCP server or native function-calling Tool surface;
- automatic publication triggered merely by merging to `main`;
- legal claims that the CLA received professional legal review;
- corporate contributors until an Entity CLA is adopted.

Package-manager distribution may be designed after v1.0 using the verified
release archives as its only source.

## 4. Licensing and brand architecture

### 4.1 Public code license

The repository root contains the unmodified GPLv3 license text in `LICENSE`.
Project metadata and documentation use `GPL-3.0-only`, never
`GPL-3.0-or-later`.

Recipients may use, modify, and commercially distribute UAWP under GPLv3. A
party that conveys a covered modified or derivative work must satisfy GPLv3's
corresponding-source and downstream-license obligations. Private internal use
does not itself trigger publication. Ordinary invocation of the standalone
UAWP CLI does not change the license of an unrelated Workspace.

### 4.2 Copyright and notices

`NOTICE` identifies:

```text
UAWP — Universal Agent Workspace Protocol
Copyright 2026 Li Rui（李锐）
Published under the Vibemaker™ brand.
```

Every binary archive contains `LICENSE`, `NOTICE`, `TRADEMARKS.md`, and the
applicable third-party notices. Generated documentation and release notes use
the same owner spelling.

### 4.3 Trademark boundary

`TRADEMARKS.md` states that Vibemaker™ is a trademark of Li Rui（李锐） and
that registration is pending. GPLv3 grants rights to code, not permission to
use the Vibemaker name or logo in a way that implies origin, sponsorship,
certification, or endorsement.

Third parties may make truthful nominative references to UAWP and Vibemaker.
Only explicitly approved distributions may use future “Vibemaker Official” or
“Vibemaker Certified” branding. No registered mark symbol appears until the
registration is granted and the repository is deliberately updated.

### 4.4 CLA and dual licensing

`CLA.md` is based on the Harmony Individual Contributor License Agreement. It
uses a non-exclusive license rather than copyright assignment. Its selected
rights permit sublicensing and release under GPLv3, compatible open-source
licenses, or a separate proprietary commercial license.

CLA Assistant records the GitHub identity, agreement version, and electronic
acceptance. A required status check blocks unaccepted external contributions.
Li Rui and explicitly identified automation accounts are exempt. A contributor
acting for a company is not accepted through the Individual CLA; that case is
paused until an Entity CLA is deliberately adopted.

The project does not claim that the CLA received professional legal review.
The Human Controller accepts the use of a mature template and the associated
legal risk.

## 5. Repository governance

The canonical repository is `vibemaker-community/uawp`. The module declaration,
badges, source links, issue links, security links, release metadata, and build
provenance all use that canonical identity.

The repository includes:

- `CONTRIBUTING.md` for contribution and CLA requirements;
- `CODE_OF_CONDUCT.md` for community behavior;
- `SECURITY.md` for supported versions and private vulnerability reporting;
- `SUPPORT.md` for Issue, Discussion, and support boundaries;
- Issue and Pull Request templates;
- ownership and review rules for release-sensitive files;
- branch protection for `main` after workflows exist remotely.

Merging to `main` requires the native CI matrix and CLA check where applicable.
Force pushes and branch deletion are disabled. Release workflow and licensing
changes receive explicit owner review.

## 6. Supported platform matrix

v1.0 supports exactly these targets:

| Platform | Go target | Archive | Native test environment |
|---|---|---|---|
| macOS Apple Silicon | `darwin/arm64` | `.tar.gz` | standard macOS ARM64 runner |
| macOS Intel | `darwin/amd64` | `.tar.gz` | standard macOS Intel runner |
| Linux x86-64 | `linux/amd64` | `.tar.gz` | standard Ubuntu x64 runner |
| Linux ARM64 | `linux/arm64` | `.tar.gz` | standard Ubuntu ARM64 runner |
| Windows x86-64 | `windows/amd64` | `.zip` | standard Windows x64 runner |

A target is removed from the support matrix if its native tests or packaged
artifact smoke test cannot pass before `v1.0.0`. Cross-compilation alone never
qualifies a target as supported.

## 7. CI architecture

### 7.1 Pull request and main checks

The CI workflow runs on pull requests and pushes to `main`. It uses a native
runner matrix for all five supported targets and performs:

- `go test ./...`;
- `go vet ./...`;
- native `go build` of `cmd/uawp`;
- deterministic CLI smoke tests;
- greenfield and brownfield non-destructive lifecycle scenarios;
- operating-system path and permission cases relevant to that runner.

Linux x86-64 additionally runs the race detector, bounded fuzz seeds, schema
validation, documentation command checks, license checks, and security scans.
Fuzzing in ordinary CI has a fixed time budget; longer fuzz campaigns are a
manual pre-release gate.

### 7.2 Workflow safety

- third-party Actions are pinned to reviewed immutable commit SHAs;
- job permissions default to read-only and are elevated only per release step;
- untrusted pull-request code never receives release credentials;
- jobs have explicit timeouts;
- concurrency cancellation stops superseded branch runs;
- generated artifacts have one-day retention unless attached to a Release;
- workflow logs and artifacts MUST NOT contain credentials or private state.

## 8. Zero-cost enforcement

Plan 5 must not create a GitHub charge.

- only standard GitHub-hosted runners are used;
- larger runners and custom images are prohibited;
- the repository/account Actions budget is set to stop usage at zero paid
  spend;
- private-stage runs remain within included minutes and stop when the included
  allowance is unavailable;
- the full repeated matrix is enabled after the repository is public, where
  standard hosted runners are free;
- caches and temporary artifacts use bounded retention;
- no paid signing, registry, scanning, or CLA service is introduced.

Budget configuration is an account setting and requires the Human Controller's
authenticated confirmation. Workflow configuration cannot override that
external safety control.

## 9. Build and packaging contract

GoReleaser Community is pinned to a reviewed version and invoked by a pinned
GitHub Action. Runtime product code remains Go standard-library-only.

Builds use `CGO_ENABLED=0`, `-trimpath`, deterministic tag-derived values, and
link-time fields for semantic version, Git commit, and tag commit timestamp.
`uawp version` reports those fields; local untagged builds report `dev` without
pretending to be release artifacts.

Canonical asset names are:

```text
uawp_1.0.0_darwin_arm64.tar.gz
uawp_1.0.0_darwin_amd64.tar.gz
uawp_1.0.0_linux_amd64.tar.gz
uawp_1.0.0_linux_arm64.tar.gz
uawp_1.0.0_windows_amd64.zip
uawp_1.0.0_checksums.txt
```

Archives contain one platform binary plus the required legal and concise
installation materials. File ordering, modes, timestamps, and compression
settings are deterministic. Two builds from the same source tag and toolchain
must produce identical SHA-256 values or the release is blocked.

Prebuilt binaries are not committed to the source tree. The current tracked
development binary is removed as part of Plan 5 after reproducible packaging is
available.

## 10. Integrity and provenance

Every candidate and final release includes SHA-256 checksums. Installation
instructions verify the selected archive before extraction or execution.

After the repository is public, GitHub artifact attestations establish the
repository, workflow, commit, and triggering tag that produced each release
asset. Attestation permissions exist only in the release job. Verification
instructions use `gh attestation verify` and also retain a plain SHA-256 path
for users who do not use GitHub CLI.

Artifact provenance proves origin, not absence of vulnerabilities. Security
review and tests remain separate mandatory gates.

## 11. Installation contract

The release provides:

- a manual download, checksum verification, and installation path on every
  supported platform;
- a POSIX installer for macOS/Linux;
- a PowerShell installer for Windows;
- explicit version selection;
- a non-admin destination option;
- no silent PATH, shell profile, registry, or project-file changes.

An installer downloads data before executing it, verifies the expected
checksum, extracts into a temporary directory, checks `uawp version`, and only
then installs the binary. Documentation never recommends an unverified
`curl | sh` pipeline.

Installation helpers affect only the UAWP executable destination. Workspace
initialization remains a separate explicit `uawp init` operation with its own
preview and approval boundary.

## 12. Security hardening

The release review covers:

- path traversal, symlinks, reparse points, and platform path semantics;
- untrusted Markdown, JSON, filenames, native instruction contents, and
  archive entries;
- preview/apply drift and immutable approval binding;
- transaction interruption and recovery;
- namespace collision and project-owned-file preservation;
- ownership, Session, generation, stale recovery, and arbitration;
- installer download, checksum, extraction, destination, and replacement;
- workflow token permissions and release-trigger provenance.

CodeQL, Go vulnerability scanning, dependency review, and secret scanning are
enabled where freely available. Findings classified Critical or High block the
release. A Medium finding requires an explicit disposition in the release
report. No automatic telemetry is introduced.

## 13. Documentation set

The public documentation includes:

- product purpose and safety invariants;
- quick start for people and Agent-driven terminal use;
- how native entry instructions lead an LLM to the UAWP CLI and Core;
- installation and checksum/provenance verification;
- supported platform and Agent adapter matrices;
- lifecycle, identity, ownership, Session, checkpoint, and handoff guidance;
- safe initialization, adapters, upgrade, repair, uninstall, and recovery;
- protocol and adapter authoring references;
- licensing, trademark, CLA, contribution, security, and support policies;
- troubleshooting and complete exit-code behavior;
- release notes and migration guidance.

Every executable documentation command is checked against the release binary
or a controlled fixture. Examples do not silently mutate a real project.

## 14. Release state machine

### 14.1 Private preparation

The canonical repository may begin private. Configuration, limited native CI,
packaging dry runs, license files, and documentation are completed without
publishing a release. Artifact attestations are not required during this stage.

### 14.2 Public release candidate

After private checks pass and the Human Controller authorizes repository
visibility, the repository becomes public. Tag `v1.0.0-rc.1` produces a GitHub
prerelease. Native jobs download their target archive, verify its checksum,
install it in a clean temporary environment, and exercise:

```text
uawp version
uawp init
uawp status
uawp resume
uawp sync
uawp checkpoint
uawp handoff
uawp uninstall
```

Original greenfield and brownfield fixture hashes are compared before and
after the permitted `.uawp/` mutations.

### 14.3 Final release

The final tag `v1.0.0` is created only from a reviewed `main` commit whose
candidate report passes every gate. Creation of the tag and public Release is a
manual, explicit Human Controller action. The release workflow cannot choose a
version or publish from an arbitrary branch.

### 14.4 Post-release verification

Fresh native runners download assets from the public Release URL, verify
checksums and provenance, install, run the smoke lifecycle, and compare the
observed version and commit with the tag. A failure marks the release as
unverified and blocks promotion or announcement until corrected.

## 15. Mandatory v1.0 Release Gates

All earlier product Release Gates remain binding. Plan 5 adds these mandatory
conditions:

1. the repository, module, documentation, and artifacts use the canonical
   `vibemaker-community/uawp` identity;
2. `GPL-3.0-only`, copyright, NOTICE, and Vibemaker™ statements are consistent;
3. CLA policy and status enforcement are active before external code merges;
4. every advertised platform passes native tests and packaged smoke tests;
5. full lifecycle and preservation tests pass on macOS, Linux, and Windows;
6. race, fuzz/property, vulnerability, static, and secret checks pass within
   their stated thresholds;
7. archives are reproducible and contain required legal materials;
8. checksum and provenance verification succeed for every release asset;
9. safe installers never silently change shell, PATH, registry, or Workspace
   files;
10. documentation command verification passes;
11. Actions paid spend is disabled and no paid dependency exists;
12. the release report contains no unresolved Critical or High issue;
13. `v1.0.0` is created only after explicit Human Controller approval;
14. post-release installation from public artifacts reproduces candidate
    results.

Any known silent-overwrite, destructive-uninstall, ownership-bypass,
approval-drift, archive-extraction, or credential-exposure risk is an automatic
release blocker.

## 16. Human Controller interactions

Implementation is designed so the Human Controller performs only authenticated
external actions that cannot safely be delegated:

- sign in to GitHub or complete two-factor/passkey verification if requested;
- authorize CLA Assistant once;
- confirm zero-paid-spend budget settings;
- authorize changing repository visibility to public;
- explicitly approve creation of the final public `v1.0.0` release.

No lawyer, email negotiation, paid service, or manual per-contributor approval
is required by this design. GitHub may independently require an account-security
email or authentication step; the project cannot bypass that control.

## 17. Failure and rollback behavior

- CI failure blocks merge or release and preserves its logs.
- Packaging failure creates no partial Release.
- A candidate failure is corrected under a later `-rc.N` tag; tags are not
  silently moved.
- A final release asset is never replaced under the same version. A corrected
  release uses a new semantic version and documents the superseded release.
- A failed visibility, CLA, branch-protection, or budget configuration step
  pauses external publication without changing local source history.
- Existing GPL releases remain available under GPLv3 even if future versions
  use a different licensing or distribution strategy.

## 18. Acceptance criteria

This design phase is accepted when:

1. the Human Controller confirms the repository identity, licensing, brand,
   supported platforms, zero-cost policy, CI architecture, and release gates;
2. this specification contains no unresolved product or technology choice;
3. the specification is committed without modifying production or release
   behavior;
4. the Human Controller reviews the committed file and approves preparation of
   the Plan 5 implementation plan.
