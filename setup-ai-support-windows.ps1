<#
.SYNOPSIS
    One-time AI-support client enrollment for Windows.

.DESCRIPTION
    Configures the existing Windows -> Ubuntu SSH AI terminal setup (same
    model as setup-opencode-windows.ps1), installs the persistent Remote
    Desktop agent, and registers this PC in the dedicated ai_support_clients
    list via the device-enrollment Edge Function.

    SSH direction is ALWAYS Windows client -> trusted Ubuntu AI-support
    host. This script never installs a Windows SSH server or a general
    inbound SSH port. The persistent Remote Desktop agent adds its existing
    program-scoped Windows Firewall rule.

    What it does, in order:
      1. Installs the Windows OpenSSH Client capability if missing
         (requires Administrator PowerShell only when the capability is missing).
      2. Creates a dedicated ed25519 key (~/.ssh/id_ed25519_ai_support).
      3. Adds the SSH host alias 'ai-support-ubuntu' (IdentitiesOnly,
         RequestTTY, ServerAliveInterval, StrictHostKeyChecking accept-new).
      4. Copies the PUBLIC key to Ubuntu (one interactive password prompt
         at most; passwords are handled by ssh itself and never printed
         or stored by this script).
      5. Verifies key-based SSH works (BatchMode, no password fallback).
      6. Installs a safe wrapper (~/.local bin/ai-support-opencode.cmd)
         that starts the existing remote-desktop-cli support-watch watcher
         on Ubuntu and then execs opencode in the remote project.
      7. Downloads and enrolls the persistent Remote Desktop agent with its
         separate purpose-scoped one-time token, then installs and starts its
         Windows service (with a separate UAC prompt). The downloaded binary
         is SHA-256 verified before elevation. This makes the PC visible and
         controllable from Ubuntu through remote-desktop-cli ai-connect.
      8. POSTs the AI-support enrollment token and public metadata to the
         device-enrollment Edge Function. That token is used exactly once.

    The script never prints or stores passwords or private key material.
    The enrollment tokens only appear in the command line that invoked it.

.NOTES
    Syntax/static validation (run on a machine with PowerShell 5+):
      powershell -NoProfile -Command "$t=$null;$e=$null;[System.Management.Automation.Language.Parser]::ParseFile('setup-ai-support-windows.ps1',[ref]$t,[ref]$e)|Out-Null;if($e){$e;exit 1}else{'OK'}"
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$EnrollmentUrl,
    [Parameter(Mandatory = $true)]
    [string]$EnrollmentToken,
    [Parameter(Mandatory = $true)]
    [string]$AgentEnrollmentToken,
    [Parameter(Mandatory = $true)]
    [string]$ClientName,
    [string]$UbuntuHost = '192.168.1.92',
    [string]$UbuntuUser = 'dennis',
    [string]$RemoteProject = '/home/dennis/projekter/aisupport',
    [int]$Port = 22,
    [string]$ClientId = '',
    [Parameter(Mandatory = $true)]
    [string]$AgentDownloadUrl,
    [Parameter(Mandatory = $true)]
    [string]$AgentSha256
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

# ---------- Input validation (no shell interpolation of arbitrary input) ----------

