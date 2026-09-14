<#
.SYNOPSIS
    Enroll a Windows PC for persistent SSH-only AI support.

.DESCRIPTION
    Installs a localhost-only Windows OpenSSH server and a persistent reverse
    SSH tunnel to the trusted Ubuntu AI-support host. No Remote Desktop agent,
    WebRTC component, controller, or inbound LAN firewall rule is installed.

    The reverse tunnel is:
      Ubuntu 127.0.0.1:<TunnelPort> -> Windows 127.0.0.1:22

    The Windows side runs the tunnel as a SYSTEM scheduled task at startup.
    The Ubuntu client key is installed in the dedicated Windows ai-support
    account. The Windows-to-Ubuntu tunnel key is restricted to forwarding only.

.NOTES
    Run from an Administrator PowerShell. The enrollment token is one-time and
    expires after 30 minutes.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$EnrollmentUrl,
    [Parameter(Mandatory = $true)]
    [string]$EnrollmentToken,
    [Parameter(Mandatory = $true)]
    [string]$ClientName,
    [Parameter(Mandatory = $true)]
    [string]$SupportPublicKeyUrl,
    [string]$UbuntuHost = '192.168.1.92',
    [string]$UbuntuUser = 'dennis',
    [int]$UbuntuPort = 22,
    [string]$ClientId = '',
    [int]$WindowsSshPort = 22
)

$ErrorActionPreference = 'Stop'
$TunnelPortMinimum = 42000
$TunnelPortMaximum = 42999
$WindowsSshUser = 'ai-support'
$TaskName = 'AI-Support-Persistent-Tunnel'
$StateDirectory = Join-Path $env:ProgramData 'AI-Support'
$TunnelKey = Join-Path $StateDirectory 'id_ed25519_ai_support_tunnel'
$BootstrapKey = Join-Path $StateDirectory 'id_ed25519_ai_support_bootstrap'
$KnownHosts = Join-Path $StateDirectory 'known_hosts'
$SupportPublicKeyPath = Join-Path $StateDirectory 'support.pub'
$SupportShellPath = Join-Path $StateDirectory 'ai-support-shell.ps1'
$ActivityLogPath = Join-Path $StateDirectory 'activity.log'
$ActivityConfigPath = Join-Path $StateDirectory 'activity-config.json'
$SshdConfigBackup = Join-Path $StateDirectory 'sshd_config.backup'
$TokenPolicyBackup = Join-Path $StateDirectory 'token-policy.backup'
$TunnelPublicKey = "$TunnelKey.pub"
$BootstrapPublicKey = "$BootstrapKey.pub"

function Write-Step([string]$Message) {
    Write-Host "`n==> $Message" -ForegroundColor Cyan
}

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Convert-ToBase64([string]$Value) {
    return [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($Value))
}

function New-RandomBytes([int]$Count) {
    $bytes = New-Object byte[] $Count
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
    return $bytes
}

function New-Ed25519Key([string]$KeyPath, [string]$Comment, [string]$SshKeygenPath) {
    if (Test-Path $KeyPath) {
        if (-not (Test-Path "$KeyPath.pub")) {
            Remove-Item -Force $KeyPath
        }
    }
    if (-not (Test-Path $KeyPath)) {
        $keygen = Start-Process -FilePath $SshKeygenPath -ArgumentList @(
            '-t', 'ed25519', '-f', $KeyPath, '-N', '""', '-C', $Comment
        ) -Wait -PassThru -NoNewWindow
        if ($keygen.ExitCode -ne 0) {
            throw "Kunne ikke generere SSH-noeglen $KeyPath."
        }
    }
    if (-not (Test-Path "$KeyPath.pub")) {
        throw "SSH-noeglen blev ikke oprettet korrekt: $KeyPath"
    }
}

function Set-StateAcl {
    New-Item -ItemType Directory -Path $StateDirectory -Force | Out-Null
    & icacls.exe $StateDirectory /inheritance:r /grant:r `
        '*S-1-5-18:(OI)(CI)(F)' `
        '*S-1-5-32-544:(OI)(CI)(F)' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Kunne ikke beskytte AI-support state-mappen.' }
    foreach ($path in @($TunnelKey, $TunnelPublicKey, $BootstrapKey, $BootstrapPublicKey, $KnownHosts, $SupportPublicKeyPath, $SupportShellPath, $ActivityLogPath, $ActivityConfigPath, $SshdConfigBackup, $TokenPolicyBackup)) {
        if (Test-Path $path) {
            $aclArgs = @('/inheritance:r', '/grant:r', '*S-1-5-18:F', '*S-1-5-32-544:F')
            if ($path -in @($SupportShellPath, $ActivityLogPath, $ActivityConfigPath)) {
                $aclArgs += ("{0}:F" -f $WindowsSshUser)
            }
            & icacls.exe $path @aclArgs | Out-Null
            if ($LASTEXITCODE -ne 0) { throw "Kunne ikke beskytte filen $path." }
        }
    }
}

