param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^v?\d+\.\d+\.\d+$')]
    [string] $Version
)

$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$normalizedVersion = $Version.Trim().TrimStart('v')
$tagVersion = "v$normalizedVersion"
$versionFile = Join-Path $root 'internal/version/version.go'
$wailsFile = Join-Path $root 'wails.json'

$versionSource = Get-Content -LiteralPath $versionFile -Raw
$versionSource = $versionSource -replace 'Version\s*=\s*"[^"]+"', "Version        = `"$normalizedVersion`""
Set-Content -LiteralPath $versionFile -Value $versionSource -NoNewline

$wails = Get-Content -LiteralPath $wailsFile -Raw | ConvertFrom-Json
$wails.info.productVersion = $normalizedVersion
$wails | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $wailsFile

Write-Host "Updated Exposely version to $normalizedVersion"
Write-Host "Next release tag: $tagVersion"