if ($EnrollmentUrl -notmatch '^https://[A-Za-z0-9._:/?=&-]{1,200}$') {
    throw 'EnrollmentUrl skal vaere en gyldig https-URL.'
}
if ([string]::IsNullOrWhiteSpace($EnrollmentToken) -or $EnrollmentToken.Length -gt 200) {
    throw 'EnrollmentToken mangler eller er ugyldigt.'
}
if ([string]::IsNullOrWhiteSpace($AgentEnrollmentToken) -or $AgentEnrollmentToken.Length -gt 200) {
    throw 'AgentEnrollmentToken mangler eller er ugyldigt.'
}
if ($AgentDownloadUrl -notmatch '^https://[A-Za-z0-9._:/?=&-]{1,200}$') {
    throw 'AgentDownloadUrl skal vaere en gyldig https-URL.'
}
if ($AgentSha256 -notmatch '^[A-Fa-f0-9]{64}$') {
    throw 'AgentSha256 skal vaere en SHA-256 hash.'
}
if ($UbuntuHost -notmatch '^[A-Za-z0-9._:-]{1,100}$') {
    throw 'UbuntuHost indeholder ugyldige tegn.'
}
if ($UbuntuUser -notmatch '^[A-Za-z0-9._-]{1,32}$') {
    throw 'UbuntuUser indeholder ugyldige tegn.'
}
if ($RemoteProject -notmatch '^/[A-Za-z0-9/._-]{1,100}$') {
    throw 'RemoteProject skal vaere en absolut sti med gyldige tegn.'
}
if ($Port -lt 1 -or $Port -gt 65535) {
    throw 'Port skal vaere mellem 1 og 65535.'
}
$safeClientName = ($ClientName -replace '[^\p{L}\p{N}\s._-]', '').Trim()
if (-not $safeClientName -or $safeClientName.Length -gt 64) {
    throw 'ClientName skal vaere mellem 1 og 64 tegn.'
}

# Stable, safe client identifier: ai-<16 lowercase hex>.
if ($ClientId -notmatch '^(ai-[a-z0-9]{8,32})?$') {
    throw 'ClientId har et ugyldigt format.'
}
if (-not $ClientId) {
    $bytes = New-Object byte[] 8
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
    $ClientId = 'ai-' + (($bytes | ForEach-Object { $_.ToString('x2') }) -join '')
}

