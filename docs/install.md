# Install and uninstall UAWP

Download only from the canonical repository:
<https://github.com/vibemaker-community/uawp/releases>.

## macOS and Linux

Download [`install/install.sh`](../install/install.sh), inspect it, then run it as a separate command:

```sh
sh install.sh --version 1.0.0 --dest "$HOME/.local/bin"
```

The installer downloads the selected archive and the versioned SHA-256 file,
requires an exact checksum entry, rejects unexpected or non-regular archive
members, and installs through a temporary file. It refuses to replace an
existing binary unless `--force` is supplied.

## Windows

Download [`install/install.ps1`](../install/install.ps1), inspect it, then run:

```powershell
powershell -NoProfile -File .\install.ps1 -Version 1.0.0 -Destination "$env:LOCALAPPDATA\UAWP\bin"
```

Use `-Force` only when deliberately replacing an existing installation.

Neither installer edits `PATH`, shell profiles, the Windows registry, package
databases, or any Workspace. Add the selected directory to `PATH` yourself if
desired. Run `uawp init` separately inside a Workspace; initialization retains
its own preview and approval boundary.

## Manual verification

Download the archive and `uawp_<version>_checksums.txt` from the same release.
On macOS use `shasum -a 256`; on Linux use `sha256sum`; on Windows use
`Get-FileHash -Algorithm SHA256`. Compare the complete lowercase digest for
the exact archive filename before extracting it.

After verification on macOS or Linux, extract and install without replacing an
existing command:

```sh
mkdir -p "$HOME/.local/bin"
tar -xzf "uawp_${VERSION}_${TARGET}.tar.gz" uawp
test ! -e "$HOME/.local/bin/uawp"
install -m 0755 uawp "$HOME/.local/bin/uawp"
"$HOME/.local/bin/uawp" version --format json
```

On Windows PowerShell, extract to a new directory and verify execution before
adding that directory to `PATH`:

```powershell
$destination = Join-Path $env:LOCALAPPDATA "UAWP\bin"
if (Test-Path (Join-Path $destination "uawp.exe")) { throw "uawp.exe already exists" }
New-Item -ItemType Directory -Force -Path $destination | Out-Null
Expand-Archive ".\uawp_${Version}_windows_amd64.zip" -DestinationPath $destination
& (Join-Path $destination "uawp.exe") version --format json
```

## Uninstall

Remove only the installed executable from the destination printed by the
installer. This does not uninstall UAWP state from a Workspace. Use
`uawp uninstall` in that Workspace to preview and approve removal of
UAWP-owned integration separately.
