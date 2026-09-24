# UAWP Open-Source Release Hardening Implementation Plan

> **For Codex:** REQUIRED SKILL: Use superpowers:executing-plans to implement this plan task by task. Apply superpowers:test-driven-development to every behavior change, superpowers:systematic-debugging to failures, and superpowers:verification-before-completion before any completion claim.

**Goal:** Turn the current UAWP repository into a production-quality, GPL-3.0-only project that can be safely published at `github.com/vibemaker-community/uawp`, installed on five supported native targets, verified by users, and released without paid GitHub services.

**Architecture:** Preserve the Agent-neutral Go core and CLI. Add deterministic build metadata, reproducible multi-platform packages, checksum-verifying installers, native CI, security scanning, release evidence, and public governance documents around the existing implementation. GitHub release publication remains a distinct human-confirmed step after local and hosted release gates pass.

**Tech Stack:** Go 1.26, GitHub Actions standard hosted runners, GoReleaser Community v2.18.0, GitHub artifact attestations, shell and PowerShell installers, Markdown documentation.

**Approved design:** `docs/superpowers/specs/2026-09-23-open-source-release-hardening-design.md`

---

## Global Constraints

- Keep the Core Agent-neutral. Provider-specific behavior remains behind adapters.
- Preserve `.uawp/` as the only UAWP-owned workspace namespace.
- Preserve non-destructive integration and the `DIRECT`, `IMPORT`, and `MANAGED_BLOCK` modes.
- Do not claim support for a provider unless its adapter behavior is grounded in that provider's official documentation and covered by tests.
- Do not modify existing project-owned or Agent-native content outside an explicitly approved managed block.
- Do not add telemetry, hosted services, an MCP server, package-manager distribution, parallel workspace writers, or automatic checkpoint rollback in this release.
- Use GPL-3.0-only for public source distribution. External contributions require the approved individual CLA.
- Show `Vibemaker™`, never `Vibemaker®`, until registration is complete.
- Use standard GitHub-hosted runners only. Configure the organization or repository spend limit to zero before hosted validation.
- Never publish a repository, release candidate, final tag, or final release without the user's explicit confirmation at the corresponding gate.
- Preserve the untracked `.DS_Store`; do not add, modify, or delete it.

## Release Targets

| Platform | Go target | GitHub runner | Archive |
|---|---|---|---|
| macOS Apple Silicon | `darwin/arm64` | `macos-15` | `.tar.gz` |
| macOS Intel | `darwin/amd64` | `macos-15-intel` | `.tar.gz` |
| Linux x86-64 | `linux/amd64` | `ubuntu-24.04` | `.tar.gz` |
| Linux ARM64 | `linux/arm64` | `ubuntu-24.04-arm` | `.tar.gz` |
| Windows x86-64 | `windows/amd64` | `windows-2025` | `.zip` |

## Review Focus

The implementation review must explicitly look for these failure classes:

1. A stale module path or public URL survives the repository identity migration — Tasks 1 and 8.
2. Two builds from the same commit produce different archives or checksums — Tasks 3 and 9.
3. An installer accepts a bad checksum, unsafe archive entry, or overwrites an existing binary — Task 4.
4. A workflow can execute untrusted code with write permissions, uses a mutable action reference, or consumes a paid runner — Tasks 5, 6, and 7.
5. A release can become public without the required human confirmation or without native smoke-test evidence — Tasks 7, 10, and 11.

---

## Task 1: Establish Canonical Identity, License, and Governance

**Files:**

- Modify: `go.mod`
- Modify: every `.go` file importing `github.com/uawp/uawp/...`
- Create: `LICENSE`
- Create: `NOTICE`
- Create: `TRADEMARKS.md`
- Create: `CLA.md`
- Create: `CONTRIBUTING.md`
- Create: `CODE_OF_CONDUCT.md`
- Create: `SECURITY.md`
- Create: `SUPPORT.md`
- Create: `.github/CODEOWNERS`
- Create: `.github/ISSUE_TEMPLATE/bug_report.yml`
- Create: `.github/ISSUE_TEMPLATE/feature_request.yml`
- Create: `.github/pull_request_template.md`
- Create: `internal/releasecontract/identity_test.go`