try {

    # ---------- 1. OpenSSH Client (admin only for this step) ----------

    $sshCommand = Get-Command ssh.exe -ErrorAction SilentlyContinue
    if (-not $sshCommand) {
        if (-not (Test-Administrator)) {
            throw 'OpenSSH Client mangler. Aabn en Administrator-PowerShell og koer scriptet igen (kun dette trin kraever admin).'
        }
        Write-Step 'Installerer Windows OpenSSH Client'
        Add-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0 | Out-Null
        $sshCommand = Get-Command ssh.exe -ErrorAction Stop
    }
    else {
        Write-Step 'OpenSSH Client fundet - ingen admin noedvendig'
    }
    $sshKeygenCommand = Get-Command ssh-keygen.exe -ErrorAction Stop

    # ---------- 2. Dedicated ed25519 key ----------

    $sshKeyDirectory = Join-Path $HOME '.ssh'
    $sshKey = Join-Path $sshKeyDirectory 'id_ed25519_ai_support'
    $sshPublicKey = "$sshKey.pub"
    $sshConfig = Join-Path $sshKeyDirectory 'config'

    Write-Step 'Opretter dedikeret SSH-noegle'
    New-Item -ItemType Directory -Path $sshKeyDirectory -Force | Out-Null
    if ((Test-Path $sshKey) -and -not (Test-Path $sshPublicKey)) {
        # A previous interrupted keygen can leave only a partial private file.
        # It is unusable without its public half and safe to remove.
        Remove-Item -Force $sshKey
    }
    if (-not (Test-Path $sshKey)) {
        # Windows PowerShell 5.1 drops an empty native argument when invoked
        # as `-N ''`. Start-Process preserves the explicit `""` argument.
        $keygen = Start-Process -FilePath $sshKeygenCommand.Source -ArgumentList @(
            '-t', 'ed25519', '-f', $sshKey, '-N', '""', '-C', $ClientId
        ) -Wait -PassThru -NoNewWindow
        if ($keygen.ExitCode -ne 0) {
            Remove-Item -Force -ErrorAction SilentlyContinue $sshKey, $sshPublicKey
            throw 'Kunne ikke generere SSH-noeglen.'
        }
    }
    if (-not (Test-Path $sshPublicKey)) { throw 'SSH-noeglen blev ikke oprettet korrekt.' }

    # ---------- 3. SSH host alias ----------

    $hostBlock = @"
Host ai-support-ubuntu
    HostName $UbuntuHost
    Port $Port
    User $UbuntuUser
    IdentityFile $sshKey
    IdentitiesOnly yes
    RequestTTY force
    ServerAliveInterval 30
    StrictHostKeyChecking accept-new
"@

    Write-Step 'Opdaterer SSH-config'
    $existingConfig = if (Test-Path $sshConfig) { Get-Content $sshConfig -Raw } else { '' }
    if ($existingConfig -notmatch '(?m)^Host ai-support-ubuntu\s*$') {
        Add-Content -Path $sshConfig -Value "`n$hostBlock" -Encoding ascii
    }

    # ---------- 4. Copy PUBLIC key to Ubuntu (existing key-copy model) ----------

    Write-Step 'Kopierer public key til Ubuntu (max. een password-promt, haandteret af ssh selv)'
    $publicKey = (Get-Content $sshPublicKey -Raw).Trim()
    # Strict format check: the key is embedded in a single-quoted remote shell
    # command, so reject anything outside the canonical openssh format.
    if ($publicKey -notmatch '^ssh-ed25519 [A-Za-z0-9+/]{68}( ai-[a-z0-9]{8,32})?$') {
        throw 'Public key har et uventet format. Slet noeglen og koer scriptet igen.'
    }
    $remoteInstall = "umask 077; mkdir -p ~/.ssh; touch ~/.ssh/authorized_keys; grep -Fqx -- '$publicKey' ~/.ssh/authorized_keys || printf '%s\n' '$publicKey' >> ~/.ssh/authorized_keys; chmod 700 ~/.ssh; chmod 600 ~/.ssh/authorized_keys"
    & ssh.exe -o StrictHostKeyChecking=accept-new -o IdentitiesOnly=yes -i $sshKey -p $Port "$UbuntuUser@$UbuntuHost" $remoteInstall
    if ($LASTEXITCODE -ne 0) {
        throw 'Kunne ikke kopiere SSH-noeglen til Ubuntu.'
    }

    # ---------- 5. Verify key-based SSH (no password fallback) ----------

    Write-Step 'Verificerer SSH-opsaetning'
    & ssh.exe -o BatchMode=yes ai-support-ubuntu 'echo ai-support-ok'
    if ($LASTEXITCODE -ne 0) {
        throw 'SSH-verifikation fejlede (noeglebaseret login virker ikke endnu).'
    }

    # ---------- 6. Safe opencode wrapper (starts remote support watcher) ----------

    Write-Step 'Opretter ai-support-opencode.cmd wrapper'
    $binDirectory = Join-Path $HOME 'bin'
    $wrapper = Join-Path $binDirectory 'ai-support-opencode.cmd'
    New-Item -ItemType Directory -Path $binDirectory -Force | Out-Null
    $remoteHome = "/home/$UbuntuUser"
    $wrapperContent = @"
@echo off
setlocal EnableExtensions
ssh.exe -tt ai-support-ubuntu "if [ -x $remoteHome/.local/bin/remote-desktop-cli ] && ! pgrep -f '[r]emote-desktop-cli support-watch' >/dev/null; then mkdir -p $remoteHome/.local/state/remote-desktop; nohup $remoteHome/.local/bin/remote-desktop-cli support-watch >> $remoteHome/.local/state/remote-desktop/support-watch.log 2>&1 </dev/null & fi; cd '$RemoteProject' && exec $remoteHome/.local/bin/opencode"
"@
    Set-Content -Path $wrapper -Value $wrapperContent -Encoding ascii

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $pathEntries = @($userPath -split ';' | Where-Object { $_ })
    if ($pathEntries -notcontains $binDirectory) {
        [Environment]::SetEnvironmentVariable('Path', (($pathEntries + $binDirectory) -join ';'), 'User')
    }

    # ---------- 7. Public key fingerprint (public key only) ----------

    $fingerprintOutput = (& ssh-keygen.exe -lf $sshPublicKey) | Select-Object -First 1
    $fingerprint = ''
    if ($fingerprintOutput -match '(SHA256:[A-Za-z0-9+/=]{43})') {
        $fingerprint = $Matches[1]
    }

    # ---------- 8. Persistent Remote Desktop agent enrollment ----------

    Write-Step 'Installerer persistent Remote Desktop agent'
    try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch { }
    $agentDirectory = Join-Path $env:TEMP 'RemoteDesktopAISupport'
    $agentPath = Join-Path $agentDirectory 'remote-agent.exe'
    New-Item -ItemType Directory -Path $agentDirectory -Force | Out-Null
    Invoke-WebRequest -UseBasicParsing -Uri $AgentDownloadUrl -OutFile $agentPath
    if (-not (Test-Path $agentPath)) { throw 'Remote Desktop agent blev ikke downloadet.' }
    $actualHash = (Get-FileHash -Algorithm SHA256 -Path $agentPath).Hash
    if ($actualHash -and $actualHash.ToLowerInvariant() -ne $AgentSha256.ToLowerInvariant()) {
        Remove-Item -Force -ErrorAction SilentlyContinue $agentPath
        throw 'Remote Desktop agent checksum matcher ikke det forventede artifact.'
    }
    # Start-Process joins an argument array before launching the process. Quote
    # values explicitly so names containing spaces remain one Go flag value.
    $agentArguments = '--enroll-token "' + $AgentEnrollmentToken + '" --device-name "' + $safeClientName + '" --install --start'
    $agentProcess = Start-Process -FilePath $agentPath -Verb RunAs -Wait -PassThru -ArgumentList $agentArguments
    if ($agentProcess.ExitCode -ne 0) {
        throw 'Remote Desktop agent enrollment eller service-start fejlede.'
    }
    $agentRunning = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        $service = Get-Service -Name 'RemoteDesktopAgent' -ErrorAction SilentlyContinue
        if ($service -and $service.Status -eq 'Running') {
            $agentRunning = $true
            break
        }
        Start-Sleep -Seconds 1
    }
    if (-not $agentRunning) {
        throw 'Remote Desktop agentens Windows Service kører ikke efter enrollment.'
    }

    # ---------- 9. One-time AI-support enrollment POST ----------

    Write-Step 'Registrerer klienten (engangstoken)'
    try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch { }
    $hostname = $env:COMPUTERNAME
    if ($hostname) { $hostname = $hostname.Substring(0, [Math]::Min(100, $hostname.Length)) }
    $platform = "windows-$($env:PROCESSOR_ARCHITECTURE)".ToLowerInvariant()
    if ($platform.Length -gt 50) { $platform = $platform.Substring(0, 50) }

    $payload = @{
        action               = 'enroll-ai-support'
        enrollment_token     = $EnrollmentToken
        client_id            = $ClientId
        hostname             = $hostname
        platform             = $platform
        ssh_host             = $UbuntuHost
        ssh_port             = $Port
        ssh_user             = $UbuntuUser
        ssh_key_fingerprint  = $fingerprint
    } | ConvertTo-Json

    $result = Invoke-RestMethod -Uri $EnrollmentUrl -Method Post -ContentType 'application/json' -Body $payload
    if (-not $result -or $result.status -ne 'registered') {
        throw 'Registreringen blev ikke gennemfoert.'
    }

    Write-Host "`nSetup faerdig." -ForegroundColor Green
    Write-Host "AI-support klient registreret: $($result.client_name) ($($result.client_id))"
    Write-Host 'Fra Ubuntu kan du nu koere: remote-desktop-cli ai-connect <klientnavn>'
    Write-Host 'For lokal Windows -> Ubuntu terminal: aabn en ny CMD/Windows Terminal, og koer ai-support-opencode'
    Write-Host "Ubuntu AI-support projekt: $RemoteProject"
    Write-Host 'Retning: Windows -> Ubuntu SSH + outbound WebRTC-agent. Ingen generel inbound Windows-SSH-port er aabnet.'

}
catch {
    # Bounded, generic error output. Never echo the token or key material.
    $message = $_.Exception.Message
    if ($null -ne $message -and $message.Length -gt 300) { $message = $message.Substring(0, 300) }
    Write-Host "`nAI-support setup fejlede: $message" -ForegroundColor Red
    Write-Host 'Ret fejlen og koer scriptet igen med et nyt engangstoken fra dashboardet.' -ForegroundColor Yellow
    exit 1
}
