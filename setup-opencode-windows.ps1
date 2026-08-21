[CmdletBinding()]
param(
    [string]$UbuntuHost = '192.168.1.92',
    [string]$UbuntuUser = 'dennis',
    [string]$RemoteProject = '/home/dennis',
    [string]$MountDrive = 'U:',
    [switch]$MountUbuntu
)

$ErrorActionPreference = 'Stop'

function Write-Step([string]$Message) {
    Write-Host "`n==> $Message" -ForegroundColor Cyan
}

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

$sshCommand = Get-Command ssh.exe -ErrorAction SilentlyContinue
if (-not $sshCommand) {
    if (-not (Test-Administrator)) {
        throw 'OpenSSH Client mangler. Koer PowerShell som Administrator og koer scriptet igen.'
    }
    Write-Step 'Installerer Windows OpenSSH Client'
    Add-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0 | Out-Null
    $sshCommand = Get-Command ssh.exe -ErrorAction Stop
}

$sshKeyDirectory = Join-Path $HOME '.ssh'
$sshKey = Join-Path $sshKeyDirectory 'id_ed25519_opencode'
$sshPublicKey = "$sshKey.pub"
$sshConfig = Join-Path $sshKeyDirectory 'config'
$binDirectory = Join-Path $HOME 'bin'
$wrapper = Join-Path $binDirectory 'opencode.cmd'
$guiWrapper = Join-Path $binDirectory 'opencode-gui.cmd'

Write-Step 'Opretter SSH-noegle'
New-Item -ItemType Directory -Path $sshKeyDirectory -Force | Out-Null
if (-not (Test-Path $sshKey)) {
    & ssh-keygen.exe -t ed25519 -f $sshKey -N '' -C "$env:COMPUTERNAME-opencode"
}

$hostBlock = @"
Host opencode-ubuntu
    HostName $UbuntuHost
    User $UbuntuUser
    IdentityFile $sshKey
    IdentitiesOnly yes
    RequestTTY force
    ServerAliveInterval 30
    StrictHostKeyChecking accept-new
"@

Write-Step 'Opdaterer SSH-config'
$existingConfig = if (Test-Path $sshConfig) { Get-Content $sshConfig -Raw } else { '' }
if ($existingConfig -notmatch '(?m)^Host opencode-ubuntu\s*$') {
    Add-Content -Path $sshConfig -Value "`n$hostBlock" -Encoding ascii
}

Write-Step 'Kopierer public key til Ubuntu'
$publicKey = (Get-Content $sshPublicKey -Raw).Trim()
$remoteInstall = "umask 077; mkdir -p ~/.ssh; touch ~/.ssh/authorized_keys; grep -Fqx -- '$publicKey' ~/.ssh/authorized_keys || printf '%s\n' '$publicKey' >> ~/.ssh/authorized_keys; chmod 700 ~/.ssh; chmod 600 ~/.ssh/authorized_keys"
& ssh.exe -o StrictHostKeyChecking=accept-new -o IdentitiesOnly=yes -i $sshKey "$UbuntuUser@$UbuntuHost" $remoteInstall
if ($LASTEXITCODE -ne 0) {
    throw 'Kunne ikke kopiere SSH-noeglen til Ubuntu.'
}

Write-Step 'Opretter opencode.cmd'
New-Item -ItemType Directory -Path $binDirectory -Force | Out-Null
$wrapperContent = @"
@echo off
setlocal EnableExtensions
set "REMOTE_ROOT=/home/$UbuntuUser"
set "CURRENT=%CD%"
if /I "%CURRENT:~0,2%"=="$MountDrive" (
    set "REL=%CURRENT:~2%"
    set "REMOTE_DIR=%REL:\=/%"
    set "REMOTE_DIR=%REMOTE_ROOT%%REMOTE_DIR%"
) else (
    set "REMOTE_DIR=$RemoteProject"
)
ssh.exe -tt opencode-ubuntu "if [ -x /home/$UbuntuUser/.local/bin/remote-desktop-cli ] && ! pgrep -f '[r]emote-desktop-cli support-watch' >/dev/null; then mkdir -p /home/$UbuntuUser/.local/state/remote-desktop; nohup /home/$UbuntuUser/.local/bin/remote-desktop-cli support-watch >> /home/$UbuntuUser/.local/state/remote-desktop/support-watch.log 2>&1 </dev/null & fi; cd '%REMOTE_DIR%' && exec /home/$UbuntuUser/.local/bin/opencode %*"
"@
Set-Content -Path $wrapper -Value $wrapperContent -Encoding ascii

$guiSource = Join-Path $PSScriptRoot 'opencode-gui.ps1'
if (Test-Path $guiSource) {
    Copy-Item -Path $guiSource -Destination (Join-Path $binDirectory 'opencode-gui.ps1') -Force
    Set-Content -Path $guiWrapper -Value "@echo off`npowershell.exe -NoProfile -ExecutionPolicy Bypass -File `"%~dp0opencode-gui.ps1`"" -Encoding ascii
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$pathEntries = @($userPath -split ';' | Where-Object { $_ })
if ($pathEntries -notcontains $binDirectory) {
    [Environment]::SetEnvironmentVariable('Path', (($pathEntries + $binDirectory) -join ';'), 'User')
}

if ($MountUbuntu) {
    Write-Step "Mounter Ubuntu som $MountDrive"
    $sshfs = Get-Command sshfs.exe -ErrorAction SilentlyContinue
    if (-not $sshfs) {
        Write-Warning 'SSHFS-Win/WinFsp er ikke installeret. Installer WinFsp og SSHFS-Win, og koer scriptet igen med -MountUbuntu.'
    } elseif (Test-Path "$MountDrive\") {
        Write-Warning "$MountDrive er allerede i brug. Eksisterende mount blev ikke aendret."
    } else {
        & sshfs.exe "$UbuntuUser@${UbuntuHost}:/home/$UbuntuUser" $MountDrive -o "IdentityFile=$sshKey" -o StrictHostKeyChecking=accept-new -o reconnect
    }
}

Write-Host "`nSetup faerdig." -ForegroundColor Green
Write-Host 'Aabn en ny CMD/Windows Terminal, og koer: opencode'
Write-Host 'Koer opencode-gui for at vaelge en Ubuntu-mappe uden mount.'
Write-Host "Ubuntu-projekt: $RemoteProject"
if (-not $MountUbuntu) {
    Write-Host "Valgfrit mount: koer scriptet igen med -MountUbuntu efter installation af WinFsp + SSHFS-Win."
}
