# Verify a UAWP release

Download the archive for your target and the matching
`uawp_<version>_checksums.txt` from the same GitHub release. Never accept a
digest copied from an unrelated page or message.

## SHA-256 checksum

On macOS:

```sh
shasum -a 256 uawp_<version>_darwin_<arch>.tar.gz
```

On Linux:

```sh
sha256sum uawp_<version>_linux_<arch>.tar.gz
```

On Windows PowerShell:

```powershell
Get-FileHash -Algorithm SHA256 .\uawp_<version>_windows_amd64.zip
```

The complete lowercase digest must match exactly one line for that exact
filename in the checksum manifest. The official installers automate this
comparison and also reject unsafe archive layouts.

## GitHub provenance attestation

With GitHub CLI installed and authenticated, verify the archive and checksum
manifest against the canonical repository:

```sh
gh attestation verify uawp_<version>_<os>_<arch>.<ext> --repo vibemaker-community/uawp
gh attestation verify uawp_<version>_checksums.txt --repo vibemaker-community/uawp
```

Both checksum and attestation verification are required release gates. A
checksum binds content to the manifest; the attestation binds the public asset
to the repository's release workflow.

## Inspect embedded metadata

After safe extraction, run:

```sh
uawp version --format json
```

The reported version and commit must match the tag and release page. See
[supported platforms](supported-platforms.md), [release gates](release-gates.md),
and the maintainer [release procedure](releasing.md).
