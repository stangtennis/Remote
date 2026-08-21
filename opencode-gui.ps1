param(
    [string]$StartPath = '/home/dennis'
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

function Escape-RemotePath([string]$Path) {
    return $Path -replace "'", "'\\''"
}

function Get-RemoteDirectories([string]$Path) {
    $safePath = Escape-RemotePath $Path
    $remoteCommand = "find '$safePath' -mindepth 1 -maxdepth 1 -type d -printf '%f\t%p\n' | sort -f"
    $lines = @(& ssh.exe -o BatchMode=yes opencode-ubuntu $remoteCommand 2>$null)
    foreach ($line in $lines) {
        $parts = $line -split "`t", 2
        if ($parts.Count -eq 2) {
            [PSCustomObject]@{ Name = $parts[0]; Path = $parts[1] }
        }
    }
}

if (-not (Get-Command ssh.exe -ErrorAction SilentlyContinue)) {
    [System.Windows.Forms.MessageBox]::Show('ssh.exe blev ikke fundet.') | Out-Null
    exit 1
}

$remoteHome = ((& ssh.exe -o BatchMode=yes opencode-ubuntu 'printf %s "$HOME"' 2>$null) -join '').Trim()
if ([string]::IsNullOrWhiteSpace($remoteHome)) {
    [System.Windows.Forms.MessageBox]::Show('Kunne ikke finde Ubuntu-brugerens home-mappe.') | Out-Null
    exit 1
}
$remoteCli = "$remoteHome/.local/bin/remote-desktop-cli"
$remoteOpenCode = "$remoteHome/.local/bin/opencode"

$currentPath = $StartPath
$form = New-Object System.Windows.Forms.Form
$form.Text = 'Vaelg Ubuntu-mappe til OpenCode'
$form.Size = New-Object System.Drawing.Size(760, 560)
$form.StartPosition = 'CenterScreen'

$pathLabel = New-Object System.Windows.Forms.Label
$pathLabel.Location = New-Object System.Drawing.Point(12, 12)
$pathLabel.Size = New-Object System.Drawing.Size(720, 24)
$pathLabel.Text = $currentPath
$form.Controls.Add($pathLabel)

$list = New-Object System.Windows.Forms.ListBox
$list.Location = New-Object System.Drawing.Point(12, 45)
$list.Size = New-Object System.Drawing.Size(720, 390)
$list.DisplayMember = 'Name'
$form.Controls.Add($list)

$status = New-Object System.Windows.Forms.Label
$status.Location = New-Object System.Drawing.Point(12, 445)
$status.Size = New-Object System.Drawing.Size(720, 24)
$form.Controls.Add($status)

$upButton = New-Object System.Windows.Forms.Button
$upButton.Text = 'Op'
$upButton.Location = New-Object System.Drawing.Point(12, 480)
$upButton.Size = New-Object System.Drawing.Size(100, 32)
$form.Controls.Add($upButton)

$selectButton = New-Object System.Windows.Forms.Button
$selectButton.Text = 'Start OpenCode her'
$selectButton.Location = New-Object System.Drawing.Point(570, 480)
$selectButton.Size = New-Object System.Drawing.Size(162, 32)
$form.Controls.Add($selectButton)

$cancelButton = New-Object System.Windows.Forms.Button
$cancelButton.Text = 'Annuller'
$cancelButton.Location = New-Object System.Drawing.Point(460, 480)
$cancelButton.Size = New-Object System.Drawing.Size(100, 32)
$form.Controls.Add($cancelButton)

function Load-Directories {
    $pathLabel.Text = $script:currentPath
    $status.Text = 'Henter mapper fra Ubuntu...'
    $form.Refresh()
    try {
        $items = @(Get-RemoteDirectories $script:currentPath)
        $list.DataSource = $null
        $list.DataSource = $items
        $status.Text = "$($items.Count) mapper fundet"
    } catch {
        $status.Text = 'Kunne ikke hente mapper fra Ubuntu.'
        [System.Windows.Forms.MessageBox]::Show($_.Exception.Message, 'SSH-fejl') | Out-Null
    }
}

$list.Add_DoubleClick({
    if ($null -ne $list.SelectedItem) {
        $script:currentPath = $list.SelectedItem.Path
        Load-Directories
    }
})

$upButton.Add_Click({
    if ($script:currentPath -ne '/') {
        $trimmed = $script:currentPath.TrimEnd('/')
        $separator = $trimmed.LastIndexOf('/')
        $script:currentPath = if ($separator -le 0) { '/' } else { $trimmed.Substring(0, $separator) }
        Load-Directories
    }
})

$selectButton.Add_Click({
    $form.DialogResult = [System.Windows.Forms.DialogResult]::OK
    $form.Close()
})
$cancelButton.Add_Click({
    $form.DialogResult = [System.Windows.Forms.DialogResult]::Cancel
    $form.Close()
})

Load-Directories
if ($form.ShowDialog() -ne [System.Windows.Forms.DialogResult]::OK) { exit 0 }

$safeSelectedPath = Escape-RemotePath $currentPath
$watchLog = "$remoteHome/.local/state/remote-desktop/support-watch.log"
$watcher = "if [ -x '$remoteCli' ] && ! pgrep -f '[r]emote-desktop-cli support-watch' >/dev/null; then mkdir -p '$remoteHome/.local/state/remote-desktop'; nohup '$remoteCli' support-watch >> '$watchLog' 2>&1 </dev/null & fi"
$remoteCommand = "$watcher; cd '$safeSelectedPath' && exec '$remoteOpenCode'"
& ssh.exe -tt opencode-ubuntu $remoteCommand
exit $LASTEXITCODE
