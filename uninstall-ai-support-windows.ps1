<#
.SYNOPSIS
    Fully remove the AI-support SSH client from Windows.

.DESCRIPTION
    Revokes the server-side AI-support credential first, then removes the
    Windows tunnel, localhost-only OpenSSH configuration, legacy support
    account if present, and AI-support state. Run from an Administrator
    PowerShell.
#>
[CmdletBinding()]
param(
    [string]$ClientId = '',
    [string]$WindowsUser = '',
    [switch]$LocalOnly,
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
$StateDirectory = Join-Path $env:ProgramData 'AI-Support'
$LegacyWindowsSshUser = 'ai-support'
$WindowsSshUser = if ($WindowsUser) { $WindowsUser } else { $env:USERNAME }
$TaskName = 'AI-Support-Persistent-Tunnel'
$SshdTaskName = 'AI-Support-OpenSSH'
$SshdConfig = Join-Path $env:ProgramData 'ssh\sshd_config'
$SshdConfigBackup = Join-Path $StateDirectory 'sshd_config.backup'
$TokenPolicyBackup = Join-Path $StateDirectory 'token-policy.backup'
$SshdServiceBackup = Join-Path $StateDirectory 'sshd-service.backup.json'
$FirewallRuleBackup = Join-Path $StateDirectory 'firewall-rule.backup.json'
$SupportUserOwnership = Join-Path $StateDirectory 'support-user.ownership'
$SupportAuthorizedKeysBackup = Join-Path $StateDirectory 'support-authorized_keys.backup'
$SupportAdminMembershipBackup = Join-Path $StateDirectory 'support-admin-membership.backup'
$CloudflaredPath = Join-Path $StateDirectory 'cloudflared.exe'
$ActivityConfigPath = Join-Path $StateDirectory 'activity-config.json'
$serverCleanupReady = $false
$supportUserRemoved = $false

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Stop-TaskIfPresent([string]$Name) {
    Stop-ScheduledTask -TaskName $Name -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $Name -Confirm:$false -ErrorAction SilentlyContinue
    if (Get-ScheduledTask -TaskName $Name -ErrorAction SilentlyContinue) {
        throw "Scheduled task $Name kunne ikke fjernes."
    }
}

function Restore-SshdConfig {
    if (Test-Path -LiteralPath $SshdConfigBackup) {
        New-Item -ItemType Directory -Path (Split-Path -Parent $SshdConfig) -Force | Out-Null
        Copy-Item -LiteralPath $SshdConfigBackup -Destination $SshdConfig -Force
    } elseif (Test-Path -LiteralPath $SshdConfig) {
        $config = Get-Content -LiteralPath $SshdConfig -Raw
        $config = [regex]::Replace($config, '(?ms)\r?\n?# AI_SUPPORT_GLOBAL_BEGIN\r?\n.*?\r?\n# AI_SUPPORT_GLOBAL_END\r?\n?', "`r`n")
        $config = [regex]::Replace($config, '(?ms)\r?\n?# AI_SUPPORT_MATCH_BEGIN\r?\n.*?\r?\n# AI_SUPPORT_MATCH_END\r?\n?', "`r`n")
        Set-Content -LiteralPath $SshdConfig -Value $config -Encoding ascii -ErrorAction Stop
    }
}

function Restore-TokenPolicy {
    $policyPath = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System'
    if (-not (Test-Path -LiteralPath $TokenPolicyBackup)) { return }
    $original = (Get-Content -LiteralPath $TokenPolicyBackup -Raw).Trim()
    if ($original -eq 'MISSING') {
        Remove-ItemProperty -Path $policyPath -Name LocalAccountTokenFilterPolicy -ErrorAction SilentlyContinue
    } else {
        Set-ItemProperty -Path $policyPath -Name LocalAccountTokenFilterPolicy -Value ([int]$original) -Type DWord
    }
}

function Restore-SshdService {
    if (-not (Test-Path -LiteralPath $SshdServiceBackup)) { return }
    $original = Get-Content -LiteralPath $SshdServiceBackup -Raw | ConvertFrom-Json
    $service = Get-Service -Name sshd -ErrorAction SilentlyContinue
    if (-not $service) { return }
    if ($original.exists -eq $true) {
        $startupType = switch ([string]$original.start_mode) {
            'Auto' { 'Automatic' }
            'Automatic' { 'Automatic' }
            'Disabled' { 'Disabled' }
            default { 'Manual' }
        }
        Set-Service -Name sshd -StartupType $startupType -ErrorAction Stop
        if ([string]$original.state -eq 'Running') {
            Start-Service -Name sshd -ErrorAction Stop
        } else {
            Stop-Service -Name sshd -Force -ErrorAction Stop
        }
    } else {
        Stop-Service -Name sshd -Force -ErrorAction Stop
        Set-Service -Name sshd -StartupType Manual -ErrorAction Stop
    }
}

function Restore-FirewallRule {
    if (-not (Test-Path -LiteralPath $FirewallRuleBackup)) { return }
    $original = Get-Content -LiteralPath $FirewallRuleBackup -Raw | ConvertFrom-Json
    $rule = Get-NetFirewallRule -Name 'OpenSSH-Server-In-TCP' -ErrorAction SilentlyContinue
    if ($original.exists -eq $true -and $rule) {
        if ($original.enabled -eq $true) { Enable-NetFirewallRule -Name 'OpenSSH-Server-In-TCP' -ErrorAction Stop }
        else { Disable-NetFirewallRule -Name 'OpenSSH-Server-In-TCP' -ErrorAction Stop }
    } elseif ($original.exists -ne $true -and $rule) {
        Remove-NetFirewallRule -Name 'OpenSSH-Server-In-TCP' -ErrorAction Stop
    }
}

if (-not (Test-Administrator)) { throw 'Kør afinstallationen fra en Administrator-PowerShell.' }
if (-not $Force) {
    $confirmation = Read-Host 'Skriv AFINSTALLER for at fjerne AI-support fra denne Windows-PC'
    if ($confirmation -ne 'AFINSTALLER') { throw 'Afinstallation annulleret.' }
}
$config = $null
if (Test-Path -LiteralPath $ActivityConfigPath) {
    $config = Get-Content -LiteralPath $ActivityConfigPath -Raw | ConvertFrom-Json
    if (-not $ClientId) { $ClientId = [string]$config.client_id }
}

if (-not $LocalOnly -and (-not $config -or -not $config.log_url -or -not $config.log_token -or -not $ClientId)) {
    throw 'AI-support serverkonfiguration mangler. Brug kun -LocalOnly, hvis server-side oprydning allerede er håndteret.'
}

if (-not $LocalOnly) {
    try {
        $body = @{
            action = 'ai-support-uninstall'
            phase = 'request'
            client_id = $ClientId
            log_token = [string]$config.log_token
        } | ConvertTo-Json -Compress
        $response = Invoke-RestMethod -Uri ([string]$config.log_url) -Method Post -ContentType 'application/json' -Body $body -TimeoutSec 15
        if ($response.status -ne 'uninstall_pending') {
            throw 'Serveren bekræftede ikke afinstallationen.'
        }
        $serverCleanupReady = $true
        Write-Host 'Server-side AI-support credential tilbagekaldt.' -ForegroundColor Green
    } catch {
        throw 'Server-side tilbagekaldelse kunne ikke bekræftes. Ingen lokale AI-supportfiler blev fjernet.'
    }
}

$hasAiConfig = $false
if (Test-Path -LiteralPath $SshdConfig) {
    $hasAiConfig = (Get-Content -LiteralPath $SshdConfig -Raw) -match 'AI_SUPPORT_(GLOBAL|MATCH)_BEGIN'
}
$hasAiTask = $null -ne (Get-ScheduledTask -TaskName $SshdTaskName -ErrorAction SilentlyContinue)
$shouldTouchSshd = (Test-Path -LiteralPath $StateDirectory) -or $hasAiConfig -or $hasAiTask

Stop-TaskIfPresent $TaskName
Stop-TaskIfPresent $SshdTaskName

Get-Process -Name 'cloudflared' -ErrorAction SilentlyContinue | ForEach-Object {
    $path = ''
    try { $path = $_.Path } catch { }
    if ($path -and ([IO.Path]::GetFullPath($path) -ieq [IO.Path]::GetFullPath($CloudflaredPath))) {
        Stop-Process -Id $_.Id -Force -ErrorAction Stop
    }
}

if ($shouldTouchSshd) {
    Stop-Service -Name sshd -Force -ErrorAction SilentlyContinue
    Restore-SshdConfig
    Restore-TokenPolicy
    Restore-SshdService
    Restore-FirewallRule
}

$user = if ($WindowsSshUser -eq $LegacyWindowsSshUser) {
    Get-LocalUser -Name $LegacyWindowsSshUser -ErrorAction SilentlyContinue
} else { $null }
if ($user) {
    $ownership = if (Test-Path -LiteralPath $SupportUserOwnership) {
        (Get-Content -LiteralPath $SupportUserOwnership -Raw).Trim()
    } else { '' }
    $authorizedKeys = Join-Path $env:SystemDrive "Users\$WindowsSshUser\.ssh\authorized_keys"
    $supportKey = if (Test-Path -LiteralPath (Join-Path $StateDirectory 'support.pub')) {
        (Get-Content -LiteralPath (Join-Path $StateDirectory 'support.pub') -Raw).Trim()
    } else { '' }
    $supportKeyParts = $supportKey -split '\s+'
    $authorizedKeyParts = if (Test-Path -LiteralPath $authorizedKeys) {
        ((Get-Content -LiteralPath $authorizedKeys | Where-Object { $_.Trim() })[0]) -split '\s+'
    } else { @() }
    $keyOwnedBySupport = $supportKeyParts.Count -ge 2 -and $authorizedKeyParts.Count -ge 2 -and
        $authorizedKeyParts[0] -eq $supportKeyParts[0] -and $authorizedKeyParts[1] -eq $supportKeyParts[1]
    if ($ownership -eq 'CREATED_BY_AI_SUPPORT' -or ($ownership -eq '' -and $keyOwnedBySupport -and $user.Description -eq 'SSH-only AI support account')) {
        Remove-LocalUser -Name $WindowsSshUser -ErrorAction Stop
        if (Get-LocalUser -Name $WindowsSshUser -ErrorAction SilentlyContinue) {
            throw 'AI-support-brugeren kunne ikke fjernes.'
        }
        $supportUserRemoved = $true
    } elseif ($ownership -eq 'EXISTING_ACCOUNT' -or ($ownership -eq '' -and $keyOwnedBySupport)) {
        if (Test-Path -LiteralPath $SupportAuthorizedKeysBackup) {
            Copy-Item -LiteralPath $SupportAuthorizedKeysBackup -Destination $authorizedKeys -Force -ErrorAction Stop
        } elseif ($keyOwnedBySupport) {
            $remainingKeys = @(Get-Content -LiteralPath $authorizedKeys | Where-Object {
                $parts = $_ -split '\s+'
                $parts.Count -lt 2 -or $parts[0] -ne $supportKeyParts[0] -or $parts[1] -ne $supportKeyParts[1]
            })
            Set-Content -LiteralPath $authorizedKeys -Value $remainingKeys -Encoding ascii -ErrorAction Stop
        }
        $removeSupportAdminMembership =
            ((Test-Path -LiteralPath $SupportAdminMembershipBackup) -and
                (Get-Content -LiteralPath $SupportAdminMembershipBackup -Raw).Trim() -eq 'False') -or
            ($ownership -eq '' -and $keyOwnedBySupport)
        if ($removeSupportAdminMembership) {
            $administratorsGroup = (Get-LocalGroup -SID ([Security.Principal.SecurityIdentifier]'S-1-5-32-544')).Name
            $isAdministrator = Get-LocalGroupMember -Group $administratorsGroup -ErrorAction SilentlyContinue |
                Where-Object { $_.Name -match "\\$WindowsSshUser$" }
            if ($isAdministrator) {
                Remove-LocalGroupMember -Group $administratorsGroup -Member $WindowsSshUser -ErrorAction Stop
            }
        }
    }
}
if ($supportUserRemoved) {
    Remove-Item -LiteralPath (Join-Path $env:SystemDrive "Users\$WindowsSshUser") -Recurse -Force -ErrorAction Stop
}

if (-not $LocalOnly) {
    if (-not $serverCleanupReady) { throw 'Server-side cleanup blev ikke godkendt.' }
}
if ($serverCleanupReady -and $config.log_url -and $config.log_token -and $ClientId) {
    $reported = $false
    for ($attempt = 1; $attempt -le 3 -and -not $reported; $attempt++) {
        try {
            $body = @{
                action = 'ai-support-uninstall'
                phase = 'complete'
                client_id = $ClientId
                log_token = [string]$config.log_token
            } | ConvertTo-Json -Compress
            $response = Invoke-RestMethod -Uri ([string]$config.log_url) -Method Post -ContentType 'application/json' -Body $body -TimeoutSec 15
            if ($response.status -ne 'local_uninstall_reported') { throw 'Serveren bekræftede ikke den lokale oprydning.' }
            $reported = $true
        } catch {
            if ($attempt -lt 3) { Start-Sleep -Seconds 2 }
        }
    }
    if (-not $reported) {
        throw 'Lokal oprydning er udført, men serveren kunne ikke markere den. Klienten skal afsluttes manuelt i dashboardet.'
    }
    Write-Host 'Lokal oprydning rapporteret; Ubuntu fjerner SSH-nøglen og klientrækken.' -ForegroundColor Green
}

try {
    if (Test-Path -LiteralPath $StateDirectory) {
        Remove-Item -LiteralPath $StateDirectory -Recurse -Force -ErrorAction Stop
    }
} catch {
    if ($serverCleanupReady -and $config.log_url -and $config.log_token -and $ClientId) {
        for ($attempt = 1; $attempt -le 3; $attempt++) {
            try {
                $body = @{
                    action = 'ai-support-uninstall'
                    phase = 'rollback'
                    client_id = $ClientId
                    log_token = [string]$config.log_token
                } | ConvertTo-Json -Compress
                Invoke-RestMethod -Uri ([string]$config.log_url) -Method Post -ContentType 'application/json' -Body $body -TimeoutSec 15 | Out-Null
                break
            } catch {
                if ($attempt -lt 3) { Start-Sleep -Seconds 2 }
            }
        }
    }
    throw 'AI-support state-mappen kunne ikke slettes; server-side completion blev rullet tilbage.'
}

Write-Host "`nAI-support er fuldstændig afinstalleret på $env:COMPUTERNAME." -ForegroundColor Green
Write-Host 'OpenSSH-pakken er bevaret, hvis den var installeret før AI-support.'
