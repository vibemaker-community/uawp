# Supported platforms

UAWP v1 release artifacts and hosted validation cover exactly these targets:

| User platform | Go target | Release archive | Native GitHub runner |
| --- | --- | --- | --- |
| macOS arm64 | `darwin/arm64` | `.tar.gz` | `macos-15` |
| macOS amd64 | `darwin/amd64` | `.tar.gz` | `macos-15-intel` |
| Linux amd64 | `linux/amd64` | `.tar.gz` | `ubuntu-24.04` |
| Linux arm64 | `linux/arm64` | `.tar.gz` | `ubuntu-24.04-arm` |
| Windows amd64 | `windows/amd64` | `.zip` | `windows-2025` |

Each ordinary CI run executes tests, vet, a build/version check, and lifecycle
smoke tests on all five runners. The Linux amd64 runner additionally runs the
Go race detector. A release builds one deterministic bundle, downloads that
same bundle on every runner, verifies checksums and embedded metadata, and
executes the native binary.

## Status vocabulary

- **Designed**: the target is represented in code and local contract tests.
- **Hosted verified**: the target's GitHub-hosted native job passed for the
  exact commit.
- **Public verified**: the post-release job downloaded, authenticated, and ran
  the published asset.

At the current pre-release-hardening stage, all targets are designed and local
contracts pass. They must not be described as hosted or public verified until
the corresponding GitHub Actions evidence exists.

Other operating systems and architectures are unsupported in v1. Building from
source may work, but it is not release evidence and does not expand this table.