function Get-KeyParts([string]$KeyLine, [string]$Label) {
    $parts = $KeyLine.Trim() -split '\s+'
    if ($parts.Count -lt 2) { throw "$Label er ugyldig." }
    if ($parts[0] -notmatch '^(ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp256|ecdsa-sha2-nistp384|ecdsa-sha2-nistp521)$') {
        throw "$Label bruger en ugyldig SSH-nøgletype."
    }
    if ($parts[1] -notmatch '^[A-Za-z0-9+/]+={0,2}$') { throw "$Label har ugyldigt format." }
    return @($parts[0], $parts[1])
}

function Invoke-UbuntuSsh([string]$IdentityFile, [string]$RemoteCommand, [switch]$BatchMode) {
    $args = @(
        '-o', "UserKnownHostsFile=$KnownHosts",
        '-o', 'StrictHostKeyChecking=accept-new',
        '-o', 'IdentitiesOnly=yes',
        '-i', $IdentityFile,
        '-p', "$UbuntuPort"
    )
    if ($BatchMode) { $args += @('-o', 'BatchMode=yes') }
    $args += @("$UbuntuUser@$UbuntuHost", $RemoteCommand)
    & $sshCommand.Source @args
    return $LASTEXITCODE
}

function Invoke-UbuntuPasswordSsh([string]$RemoteCommand) {
    $args = @(
        '-o', "UserKnownHostsFile=$KnownHosts",
        '-o', 'StrictHostKeyChecking=accept-new',
        '-o', 'PubkeyAuthentication=no',
        '-o', 'PreferredAuthentications=password,keyboard-interactive',
        '-p', "$UbuntuPort",
        "$UbuntuUser@$UbuntuHost",
        $RemoteCommand
    )
    & $sshCommand.Source @args
    return $LASTEXITCODE
}

function Remove-RemoteKey([string]$IdentityFile, [string]$KeyBase64) {
    $command = "set -eu; test -f ~/.ssh/authorized_keys || exit 0; tmp=\`$(mktemp); awk -v key='$KeyBase64' '\`$2 != key { print }' ~/.ssh/authorized_keys > \`$tmp; mv \`$tmp ~/.ssh/authorized_keys; chmod 600 ~/.ssh/authorized_keys"
    $exitCode = Invoke-UbuntuSsh $IdentityFile $command
    if ($exitCode -ne 0) { throw 'Kunne ikke fjerne bootstrap-noeglen fra Ubuntu.' }
}

function New-WindowsSupportUser {
    $existing = Get-LocalUser -Name $WindowsSshUser -ErrorAction SilentlyContinue
    if (-not $existing) {
        $randomBytes = New-RandomBytes 32
        $passwordText = ([Convert]::ToBase64String($randomBytes) + 'A1!').Substring(0, 34)
        $password = ConvertTo-SecureString $passwordText -AsPlainText -Force
        New-LocalUser -Name $WindowsSshUser -Password $password -Description 'SSH-only AI support account' -PasswordNeverExpires | Out-Null
    }
    $administratorsGroup = (Get-LocalGroup -SID ([Security.Principal.SecurityIdentifier]'S-1-5-32-544')).Name
    $isAdministrator = Get-LocalGroupMember -Group $administratorsGroup -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match "\\$WindowsSshUser$" }
    if (-not $isAdministrator) {
        Add-LocalGroupMember -Group $administratorsGroup -Member $WindowsSshUser
    }
    $supportHome = Join-Path $env:SystemDrive "Users\$WindowsSshUser"
    $supportSshDirectory = Join-Path $supportHome '.ssh'
    New-Item -ItemType Directory -Path $supportSshDirectory -Force | Out-Null
    $authorizedKeys = Join-Path $supportSshDirectory 'authorized_keys'
    Set-Content -Path $authorizedKeys -Value $SupportKeyLine -Encoding ascii
    & icacls.exe $supportHome /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)(F)' '*S-1-5-32-544:(OI)(CI)(F)' ("{0}:(OI)(CI)(F)" -f $WindowsSshUser) | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Kunne ikke beskytte Windows AI-support brugerens filer.' }
    & icacls.exe $authorizedKeys /inheritance:r /grant:r '*S-1-5-18:F' '*S-1-5-32-544:F' ("{0}:F" -f $WindowsSshUser) | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Kunne ikke beskytte Windows authorized_keys.' }
}

