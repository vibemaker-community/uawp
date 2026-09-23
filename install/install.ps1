[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Version,
    [string]$Destination = (Join-Path $env:LOCALAPPDATA "UAWP\bin"),
    [switch]$Force
)

$ErrorActionPreference = "Stop"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$Version = $Version.TrimStart("v")
$target = if ([Environment]::Is64BitOperatingSystem) { "windows_amd64" } else { throw "Unsupported Windows architecture" }
$asset = "uawp_${Version}_${target}.zip"
$checksums = "uawp_${Version}_checksums.txt"
$baseUrl = "https://github.com/vibemaker-community/uawp/releases/download/v${Version}"
if ($env:UAWP_INSTALL_BASE_URL) {
    if ($env:UAWP_INSTALL_TESTING -ne "1") { throw "UAWP_INSTALL_BASE_URL is available only in test mode" }
    $baseUrl = $env:UAWP_INSTALL_BASE_URL.TrimEnd("/")
}
if (-not $baseUrl.StartsWith("https://") -and $env:UAWP_INSTALL_TESTING -ne "1") { throw "HTTPS is required" }

$finalPath = Join-Path $Destination "uawp.exe"
if ((Test-Path -LiteralPath $finalPath) -and -not $Force) { throw "$finalPath already exists; use -Force to replace it" }
$workDir = Join-Path ([IO.Path]::GetTempPath()) ("uawp-install-" + [Guid]::NewGuid())
$stagingPath = $null
New-Item -ItemType Directory -Path $workDir | Out-Null
try {
    $archivePath = Join-Path $workDir $asset
    $checksumPath = Join-Path $workDir $checksums
    Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/$checksums" -OutFile $checksumPath
    Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/$asset" -OutFile $archivePath
    $matches = @(Get-Content -LiteralPath $checksumPath | Where-Object { $_ -match "^([0-9a-fA-F]{64})  $([regex]::Escape($asset))$" })
    if ($matches.Count -ne 1) { throw "Checksum file must contain exactly one entry for $asset" }
    $expected = $matches[0].Substring(0, 64).ToLowerInvariant()
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $archivePath).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw "SHA-256 verification failed for $asset" }

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [IO.Compression.ZipFile]::OpenRead($archivePath)
    try {
        $expectedEntries = @("LICENSE", "NOTICE", "README.md", "THIRD_PARTY_NOTICES.md", "TRADEMARKS.md", "release-metadata.json", "uawp.exe")
        $names = @($zip.Entries | ForEach-Object { $_.FullName } | Sort-Object)
        if (($names -join "`n") -ne (($expectedEntries | Sort-Object) -join "`n")) { throw "Archive contains missing, duplicate, or unsafe entries" }
        foreach ($entry in $zip.Entries) {
            if ([IO.Path]::GetFileName($entry.FullName) -ne $entry.FullName -or $entry.FullName.Contains("..") -or $entry.FullName.Contains("\")) { throw "Unsafe archive entry" }
            $unixType = ($entry.ExternalAttributes -shr 16) -band 0xF000
            if ($unixType -eq 0xA000) { throw "Symbolic links are not allowed" }
        }
    } finally { $zip.Dispose() }

    $extractDir = Join-Path $workDir "extract"
    [IO.Compression.ZipFile]::ExtractToDirectory($archivePath, $extractDir)
    $extractedBinary = Join-Path $extractDir "uawp.exe"
    $versionOutput = & $extractedBinary version --format json | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or $versionOutput.version -ne $Version) { throw "Downloaded uawp executable failed its version check" }
    New-Item -ItemType Directory -Force -Path $Destination | Out-Null
    $stagingPath = Join-Path $Destination (".uawp-install-" + [Guid]::NewGuid() + ".exe")
    Copy-Item -LiteralPath $extractedBinary -Destination $stagingPath
    if (-not $Force) {
        [IO.File]::Move($stagingPath, $finalPath)
    } elseif (Test-Path -LiteralPath $finalPath) {
        $backup = Join-Path $workDir "uawp.previous.exe"
        [IO.File]::Replace($stagingPath, $finalPath, $backup)
    } else {
        [IO.File]::Move($stagingPath, $finalPath)
    }
    $stagingPath = $null
    Write-Output "Installed UAWP $Version to $finalPath"
    Write-Output "PATH and the registry were not changed."
} finally {
    if ($stagingPath -and (Test-Path -LiteralPath $stagingPath)) { Remove-Item -LiteralPath $stagingPath -Force }
    if (Test-Path -LiteralPath $workDir) { Remove-Item -LiteralPath $workDir -Recurse -Force }
}
