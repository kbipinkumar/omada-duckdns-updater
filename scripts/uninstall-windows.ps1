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

function Get-ServiceByName {
    param(
        [Parameter(Mandatory = $true)][string]$Name
    )
    try {
        return Get-Service -Name $Name -ErrorAction Stop
    }
    catch [Microsoft.PowerShell.Commands.ServiceCommandException] {
        # Documented not-found path from Get-Service; rethrow anything else.
        if ($_.FullyQualifiedErrorId -notlike "NoServiceFoundForGivenName*") {
            throw
        }
        return $null
    }
}

function Wait-ServiceStatusStopped {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [int]$TimeoutSeconds = 30
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $svc = Get-ServiceByName -Name $Name
        if (-not $svc -or $svc.Status -eq "Stopped") {
            return
        }
        Start-Sleep -Milliseconds 300
    }
    $svc = Get-ServiceByName -Name $Name
    if ($svc -and $svc.Status -ne "Stopped") {
        Write-Error "Timed out waiting for service '$Name' to reach Stopped (status: $($svc.Status))."
    }
}

function Wait-ServiceRemoved {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [int]$TimeoutSeconds = 15
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (-not (Get-ServiceByName -Name $Name)) {
            return
        }
        Start-Sleep -Milliseconds 300
    }
    if (Get-ServiceByName -Name $Name) {
        Write-Error "Service '$Name' still exists after uninstall/delete."
    }
}

$existing = Get-ServiceByName -Name $ServiceName
if ($existing) {
    if (Test-Path -LiteralPath $DestExe) {
        if ($existing.Status -ne "Stopped") {
            Write-Host "Stopping service..."
            & $DestExe -service stop
        }
        Wait-ServiceStatusStopped -Name $ServiceName
        Write-Host "Uninstalling service..."
        & $DestExe -service uninstall
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Service uninstall failed with exit code $LASTEXITCODE"
        }
    } else {
        Write-Host "Executable missing; removing service via sc.exe..."
        if ($existing.Status -ne "Stopped") {
            sc.exe stop $ServiceName | Out-Null
        }
        Wait-ServiceStatusStopped -Name $ServiceName
        sc.exe delete $ServiceName | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Write-Error "sc.exe delete failed with exit code $LASTEXITCODE"
        }
    }
    Wait-ServiceRemoved -Name $ServiceName
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