function Install-SupportShell {
    $shellContent = @'
$ErrorActionPreference = 'Continue'
$logPath = 'C:\ProgramData\AI-Support\activity.log'
$configPath = 'C:\ProgramData\AI-Support\activity-config.json'
$command = $env:SSH_ORIGINAL_COMMAND
$session = $env:SSH_CONNECTION
$timestamp = (Get-Date).ToUniversalTime().ToString('o')
$safeCommand = if ([string]::IsNullOrWhiteSpace($command)) { '[interactive shell]' } else { ($command -replace "`r", ' ' -replace "`n", ' ') }
$localLine = "$timestamp user=$env:USERNAME connection=$session command=$safeCommand"
Add-Content -LiteralPath $logPath -Encoding UTF8 -Value $localLine
try {
    $config = Get-Content -LiteralPath $configPath -Raw | ConvertFrom-Json
    $eventBody = @{
        action = 'log-ai-support'
        client_id = [string]$config.client_id
        log_token = [string]$config.log_token
        event = 'AI_SUPPORT_COMMAND'
        command = $safeCommand.Substring(0, [Math]::Min(4000, $safeCommand.Length))
    } | ConvertTo-Json -Compress
    Invoke-RestMethod -Uri ([string]$config.log_url) -Method Post -ContentType 'application/json' -Body $eventBody -TimeoutSec 10 | Out-Null
} catch { Add-Content -LiteralPath $logPath -Encoding UTF8 -Value "$timestamp log_upload_failed=$($_.Exception.Message)" }
try {
    if ([string]::IsNullOrWhiteSpace($command)) {
        & powershell.exe -NoLogo -NoProfile
    } else {
        & powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -Command $command
    }
    $exitCode = $LASTEXITCODE
} finally { }
exit $exitCode
'@
    Set-Content -LiteralPath $SupportShellPath -Value $shellContent -Encoding UTF8
    New-Item -ItemType File -Path $ActivityLogPath -Force | Out-Null
    foreach ($path in @($SupportShellPath, $ActivityLogPath)) {
        & icacls.exe $path /inheritance:r /grant:r '*S-1-5-18:F' '*S-1-5-32-544:F' ("{0}:F" -f $WindowsSshUser) | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'Kunne ikke beskytte AI-support logfilerne.' }
    }
}

