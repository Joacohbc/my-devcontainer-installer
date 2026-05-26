# devcontainer-cli uninstaller (Windows)
# Mirror of uninstall.sh: removes the binary and install dir, and strips the
# install dir from the user PATH that install.ps1 added. Honors the same
# INSTALL_DIR override.
# Usage:
#   irm https://raw.githubusercontent.com/Joacohbc/my-devcontainer-installer/main/cli/uninstall.ps1 | iex
#   $env:KEEP_CONFIG="1"; irm .../uninstall.ps1 | iex   # leave PATH untouched

[CmdletBinding()]
param(
    [string]$InstallDir = $(if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'devcontainer-cli' }),
    [string]$KeepConfig = $(if ($env:KEEP_CONFIG) { $env:KEEP_CONFIG } else { '0' })
)

$ErrorActionPreference = 'Stop'

function Info($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }

# Install dir (binary + completions)
if (Test-Path $InstallDir) {
    Remove-Item -Recurse -Force $InstallDir
    Info "Removed: $InstallDir"
}

# user PATH entry
if ($KeepConfig -eq '1') {
    Info "KEEP_CONFIG=1 — leaving user PATH untouched."
} else {
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($userPath) {
        $kept = $userPath -split ';' | Where-Object { $_ -and $_ -ne $InstallDir }
        $newPath = ($kept -join ';')
        if ($newPath -ne $userPath) {
            [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
            Info "Removed $InstallDir from user PATH (open new terminal to apply)."
        }
    }
}

Info "Uninstalled devcontainer-cli."
Info "Project files (.dc_<workspace>/, devcontainer.config.json) are left intact."