### Step 1: Write the failing identity contract test

The test must walk tracked public text files and assert:

- `go.mod` declares `module github.com/vibemaker-community/uawp`.
- no Go import contains `github.com/uawp/uawp`.
- public links use `https://github.com/vibemaker-community/uawp`.
- `LICENSE` is the exact GNU GPL version 3 text and has SHA-256 `3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986`.
- `NOTICE` contains `Copyright 2026 Li Rui（李锐）` and `Vibemaker™`.
- no tracked file uses `Vibemaker®`.

Run `go test ./internal/releasecontract -run TestCanonicalIdentity`; verify it fails on the old module path.

### Step 2: Apply the canonical identity migration

Change the module path and all internal imports in one patch. Run `go mod tidy`, then run `go test ./...` to detect stale imports.

### Step 3: Add legal and governance documents

Use the exact GPL-3.0-only license text. State that:

- the public project is licensed under GPL-3.0-only;
- contributors retain copyright and grant the rights described in `CLA.md`;
- Li Rui may offer separately negotiated proprietary licenses;
- `Vibemaker™` is a pending trademark and the open-source license does not grant trademark rights;
- company contributions are not accepted until an Entity CLA is published;
- security reports use a private channel documented in `SECURITY.md`.

The CLA must be a Harmony-derived individual agreement that grants Li Rui perpetual, worldwide, irrevocable, transferable, sublicensable copyright and patent rights sufficient for GPL distribution, dual licensing, and proprietary releases.

### Step 4: Add contribution intake templates

Issue forms must request reproduction details without collecting secrets. The pull request template must require tests, documentation impact, non-destructive behavior review, provider documentation evidence for adapter changes, and CLA completion.

### Step 5: Verify and commit

Run:

```sh
go test ./internal/releasecontract -run TestCanonicalIdentity
go test ./...
git diff --check
```

Commit as `chore: establish public project identity and governance`.

---

## Task 2: Add Deterministic Build and Version Metadata

**Files:**

- Create: `internal/buildinfo/info.go`
- Create: `internal/buildinfo/info_test.go`
- Create: `internal/cli/version.go`
- Create: `internal/cli/version_test.go`
- Modify: `internal/cli/run.go`
- Modify: `Makefile`

### Step 1: Define failing version-output tests

Test both human and JSON output. The public metadata fields are:

```go
type Info struct {
    Version string `json:"version"`
    Commit  string `json:"commit"`
    BuiltAt string `json:"builtAt"`
}
```

Required behavior:

- an untagged developer build reports `version=dev`, `commit=unknown`, and an empty `builtAt`;
- a release build reports the injected semantic version without a leading `v`;
- JSON output is stable and machine-readable;
- no runtime clock value is used as a fallback.

Run `go test ./internal/cli -run Version`; verify failure against the hardcoded `uawp dev` implementation.

### Step 2: Implement build metadata

Expose package variables that Go linker flags can set. Keep default values deterministic. Route the CLI's `version` command through `buildinfo.Info` and support the existing global `--format json` behavior.

### Step 3: Add build commands

Add a Make target that injects:

```text
Version=<tag without v>
Commit=<full git commit>
BuiltAt=<SOURCE_DATE_EPOCH rendered as UTC RFC3339>
```

The build must fail if a release tag and the version passed to the linker disagree.

### Step 4: Verify and commit

Run `go test ./internal/buildinfo ./internal/cli` and build twice with identical injected values. Confirm the two executables have identical SHA-256 values. Commit as `feat(cli): add deterministic version metadata`.

---

## Task 3: Produce Reproducible Release Archives

**Files:**

- Create: `.goreleaser.yaml`
- Create: `scripts/verify-release.go`
- Create: `scripts/verify-release_test.go`
- Modify: `.gitignore`
- Delete: `bin/uawp`
- Modify: `Makefile`

### Step 1: Write failing archive-contract tests

The verifier must reject:

- missing or extra target archives;
- unexpected archive names;
- duplicate checksum entries;
- an archive whose checksum does not match;
- a binary whose embedded version differs from the release version;
- archive entries with absolute paths, `..`, symlinks, hard links, or device entries;
- a Windows archive without `uawp.exe` or a Unix archive without `uawp`.

