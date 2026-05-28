# devcontainer-cli installer (Windows)
# Usage:
#   irm https://raw.githubusercontent.com/Joacohbc/my-devcontainer-installer/main/cli/install.ps1 | iex
#   $env:VERSION="v1.0.0"; irm .../install.ps1 | iex

[CmdletBinding()]
param(
    [string]$Repo       = $(if ($env:REPO)        { $env:REPO }        else { 'Joacohbc/my-devcontainer-installer' }),
    [string]$Version    = $(if ($env:VERSION)     { $env:VERSION }     else { 'latest' }),
    [string]$InstallDir = $(if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'devcontainer-cli' })
)

$ErrorActionPreference = 'Stop'

function Info($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }
function Fail($msg) { Write-Host "error: $msg" -ForegroundColor Red; exit 1 }

$target = 'windows-x64'
$asset  = "devcontainer-cli-$target.exe"
$url    = if ($Version -eq 'latest') {
    "https://github.com/$Repo/releases/latest/download/$asset"
} else {
    "https://github.com/$Repo/releases/download/$Version/$asset"
}

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

$binaryPath = Join-Path $InstallDir 'devcontainer-cli.exe'
$tmpPath    = "$binaryPath.download"

try {
    Info "Downloading $asset ($Version)"
    Invoke-WebRequest -Uri $url -OutFile $tmpPath -UseBasicParsing

    # Verify the published sha256 before trusting the binary. The release ships
    # a per-asset <asset>.sha256 (goreleaser checksum.split); refuse to install
    # if it is missing or does not match.
    $sumUrl  = "$url.sha256"
    $sumPath = "$tmpPath.sha256"
    Info "Verifying checksum"
    Invoke-WebRequest -Uri $sumUrl -OutFile $sumPath -UseBasicParsing
    $expected = ((Get-Content $sumPath -TotalCount 1) -split '\s+')[0]
    Remove-Item -Force $sumPath -ErrorAction SilentlyContinue
    if (-not $expected) { Fail "empty or invalid checksum file" }
    $actual = (Get-FileHash -Path $tmpPath -Algorithm SHA256).Hash.ToLower()
    if ($expected.ToLower() -ne $actual) {
        Fail "checksum mismatch: expected $expected, got $actual"
    }

    # On Windows you cannot overwrite a running .exe, but you can rename it.
    if (Test-Path $binaryPath) {
        $oldPath = "$binaryPath.old"
        if (Test-Path $oldPath) { Remove-Item -Force $oldPath -ErrorAction SilentlyContinue }
        Rename-Item -Path $binaryPath -NewName 'devcontainer-cli.exe.old' -ErrorAction SilentlyContinue
    }
    Move-Item -Path $tmpPath -Destination $binaryPath -Force

    Info "Installed:"
    Write-Host "  binary : $binaryPath"

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not ($userPath -split ';' | Where-Object { $_ -eq $InstallDir })) {
        Info "Adding $InstallDir to user PATH"
        [Environment]::SetEnvironmentVariable('Path', ($userPath.TrimEnd(';') + ';' + $InstallDir), 'User')
        Write-Host "  (open new terminal to pick up PATH change)"
    }

    Info "Verify: devcontainer-cli --help"
    Info "Self-update: devcontainer-cli upgrade-cli"
}
catch {
    if (Test-Path $tmpPath) { Remove-Item -Force $tmpPath -ErrorAction SilentlyContinue }
    Fail $_.Exception.Message
}
