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

.PARAMETER AllowPublicFirewall
  Also apply the inbound TCP 5381 rule to the Public profile. Without this,
  the rule defaults to Domain and Private only; an interactive prompt asks
  before including Public when this switch is not set.
#>
param(
    [string]$SourceDir = $PSScriptRoot,
    [switch]$AllowPublicFirewall
)

$ErrorActionPreference = "Stop"

$ServiceName = "OmadaDuckDNSUpdater"
$InstallDir = Join-Path ${env:ProgramFiles} "omada-duckdns-updater"
$DataDir = Join-Path $env:ProgramData "omada-duckdns-updater"
$ExeName = "omada-duckdns-updater.exe"
$SourceExe = Join-Path $SourceDir $ExeName
$DestExe = Join-Path $InstallDir $ExeName
$FirewallRule = "Omada DuckDNS Updater Web UI"

function Wait-ServiceStatusStopped {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [int]$TimeoutSeconds = 30
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $svc = Get-Service -Name $Name -ErrorAction SilentlyContinue
        if (-not $svc -or $svc.Status -eq "Stopped") {
            return
        }
        Start-Sleep -Milliseconds 300
    }
    $svc = Get-Service -Name $Name -ErrorAction SilentlyContinue
    if ($svc -and $svc.Status -ne "Stopped") {
        Write-Error "Timed out waiting for service '$Name' to reach Stopped (status: $($svc.Status))."
    }
}

function Wait-ServiceStatusRunning {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [int]$TimeoutSeconds = 30
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $svc = Get-Service -Name $Name -ErrorAction SilentlyContinue
        if ($svc -and $svc.Status -eq "Running") {
            return
        }
        Start-Sleep -Milliseconds 300
    }
    $svc = Get-Service -Name $Name -ErrorAction SilentlyContinue
    $status = if ($svc) { $svc.Status } else { "Missing" }
    Write-Error "Timed out waiting for service '$Name' to reach Running (status: $status)."
}

function Wait-ServiceRemoved {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [int]$TimeoutSeconds = 15
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (-not (Get-Service -Name $Name -ErrorAction SilentlyContinue)) {
            return
        }
        Start-Sleep -Milliseconds 300
    }
    if (Get-Service -Name $Name -ErrorAction SilentlyContinue) {
        Write-Error "Service '$Name' still exists after uninstall/delete."
    }
}

if (-not (Test-Path -LiteralPath $SourceExe)) {
    Write-Error "Executable not found: $SourceExe"
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null

$existing = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existing) {
    Write-Host "Upgrading existing installation..."
    if ($existing.Status -ne "Stopped") {
        if (Test-Path -LiteralPath $DestExe) {
            Write-Host "Stopping service..."
            & $DestExe -service stop
        } else {
            Write-Host "Stopping service via sc.exe..."
            sc.exe stop $ServiceName | Out-Null
        }
        Wait-ServiceStatusStopped -Name $ServiceName
    }

    Write-Host "Installing updated binary to $InstallDir ..."
    Copy-Item -LiteralPath $SourceExe -Destination $DestExe -Force

    Write-Host "Starting service..."
    & $DestExe -service start
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Service start failed with exit code $LASTEXITCODE"
    }
    Wait-ServiceStatusRunning -Name $ServiceName

    Write-Host ""
    Write-Host "Upgrade complete."
    Write-Host "  Service:    $ServiceName"
    Write-Host "  Binary:     $DestExe"
    Write-Host "  Config/log: $DataDir"
    Write-Host "  Web UI:     http://localhost:5381/"
    Write-Host ""
    Write-Host "Manage with: Get-Service $ServiceName"
    Write-Host "Logs:        $DataDir\updater.log  (also Windows Event Log source $ServiceName)"
    exit 0
}

# Fresh install path
Write-Host "Installing to $InstallDir ..."
Copy-Item -LiteralPath $SourceExe -Destination $DestExe -Force

Write-Host "Registering Windows Service..."
& $DestExe -service install
if ($LASTEXITCODE -ne 0) {
    Write-Error "Service install failed with exit code $LASTEXITCODE"
}

$rule = Get-NetFirewallRule -DisplayName $FirewallRule -ErrorAction SilentlyContinue
if (-not $rule) {
    $firewallProfiles = @("Domain", "Private")
    $includePublic = $AllowPublicFirewall.IsPresent
    if (-not $includePublic -and [Environment]::UserInteractive) {
        $answer = Read-Host "Also allow inbound TCP 5381 on the Public network profile? [y/N]"
        if ($answer -match '^(?i)y(es)?$') {
            $includePublic = $true
        }
    }
    if ($includePublic) {
        $firewallProfiles += "Public"
    }

    Write-Host "Creating firewall rule for TCP 5381 (profiles: $($firewallProfiles -join ', '))..."
    New-NetFirewallRule -DisplayName $FirewallRule `
        -Direction Inbound -Protocol TCP -LocalPort 5381 `
        -Action Allow -Profile ($firewallProfiles -join ",") | Out-Null
} else {
    Write-Host "Firewall rule already exists."
}

Write-Host "Starting service..."
& $DestExe -service start
if ($LASTEXITCODE -ne 0) {
    Write-Error "Service start failed with exit code $LASTEXITCODE"
}
Wait-ServiceStatusRunning -Name $ServiceName

Write-Host ""
Write-Host "Installation complete."
Write-Host "  Service:    $ServiceName"
Write-Host "  Binary:     $DestExe"
Write-Host "  Config/log: $DataDir"
Write-Host "  Web UI:     http://localhost:5381/"
Write-Host ""
Write-Host "Manage with: Get-Service $ServiceName"
Write-Host "Logs:        $DataDir\updater.log  (also Windows Event Log source $ServiceName)"
