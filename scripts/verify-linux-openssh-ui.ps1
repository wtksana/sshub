param(
    [Parameter(Mandatory)][ValidateRange(1,65535)][int]$Port,
    [Parameter(Mandatory)][string]$Identity,
    [Parameter(Mandatory)][string[]]$HostFingerprints,
    [string]$Username = 'alpine',
    [switch]$SkipBuild,
    [string]$BuildDirectory = 'build/p6'
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/ui-helpers.ps1"
$Identity = (Resolve-Path -LiteralPath $Identity).Path
Push-Location $root
$appProcess = $null
$previousSession = $env:MYGO_AUTOMATION_SESSION
try {
    if (!$SkipBuild) { & "$PSScriptRoot/build.ps1" -DebugBuild -OutputDirectory $BuildDirectory }
    $directory = Join-Path $root ('artifacts/linux-openssh-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $dataDir = Join-Path $directory 'data'
    New-Item -ItemType Directory -Force $dataDir | Out-Null
    @{version=1;profiles=@(@{id='linux';name='Linux OpenSSH';host='127.0.0.1';port=$Port;username=$Username;auth='private_key';identity_path=$Identity;favorite=$false})} |
        ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $dataDir 'connections.json')
    $cli = Join-Path $root 'build/tools/mygo-automation.exe'
    $socket = Join-Path $directory 'app.sock'
    $env:MYGO_AUTOMATION_SESSION = 'sshub-linux-' + [guid]::NewGuid().ToString('N')
    function UI { return Invoke-ProductUI @args }
    function Wait-Text([string]$value) { UI wait --text $value --timeout 20s --settle | Out-Null }
    function Send-Line([string]$value) {
        UI click "key=terminal-$tab" --settle | Out-Null
        UI type $value --settle | Out-Null
        UI key Enter --settle | Out-Null
    }
    function Search-Output([string]$value) {
        UI press key=terminal-search --settle | Out-Null
        UI fill "key=terminal-query-$tab" $value --settle | Out-Null
        Wait-Text '1 处匹配'
        UI press "key=search-close-$tab" --settle | Out-Null
    }
    function Entry-Menu([string]$name, [string]$key) {
        $ref = [regex]::Match((Get-ProductSnapshot), '(?m)^\s*(@e\d+) \[button\] "' + [regex]::Escape($name) + '".*key=remote-entry-').Groups[1].Value
        if (!$ref) { throw "Missing remote entry: $name" }
        UI click $ref --button right --settle | Out-Null
        UI press "key={$key}" --settle | Out-Null
    }
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = Join-Path $root "$BuildDirectory/windows-amd64/SSHub.exe"
    $start.UseShellExecute = $false; $start.CreateNoWindow = $true
    $start.Environment['MYGO_AUTOMATION'] = $socket
    $start.ArgumentList.Add('-data-dir'); $start.ArgumentList.Add($dataDir)
    $appProcess = [Diagnostics.Process]::Start($start)
    $timer = [Diagnostics.Stopwatch]::StartNew()
    while (!(Test-Path -LiteralPath $socket)) {
        if ($appProcess.HasExited -or $timer.Elapsed.TotalSeconds -gt 12) { throw 'App did not start' }
        Start-Sleep -Milliseconds 50
    }
    UI open --socket $socket | Out-Null
    UI click key=profile-linux --clicks 2 --settle | Out-Null
    Wait-Text '确认服务器身份'
    $snapshot = Get-ProductSnapshot
    $matched = @($HostFingerprints | Where-Object { $_ -match '^SHA256:[A-Za-z0-9+/]+$' -and $snapshot.Contains($_) })
    if ($matched.Count -ne 1) { throw 'Host fingerprint did not match the independent fixture record' }
    UI press key=accept-connection-prompt --settle | Out-Null
    Wait-Text 'Linux OpenSSH · 已连接'
    $tab = [int]([regex]::Matches((Get-ProductSnapshot), 'key=tab-(\d+)') | ForEach-Object { [int]$_.Groups[1].Value } | Measure-Object -Maximum).Maximum
    Send-Line 'proof=SSHUB_LINUX_; printf "%s%s\n" "$proof"PTY'
    Search-Output 'SSHUB_LINUX_PTY'
    $pathKey = [regex]::Match((Get-ProductSnapshot), 'key=(remote-path-\d+)').Groups[1].Value
    if (!$pathKey) { throw 'Remote path input missing' }
    UI fill "key=$pathKey" "/home/$Username/fixture" --settle | Out-Null
    UI key Enter --settle | Out-Null
    Wait-Text 'Linux 中文.txt'
    Entry-Menu 'Linux 中文.txt' file-properties
    Wait-Text '属性与权限'
    if (!(Get-ProductSnapshot).Contains('value="640"')) { throw 'Initial Unix mode 640 missing' }
    UI fill key=file-edit-value '600' --settle | Out-Null
    UI press key=file-edit-confirm --settle | Out-Null
    Wait-Text 'Linux 中文.txt'
    Send-Line 'printf "SSHUB_MODE_%s\n" "$(stat -c %a "$HOME/fixture/Linux 中文.txt")"'
    Search-Output 'SSHUB_MODE_600'
    Entry-Menu 'Linux 中文.txt' file-properties
    if (!(Get-ProductSnapshot).Contains('value="600"')) { throw 'Refreshed Unix mode 600 missing' }
    UI screenshot (Join-Path $directory 'unix-mode.png') | Out-Null
    UI press key=file-edit-cancel --settle | Out-Null
    UI press key=files-menu --settle | Out-Null
    UI press 'key={files-mkdir}' --settle | Out-Null
    UI fill key=file-edit-value 'Linux 新建目录' --settle | Out-Null
    UI press key=file-edit-confirm --settle | Out-Null
    Wait-Text 'Linux 新建目录/'
    Send-Line 'test -d "$HOME/fixture/Linux 新建目录" && printf "%s%s\n" "$proof"MKDIR'
    Search-Output 'SSHUB_LINUX_MKDIR'
    Send-Line 'top -d 1'
    Start-Sleep -Seconds 2
    UI screenshot (Join-Path $directory 'linux-top.png') | Out-Null
    UI click "key=terminal-$tab" --settle | Out-Null
    UI key q --settle | Out-Null
    Send-Line 'printf "%s%s\n" "$proof"RETURNED'
    Search-Output 'SSHUB_LINUX_RETURNED'
    UI screenshot (Join-Path $directory 'shell-returned.png') | Out-Null
    @{passed=$true;hostFingerprint=$matched[0];privateKeyAuthentication=$true;ptyShell=$true;sftpUnicodeDirectory=$true;unixModeBefore='640';unixModeAfter='600';unixModeCheckedViaStat=$true;sftpMkdirCheckedViaShell=$true;topScreenshot='linux-top.png';shellRecoveredAfterTop=$true} |
        ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $directory 'summary.json')
    UI press key=window-close --settle | Out-Null
    Wait-Text '退出 SSHub？'
    $ref = [regex]::Match((Get-ProductSnapshot), '(?m)^\s*(@e\d+) \[button\] "断开并退出"').Groups[1].Value
    if (!$ref) { throw 'Exit confirmation missing' }
    UI press $ref | Out-Null
    if (!$appProcess.WaitForExit(5000) -or $appProcess.ExitCode -ne 0) { throw 'App failed to exit cleanly' }
    Write-Output "PASS: Linux OpenSSH, Unix mode, SFTP mkdir and shell recovery after top. Visually review linux-top.png. Artifacts: $directory"
} catch {
    if ($appProcess -and !$appProcess.HasExited) {
        try { Get-ProductSnapshot | Set-Content -Encoding utf8 (Join-Path $directory 'failure.txt'); UI screenshot (Join-Path $directory 'failure.png') | Out-Null } catch {}
    }
    throw
} finally {
    if ($appProcess) { if (!$appProcess.HasExited) { $appProcess.Kill(); $appProcess.WaitForExit() }; $appProcess.Dispose() }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
    Pop-Location
}