Expected assets for version `1.0.0` are exactly:

```text
uawp_1.0.0_darwin_arm64.tar.gz
uawp_1.0.0_darwin_amd64.tar.gz
uawp_1.0.0_linux_amd64.tar.gz
uawp_1.0.0_linux_arm64.tar.gz
uawp_1.0.0_windows_amd64.zip
checksums.txt
```

Run `go test ./scripts -run Release`; verify the tests fail before the verifier exists.

### Step 2: Configure GoReleaser Community

Pin GoReleaser to `v2.18.0` in automation. Configure `CGO_ENABLED=0`, the five targets above, deterministic archive ownership and modification timestamps, GPL/NOTICE/README inclusion, and SHA-256 checksums. Do not publish from the GoReleaser command; it only builds the verified bundle.

### Step 3: Implement strict verification

The verifier takes `--dist`, `--version`, and `--commit`, parses archives without extracting them, validates checksums and archive safety, executes only the native binary when appropriate, and reads cross-platform build metadata through a generated metadata manifest.

### Step 4: Remove generated binaries from source control

Delete the tracked `bin/uawp`. Ignore `/bin/`, `/dist/`, root-level `uawp`, and root-level `uawp.exe`. Keep source scripts tracked.

### Step 5: Prove reproducibility

Run GoReleaser twice from the same clean commit with identical `SOURCE_DATE_EPOCH`, save each `checksums.txt`, and compare both the checksum files and every asset digest. A mismatch blocks the release.

### Step 6: Verify and commit

Run `go test ./scripts`, `make check`, and the two-build reproducibility check. Commit as `build: add reproducible release archives`.

---

## Task 4: Add Checksum-Verifying Installers

**Files:**

- Create: `install/install.sh`
- Create: `install/install.ps1`
- Create: `internal/installtest/server_test.go`
- Create: `internal/installtest/unix_test.go`
- Create: `internal/installtest/windows_test.go`
- Create: `docs/install.md`

### Step 1: Build local malicious and valid fixtures

Use `httptest.Server` to serve release metadata, archives, and checksums. Fixtures must cover a valid release, wrong digest, missing digest, traversal entry, symlink entry, duplicate binary, unsupported platform, unavailable network response, and existing destination binary.

### Step 2: Write failing installer tests

Tests must prove that both installers:

- choose only an explicitly supported platform asset;
- download the checksum file and require an exact filename match;
- hash the archive before extraction;
- reject unsafe archive entries;
- install through a temporary directory and atomic rename;
- refuse to overwrite an existing binary unless the user supplies an explicit force flag;
- leave the prior binary untouched after any failure;
- do not edit shell profiles, `PATH`, the Windows registry, or system package databases.

### Step 3: Implement the Unix installer

Use POSIX-compatible shell where practical. Default to a user-owned installation directory, print the exact chosen path, require HTTPS for non-test URLs, and support a test-only base URL through an explicit environment variable documented as non-public behavior.

### Step 4: Implement the PowerShell installer

Require TLS, use `Get-FileHash -Algorithm SHA256`, validate zip entry paths before extraction, and preserve the same overwrite semantics as Unix.

### Step 5: Document installation and uninstall

Document direct archive installation, verification, installer use, force behavior, and complete uninstall. State clearly that installers do not change `PATH` automatically.

### Step 6: Verify and commit

Run installer tests locally on macOS and run shell syntax validation. Windows behavior remains gated by the native Windows CI task. Commit as `feat(install): add verified non-destructive installers`.

---

## Task 5: Add Native CI and Fuzz Gates

**Files:**

- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/fuzz.yml`
- Create: `internal/releasecontract/workflows_test.go`
- Modify: `Makefile`

### Step 1: Write failing workflow contract tests

Parse all workflow files and require:

- every third-party action uses a full 40-character commit SHA;
- no workflow uses `pull_request_target`;
- pull-request jobs have read-only permissions;
- no runner label contains `larger`, `xlarge`, or a self-hosted label;
- timeouts are present;
- shell commands do not interpolate untrusted PR titles, bodies, labels, or branch names;
- the native matrix contains exactly the five approved runners.

### Step 2: Add the native test workflow

Use immutable references:

- checkout: `d23441a48e516b6c34aea4fa41551a30e30af803`
- setup-go: `40f1582b2485089dde7abd97c1529aa768e1baff`

Each native job runs formatting verification, `go test ./...`, `go test -race ./...`, `go vet ./...`, a local build, `uawp version --format json`, and representative `init`, `resume`, `sync`, `checkpoint`, and `handoff` smoke tests in a temporary workspace.

### Step 3: Add bounded fuzzing

Run existing fuzz targets for a fixed duration on Ubuntu. Fail on a new crashing corpus. Keep the scheduled duration short enough to stay within free standard-runner usage.

### Step 4: Add local workflow validation

Pin actionlint `v1.7.7` in the Make target and document the exact checksum or verified installer used to obtain it. Run workflow contract tests even when actionlint is unavailable.

### Step 5: Verify and commit

Run `make check`, workflow contract tests, and actionlint. Commit as `ci: add native test and fuzz gates`.

---

## Task 6: Add Security Analysis and Permission Boundaries

**Files:**

- Create: `.github/workflows/codeql.yml`
- Create: `.github/workflows/dependency-review.yml`
- Create: `.github/workflows/govulncheck.yml`
- Create: `docs/security/threat-model.md`
- Modify: `internal/releasecontract/workflows_test.go`

### Step 1: Extend failing workflow tests

Require top-level `permissions: contents: read` and test each exception. Only the final release job may request `contents: write`, `id-token: write`, or `attestations: write`. No PR-triggered job may have those permissions.

### Step 2: Add pinned security workflows

Use these immutable action commits:

- CodeQL: `1c5b675653bb5c22dbe9b12b556ec555138e09fd`
- dependency review: `2031cfc080254a8a887f58cffee85186f0e49e48`
- govulncheck: `032d45514ae346b1db93c04b0c90b841c370344f`

CodeQL runs on pushes, pull requests, and a bounded schedule. Dependency review runs only on pull requests. Govulncheck runs on pushes, pull requests, and before release.

### Step 3: Document the threat model

Cover workspace file overwrite, managed-block spoofing, stale ownership takeover, archive traversal, checksum substitution, workflow token misuse, compromised dependencies, mutable release assets, and unauthorized release publication. For each threat, link to its control and verification command.

### Step 4: Verify and commit

Run security workflow contract tests, `govulncheck ./...`, CodeQL workflow syntax validation, and `git diff --check`. Commit as `security: add analysis gates and threat model`.

---

## Task 7: Implement a Human-Gated Release Pipeline

**Files:**

- Create: `cmd/releasecheck/main.go`
- Create: `cmd/releasecheck/main_test.go`
- Create: `.github/workflows/release.yml`
- Create: `.github/workflows/post-release.yml`
- Create: `docs/releasing.md`
- Modify: `internal/releasecontract/workflows_test.go`

### Step 1: Write failing tag and release-state tests

The release checker must reject:

- malformed tags;
- a tag version inconsistent with build metadata;
- final `v1.0.0` when the changelog still identifies the release as a candidate;
- a moved or pre-existing tag;
- missing native smoke-test results;
- absent checksum verification evidence.

Accept `v1.0.0-rc.1` and `v1.0.0`, with later corrections using increasing RC numbers rather than moving a tag.

### Step 2: Build once, test everywhere

The release workflow must:

1. validate the tag and repository state;
2. run all source, security, and governance checks;
3. invoke GoReleaser with `--skip=publish` exactly once;
4. verify the generated bundle;
5. upload one short-retention transfer artifact;
6. download that identical artifact on all five native runners;
7. verify checksums and run native CLI smoke tests;
8. aggregate signed evidence;
9. create attestations for public-release assets;
10. create the GitHub release only after every gate succeeds.

Use immutable references for artifact actions:

- upload: `ea165f8d65b6e75b540449e92b4886f43607fa02`
- download: `634f93cb2916e3fdff6788551b99b062d0335ce0`
- attest: `96278af6caaf10aea03fd8d33a09a777ca52d62f`
- GoReleaser: `f06c13b6b1a9625abc9e6e439d9c05a8f2190e94`

Set transfer-artifact retention to one day. Do not use GitHub environments as the sole human gate because the project must remain usable on a zero-cost account; the human gate is explicit user confirmation before the local tag is created and pushed.

### Step 3: Add public post-release verification

After publication, download assets from the public release URL on each native runner, verify checksums and attestations, execute the installed binary, and save the evidence summary. Failure must mark the release verification as failed and block any claim that release is complete.

### Step 4: Document recovery

Document failure behavior for pre-tag, post-tag/pre-publish, and post-publish failures. Never move or reuse a public tag. A faulty candidate is superseded by the next RC; a faulty final release receives a new patch version and a clear advisory.

### Step 5: Verify and commit

Run release checker tests, workflow contract tests, actionlint, and a local candidate dry run. Commit as `release: add human-gated native release pipeline`.

---

## Task 8: Write the Public Product Documentation

**Files:**

- Rewrite: `README.md`
- Create: `CHANGELOG.md`
- Create: `ROADMAP.md`
- Create: `docs/architecture.md`
- Create: `docs/supported-platforms.md`
- Create: `docs/verify-release.md`
- Create: `docs/agent-integration.md`
- Create: `docs/release-gates.md`
- Create: `internal/releasecontract/docs_test.go`

### Step 1: Write failing documentation tests

Check internal Markdown links, canonical GitHub URLs, command names, supported platforms, license identifier, brand spelling, and referenced files. Extract fenced shell commands marked as testable and execute them in temporary workspaces.

### Step 2: Rewrite the README around user outcomes

The README must include:

- what UAWP solves and what it does not solve;
- Agent-neutral Core and adapter boundaries;
- `.uawp/` namespace ownership;
- non-destructive integration modes;
- single active worker and human arbitration of stale ownership;
- installation, quick start, and uninstall;
- ordinary user, expert, and Agent-automation interaction modes;
- license, CLA, trademark, security, and support links;
- verified platform status and current release maturity.

### Step 3: Explain Agent-driven CLI operation

In `docs/agent-integration.md`, explain that the Agent loads its native project entry instructions, those instructions direct it to `.uawp/INSTRUCTIONS.md`, the LLM interprets the user's natural-language request, and the Agent executes the UAWP CLI through its terminal capability. Clarify that UAWP does not read hidden provider conversation memory and `sync --context-file` imports an explicit Agent-prepared file.

### Step 4: Document release verification and gates

Show checksum and attestation verification for every supported OS. `docs/release-gates.md` must map every mandatory gate to its workflow job or local command and state what evidence is retained.

### Step 5: Verify and commit

Run documentation contract tests, all README examples, link checking, and `git diff --check`. Commit as `docs: publish complete user and release documentation`.

---

## Task 9: Perform Local Whole-Branch Verification and Independent Review

**Files:**

- Create: `docs/release-evidence/local-prepublish.md`
- Modify: files found defective during review

### Step 1: Start from a clean tracked tree

Record the commit SHA, Go version, GoReleaser version, actionlint version, operating system, and `SOURCE_DATE_EPOCH`. The only permitted unrelated untracked file is the existing `.DS_Store`.

### Step 2: Run the complete local gate

Run formatting checks, unit tests, integration tests, race tests, vet, vulnerability scan, fuzz smoke tests, workflow contract tests, actionlint, documentation tests, installer fixture tests, release dry run, archive verification, and two-build reproducibility comparison.

### Step 3: Review the branch independently

Use `superpowers:requesting-code-review`. The reviewer must examine the five failure classes in Review Focus, the approved design, every changed file, and the actual verification output. Resolve all high- and medium-severity findings through tests before proceeding.

### Step 4: Record evidence and commit

Write commands, versions, pass/fail results, artifact checksums, and resolved review findings to the local prepublication evidence file. Commit as `test: record local prepublication evidence`.

---

## Task 10: Bootstrap the Private GitHub Repository and Hosted Gates

**External state:** `github.com/vibemaker-community/uawp`

### Step 1: Stop for explicit user confirmation

Present the exact repository name, owner, initial private visibility, remote URL, and actions that will occur. Do not create or push anything until the user confirms.

### Step 2: Authenticate and create the private repository

The user may need to complete GitHub login or two-factor authentication. After authentication, create `vibemaker-community/uawp` as private, add `origin`, and push `main` without force.

### Step 3: Enforce zero-cost controls before workflows run

Have the user set the applicable GitHub Actions spending limit to zero. Verify that workflows use only standard hosted runners and no paid marketplace service. If the account cannot provide a zero-charge guarantee, stop before enabling hosted runs.

### Step 4: Configure repository protections

Enable branch protection or rulesets for `main`: pull requests, required native/security checks, no force pushes, no deletions, and dismissal of stale approvals. Configure security reporting and Dependabot only where it does not create a charge.

### Step 5: Configure CLA Assistant

The user completes the CLA Assistant OAuth authorization if prompted. Link it to `CLA.md`, require its status on external pull requests, and verify using a harmless test pull request. Do not accept company contributions under the individual CLA.

### Step 6: Run hosted validation while private

Run native CI, security workflows, fuzzing, installer tests, and a release dry run without publishing a release. Resolve every failure locally, push normally, and rerun. Record run URLs and immutable commit SHA in `docs/release-evidence/hosted-prepublish.md`.

### Step 7: Commit hosted evidence

Commit as `test: record hosted prepublication evidence` and rerun required checks on that exact commit.

---

## Task 11: Publish the Repository, Release Candidate, and Final v1.0.0

**Files:**

- Modify: `CHANGELOG.md`
- Create: `docs/release-evidence/v1.0.0-rc.1.md`
- Create: `docs/release-evidence/v1.0.0.md`

### Step 1: Stop for repository-publication confirmation

Show the complete gate report, unresolved risks, license/CLA summary, exact public URL, and the irreversible effect of making the source public. Change visibility only after explicit confirmation.

### Step 2: Verify public-repository controls

Confirm public standard runners are available without charge under current GitHub terms, artifact attestations are enabled, required checks still apply, security reporting works, and all public links resolve.

### Step 3: Prepare the release candidate

Update the changelog for `v1.0.0-rc.1`, run the complete local gate, push the commit, wait for all hosted checks, and present the exact tag command and release consequences.

### Step 4: Stop for release-candidate confirmation

Create and push `v1.0.0-rc.1` only after explicit confirmation. Wait for the build-once/native-smoke/publish pipeline and public post-release verification. Record release URL, checksums, attestations, job URLs, and findings.

### Step 5: Stabilize without moving tags

Fix defects on `main`. If another candidate is required, use `v1.0.0-rc.2` or a later increasing candidate. Repeat all gates.

### Step 6: Prepare the final release

Update the changelog from candidate to final, rerun the complete local and hosted gates, and produce a final release report for the exact commit.

### Step 7: Stop for final-release confirmation

Create and push `v1.0.0` only after the user explicitly approves that exact commit. Wait for all release and post-release jobs.

### Step 8: Close the release only on verified evidence

The release is complete only when all five public assets install and execute on their native runners, checksums and attestations verify, documentation URLs resolve, and the final evidence document is committed. Use `superpowers:verification-before-completion` before reporting completion.

---

## Final Acceptance Criteria

- The canonical source is `github.com/vibemaker-community/uawp`; no stale public identity remains.
- License, copyright, trademark, CLA, contribution, security, and support terms are internally consistent.
- The same commit produces byte-identical release assets under the documented environment.
- All five supported target archives exist, pass checksum and archive-safety verification, and run on native GitHub runners.
- Installers verify integrity, preserve an existing binary by default, and leave no partial installation after failure.
- CI and release workflows use immutable action SHAs, minimum permissions, timeouts, standard hosted runners, and zero paid services.
- No public release can occur before explicit human confirmation and successful native validation.
- Public documentation accurately explains Agent-driven CLI use, `.uawp/` ownership, adapters, supported modes, limitations, and verification.
- A release candidate completes successfully before final `v1.0.0`.
- Final completion is backed by committed local, hosted, candidate, and final evidence.
