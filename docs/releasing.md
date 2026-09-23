# Releasing UAWP

UAWP releases are deliberately human-gated. Automation validates, builds,
tests, attests, and publishes an already-authorized tag; it does not decide
that a release should exist. The maintainer's explicit creation and push of a
new immutable tag is the authorization event.

The workflow uses only standard GitHub-hosted runners and short-lived transfer
artifacts. It does not require a paid GitHub Environment approval gate.

## Release invariants

- Tags use `vMAJOR.MINOR.PATCH-rc.N` or `vMAJOR.MINOR.PATCH`.
- A tag is created once and never moved, deleted for reuse, or force-pushed.
- A release candidate correction increments `N`; it does not replace an RC.
- A final-release correction receives a new patch version and an advisory.
- One bundle is built on Linux and that exact bundle is tested on both macOS
  architectures, both Linux architectures, and Windows x86-64.
- Publication happens only after source tests, vulnerability checks, checksum
  verification, and every native smoke test pass.
- A release is not considered complete until the post-release workflow has
  downloaded the public assets, verified their attestations and checksums, and
  executed the matching binary on all five native runners.

## Human gate and candidate preparation

1. Work from a clean `main` commit that has passed CI and security workflows.
   Fetch tags first with `git fetch --tags` so the local uniqueness check sees
   every existing release identity.
2. For a final release, replace `Unreleased` with an exact version heading such
   as `## 1.0.0`. A candidate may retain `Unreleased`.
3. Select a tag that has never existed. Start with `v1.0.0-rc.1` and increment
   the RC number for corrections.
4. Run the pre-tag checks, substituting the intended tag:

   ```sh
   tag=v1.0.0-rc.1
   version=${tag#v}
   commit=$(git rev-parse HEAD)
   go run ./cmd/releasecheck --mode prepare --tag "$tag" --version "$version" --commit "$commit"
   make check
   GOPROXY=https://proxy.golang.org,direct go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
   ```

5. Stop and obtain the maintainer's explicit confirmation to publish that exact
   tag at that exact commit. Creating or pushing a tag is never implied by a
   passing check.
6. After confirmation, create and push the immutable tag:

   ```sh
   git tag -a "$tag" "$commit" -m "UAWP $tag"
   git push origin "$tag"
   ```

The tag push starts `Release`. It validates the tag, builds once with pinned
GoReleaser, verifies the bundle, transfers it with one-day retention, runs the
native matrix, validates aggregated evidence, creates provenance attestations,
and finally creates the GitHub release.

## Evidence and completion

The release workflow produces one `release-bundle` transfer artifact and five
per-platform evidence artifacts. The final publish job accepts only evidence
whose tag, commit, target, and passed status match the immutable tag.

Publishing triggers `Post-release Verification`. Inspect that workflow in
GitHub Actions. Only a green result on all five platforms establishes that the
public release is complete. A published GitHub release whose post-release
verification is red must be treated as faulty and must not be announced as a
successful release.

## Failure recovery

### Before tag creation

Fix the issue normally, rerun the preparation checks, and request confirmation
again for the new commit. No public release identity exists yet.

### After tag creation but before publication

Do not move or reuse the tag. Preserve the failed run as evidence. Fix the
issue on a new commit and choose the next RC number—for example, supersede
`v1.0.0-rc.1` with `v1.0.0-rc.2`. Run preparation and obtain a new explicit
confirmation before creating the new tag.

### After publication

Do not replace assets under the published tag and do not move the tag. Mark a
faulty candidate as superseded and publish the next RC. For a faulty final
release, publish a clear security or correctness advisory as appropriate, fix
the issue, increment the patch version, and run the entire release process
again. Historical assets remain immutable so users can audit what was shipped.

## Local rehearsal

Local snapshot builds are non-publishing and safe to repeat:

```sh
make release-snapshot GORELEASER="go run github.com/goreleaser/goreleaser/v2@v2.18.0"
```

This checks deterministic archive structure and embedded build metadata. It is
not a substitute for the native hosted matrix or public post-release checks.
