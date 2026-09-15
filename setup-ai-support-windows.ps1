<#
.SYNOPSIS
    Enroll a Windows PC for persistent SSH-only AI support.

.DESCRIPTION
    Installs a localhost-only Windows OpenSSH server and a persistent reverse
    SSH tunnel to the trusted Ubuntu AI-support host through Cloudflare Access.
    No Remote Desktop agent, WebRTC component, controller, or inbound LAN
    firewall rule is installed.

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
    [string]$CloudflareAccessHostname = 'ssh.hawkeye123.dk',
    [string]$CloudflareServiceTokenId,
    [System.Security.SecureString]$CloudflareServiceTokenSecret,
    [int]$CloudflareLocalPort = 43000,
    [string]$UbuntuUser = 'dennis',
    [string]$ClientId = '',
    [int]$WindowsSshPort = 22,
    [switch]$ConfigureOnly
)

$ErrorActionPreference = 'Stop'
$TunnelPortMinimum = 42000
$TunnelPortMaximum = 42999
$WindowsSshUser = 'ai-support'
$TaskName = 'AI-Support-Persistent-Tunnel'
$SshdTaskName = 'AI-Support-OpenSSH'
$StateDirectory = Join-Path $env:ProgramData 'AI-Support'
$TunnelKey = Join-Path $StateDirectory 'id_ed25519_ai_support_tunnel'
$BootstrapKey = Join-Path $StateDirectory 'id_ed25519_ai_support_bootstrap'
$KnownHosts = Join-Path $StateDirectory 'known_hosts'
$SupportPublicKeyPath = Join-Path $StateDirectory 'support.pub'
$SupportShellPath = Join-Path $StateDirectory 'ai-support-shell.ps1'
$ActivityLogPath = Join-Path $StateDirectory 'activity.log'
$ActivityConfigPath = Join-Path $StateDirectory 'activity-config.json'
$CloudflaredPath = Join-Path $StateDirectory 'cloudflared.exe'
$CloudflaredTaskLogPath = Join-Path $StateDirectory 'cloudflared.log'
$CloudflaredErrorLogPath = Join-Path $StateDirectory 'cloudflared-error.log'
$CloudflareTokenIdPath = Join-Path $StateDirectory 'cloudflare-service-token-id'
$CloudflareTokenSecretPath = Join-Path $StateDirectory 'cloudflare-service-token-secret.dpapi'
$TunnelRunnerPath = Join-Path $StateDirectory 'run-persistent-tunnel.ps1'
$TunnelTaskLogPath = Join-Path $StateDirectory 'persistent-tunnel-ssh.log'
$SshdConfigBackup = Join-Path $StateDirectory 'sshd_config.backup'
$TokenPolicyBackup = Join-Path $StateDirectory 'token-policy.backup'
$TunnelPublicKey = "$TunnelKey.pub"
$BootstrapPublicKey = "$BootstrapKey.pub"
$PortableOpenSshMsiUrl = 'https://github.com/PowerShell/Win32-OpenSSH/releases/download/10.0.0.0p2-Preview/OpenSSH-Win64-v10.0.0.0.msi'
$PortableOpenSshMsiSha256 = 'ddec9c53864280759cf9f74791cefd387100e3946aa849a1c138a4ed1b96b7d9'
$PortableOpenSshMsiPath = Join-Path $env:TEMP 'AI-Support-OpenSSH-Win64.msi'
$CloudflaredUrl = 'https://github.com/cloudflare/cloudflared/releases/download/2026.9.1/cloudflared-windows-amd64.exe'
$CloudflaredSha256 = '2837888cc0f5d58f15b6dc478376de90b4d3ba5241c7947455d1e0a0df429712'
$CloudflareSshPort = 22
$cloudflareBridgeProcess = $null
$persistentTaskRegistered = $false
$bootstrapInstalled = $false
$bootstrapRemoved = $false

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
    foreach ($path in @($TunnelKey, $TunnelPublicKey, $BootstrapKey, $BootstrapPublicKey, $KnownHosts, $SupportPublicKeyPath, $SupportShellPath, $ActivityLogPath, $ActivityConfigPath, $CloudflaredPath, $CloudflaredTaskLogPath, $CloudflaredErrorLogPath, $CloudflareTokenIdPath, $CloudflareTokenSecretPath, $TunnelRunnerPath, $TunnelTaskLogPath, $SshdConfigBackup, $TokenPolicyBackup)) {
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

function Set-SystemPrivateKeyAcl([string]$Path) {
    $acl = Get-Acl -LiteralPath $Path
    $acl.SetAccessRuleProtection($true, $false)
    foreach ($rule in @($acl.Access)) {
        [void]$acl.RemoveAccessRule($rule)
    }
    $systemSid = New-Object System.Security.Principal.SecurityIdentifier('S-1-5-18')
    $administratorsSid = New-Object System.Security.Principal.SecurityIdentifier('S-1-5-32-544')
    $fullControl = [System.Security.AccessControl.FileSystemRights]::FullControl
    $allow = [System.Security.AccessControl.AccessControlType]::Allow
    $acl.AddAccessRule([System.Security.AccessControl.FileSystemAccessRule]::new($systemSid, $fullControl, $allow))
    $acl.AddAccessRule([System.Security.AccessControl.FileSystemAccessRule]::new($administratorsSid, $fullControl, $allow))
    $acl.SetOwner($systemSid)
    Set-Acl -LiteralPath $Path -AclObject $acl
}

function Resolve-OpenSshPath([string]$Name) {
    $inboxPath = Join-Path $env:WINDIR "System32\OpenSSH\$Name"
    if (Test-Path $inboxPath) { return $inboxPath }
    $command = Get-Command $Name -ErrorAction SilentlyContinue
    if ($command -and $command.Source -and (Test-Path $command.Source)) { return $command.Source }
    $programFilesPath = Join-Path ${env:ProgramFiles} "OpenSSH\$Name"
    if (Test-Path $programFilesPath) { return $programFilesPath }
    throw "OpenSSH $Name blev ikke fundet efter installationen. Genstart Windows hvis OpenSSH-capability kræver reboot, og kør setup igen."
}

function Install-Cloudflared {
    if (Test-Path -LiteralPath $CloudflaredPath) {
        $actualHash = (Get-FileHash -LiteralPath $CloudflaredPath -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actualHash -ne $CloudflaredSha256) {
            throw 'Den eksisterende cloudflared.exe havde ikke den forventede SHA-256 hash.'
        }
        Set-SystemPrivateKeyAcl $CloudflaredPath
        return
    }

    Write-Step 'Installerer pinned officiel cloudflared bridge'
    $downloadPath = Join-Path $StateDirectory (".cloudflared-{0}.download" -f [guid]::NewGuid().ToString('N'))
    try {
        Invoke-WebRequest -UseBasicParsing -Uri $CloudflaredUrl -OutFile $downloadPath
        $actualHash = (Get-FileHash -LiteralPath $downloadPath -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actualHash -ne $CloudflaredSha256) {
            throw 'cloudflared-pakken havde ikke den forventede SHA-256 hash.'
        }
        Move-Item -LiteralPath $downloadPath -Destination $CloudflaredPath -Force
        Set-SystemPrivateKeyAcl $CloudflaredPath
    } finally {
        Remove-Item -LiteralPath $downloadPath -Force -ErrorAction SilentlyContinue
    }
}

function Protect-CloudflareTokenSecret([System.Security.SecureString]$SecureSecret) {
    $secretPointer = [IntPtr]::Zero
    $secretChars = $null
    $secretBytes = $null
    $protectedBytes = $null
    try {
        $secretPointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($SecureSecret)
        $byteLength = [Runtime.InteropServices.Marshal]::ReadInt32($secretPointer, -4)
        if ($byteLength -le 0 -or ($byteLength % 2) -ne 0) { throw 'Cloudflare service-token secret mangler.' }
        $secretChars = New-Object char[] ($byteLength / 2)
        [Runtime.InteropServices.Marshal]::Copy($secretPointer, $secretChars, 0, $secretChars.Length)
        $secretBytes = [Text.Encoding]::Unicode.GetBytes($secretChars)
        $protectedBytes = [Security.Cryptography.ProtectedData]::Protect(
            $secretBytes, $null, [Security.Cryptography.DataProtectionScope]::LocalMachine)
        [IO.File]::WriteAllBytes($CloudflareTokenSecretPath, $protectedBytes)
        Set-SystemPrivateKeyAcl $CloudflareTokenSecretPath
    } finally {
        if ($secretChars) { [Array]::Clear($secretChars, 0, $secretChars.Length) }
        if ($secretBytes) { [Array]::Clear($secretBytes, 0, $secretBytes.Length) }
        if ($protectedBytes) { [Array]::Clear($protectedBytes, 0, $protectedBytes.Length) }
        if ($secretPointer -ne [IntPtr]::Zero) {
            [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($secretPointer)
        }
    }
}

function Convert-ProtectedCloudflareSecretToPlainText {
    $protectedBytes = [IO.File]::ReadAllBytes($CloudflareTokenSecretPath)
    $secretBytes = $null
    try {
        $secretBytes = [Security.Cryptography.ProtectedData]::Unprotect(
            $protectedBytes, $null, [Security.Cryptography.DataProtectionScope]::LocalMachine)
        return [Text.Encoding]::Unicode.GetString($secretBytes)
    } finally {
        if ($protectedBytes) { [Array]::Clear($protectedBytes, 0, $protectedBytes.Length) }
        if ($secretBytes) { [Array]::Clear($secretBytes, 0, $secretBytes.Length) }
    }
}

function Test-CloudflareLocalListener([int]$OwningProcessId = 0) {
    $listener = Get-NetTCPConnection -LocalPort $CloudflareLocalPort -State Listen -ErrorAction SilentlyContinue |
        Where-Object { $_.LocalAddress -eq '127.0.0.1' }
    if ($OwningProcessId -gt 0) {
        $listener = $listener | Where-Object { $_.OwningProcess -eq $OwningProcessId }
    }
    return [bool]$listener
}

function Stop-CloudflareBridge([System.Diagnostics.Process]$Process) {
    if ($Process -and -not $Process.HasExited) {
        Stop-Process -InputObject $Process -Force -ErrorAction SilentlyContinue
        try { $Process.WaitForExit(10000) } catch { }
    }
}

function Start-CloudflareBridge {
    if (Test-CloudflareLocalListener) {
        throw "Cloudflare bridge-port $CloudflareLocalPort er allerede i brug på loopback."
    }
    $secretText = Convert-ProtectedCloudflareSecretToPlainText
    try {
        $env:TUNNEL_SERVICE_TOKEN_ID = $CloudflareServiceTokenId
        $env:TUNNEL_SERVICE_TOKEN_SECRET = $secretText
        try {
            $process = Start-Process -FilePath $CloudflaredPath -ArgumentList @(
                'access', 'tcp', '--hostname', $CloudflareAccessHostname,
                '--url', "127.0.0.1:$CloudflareLocalPort"
            ) -RedirectStandardOutput $CloudflaredTaskLogPath -RedirectStandardError $CloudflaredErrorLogPath -WindowStyle Hidden -PassThru
        } finally {
            Remove-Item Env:TUNNEL_SERVICE_TOKEN_ID -ErrorAction SilentlyContinue
            Remove-Item Env:TUNNEL_SERVICE_TOKEN_SECRET -ErrorAction SilentlyContinue
        }
    } finally {
        $secretText = $null
    }
    for ($wait = 0; $wait -lt 30; $wait++) {
        if ($process.HasExited) { throw 'Cloudflare bridge kunne ikke starte.' }
        if (Test-CloudflareLocalListener $process.Id) { return $process }
        Start-Sleep -Seconds 1
    }
    Stop-CloudflareBridge $process
    throw "Cloudflare bridge lyttede ikke på 127.0.0.1:$CloudflareLocalPort."
}

function Wait-ForOpenSshPath([string]$Name) {
    $lastError = $null
    for ($wait = 0; $wait -lt 15; $wait++) {
        try { return Resolve-OpenSshPath $Name }
        catch { $lastError = $_.Exception.Message }
        Start-Sleep -Seconds 2
    }
    throw $lastError
}

function Install-PortableOpenSsh {
    if (Test-Path (Join-Path ${env:ProgramFiles} 'OpenSSH\sshd.exe')) { return }
    Write-Step 'Installerer officiel OpenSSH fallback uden reboot'
    Invoke-WebRequest -UseBasicParsing -Uri $PortableOpenSshMsiUrl -OutFile $PortableOpenSshMsiPath
    $actualHash = (Get-FileHash -LiteralPath $PortableOpenSshMsiPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualHash -ne $PortableOpenSshMsiSha256) {
        Remove-Item -LiteralPath $PortableOpenSshMsiPath -Force -ErrorAction SilentlyContinue
        throw 'OpenSSH fallback-pakken havde ikke den forventede SHA-256 hash.'
    }
    $msiexec = Join-Path $env:WINDIR 'System32\msiexec.exe'
    $install = Start-Process -FilePath $msiexec -ArgumentList @(
        '/i', $PortableOpenSshMsiPath, '/qn', '/norestart', 'REBOOT=ReallySuppress'
    ) -Wait -PassThru -WindowStyle Hidden
    Remove-Item -LiteralPath $PortableOpenSshMsiPath -Force -ErrorAction SilentlyContinue
    if ($install.ExitCode -notin @(0, 3010)) {
        throw "OpenSSH fallback MSI fejlede (exit code $($install.ExitCode))."
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
        '-p', "$CloudflareLocalPort"
    )
    if ($BatchMode) { $args += @('-o', 'BatchMode=yes') }
    $args += @("$UbuntuUser@127.0.0.1", $RemoteCommand)
    & $sshCommand.Source @args
    return $LASTEXITCODE
}

function Invoke-UbuntuPasswordSsh([string]$RemoteCommand) {
    $args = @(
        '-o', "UserKnownHostsFile=$KnownHosts",
        '-o', 'StrictHostKeyChecking=accept-new',
        '-o', 'PubkeyAuthentication=no',
        '-o', 'PreferredAuthentications=password,keyboard-interactive',
        '-p', "$CloudflareLocalPort",
        "$UbuntuUser@127.0.0.1",
        $RemoteCommand
    )
    & $sshCommand.Source @args
    return $LASTEXITCODE
}

function Remove-RemoteKey([string]$IdentityFile, [string]$KeyBase64) {
    $command = "set -eu; test -f ~/.ssh/authorized_keys || exit 0; tmp=`$(mktemp); awk -v target='$KeyBase64' '{ current = 0; for (i=1; i<NF; i++) if (`$i ~ /^(ssh-|ecdsa-)/) { current = `$(i+1); break } } current != target { print }' ~/.ssh/authorized_keys > `$tmp; mv `$tmp ~/.ssh/authorized_keys; chmod 600 ~/.ssh/authorized_keys"
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
    $sshdPath = Resolve-OpenSshPath 'sshd.exe'
    $effective = (& $sshdPath -T -f $SshdConfig 2>&1 |
        Where-Object { $_ -match '^(port|listenaddress|hostkey|authorizedkeysfile|forcecommand)\s' } |
        Out-String).Trim()
    if ($effective) { $details += ('sshd_effective_config=' + ($effective -replace '\s+', ' ')) }
    return ($details -join '; ')
}

function Start-SshdFallbackTask([string]$SshdConfig) {
    $sshdPath = Resolve-OpenSshPath 'sshd.exe'
    Stop-ScheduledTask -TaskName $SshdTaskName -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $SshdTaskName -Confirm:$false -ErrorAction SilentlyContinue
    $taskAction = New-ScheduledTaskAction -Execute $sshdPath -Argument "-D -f `"$SshdConfig`""
    $taskTrigger = New-ScheduledTaskTrigger -AtStartup
    $taskSettings = New-ScheduledTaskSettingsSet -Hidden -StartWhenAvailable -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1)
    $taskPrincipal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
    Register-ScheduledTask -TaskName $SshdTaskName -Action $taskAction -Trigger $taskTrigger -Settings $taskSettings -Principal $taskPrincipal -Force -ErrorAction Stop | Out-Null
    Start-ScheduledTask -TaskName $SshdTaskName -ErrorAction Stop
    for ($wait = 0; $wait -lt 10; $wait++) {
        Start-Sleep -Seconds 1
        $listener = Get-NetTCPConnection -LocalPort $WindowsSshPort -State Listen -ErrorAction SilentlyContinue |
            Where-Object { $_.LocalAddress -eq '127.0.0.1' }
        if ($listener) { return }
    }
    throw 'OpenSSH fallback-tasken kunne ikke starte localhost-listeneren.'
}

function Configure-WindowsSshd {
    $sshdConfig = Join-Path $env:ProgramData 'ssh\sshd_config'
    $sshdPath = Wait-ForOpenSshPath 'sshd.exe'
    if (-not (Test-Path $sshdConfig)) {
        $defaultConfig = Join-Path (Split-Path -Parent $sshdPath) 'sshd_config_default'
        New-Item -ItemType Directory -Path (Split-Path -Parent $sshdConfig) -Force | Out-Null
        if (Test-Path $defaultConfig) {
            Copy-Item -LiteralPath $defaultConfig -Destination $sshdConfig -Force
        } else {
            # Some newer Windows capability packages omit the template. Create
            # a safe base instead of starting sshd with an unrestricted default.
            $minimalConfig = @'
Port 22
ListenAddress 127.0.0.1
PubkeyAuthentication yes
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitEmptyPasswords no
AllowTcpForwarding no
X11Forwarding no
'@
            Set-Content -LiteralPath $sshdConfig -Value $minimalConfig -Encoding ascii
        }
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
    # The stock config can contain the English group name "administrators".
    # It is not resolvable on localized Windows installations and can crash
    # the service even though sshd -t accepts the configuration.
    $config = [regex]::Replace($config, '(?im)^[ \t]*Match[ \t]+Group[ \t]+administrators[ \t]*\r?\n(?:(?!^[ \t]*Match[ \t]).*(?:\r?\n|$))*', '')
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
    AuthorizedKeysFile C:/Users/$WindowsSshUser/.ssh/authorized_keys
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
    $hostKeygen = Wait-ForOpenSshPath 'ssh-keygen.exe'
    & $hostKeygen -A | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Windows OpenSSH hostkeys kunne ikke oprettes.' }
    $sshLogsDirectory = Join-Path $env:ProgramData 'ssh\logs'
    if (Test-Path $sshLogsDirectory) {
        $logsBackup = Join-Path $StateDirectory ("openssh-logs-backup-{0}" -f (Get-Date -Format 'yyyyMMddHHmmss'))
        Move-Item -LiteralPath $sshLogsDirectory -Destination $logsBackup -Force -ErrorAction Stop
    }
    $hostKeys = Get-ChildItem -LiteralPath (Join-Path $env:ProgramData 'ssh') -Filter 'ssh_host_*_key' -File -ErrorAction SilentlyContinue
    if (-not $hostKeys) { throw 'Windows OpenSSH hostkeys blev ikke oprettet.' }
    $sshDirectory = Join-Path $env:ProgramData 'ssh'
    & icacls.exe $sshDirectory /reset /T /C | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Kunne ikke nulstille OpenSSH-mappens ACL.' }
    & icacls.exe $sshDirectory /remove 'NT SERVICE\sshd' | Out-Null
    & icacls.exe $sshDirectory /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)(F)' '*S-1-5-32-544:(OI)(CI)(F)' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Kunne ikke beskytte OpenSSH-mappen.' }
    $sshFiles = @($sshdConfig) + @($hostKeys | ForEach-Object { $_.FullName })
    foreach ($sshFile in $sshFiles) {
        & icacls.exe $sshFile /reset | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Kunne ikke nulstille OpenSSH-filen $sshFile." }
        & icacls.exe $sshFile /remove 'NT SERVICE\sshd' | Out-Null
        & icacls.exe $sshFile /inheritance:r /grant:r '*S-1-5-18:F' '*S-1-5-32-544:F' | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Kunne ikke beskytte OpenSSH-filen $sshFile." }
    }
    foreach ($hostKey in $hostKeys) {
        if (-not (Test-Path $hostKey.FullName)) { throw "OpenSSH hostkey forsvandt: $($hostKey.Name)." }
        & icacls.exe $hostKey.FullName /setowner '*S-1-5-18' | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Kunne ikke sætte SYSTEM som ejer af OpenSSH hostkey $($hostKey.Name)." }
    }
    $configTest = (& $sshdPath -t -f $sshdConfig 2>&1 | Out-String).Trim()
    if ($LASTEXITCODE -ne 0) {
        if ($configTest) { throw "OpenSSH Server konfigurationen er ugyldig: $configTest" }
        throw 'OpenSSH Server konfigurationen er ugyldig.'
    }
    Set-Service -Name sshd -StartupType Automatic
    Stop-ScheduledTask -TaskName $SshdTaskName -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $SshdTaskName -Confirm:$false -ErrorAction SilentlyContinue
    $serviceStarted = $false
    try {
        Restart-Service -Name sshd -Force -ErrorAction Stop
        $serviceStarted = $true
    } catch {
        Write-Host 'OpenSSH Windows-servicen kunne ikke starte; bruger SYSTEM fallback-task.' -ForegroundColor Yellow
    }
    if (-not $serviceStarted) {
        try {
            Start-SshdFallbackTask $sshdConfig
        } catch {
            $details = Get-SshdStartFailureDetails $sshdConfig
            if ($details) { throw "OpenSSH Server kunne ikke starte: $details" }
            throw "OpenSSH Server kunne ikke starte: $($_.Exception.Message)"
        }
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
        '-p', "$CloudflareLocalPort",
        '-R', "127.0.0.1:${Port}:127.0.0.1:${WindowsSshPort}",
        "$UbuntuUser@127.0.0.1"
    )
}

function Write-TunnelRunner([string]$SshPath, [int]$Port) {
    $runner = @"
`$ErrorActionPreference = 'Stop'
`$cloudflaredPath = '$CloudflaredPath'
`$cloudflaredSha256 = '$CloudflaredSha256'
`$cloudflaredLogPath = '$CloudflaredTaskLogPath'
`$cloudflaredErrorLogPath = '$CloudflaredErrorLogPath'
`$tokenIdPath = '$CloudflareTokenIdPath'
`$tokenSecretPath = '$CloudflareTokenSecretPath'
`$cloudflareHostname = '$CloudflareAccessHostname'
`$cloudflareLocalPort = $CloudflareLocalPort
`$sshPath = '$SshPath'
`$tunnelTaskLogPath = '$TunnelTaskLogPath'
`$actualHash = (Get-FileHash -LiteralPath `$cloudflaredPath -Algorithm SHA256).Hash.ToLowerInvariant()
if (`$actualHash -ne `$cloudflaredSha256) { throw 'cloudflared.exe hash matcher ikke den pinned version.' }
`$protectedBytes = [IO.File]::ReadAllBytes(`$tokenSecretPath)
`$secretBytes = `$null
`$secretText = `$null
`$bridge = `$null
Add-Type -AssemblyName System.Security
function Test-BridgeListener([int]`$OwningProcessId = 0) {
    `$listener = Get-NetTCPConnection -LocalPort `$cloudflareLocalPort -State Listen -ErrorAction SilentlyContinue |
        Where-Object { `$_.LocalAddress -eq '127.0.0.1' }
    if (`$OwningProcessId -gt 0) {
        `$listener = `$listener | Where-Object { `$_.OwningProcess -eq `$OwningProcessId }
    }
    return [bool]`$listener
}
function Stop-Bridge([System.Diagnostics.Process]`$Process) {
    if (`$Process -and -not `$Process.HasExited) {
        Stop-Process -InputObject `$Process -Force -ErrorAction SilentlyContinue
        try { `$Process.WaitForExit(10000) } catch { }
    }
}
if (Test-BridgeListener) { throw "Cloudflare bridge-port `$cloudflareLocalPort er allerede i brug på loopback." }
try {
    `$secretBytes = [Security.Cryptography.ProtectedData]::Unprotect(
        `$protectedBytes, `$null, [Security.Cryptography.DataProtectionScope]::LocalMachine)
    `$secretText = [Text.Encoding]::Unicode.GetString(`$secretBytes)
    `$serviceTokenId = (Get-Content -LiteralPath `$tokenIdPath -Raw).Trim()
    `$env:TUNNEL_SERVICE_TOKEN_ID = `$serviceTokenId
    `$env:TUNNEL_SERVICE_TOKEN_SECRET = `$secretText
    try {
        `$bridge = Start-Process -FilePath `$cloudflaredPath -ArgumentList @(
            'access', 'tcp', '--hostname', `$cloudflareHostname,
            '--url', "127.0.0.1:`$cloudflareLocalPort"
        ) -RedirectStandardOutput `$cloudflaredLogPath -RedirectStandardError `$cloudflaredErrorLogPath -WindowStyle Hidden -PassThru
    } finally {
        Remove-Item Env:TUNNEL_SERVICE_TOKEN_ID -ErrorAction SilentlyContinue
        Remove-Item Env:TUNNEL_SERVICE_TOKEN_SECRET -ErrorAction SilentlyContinue
    }
} finally {
    if (`$protectedBytes) { [Array]::Clear(`$protectedBytes, 0, `$protectedBytes.Length) }
    if (`$secretBytes) { [Array]::Clear(`$secretBytes, 0, `$secretBytes.Length) }
    `$secretText = `$null
}
for (`$wait = 0; `$wait -lt 30; `$wait++) {
    if (`$bridge.HasExited) { throw 'Cloudflare bridge kunne ikke starte.' }
    if (Test-BridgeListener `$bridge.Id) { break }
    Start-Sleep -Seconds 1
}
if (-not (Test-BridgeListener `$bridge.Id)) {
        Stop-Bridge `$bridge
        throw "Cloudflare bridge lyttede ikke på 127.0.0.1:`$cloudflareLocalPort."
}
try {
    'AI-support persistent tunnel runner started.' | Set-Content -LiteralPath `$tunnelTaskLogPath -Encoding ASCII
    `$sshArgs = @(
    '-N', '-T',
    '-o', 'BatchMode=yes',
    '-o', 'ExitOnForwardFailure=yes',
    '-o', 'ServerAliveInterval=30',
    '-o', 'ServerAliveCountMax=3',
    '-o', 'ConnectTimeout=15',
    '-o', 'UserKnownHostsFile=$KnownHosts',
    '-o', 'StrictHostKeyChecking=accept-new',
    '-o', 'IdentitiesOnly=yes',
    '-E', `$tunnelTaskLogPath,
    '-i', '$TunnelKey',
    '-p', '$CloudflareLocalPort',
    '-R', '127.0.0.1:${Port}:127.0.0.1:${WindowsSshPort}',
    '$UbuntuUser@127.0.0.1'
    )
    & `$sshPath @sshArgs
    `$exitCode = `$LASTEXITCODE
    "ssh exit code: `$exitCode" | Add-Content -LiteralPath '$TunnelTaskLogPath' -Encoding ASCII
} finally {
    Stop-Bridge `$bridge
}
# A clean SSH exit must still make Task Scheduler restart the runner.
exit 1
"@
    Set-Content -LiteralPath $TunnelRunnerPath -Value $runner -Encoding ASCII
}

if (-not (Test-Administrator)) { throw 'Dette setup skal koeres fra en Administrator-PowerShell.' }
if ($EnrollmentUrl -notmatch '^https://[A-Za-z0-9._:/?=&-]{1,200}$') { throw 'EnrollmentUrl skal vaere en gyldig https-URL.' }
if ($SupportPublicKeyUrl -notmatch '^https://[A-Za-z0-9._:/?=&-]{1,200}$') { throw 'SupportPublicKeyUrl skal vaere en gyldig https-URL.' }
if ([string]::IsNullOrWhiteSpace($EnrollmentToken) -or $EnrollmentToken.Length -gt 200) { throw 'EnrollmentToken mangler eller er ugyldigt.' }
if ($CloudflareAccessHostname -notmatch '^[A-Za-z0-9](?:[A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$') { throw 'CloudflareAccessHostname indeholder ugyldige tegn.' }
if ($CloudflareLocalPort -lt 1024 -or $CloudflareLocalPort -gt 65535) { throw 'CloudflareLocalPort skal vaere mellem 1024 og 65535.' }
if ($UbuntuUser -notmatch '^[A-Za-z0-9._-]{1,32}$') { throw 'UbuntuUser indeholder ugyldige tegn.' }
if ($WindowsSshPort -lt 1 -or $WindowsSshPort -gt 65535) { throw 'WindowsSshPort skal vaere mellem 1 og 65535.' }
if ($CloudflareLocalPort -eq $WindowsSshPort) { throw 'CloudflareLocalPort skal vaere forskellig fra WindowsSshPort.' }
$safeClientName = ($ClientName -replace '[^\p{L}\p{N}\s._-]', '').Trim()
if (-not $safeClientName -or $safeClientName.Length -gt 64) { throw 'ClientName skal vaere mellem 1 og 64 tegn.' }
if ($ClientId -notmatch '^(ai-[a-z0-9]{8,32})?$') { throw 'ClientId har et ugyldigt format.' }
if (-not $ClientId) {
    $bytes = New-RandomBytes 8
    $ClientId = 'ai-' + (($bytes | ForEach-Object { $_.ToString('x2') }) -join '')
}
if (-not $ConfigureOnly) {
    if ($CloudflareServiceTokenId -notmatch '^[A-Za-z0-9._:-]{1,200}$') { throw 'Cloudflare service-token client ID mangler eller er ugyldigt.' }
    if ($null -eq $CloudflareServiceTokenSecret -or $CloudflareServiceTokenSecret.Length -eq 0) { throw 'Cloudflare service-token secret mangler.' }
}
try {
    $sshCommand = Get-Command ssh.exe -ErrorAction SilentlyContinue
    $sshKeygenCommand = Get-Command ssh-keygen.exe -ErrorAction SilentlyContinue
    if (-not $sshCommand -or -not $sshKeygenCommand) {
        Write-Step 'Installerer Windows OpenSSH Client'
        try {
            Add-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0 | Out-Null
            $sshCommand = Get-Command ssh.exe -ErrorAction Stop
            $sshKeygenCommand = Get-Command ssh-keygen.exe -ErrorAction Stop
        } catch {
            Write-Host "Windows OpenSSH Client kunne ikke gøres klar endnu; bruger fallback-pakken hvis nødvendigt. ($($_.Exception.Message))" -ForegroundColor Yellow
        }
    }
    $serverCapability = Get-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
    $serverReady = $false
    try { $null = Resolve-OpenSshPath 'sshd.exe'; $serverReady = $true } catch { }
    if (-not $serverReady -and $serverCapability.State -ne 'Installed') {
        Write-Step 'Installerer Windows OpenSSH Server'
        try {
            $installResult = Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
            if ($installResult.RestartNeeded) {
                Write-Host 'Windows rapporterer RestartNeeded; forsøger portable OpenSSH fallback uden reboot.' -ForegroundColor Yellow
            }
        } catch {
            Write-Host "Windows Feature-on-Demand kunne ikke installeres færdigt; forsøger portable OpenSSH fallback. ($($_.Exception.Message))" -ForegroundColor Yellow
        }
        $serverCapability = Get-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
    }
    if (-not $serverReady) {
        try { $null = Wait-ForOpenSshPath 'sshd.exe'; $serverReady = $true } catch { }
    }
    if (-not $serverReady) {
        Write-Host "Windows OpenSSH Server er ikke klar (state=$($serverCapability.State)); installerer portable fallback." -ForegroundColor Yellow
        Install-PortableOpenSsh
    }
    if (-not $sshCommand -or -not (Test-Path $sshCommand.Source)) {
        $sshPath = Wait-ForOpenSshPath 'ssh.exe'
        $sshCommand = [pscustomobject]@{ Source = $sshPath }
    }
    if (-not $sshKeygenCommand -or -not (Test-Path $sshKeygenCommand.Source)) {
        $sshKeygenPath = Wait-ForOpenSshPath 'ssh-keygen.exe'
        $sshKeygenCommand = [pscustomobject]@{ Source = $sshKeygenPath }
    }

    New-Item -ItemType Directory -Path $StateDirectory -Force | Out-Null
    Set-StateAcl
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

    if ($ConfigureOnly) {
        Write-Host "`nSSH-konfigurationstest fuldført på $env:COMPUTERNAME." -ForegroundColor Green
        exit 0
    }

    Add-Type -AssemblyName System.Security
    Install-Cloudflared
    Set-Content -LiteralPath $CloudflareTokenIdPath -Value $CloudflareServiceTokenId -Encoding ASCII
    Set-SystemPrivateKeyAcl $CloudflareTokenIdPath
    Protect-CloudflareTokenSecret $CloudflareServiceTokenSecret
    Set-StateAcl
    Write-Step 'Starter Cloudflare Access TCP bridge'
    $cloudflareBridgeProcess = Start-CloudflareBridge

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
    $remoteInstall = "set -eu; umask 077; mkdir -p ~/.ssh; touch ~/.ssh/authorized_keys; tmp=`$(mktemp); awk -v k='$bootstrapBase64' -v t='$tunnelBase64' '{ current = 0; for (i=1; i<NF; i++) if (`$i ~ /^(ssh-|ecdsa-)/) { current = `$(i+1); break } } current != k && current != t { print }' ~/.ssh/authorized_keys > `$tmp; printf '%s\n' '$bootstrapLineEncoded' | base64 -d >> `$tmp; mv `$tmp ~/.ssh/authorized_keys; chmod 700 ~/.ssh; chmod 600 ~/.ssh/authorized_keys"
    if ((Invoke-UbuntuPasswordSsh $remoteInstall) -ne 0) { throw 'Kunne ikke installere bootstrap-noeglen paa Ubuntu.' }
    $bootstrapInstalled = $true

    $tunnelVerified = $false
    $result = $null
    for ($attempt = 1; $attempt -le 5 -and -not $tunnelVerified; $attempt++) {
        $tunnelPort = Get-Random -Minimum $TunnelPortMinimum -Maximum ($TunnelPortMaximum + 1)
        $tunnelKeyLine = "command=`"if [ -n \`"`$SSH_ORIGINAL_COMMAND\`" ]; then exit 1; fi; exec /usr/bin/env AI_SUPPORT_CLIENT_ID=$ClientId /usr/bin/sleep infinity`",no-pty,no-agent-forwarding,no-X11-forwarding,no-user-rc,permitlisten=`"127.0.0.1:$tunnelPort`" $($tunnelParts[0]) $tunnelBase64 $ClientId-tunnel"
        $tunnelLineEncoded = Convert-ToBase64 $tunnelKeyLine
        $remoteTunnelKey = "set -eu; tmp=`$(mktemp); awk -v target='$tunnelBase64' '{ current = 0; for (i=1; i<NF; i++) if (`$i ~ /^(ssh-|ecdsa-)/) { current = `$(i+1); break } } current != target { print }' ~/.ssh/authorized_keys > `$tmp; printf '%s\n' '$tunnelLineEncoded' | base64 -d >> `$tmp; mv `$tmp ~/.ssh/authorized_keys; chmod 600 ~/.ssh/authorized_keys"
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
            if ($temporaryTunnel -and -not $temporaryTunnel.HasExited) {
                Stop-Process -Id $temporaryTunnel.Id -Force -ErrorAction SilentlyContinue
                Wait-Process -Id $temporaryTunnel.Id -Timeout 10 -ErrorAction SilentlyContinue
            }
        }
    }
    if (-not $tunnelVerified) { throw 'AI-support tunnel kunne ikke verificeres.' }

    # Do not reuse the temporary reverse-forward until Ubuntu has released it.
    for ($wait = 0; $wait -lt 15; $wait++) {
        $checkCommand = "ss -ltn | grep -Eq '[.:]$tunnelPort[[:space:]]'"
        if ((Invoke-UbuntuSsh $BootstrapKey $checkCommand -BatchMode) -ne 0) { break }
        Start-Sleep -Seconds 1
    }

    Write-Step 'Installerer persistent tunnel ved Windows-opstart'
    Set-SystemPrivateKeyAcl $TunnelKey
    Write-TunnelRunner $sshCommand.Source $tunnelPort
    Set-StateAcl
    $powershellPath = Join-Path $env:WINDIR 'System32\WindowsPowerShell\v1.0\powershell.exe'
    $taskArgs = "-NoProfile -ExecutionPolicy Bypass -File `"$TunnelRunnerPath`""
    $taskAction = New-ScheduledTaskAction -Execute $powershellPath -Argument $taskArgs -WorkingDirectory $StateDirectory
    $taskTrigger = New-ScheduledTaskTrigger -AtStartup
    $taskSettings = New-ScheduledTaskSettingsSet -Hidden -StartWhenAvailable -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1)
    $taskPrincipal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
    Register-ScheduledTask -TaskName $TaskName -Action $taskAction -Trigger $taskTrigger -Settings $taskSettings -Principal $taskPrincipal -Force -ErrorAction Stop | Out-Null
    $persistentTaskRegistered = $true

    # The SYSTEM runner owns its own bridge, so release the enrollment bridge
    # before starting the task on the same fixed loopback port.
    Stop-CloudflareBridge $cloudflareBridgeProcess
    $cloudflareBridgeProcess = $null
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
    if (-not $taskReady) {
        $taskInfo = Get-ScheduledTaskInfo -TaskName $TaskName -ErrorAction SilentlyContinue
        $lastResult = if ($taskInfo) { $taskInfo.LastTaskResult } else { 'unknown' }
        $taskState = (Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue).State
        if (-not $taskState) { $taskState = 'unknown' }
        throw "Persistent tunnel-tasken kunne ikke startes eller verificeres (state=$taskState, result=$lastResult)."
    }

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
        ssh_host              = $CloudflareAccessHostname
        ssh_port              = $CloudflareSshPort
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
    $bootstrapRemoved = $true
    Remove-Item -Force -ErrorAction SilentlyContinue $BootstrapKey, $BootstrapPublicKey
    Set-StateAcl
    Stop-CloudflareBridge $cloudflareBridgeProcess
    $cloudflareBridgeProcess = $null

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
    if ($persistentTaskRegistered) {
        Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
    }
    if ($bootstrapInstalled -and -not $bootstrapRemoved -and (Test-Path $BootstrapKey)) {
        try {
            if (-not $cloudflareBridgeProcess) { $cloudflareBridgeProcess = Start-CloudflareBridge }
            Remove-RemoteKey $BootstrapKey $bootstrapBase64
            $bootstrapRemoved = $true
        } catch { }
    }
    Stop-CloudflareBridge $cloudflareBridgeProcess
    Remove-Item -Force -ErrorAction SilentlyContinue $BootstrapKey, $BootstrapPublicKey
    Stop-ScheduledTask -TaskName $SshdTaskName -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName $SshdTaskName -Confirm:$false -ErrorAction SilentlyContinue
    Write-Host "`nAI-support SSH setup fejlede: $message" -ForegroundColor Red
    Write-Host 'Kør setup igen med et nyt engangstoken fra dashboardet.' -ForegroundColor Yellow
    exit 1
}
