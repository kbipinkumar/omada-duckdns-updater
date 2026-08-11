#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Installs omada-duckdns-updater as a Windows Service.

.DESCRIPTION
  Copies the executable to Program Files, creates the data directory under
  ProgramData, opens firewall TCP 5381 for the Web UI, installs and starts
  the OmadaDuckDNSUpdater Windows Service.

.PARAMETER SourceDir
  Directory containing omada-duckdns-updater.exe (defaults to this script's directory).
#>
param(
    [string]$SourceDir = $PSScriptRoot
)

$ErrorActionPreference = "Stop"

$ServiceName = "OmadaDuckDNSUpdater"
$InstallDir = Join-Path ${env:ProgramFiles} "omada-duckdns-updater"
$DataDir = Join-Path $env:ProgramData "omada-duckdns-updater"
$ExeName = "omada-duckdns-updater.exe"
$SourceExe = Join-Path $SourceDir $ExeName
$DestExe = Join-Path $InstallDir $ExeName
$FirewallRule = "Omada DuckDNS Updater Web UI"

if (-not (Test-Path -LiteralPath $SourceExe)) {
    Write-Error "Executable not found: $SourceExe"
}

Write-Host "Installing to $InstallDir ..."
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null
Copy-Item -LiteralPath $SourceExe -Destination $DestExe -Force

# Stop existing service if present so the binary can be replaced
$existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existing) {
    if ($existing.Status -eq "Running") {
        Write-Host "Stopping existing service..."
        & $DestExe -service stop
    }
    Write-Host "Removing existing service registration..."
    & $DestExe -service uninstall
}

Write-Host "Registering Windows Service..."
& $DestExe -service install
if ($LASTEXITCODE -ne 0) {
    Write-Error "Service install failed with exit code $LASTEXITCODE"
}

$rule = Get-NetFirewallRule -DisplayName $FirewallRule -ErrorAction SilentlyContinue
if (-not $rule) {
    Write-Host "Creating firewall rule for TCP 5381..."
    New-NetFirewallRule -DisplayName $FirewallRule `
        -Direction Inbound -Protocol TCP -LocalPort 5381 `
        -Action Allow -Profile Any | Out-Null
} else {
    Write-Host "Firewall rule already exists."
}

Write-Host "Starting service..."
& $DestExe -service start
if ($LASTEXITCODE -ne 0) {
    Write-Error "Service start failed with exit code $LASTEXITCODE"
}

Write-Host ""
Write-Host "Installation complete."
Write-Host "  Service:    $ServiceName"
Write-Host "  Binary:     $DestExe"
Write-Host "  Config/log: $DataDir"
Write-Host "  Web UI:     http://localhost:5381/"
Write-Host ""
Write-Host "Manage with: Get-Service $ServiceName"
Write-Host "Logs:        $DataDir\updater.log  (also Windows Event Log source $ServiceName)"
