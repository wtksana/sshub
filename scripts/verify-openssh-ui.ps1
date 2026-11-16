param(
    [Parameter(Mandatory)][string]$OpenSSHDirectory,
    [switch]$SkipBuild,
    [string]$BuildDirectory = 'build/p6',
    [string]$Vim = 'C:/Program Files/Git/usr/bin/vim.exe'
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/ui-helpers.ps1"
$OpenSSHDirectory = (Resolve-Path -LiteralPath $OpenSSHDirectory).Path
foreach ($binary in @('sshd.exe','ssh-keygen.exe','sftp-server.exe')) {
    if (!(Test-Path -LiteralPath (Join-Path $OpenSSHDirectory $binary))) { throw "Missing OpenSSH binary: $binary" }
}
if (!(Test-Path -LiteralPath $Vim)) { throw "Missing Vim: $Vim" }
Push-Location $root
$appProcess = $null
$serverProcess = $null
$serverLog = $null
$previousSession = $env:MYGO_AUTOMATION_SESSION
try {
    if (!$SkipBuild) { & "$PSScriptRoot/build.ps1" -DebugBuild -OutputDirectory $BuildDirectory }
    $directory = Join-Path $root ('artifacts/openssh-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $dataDir = Join-Path $directory 'data'
    $remoteDir = Join-Path $directory 'files'
    New-Item -ItemType Directory -Force $dataDir,$remoteDir | Out-Null
    $keygen = Join-Path $OpenSSHDirectory 'ssh-keygen.exe'
    foreach ($name in @('host_ed25519','user_ed25519')) {
        & $keygen -q -t ed25519 -N '' -f (Join-Path $directory $name)
        if ($LASTEXITCODE -ne 0) { throw 'Temporary key generation failed' }
    }
    $authorizedKeys = Join-Path $directory 'authorized_keys'
    Copy-Item -LiteralPath (Join-Path $directory 'user_ed25519.pub') -Destination $authorizedKeys
    # OpenSSH checks this generated file's Windows ACL even for a standalone server.
    $acl = [Security.AccessControl.FileSecurity]::new()
    $acl.SetAccessRuleProtection($true,$false)
    $acl.SetOwner([Security.Principal.WindowsIdentity]::GetCurrent().User)
    foreach ($sid in @([Security.Principal.WindowsIdentity]::GetCurrent().User.Value,'S-1-5-18','S-1-5-32-544')) {
        $acl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new([Security.Principal.SecurityIdentifier]::new($sid), 'FullControl', 'Allow'))
    }
    Set-Acl -LiteralPath $authorizedKeys -AclObject $acl
    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    $listener.Start(); $port = $listener.LocalEndpoint.Port; $listener.Stop()
    $unixDir = $directory.Replace('\','/')
    $config = @(
        "Port $port", 'ListenAddress 127.0.0.1', "HostKey `"$unixDir/host_ed25519`"",
        "PidFile `"$unixDir/sshd.pid`"", "AuthorizedKeysFile `"$unixDir/authorized_keys`"",
        'PasswordAuthentication no', 'KbdInteractiveAuthentication no', 'PubkeyAuthentication yes',
        ('AllowUsers ' + $env:USERNAME.ToLowerInvariant()),
        'StrictModes yes', 'AllowTcpForwarding no', 'PermitTunnel no', 'LogLevel VERBOSE',
        ('Subsystem sftp "' + (Join-Path $OpenSSHDirectory 'sftp-server.exe').Replace('\','/') + '" -d "' + $unixDir + '/files"')
    )
    $configPath = Join-Path $directory 'sshd_config'
    [IO.File]::WriteAllText($configPath, ($config -join "`n") + "`n", [Text.UTF8Encoding]::new($false))
    & (Join-Path $OpenSSHDirectory 'sshd.exe') -t -f $configPath
    if ($LASTEXITCODE -ne 0) { throw 'OpenSSH configuration check failed' }
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = Join-Path $OpenSSHDirectory 'sshd.exe'
    $start.UseShellExecute = $false; $start.CreateNoWindow = $true; $start.RedirectStandardError = $true
    foreach ($value in @('-D','-e','-f',$configPath)) { $start.ArgumentList.Add($value) }
    $serverProcess = [Diagnostics.Process]::Start($start)
    $serverLog = $serverProcess.StandardError.ReadToEndAsync()
    $fingerprint = [regex]::Match((& $keygen -l -f (Join-Path $directory 'host_ed25519.pub')), 'SHA256:[^ ]+').Value
    if (!$fingerprint) { throw 'Host fingerprint missing' }
    [IO.File]::WriteAllText((Join-Path $remoteDir 'OpenSSH 中文.txt'), 'real OpenSSH SFTP content', [Text.UTF8Encoding]::new($false))
    @{version=1;profiles=@(@{id='openssh';name='真实 OpenSSH';host='127.0.0.1';port=$port;username=$env:USERNAME;auth='private_key';identity_path=(Join-Path $directory 'user_ed25519');favorite=$false})} |
        ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $dataDir 'connections.json')
    $cli = Join-Path $root 'build/tools/mygo-automation.exe'
    if (!(Test-Path -LiteralPath $cli)) {
        go build -o $cli github.com/egoist/mygo/plugins/automation/cmd/mygo-automation
        if ($LASTEXITCODE -ne 0) { throw 'Automation build failed' }
    }
    $socket = Join-Path $directory 'app.sock'
    $env:MYGO_AUTOMATION_SESSION = 'sshub-openssh-' + [guid]::NewGuid().ToString('N')
    function UI { return Invoke-ProductUI @args }
    function Wait-Text([string]$value) { UI wait --text $value --timeout 15s --settle | Out-Null }
    function Search-Output([int]$tab, [string]$value, [switch]$KeepOpen) {
        UI press key=terminal-search --settle | Out-Null
        UI fill "key=terminal-query-$tab" $value --settle | Out-Null
        Wait-Text '1 处匹配'
        if (!$KeepOpen) { UI press "key=search-close-$tab" --settle | Out-Null }
    }
    function Send-Line([int]$tab, [string]$value) {
        UI click "key=terminal-$tab" --settle | Out-Null
        UI type $value --settle | Out-Null
        UI key Enter --settle | Out-Null
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
    UI click key=profile-openssh --clicks 2 --settle | Out-Null
    Wait-Text '确认服务器身份'
    if (!(Get-ProductSnapshot).Contains($fingerprint)) { throw 'Product host fingerprint did not match the generated OpenSSH host key' }
    UI press key=accept-connection-prompt --settle | Out-Null
    Wait-Text '真实 OpenSSH · 已连接'
    Wait-Text 'OpenSSH 中文.txt'
    $tab = [int]([regex]::Matches((Get-ProductSnapshot), 'key=tab-(\d+)') | ForEach-Object { [int]$_.Groups[1].Value } | Measure-Object -Maximum).Maximum
    UI wait "key=terminal-$tab" --timeout 15s --settle | Out-Null
    # Splitting the value across commands distinguishes server output from local command echo.
    Send-Line $tab 'set SSHUB_TEST_PREFIX=SSHUB_REAL_'
    Send-Line $tab 'echo %SSHUB_TEST_PREFIX%OPENSSH'
    Search-Output $tab 'SSHUB_REAL_OPENSSH'
    UI screenshot (Join-Path $directory 'openssh-shell.png') | Out-Null
    UI press key=files-menu --settle | Out-Null
    UI press 'key={files-mkdir}' --settle | Out-Null
    UI fill key=file-edit-value 'OpenSSH 新建目录' --settle | Out-Null
    UI press key=file-edit-confirm --settle | Out-Null
    Wait-Text 'OpenSSH 新建目录/'
    if (!(Test-Path -LiteralPath (Join-Path $remoteDir 'OpenSSH 新建目录'))) { throw 'SFTP mkdir did not reach the real OpenSSH directory' }
    $vimFile = Join-Path $remoteDir 'vim-结果.txt'
    Send-Line $tab ('"' + $Vim.Replace('\','/') + '" -u NONE -U NONE -i NONE -n "' + $vimFile.Replace('\','/') + '"')
    UI click "key=terminal-$tab" --settle | Out-Null
    UI key i --settle | Out-Null
    UI type 'SSHub_VIM_中文 edit' --settle | Out-Null
    Search-Output $tab 'SSHub_VIM_中文 edit' -KeepOpen
    UI screenshot (Join-Path $directory 'vim-editing.png') | Out-Null
    UI press "key=search-close-$tab" --settle | Out-Null
    UI click "key=terminal-$tab" --settle | Out-Null
    UI key Escape --settle | Out-Null
    UI type ':wq' --settle | Out-Null
    UI key Enter --settle | Out-Null
    $timer.Restart()
    while (!(Test-Path -LiteralPath $vimFile)) {
        if ($timer.Elapsed.TotalSeconds -gt 15) { throw 'Vim did not save its edited file' }
        Start-Sleep -Milliseconds 100
    }
    if ([IO.File]::ReadAllText($vimFile).Trim() -cne 'SSHub_VIM_中文 edit') { throw 'Vim file content mismatch' }
    Send-Line $tab 'echo %SSHUB_TEST_PREFIX%RETURNED'
    Search-Output $tab 'SSHUB_REAL_RETURNED'
    UI screenshot (Join-Path $directory 'vim-saved.png') | Out-Null
    @{passed=$true;serverVersion=((& (Join-Path $OpenSSHDirectory 'sshd.exe') -V 2>&1) -join '');serverExecutableSHA256=(Get-FileHash (Join-Path $OpenSSHDirectory 'sshd.exe')).Hash;hostFingerprintMatched=$true;shellEchoVerified=$true;sftpMkdir=$true;vimVersion=((& $Vim --version | Select-Object -First 1) -join '');vimExecutableSHA256=(Get-FileHash -LiteralPath $Vim).Hash;vimUnicodeFileSaved=$true;vimSavedFileSHA256=(Get-FileHash -LiteralPath $vimFile).Hash;shellRecoveredAfterVim=$true;systemServiceInstalled=$false} |
        ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $directory 'summary.json')
    UI press key=window-close --settle | Out-Null
    Wait-Text '退出 SSHub？'
    $ref = [regex]::Match((Get-ProductSnapshot), '(?m)^\s*(@e\d+) \[button\] "断开并退出"').Groups[1].Value
    if (!$ref) { throw 'Exit confirmation missing' }
    UI press $ref | Out-Null
    if (!$appProcess.WaitForExit(5000) -or $appProcess.ExitCode -ne 0) { throw 'App failed to exit cleanly' }
    Write-Output "PASS: real OpenSSH host verification, key authentication, SFTP mkdir and Vim editing. Artifacts: $directory"
} catch {
    if ($appProcess -and !$appProcess.HasExited) {
        try { Get-ProductSnapshot | Set-Content -Encoding utf8 (Join-Path $directory 'failure.txt'); UI screenshot (Join-Path $directory 'failure.png') | Out-Null } catch {}
    }
    throw
} finally {
    if ($appProcess) { if (!$appProcess.HasExited) { $appProcess.Kill(); $appProcess.WaitForExit() }; $appProcess.Dispose() }
    if ($serverProcess) {
        if (!$serverProcess.HasExited) { $serverProcess.Kill(); $serverProcess.WaitForExit() }
        if ($serverLog) { [IO.File]::WriteAllText((Join-Path $directory 'sshd.log'), $serverLog.GetAwaiter().GetResult(), [Text.UTF8Encoding]::new($false)) }
        $serverProcess.Dispose()
    }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
    Pop-Location
}
