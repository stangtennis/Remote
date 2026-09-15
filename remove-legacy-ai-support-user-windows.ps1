<#
.SYNOPSIS
    Remove only the legacy local AI-support Windows account.

.DESCRIPTION
    This is a migration cleanup for installations that created the local
    `ai-support` account. It deliberately leaves the current-user tunnel,
    Remote Desktop agent, OpenSSH service/configuration, and C:\ProgramData\AI-Support
    untouched.

    Run from an elevated PowerShell. The script fails closed unless the account
    has the exact legacy description and an additional legacy installation
    marker (local Administrators membership or its legacy profile path).
#>
[CmdletBinding(ConfirmImpact = 'High', SupportsShouldProcess)]
param(
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
$LegacyUserName = 'ai-support'
$LegacyDescription = 'SSH-only AI support account'
$LegacyProfilePath = Join-Path $env:SystemDrive 'Users\ai-support'

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

if (-not [Environment]::Is64BitOperatingSystem) {
    throw 'Denne oprydning kræver et 64-bit Windows-system.'
}
if (-not (Test-Administrator)) {
    throw 'Kør oprydningen fra en Administrator-PowerShell.'
}

$account = Get-LocalUser -Name $LegacyUserName -ErrorAction SilentlyContinue
if (-not $account) {
    Write-Host 'Den gamle lokale ai-support-bruger findes ikke. Ingen ændringer udført.' -ForegroundColor Yellow
    exit 0
}

$currentSid = [Security.Principal.WindowsIdentity]::GetCurrent().User.Value
if ([string]$account.SID.Value -eq $currentSid) {
    throw 'Den gamle ai-support-bruger er den aktuelle bruger og kan ikke fjernes fra denne session.'
}
if ([string]$account.Description -cne $LegacyDescription) {
    throw 'Brugeren ai-support har ikke den forventede legacy-beskrivelse. Ingen ændringer udført.'
}

$administratorsGroup = (Get-LocalGroup -SID ([Security.Principal.SecurityIdentifier]'S-1-5-32-544')).Name
$isAdministrator = @(Get-LocalGroupMember -Group $administratorsGroup -ErrorAction SilentlyContinue |
    Where-Object { $_.SID -and $_.SID.Value -eq $account.SID.Value }).Count -gt 0
$profile = Get-CimInstance Win32_UserProfile -Filter ("SID='{0}'" -f $account.SID.Value) -ErrorAction SilentlyContinue
$hasLegacyEvidence = $isAdministrator -or ($profile -and ([IO.Path]::GetFullPath($profile.LocalPath) -ieq [IO.Path]::GetFullPath($LegacyProfilePath)))
if (-not $hasLegacyEvidence) {
    throw 'Brugeren mangler et ekstra legacy-installationsspor. Ingen ændringer udført.'
}

$tasksUsingAccount = @(Get-ScheduledTask -ErrorAction SilentlyContinue | Where-Object {
    $principal = $_.Principal
    $principal.UserId -and (
        $principal.UserId -ieq $LegacyUserName -or
        $principal.UserId -ieq ".\$LegacyUserName" -or
        $principal.UserId -ieq $account.SID.Value
    )
})
if ($tasksUsingAccount.Count -gt 0) {
    $taskNames = ($tasksUsingAccount | ForEach-Object TaskName) -join ', '
    throw "Planlagte opgaver bruger stadig ai-support ($taskNames). Kør fuld AI-support-afinstallation først."
}

$runningProcesses = @()
foreach ($process in @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue)) {
    try {
        $owner = Invoke-CimMethod -InputObject $process -MethodName GetOwner -ErrorAction Stop
        if ($owner.User -ieq $LegacyUserName) {
            $runningProcesses += $process
        }
    } catch {
        # Some protected/system processes do not expose an owner.
    }
}
if ($runningProcesses.Count -gt 0) {
    $processNames = ($runningProcesses | ForEach-Object { "$($_.Name) (PID $($_.ProcessId))" }) -join ', '
    throw "Processer kører stadig som ai-support: $processNames. Ingen ændringer udført."
}

if (-not $Force) {
    $confirmation = Read-Host 'Skriv FJERN LEGACY AI-SUPPORT for at fjerne den gamle lokale bruger'
    if ($confirmation -cne 'FJERN LEGACY AI-SUPPORT') {
        throw 'Oprydning annulleret.'
    }
}

if (-not $PSCmdlet.ShouldProcess($LegacyUserName, 'Fjern gammel lokal AI-support-bruger')) {
    return
}

Remove-LocalUser -InputObject $account -ErrorAction Stop
if (Get-LocalUser -Name $LegacyUserName -ErrorAction SilentlyContinue) {
    throw 'Den gamle ai-support-bruger kunne ikke fjernes.'
}

if ($profile -and (Test-Path -LiteralPath $profile.LocalPath)) {
    Write-Warning "Brugerkontoen er fjernet. Den gamle profilmappe blev bevaret: $($profile.LocalPath)"
}
Write-Host 'Den gamle lokale ai-support-bruger er fjernet.' -ForegroundColor Green
Write-Host 'Ingen AI-support state, tunnel, OpenSSH-konfiguration eller Remote Desktop-agent blev ændret.' -ForegroundColor Green
