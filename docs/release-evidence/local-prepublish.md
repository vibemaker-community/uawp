# Local prepublication evidence

Date: 2026-09-23 (Asia/Shanghai)

This record covers the local, non-publishing verification of UAWP Plan 5 after
the independent review fixes. It is not hosted-native or public-release
evidence; those gates remain mandatory before any release is described as
complete.

## Source and tools

| Item | Observed value |
| --- | --- |
| Commit | `91af4b63beba9cf1eb120dbf992c3d70c96d25a8` |
| Tracked tree | Clean before the gate |
| Go | `go1.26.8 darwin/arm64` |
| GoReleaser | `v2.18.0`, executed with `go1.27.1 darwin/arm64` |
| actionlint | `v1.7.7`, built with `go1.26.8 darwin/arm64` |
| Operating system | macOS `15.0.1` (`24A348`), arm64 |
| `SOURCE_DATE_EPOCH` | `1790176784` |

The host exported `C.UTF-8`, which its Perl runtime does not support. Artifact
digest reporting therefore used `LC_ALL=C LANG=C`; this changes display locale
only and not artifact bytes.

## Complete local gate

All commands below completed successfully unless a diagnostic is explicitly
recorded.

```sh
make check
GOPROXY=https://goproxy.cn,direct go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
GOPROXY=https://goproxy.cn,direct make actionlint
go test -count=1 ./internal/releasecontract -run 'TestPublicDocumentation|TestMarkedShellExamples|TestWorkflow|TestNative'
go test -count=1 ./internal/installtest
git diff --check
```

Results: formatting, unit/integration tests, race tests, vet, workflow
contracts, documentation links and executable examples, installer fixtures,
and diff checks passed. `govulncheck` reported `No vulnerabilities found.`

Each fuzz target passed with a five-second mutation budget and one worker:

```sh
go test ./internal/e2e -run '^$' -fuzz '^FuzzDecodeManifest$' -fuzztime 5s -parallel 1
go test ./internal/e2e -run '^$' -fuzz '^FuzzManagedBlockEditing$' -fuzztime 5s -parallel 1
go test ./internal/e2e -run '^$' -fuzz '^FuzzResolveUAWP$' -fuzztime 5s -parallel 1
go test ./internal/transaction -run '^$' -fuzz '^FuzzJournal$' -fuzztime 5s -parallel 1
go test ./internal/transaction -run '^$' -fuzz '^FuzzClassification$' -fuzztime 5s -parallel 1
```

An earlier parallel run ended during temporary-directory cleanup with
`context deadline exceeded`. Investigation showed that `FuzzResolveUAWP`
created a new directory for every input despite testing a read-only resolver.
The fixture now reuses one root outside the fuzz loop; all five bounded runs
then passed. Serial workers keep the short bounded gate stable under load.

PowerShell syntax is checked locally when PowerShell is present. Full
PowerShell installer behavior is implemented as native Windows tests and must
pass on `windows-2025`; this macOS record does not claim that hosted result.

## Reproducible release rehearsal

The following sequence was run twice from the same commit and epoch:

```sh
make release-snapshot GORELEASER="env GOPROXY=https://goproxy.cn,direct go run github.com/goreleaser/goreleaser/v2@v2.18.0"
go run ./scripts/verifyrelease.go --dist dist/release --version 1.0.0-test --commit 91af4b63beba9cf1eb120dbf992c3d70c96d25a8
```

The first bundle was copied aside and `diff -rq` compared it with the second.
There was no output: all six assets were byte-identical. Both rehearsals passed
archive allowlist, checksum, metadata, and native binary verification.

| Asset | SHA-256 |
| --- | --- |
| `uawp_1.0.0-test_checksums.txt` | `60ff8643445db428ccf51fd096d95c56be57f9a7521a162834e3b117dc463545` |
| `uawp_1.0.0-test_darwin_amd64.tar.gz` | `69f912f109bd9830a2b35c7ce06d4d657e51a8b05e09f42fa61b8f1f31b0bfe6` |
| `uawp_1.0.0-test_darwin_arm64.tar.gz` | `1824426d9fed58d55f3d7220ef386ec3c3338837a753a134616ded86cae88061` |
| `uawp_1.0.0-test_linux_amd64.tar.gz` | `48c9f904731ceb816764827dababae835b69e35a5f584a84a6256a27765efbdb` |
| `uawp_1.0.0-test_linux_arm64.tar.gz` | `a0a67925ea64249ef446e8c8d8f1235a265eb809e2e1f4d4dc973d9bf148931e` |
| `uawp_1.0.0-test_windows_amd64.zip` | `fe2c2f6d133433c847f39564138de4a160fd178922b60a73dc55d787a4a2cd27` |

## Independent review and resolutions

An independent read-only reviewer examined every changed file, the approved
design, ledger rulings, verification output, and the five required failure
classes. The initial verdict was **not ready** with no Critical findings and 11
Important findings. All Important findings were resolved before this evidence
run:

1. Windows native verification now writes and launches `uawp.exe`.
2. Public verification is chained after publication in the same workflow, so
   `GITHUB_TOKEN` event suppression cannot skip it.
3. Attestations are verified one asset at a time.
4. A packaged-binary harness now exercises init, acquire, status, resume, sync,
   checkpoint, handoff, uninstall, and brownfield preservation before evidence
   is emitted.
5. Non-forced installers use atomic no-replace behavior and cover arrival races
   and dangling-path detection.
6. Both installers execute the extracted binary and validate its requested
   version before replacement.
7. Native Windows behavioral fixtures cover success, checksum rejection,
   traversal/duplicates, invalid executables, force, preservation, and races.
8. Local and hosted release builds derive one canonical UTC `BuiltAt` value
   from the commit epoch.
9. Release validation requires the exact reviewed `origin/main` commit,
   successful native/CodeQL/fuzz/vulnerability checks, and candidate evidence
   before final promotion. Write permission exists only in the final publish
   job after read-only evidence validation.
10. RC tags publish with `--prerelease --latest=false`.
11. Archives, installers, and verifiers now include applicable Go third-party
    notices.

The review's four Minor findings were also addressed: ZIP regularity/size
checks, HTTPS-only redirect policy outside test mode, effective concurrency
cancellation, and complete manual installation guidance.

The reviewer declined live hosted settings, actual hosted/public runs, legal
opinions, accepted historical identity exclusions, and a new audit of unchanged
earlier Core internals. These remain correctly outside this local evidence
claim.

## Remaining gates

No tag, push, GitHub Release, external repository setting, or publication was
performed. Hosted Tasks 10 and 11 still require explicit maintainer approval.
The supported-platform and public-release claims remain pending until those
native jobs and public verification records are green.
