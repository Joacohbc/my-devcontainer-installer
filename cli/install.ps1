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
$asset  = "devcontainer-cli-$target.zip"
$url    = if ($Version -eq 'latest') {
    "https://github.com/$Repo/releases/latest/download/$asset"
} else {
    "https://github.com/$Repo/releases/download/$Version/$asset"
}

$tmp = Join-Path $env:TEMP ("devcontainer-cli-" + [Guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null

try {
    Info "Downloading $asset ($Version)"
    $zip = Join-Path $tmp $asset
    Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing

    Info "Extracting"
    Expand-Archive -Path $zip -DestinationPath $tmp -Force

    if (Test-Path $InstallDir) { Remove-Item -Recurse -Force $InstallDir }
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
    Copy-Item -Recurse -Force (Join-Path $tmp "devcontainer-cli-$target\*") $InstallDir

    Info "Installed:"
    Write-Host "  binary : $InstallDir\devcontainer-cli.exe"
    Write-Host "  assets : $InstallDir\assets"

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not ($userPath -split ';' | Where-Object { $_ -eq $InstallDir })) {
        Info "Adding $InstallDir to user PATH"
        [Environment]::SetEnvironmentVariable('Path', ($userPath.TrimEnd(';') + ';' + $InstallDir), 'User')
        Write-Host "  (open new terminal to pick up PATH change)"
    }

    Info "Verify: devcontainer-cli --help"
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