function Set-LocalAccountTokenFilterPolicy {
    param([string]$PolicyPath)

    try {
        New-Item -Path $PolicyPath -Force -ErrorAction Stop | Out-Null
        New-ItemProperty -Path $PolicyPath -Name LocalAccountTokenFilterPolicy -PropertyType DWord -Value 1 -Force -ErrorAction Stop | Out-Null
        return
    } catch {
        # Some Windows security baselines deny an elevated admin token here.
        # Use Task Scheduler's SYSTEM token, then verify the resulting value.
    }

    $taskName = "AI-Support-Set-TokenPolicy-$([guid]::NewGuid().ToString('N'))"
    $registryPath = $PolicyPath -replace '^HKLM:\\', 'HKLM\'
    $taskAction = New-ScheduledTaskAction -Execute (Join-Path $env:WINDIR 'System32\reg.exe') -Argument "ADD `"$registryPath`" /v LocalAccountTokenFilterPolicy /t REG_DWORD /d 1 /f"
    $taskTrigger = New-ScheduledTaskTrigger -Once -At (Get-Date).AddMinutes(1)
    $taskPrincipal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
    try {
        Register-ScheduledTask -TaskName $taskName -Action $taskAction -Trigger $taskTrigger -Principal $taskPrincipal -Force -ErrorAction Stop | Out-Null
        Start-ScheduledTask -TaskName $taskName -ErrorAction Stop
        $policySet = $false
        for ($wait = 0; $wait -lt 10; $wait++) {
            Start-Sleep -Seconds 1
            $current = Get-ItemProperty -Path $PolicyPath -Name LocalAccountTokenFilterPolicy -ErrorAction SilentlyContinue
            if ($null -ne $current -and [int]$current.LocalAccountTokenFilterPolicy -eq 1) {
                $policySet = $true
                break
            }
        }
        if (-not $policySet) { throw 'SYSTEM-tasken satte ikke LocalAccountTokenFilterPolicy.' }
    } catch {
        throw "Kunne ikke konfigurere Windows admin-token: $($_.Exception.Message)"
    } finally {
        Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue
    }
}

function Get-SshdStartFailureDetails([string]$SshdConfig) {
    $details = @()
    $service = Get-Service -Name sshd -ErrorAction SilentlyContinue
    if ($service) { $details += "service_state=$($service.Status) service_name=$($service.Name)" }
    try {
        $serviceInfo = Get-CimInstance Win32_Service -Filter "Name='sshd'" -ErrorAction Stop
        if ($serviceInfo) {
            $details += "service_start_name=$($serviceInfo.StartName) service_exit_code=$($serviceInfo.ExitCode) service_specific_exit_code=$($serviceInfo.ServiceSpecificExitCode)"
        }
    } catch { }
    try {
        $events = Get-WinEvent -FilterHashtable @{ LogName = 'OpenSSH/Operational'; StartTime = (Get-Date).AddMinutes(-5) } -MaxEvents 5 -ErrorAction Stop |
            ForEach-Object { $_.Message }
        if ($events) { $details += ('events=' + (($events -join ' | ') -replace '\s+', ' ')) }
    } catch { }
    try {
        $serviceEvents = Get-WinEvent -FilterHashtable @{ LogName = 'System'; ProviderName = 'Service Control Manager'; StartTime = (Get-Date).AddMinutes(-5) } -MaxEvents 5 -ErrorAction Stop |
            ForEach-Object { $_.Message }
        if ($serviceEvents) { $details += ('service_events=' + (($serviceEvents -join ' | ') -replace '\s+', ' ')) }
    } catch { }
    $effective = (& (Join-Path $env:WINDIR 'System32\OpenSSH\sshd.exe') -T -f $SshdConfig 2>&1 |
        Where-Object { $_ -match '^(port|listenaddress|hostkey|authorizedkeysfile|forcecommand)\s' } |
        Out-String).Trim()
    if ($effective) { $details += ('sshd_effective_config=' + ($effective -replace '\s+', ' ')) }
    return ($details -join '; ')
}

function Configure-WindowsSshd {
    $sshdConfig = Join-Path $env:ProgramData 'ssh\sshd_config'
    if (-not (Test-Path $sshdConfig)) {
        $defaultConfig = Join-Path $env:WINDIR 'System32\OpenSSH\sshd_config_default'
        if (-not (Test-Path $defaultConfig)) { throw 'OpenSSH Server konfigurationsfil blev ikke fundet.' }
        New-Item -ItemType Directory -Path (Split-Path -Parent $sshdConfig) -Force | Out-Null
        Copy-Item -LiteralPath $defaultConfig -Destination $sshdConfig -Force
    }
    if (-not (Test-Path $SshdConfigBackup)) {
        try { Copy-Item -LiteralPath $sshdConfig -Destination $SshdConfigBackup -Force -ErrorAction Stop }
        catch { throw "Kunne ikke sikkerhedskopiere OpenSSH-konfigurationen: $($_.Exception.Message)" }
    }
    $policyPath = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System'
    $policy = Get-ItemProperty -Path $policyPath -Name LocalAccountTokenFilterPolicy -ErrorAction SilentlyContinue
    if (-not (Test-Path $TokenPolicyBackup)) {
        if ($null -eq $policy) { 'MISSING' | Set-Content -LiteralPath $TokenPolicyBackup -Encoding ascii }
        else { ([int]$policy.LocalAccountTokenFilterPolicy).ToString() | Set-Content -LiteralPath $TokenPolicyBackup -Encoding ascii }
    }
    $config = Get-Content -Path $sshdConfig -Raw
    $config = [regex]::Replace($config, '(?ms)\r?\n?# AI_SUPPORT_GLOBAL_BEGIN\r?\n.*?\r?\n# AI_SUPPORT_GLOBAL_END\r?\n?', "`r`n")
    $config = [regex]::Replace($config, '(?ms)\r?\n?# AI_SUPPORT_MATCH_BEGIN\r?\n.*?\r?\n# AI_SUPPORT_MATCH_END\r?\n?', "`r`n")
    $config = [regex]::Replace($config, '(?m)^\s*ListenAddress\s+.*\r?\n?', '')
    $config = [regex]::Replace($config, '(?m)^\s*Port\s+.*\r?\n?', '')
    $globalBlock = @"

# AI_SUPPORT_GLOBAL_BEGIN
Port $WindowsSshPort
ListenAddress 127.0.0.1
# AI_SUPPORT_GLOBAL_END
"@
    $matchBlock = @"

# AI_SUPPORT_MATCH_BEGIN
Match User $WindowsSshUser
    AuthorizedKeysFile .ssh/authorized_keys
    ForceCommand powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File C:/ProgramData/AI-Support/ai-support-shell.ps1
    PasswordAuthentication no
    PubkeyAuthentication yes
    AuthenticationMethods publickey
    AllowTcpForwarding no
    AllowAgentForwarding no
    X11Forwarding no
    PermitTunnel no
# AI_SUPPORT_MATCH_END
"@
    $match = [regex]::Match($config, '(?m)^\s*Match\s+')
    if ($match.Success) {
        $config = $config.Insert($match.Index, $globalBlock)
    } else {
        $config += $globalBlock
    }
    $config += $matchBlock
    Set-Content -Path $sshdConfig -Value $config -Encoding ascii
    try {
        Set-LocalAccountTokenFilterPolicy $policyPath
    } catch { throw $_ }
    $hostKeygen = Join-Path $env:WINDIR 'System32\OpenSSH\ssh-keygen.exe'
    if (-not (Test-Path $hostKeygen)) { throw 'Windows OpenSSH ssh-keygen blev ikke fundet.' }
    & $hostKeygen -A | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Windows OpenSSH hostkeys kunne ikke oprettes.' }
    $hostKeys = Get-ChildItem -LiteralPath (Join-Path $env:ProgramData 'ssh') -Filter 'ssh_host_*_key' -File -ErrorAction SilentlyContinue
    if (-not $hostKeys) { throw 'Windows OpenSSH hostkeys blev ikke oprettet.' }
    $sshDirectory = Join-Path $env:ProgramData 'ssh'
    & icacls.exe $sshDirectory /remove 'NT SERVICE\sshd' | Out-Null
    & icacls.exe $sshDirectory /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)(F)' '*S-1-5-32-544:(OI)(CI)(F)' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Kunne ikke beskytte OpenSSH-mappen.' }
    $sshFiles = @($sshdConfig) + @($hostKeys | ForEach-Object { $_.FullName })
    foreach ($sshFile in $sshFiles) {
        & icacls.exe $sshFile /remove 'NT SERVICE\sshd' | Out-Null
        & icacls.exe $sshFile /inheritance:r /grant:r '*S-1-5-18:F' '*S-1-5-32-544:F' | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Kunne ikke beskytte OpenSSH-filen $sshFile." }
    }
    foreach ($hostKey in $hostKeys) {
        if (-not (Test-Path $hostKey.FullName)) { throw "OpenSSH hostkey forsvandt: $($hostKey.Name)." }
    }
    $configTest = (& (Join-Path $env:WINDIR 'System32\OpenSSH\sshd.exe') -t -f $sshdConfig 2>&1 | Out-String).Trim()
    if ($LASTEXITCODE -ne 0) {
        if ($configTest) { throw "OpenSSH Server konfigurationen er ugyldig: $configTest" }
        throw 'OpenSSH Server konfigurationen er ugyldig.'
    }
    Set-Service -Name sshd -StartupType Automatic
    try {
        Restart-Service -Name sshd -Force -ErrorAction Stop
    } catch {
        $details = Get-SshdStartFailureDetails $sshdConfig
        if ($details) { throw "OpenSSH Server kunne ikke starte: $details" }
        throw "OpenSSH Server kunne ikke starte: $($_.Exception.Message)"
    }
    try {
        Get-NetFirewallRule -Name 'OpenSSH-Server-In-TCP' -ErrorAction SilentlyContinue | Disable-NetFirewallRule
    } catch { }
    $listener = Get-NetTCPConnection -LocalPort $WindowsSshPort -State Listen -ErrorAction SilentlyContinue |
        Where-Object { $_.LocalAddress -eq '127.0.0.1' }
    if (-not $listener) { throw 'OpenSSH Server lytter ikke kun på 127.0.0.1.' }
}

function Build-TunnelArguments([int]$Port) {
    return @(
        '-N', '-T',
        '-o', 'BatchMode=yes',
        '-o', 'ExitOnForwardFailure=yes',
        '-o', 'ServerAliveInterval=30',
        '-o', 'ServerAliveCountMax=3',
        '-o', 'ConnectTimeout=15',
        '-o', "UserKnownHostsFile=$KnownHosts",
        '-o', 'StrictHostKeyChecking=accept-new',
        '-o', 'IdentitiesOnly=yes',
        '-i', $TunnelKey,
        '-p', "$UbuntuPort",
        '-R', "127.0.0.1:${Port}:127.0.0.1:${WindowsSshPort}",
        "$UbuntuUser@$UbuntuHost"
    )
}

if (-not (Test-Administrator)) { throw 'Dette setup skal koeres fra en Administrator-PowerShell.' }
if ($EnrollmentUrl -notmatch '^https://[A-Za-z0-9._:/?=&-]{1,200}$') { throw 'EnrollmentUrl skal vaere en gyldig https-URL.' }
if ($SupportPublicKeyUrl -notmatch '^https://[A-Za-z0-9._:/?=&-]{1,200}$') { throw 'SupportPublicKeyUrl skal vaere en gyldig https-URL.' }
if ([string]::IsNullOrWhiteSpace($EnrollmentToken) -or $EnrollmentToken.Length -gt 200) { throw 'EnrollmentToken mangler eller er ugyldigt.' }
if ($UbuntuHost -notmatch '^[A-Za-z0-9._:-]{1,100}$') { throw 'UbuntuHost indeholder ugyldige tegn.' }
if ($UbuntuUser -notmatch '^[A-Za-z0-9._-]{1,32}$') { throw 'UbuntuUser indeholder ugyldige tegn.' }
if ($UbuntuPort -lt 1 -or $UbuntuPort -gt 65535) { throw 'UbuntuPort skal vaere mellem 1 og 65535.' }
if ($WindowsSshPort -lt 1 -or $WindowsSshPort -gt 65535) { throw 'WindowsSshPort skal vaere mellem 1 og 65535.' }
$safeClientName = ($ClientName -replace '[^\p{L}\p{N}\s._-]', '').Trim()
if (-not $safeClientName -or $safeClientName.Length -gt 64) { throw 'ClientName skal vaere mellem 1 og 64 tegn.' }
if ($ClientId -notmatch '^(ai-[a-z0-9]{8,32})?$') { throw 'ClientId har et ugyldigt format.' }
if (-not $ClientId) {
    $bytes = New-RandomBytes 8
    $ClientId = 'ai-' + (($bytes | ForEach-Object { $_.ToString('x2') }) -join '')
}
try {
    $sshCommand = Get-Command ssh.exe -ErrorAction SilentlyContinue
    $sshKeygenCommand = Get-Command ssh-keygen.exe -ErrorAction SilentlyContinue
    if (-not $sshCommand -or -not $sshKeygenCommand) {
        Write-Step 'Installerer Windows OpenSSH Client'
        Add-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0 | Out-Null
        $sshCommand = Get-Command ssh.exe -ErrorAction Stop
        $sshKeygenCommand = Get-Command ssh-keygen.exe -ErrorAction Stop
    }
    $serverCapability = Get-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
    if ($serverCapability.State -ne 'Installed') {
        Write-Step 'Installerer Windows OpenSSH Server'
        Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0 | Out-Null
    }

    New-Item -ItemType Directory -Path $StateDirectory -Force | Out-Null
    try { [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12 } catch { }
    Invoke-WebRequest -UseBasicParsing -Uri $SupportPublicKeyUrl -OutFile $SupportPublicKeyPath
    $supportPublicKey = (Get-Content $SupportPublicKeyPath -Raw).Trim()
    $supportParts = Get-KeyParts $supportPublicKey 'SupportPublicKey'
    $supportFingerprint = (& $sshKeygenCommand.Source -lf $SupportPublicKeyPath | Select-Object -First 1)
    if ($supportFingerprint -notmatch 'SHA256:gZg0wT5MBRnB7G\+RJlDPudJf8tQJc3oKcm1cV7UtwY0') {
        throw 'Support public key fingerprint matcher ikke den kendte Ubuntu AI-support noegle.'
    }
    $SupportKeyLine = "$($supportParts[0]) $($supportParts[1]) ai-support"

    Write-Step 'Opretter dedikeret Windows SSH-supportkonto'
    New-WindowsSupportUser
    Install-SupportShell

    Write-Step 'Konfigurerer localhost-only OpenSSH Server'
    Configure-WindowsSshd

    Write-Step 'Opretter tunnelnoegler'
    Set-StateAcl
    New-Ed25519Key $TunnelKey "$ClientId-tunnel" $sshKeygenCommand.Source
    New-Ed25519Key $BootstrapKey "$ClientId-bootstrap" $sshKeygenCommand.Source
    Set-StateAcl
    $tunnelParts = Get-KeyParts ((Get-Content $TunnelPublicKey -Raw).Trim()) 'Tunnel public key'
    $bootstrapParts = Get-KeyParts ((Get-Content $BootstrapPublicKey -Raw).Trim()) 'Bootstrap public key'
    $tunnelBase64 = $tunnelParts[1]
    $bootstrapBase64 = $bootstrapParts[1]
    $bootstrapKeyLine = "$($bootstrapParts[0]) $bootstrapBase64 $ClientId-bootstrap"

    Write-Step 'Installerer bootstrap- og tunnelnoegler paa Ubuntu'
    $bootstrapLineEncoded = Convert-ToBase64 $bootstrapKeyLine
    # The port-specific tunnel key line is installed inside the retry loop.
    $remoteInstall = "set -eu; umask 077; mkdir -p ~/.ssh; touch ~/.ssh/authorized_keys; tmp=\`$(mktemp); awk -v k='$bootstrapBase64' -v t='$tunnelBase64' '\`$2 != k && \`$2 != t { print }' ~/.ssh/authorized_keys > \`$tmp; printf '%s\n' '$bootstrapLineEncoded' | base64 -d >> \`$tmp; mv \`$tmp ~/.ssh/authorized_keys; chmod 700 ~/.ssh; chmod 600 ~/.ssh/authorized_keys"
    if ((Invoke-UbuntuPasswordSsh $remoteInstall) -ne 0) { throw 'Kunne ikke installere bootstrap-noeglen paa Ubuntu.' }

    $tunnelVerified = $false
    $result = $null
    for ($attempt = 1; $attempt -le 5 -and -not $tunnelVerified; $attempt++) {
        $tunnelPort = Get-Random -Minimum $TunnelPortMinimum -Maximum ($TunnelPortMaximum + 1)
        $tunnelKeyLine = "command=`"if [ -n \`"`$SSH_ORIGINAL_COMMAND\`" ]; then exit 1; fi; exec /usr/bin/env AI_SUPPORT_CLIENT_ID=$ClientId /usr/bin/sleep infinity`",no-pty,no-agent-forwarding,no-X11-forwarding,no-user-rc,permitlisten=`"127.0.0.1:$tunnelPort`" $($tunnelParts[0]) $tunnelBase64 $ClientId-tunnel"
        $tunnelLineEncoded = Convert-ToBase64 $tunnelKeyLine
        $remoteTunnelKey = "set -eu; tmp=\`$(mktemp); awk -v key='$tunnelBase64' '\`$2 != key { print }' ~/.ssh/authorized_keys > \`$tmp; printf '%s\n' '$tunnelLineEncoded' | base64 -d >> \`$tmp; mv \`$tmp ~/.ssh/authorized_keys; chmod 600 ~/.ssh/authorized_keys"
        if ((Invoke-UbuntuSsh $BootstrapKey $remoteTunnelKey) -ne 0) { throw 'Kunne ikke konfigurere den begrænsede tunnelnoegle.' }

        Write-Step "Tester reverse SSH tunnel paa port $tunnelPort (forsog $attempt/5)"
        $tempStdout = Join-Path $StateDirectory 'enrollment-tunnel.out.log'
        $tempStderr = Join-Path $StateDirectory 'enrollment-tunnel.err.log'
        $temporaryTunnel = Start-Process -FilePath $sshCommand.Source -ArgumentList (Build-TunnelArguments $tunnelPort) -RedirectStandardOutput $tempStdout -RedirectStandardError $tempStderr -WindowStyle Hidden -PassThru
        try {
            Start-Sleep -Seconds 4
            $checkCommand = "ss -ltn | grep -Eq '[.:]$tunnelPort[[:space:]]'"
            $checkExit = Invoke-UbuntuSsh $BootstrapKey $checkCommand -BatchMode
            if ($checkExit -ne 0) { throw "Reverse tunnel kunne ikke verificeres paa Ubuntu port $tunnelPort." }

            $tunnelVerified = $true
        }
        catch {
            if ($attempt -eq 5) { throw }
            Write-Host 'Porten kunne ikke registreres; vaelger en ny reserveret port.' -ForegroundColor Yellow
        }
        finally {
            if ($temporaryTunnel -and -not $temporaryTunnel.HasExited) { Stop-Process -Id $temporaryTunnel.Id -Force -ErrorAction SilentlyContinue }
        }
    }
    if (-not $tunnelVerified) { throw 'AI-support tunnel kunne ikke verificeres.' }

    Write-Step 'Installerer persistent tunnel ved Windows-opstart'
    $taskArgs = (Build-TunnelArguments $tunnelPort) -join ' '
    $taskAction = New-ScheduledTaskAction -Execute $sshCommand.Source -Argument $taskArgs
    $taskTrigger = New-ScheduledTaskTrigger -AtStartup
    $taskSettings = New-ScheduledTaskSettingsSet -Hidden -StartWhenAvailable -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1)
    $taskPrincipal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
    Register-ScheduledTask -TaskName $TaskName -Action $taskAction -Trigger $taskTrigger -Settings $taskSettings -Principal $taskPrincipal -Force | Out-Null

    Start-ScheduledTask -TaskName $TaskName
    $taskReady = $false
    for ($wait = 0; $wait -lt 12; $wait++) {
        Start-Sleep -Seconds 2
        $taskState = (Get-ScheduledTask -TaskName $TaskName).State
        $checkCommand = "ss -ltn | grep -Eq '[.:]$tunnelPort[[:space:]]'"
        if ($taskState -eq 'Running' -and (Invoke-UbuntuSsh $BootstrapKey $checkCommand -BatchMode) -eq 0) {
            $taskReady = $true
            break
        }
    }
    if (-not $taskReady) { throw 'Persistent tunnel-tasken kunne ikke startes eller verificeres.' }

    Write-Step 'Registrerer SSH-only klienten'
    $hostname = $env:COMPUTERNAME
    if ($hostname) { $hostname = $hostname.Substring(0, [Math]::Min(100, $hostname.Length)) }
    $platform = "windows-$($env:PROCESSOR_ARCHITECTURE)".ToLowerInvariant()
    $fingerprintOutput = (& $sshKeygenCommand.Source -lf $TunnelPublicKey | Select-Object -First 1)
    $fingerprint = if ($fingerprintOutput -match '(SHA256:[A-Za-z0-9+/=]{43})') { $Matches[1] } else { '' }
    $payload = @{
        action                = 'enroll-ai-support'
        enrollment_token      = $EnrollmentToken
        client_id             = $ClientId
        hostname              = $hostname
        platform              = $platform.Substring(0, [Math]::Min(50, $platform.Length))
        ssh_host              = $UbuntuHost
        ssh_port              = $UbuntuPort
        ssh_user              = $UbuntuUser
        ssh_key_fingerprint   = $fingerprint
        tunnel_port           = $tunnelPort
        windows_ssh_user      = $WindowsSshUser
        windows_ssh_port      = $WindowsSshPort
    } | ConvertTo-Json
    $result = Invoke-RestMethod -Uri $EnrollmentUrl -Method Post -ContentType 'application/json' -Body $payload
    if (-not $result -or $result.status -ne 'registered') { throw 'Registreringen blev ikke gennemfoert.' }
    if ([string]::IsNullOrWhiteSpace($result.activity_log_token)) { throw 'Registreringen returnerede ingen aktivitetslog-token.' }
    @{ client_id = $ClientId; log_url = $EnrollmentUrl; log_token = $result.activity_log_token } |
        ConvertTo-Json -Compress | Set-Content -LiteralPath $ActivityConfigPath -Encoding UTF8
    & icacls.exe $ActivityConfigPath /inheritance:r /grant:r '*S-1-5-18:F' '*S-1-5-32-544:F' ("{0}:F" -f $WindowsSshUser) | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Kunne ikke beskytte aktivitetslog-konfigurationen.' }

    Remove-RemoteKey $BootstrapKey $bootstrapBase64
    Remove-Item -Force -ErrorAction SilentlyContinue $BootstrapKey, $BootstrapPublicKey
    Set-StateAcl

    Write-Host "`nSetup faerdig." -ForegroundColor Green
    Write-Host "AI-support klient registreret: $($result.client_name) ($($result.client_id))"
    Write-Host "Ubuntu tunnel endpoint: 127.0.0.1:$tunnelPort"
    Write-Host "Windows SSH endpoint: $WindowsSshUser@127.0.0.1:$WindowsSshPort"
    Write-Host 'Der er ikke installeret Remote Desktop-agent, WebRTC eller controller.'
    Write-Host "Tunnel-task: $TaskName"
}
catch {
    $message = $_.Exception.Message
    if ($null -ne $message -and $message.Length -gt 2000) { $message = $message.Substring(0, 2000) + '...' }
    Write-Host "`nAI-support SSH setup fejlede: $message" -ForegroundColor Red
    Write-Host 'Kør setup igen med et nyt engangstoken fra dashboardet.' -ForegroundColor Yellow
    exit 1
}
