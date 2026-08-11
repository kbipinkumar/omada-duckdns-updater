#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Uninstalls the omada-duckdns-updater Windows Service.

.DESCRIPTION
  Stops and removes the OmadaDuckDNSUpdater service, removes the firewall rule,
  and optionally deletes Program Files and ProgramData directories.

.PARAMETER KeepData
  If set, leaves %ProgramData%\omada-duckdns-updater (config and logs) in place.
#>
param(
    [switch]$KeepData
)

$ErrorActionPreference = "Stop"

$ServiceName = "OmadaDuckDNSUpdater"
$InstallDir = Join-Path ${env:ProgramFiles} "omada-duckdns-updater"
$DataDir = Join-Path $env:ProgramData "omada-duckdns-updater"
$DestExe = Join-Path $InstallDir "omada-duckdns-updater.exe"
$FirewallRule = "Omada DuckDNS Updater Web UI"

$existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existing) {
    if (Test-Path -LiteralPath $DestExe) {
        if ($existing.Status -eq "Running") {
            Write-Host "Stopping service..."
            & $DestExe -service stop
        }
        Write-Host "Uninstalling service..."
        & $DestExe -service uninstall
    } else {
        Write-Host "Executable missing; removing service via sc.exe..."
        if ($existing.Status -eq "Running") {
            sc.exe stop $ServiceName | Out-Null
            Start-Sleep -Seconds 2
        }
        sc.exe delete $ServiceName | Out-Null
    }
} else {
    Write-Host "Service $ServiceName is not installed."
}

$rule = Get-NetFirewallRule -DisplayName $FirewallRule -ErrorAction SilentlyContinue
if ($rule) {
    Write-Host "Removing firewall rule..."
    Remove-NetFirewallRule -DisplayName $FirewallRule
}

if (Test-Path -LiteralPath $InstallDir) {
    Write-Host "Removing $InstallDir ..."
    Remove-Item -LiteralPath $InstallDir -Recurse -Force
}

if (-not $KeepData -and (Test-Path -LiteralPath $DataDir)) {
    Write-Host "Removing $DataDir ..."
    Remove-Item -LiteralPath $DataDir -Recurse -Force
} elseif ($KeepData) {
    Write-Host "Keeping data directory: $DataDir"
}

Write-Host "Uninstall complete."
