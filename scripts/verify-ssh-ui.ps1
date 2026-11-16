param([switch]$SkipBuild)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
$appProcess = $null
$serverProcess = $null
$previousSession = $env:MYGO_AUTOMATION_SESSION
try {
    if (!$SkipBuild) { & "$PSScriptRoot/build.ps1" -DebugBuild }
    go build -o build/tools/mygo-automation.exe github.com/egoist/mygo/plugins/automation/cmd/mygo-automation
    if ($LASTEXITCODE -ne 0) { throw 'Automation build failed' }
    go build -tags e2e -o build/tools/ssh-fixture.exe ./internal/testssh/cmd
    if ($LASTEXITCODE -ne 0) { throw 'Fixture build failed' }
    $directory = Join-Path $root ('artifacts/ssh-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $dataDir = Join-Path $directory 'data'
    New-Item -ItemType Directory -Force $dataDir | Out-Null
    $cli = Join-Path $root 'build/tools/mygo-automation.exe'
    $socket = Join-Path $directory 'app.sock'
    $env:MYGO_AUTOMATION_SESSION = 'sshub-ssh-' + [guid]::NewGuid().ToString('N')

    function UI {
        $output = & $cli @args
        if ($LASTEXITCODE -ne 0) { throw "UI command failed: $($args[0])" }
        return $output
    }
    function Snapshot { return (UI snapshot --full) -join "`n" }
    function Wait-Text([string]$text) { UI wait --text $text --timeout 8s --settle | Out-Null }
    function Press-Label([string]$label) {
        $match = [regex]::Match((Snapshot), '(?m)^\s*(@e\d+) \[button\] "' + [regex]::Escape($label) + '"')
        if (!$match.Success) { throw "Button missing: $label" }
        UI press $match.Groups[1].Value --settle | Out-Null
    }
    function Start-Server {
        $fixtureDir = Join-Path $directory ('server-' + [guid]::NewGuid().ToString('N'))
        $start = [Diagnostics.ProcessStartInfo]::new()
        $start.FileName = Join-Path $root 'build/tools/ssh-fixture.exe'
        $start.UseShellExecute = $false
        $start.CreateNoWindow = $true
        $start.RedirectStandardInput = $true
        $start.ArgumentList.Add('-dir'); $start.ArgumentList.Add($fixtureDir)
        $script:serverProcess = [Diagnostics.Process]::Start($start)
        $timer = [Diagnostics.Stopwatch]::StartNew()
        $metadata = Join-Path $fixtureDir 'server.json'
        while (!(Test-Path -LiteralPath $metadata)) {
            if ($script:serverProcess.HasExited -or $timer.Elapsed.TotalSeconds -gt 8) { throw 'Fixture did not start' }
            Start-Sleep -Milliseconds 50
        }
        return Get-Content -Raw $metadata | ConvertFrom-Json
    }
    function Start-App {
        $start = [Diagnostics.ProcessStartInfo]::new()
        $start.FileName = Join-Path $root 'build/windows-amd64/SSHub.exe'
        $start.UseShellExecute = $false; $start.CreateNoWindow = $true
        $start.Environment['MYGO_AUTOMATION'] = $socket
        $start.ArgumentList.Add('-data-dir'); $start.ArgumentList.Add($dataDir)
        $script:appProcess = [Diagnostics.Process]::Start($start)
        $timer = [Diagnostics.Stopwatch]::StartNew()
        while (!(Test-Path -LiteralPath $socket)) {
            if ($script:appProcess.HasExited -or $timer.Elapsed.TotalSeconds -gt 12) { throw 'App did not start' }
            Start-Sleep -Milliseconds 100
        }
        UI open --socket $socket | Out-Null
        Wait-Text 'SSHub'
    }
    function Stop-App {
        UI close | Out-Null
        $script:appProcess.CloseMainWindow() | Out-Null
        if (!$script:appProcess.WaitForExit(5000)) { throw 'App did not close' }
        if ($script:appProcess.ExitCode -ne 0) { throw 'App crashed' }
        $script:appProcess.Dispose(); $script:appProcess = $null
    }
    function Open-Workspace {
        UI press key=profile-fixture --settle | Out-Null
        UI press key=open-workspace --settle | Out-Null
        UI press key=connect-pending --settle | Out-Null
    }

    $server = Start-Server
    $port = [int]($server.address.Split(':')[-1])
    @{version=1;profiles=@(
        @{id='fixture';name='本地安全 SSH';host='127.0.0.1';port=$port;username='test';auth='password';favorite=$false},
        @{id='encrypted';name='加密私钥 SSH';host='127.0.0.1';port=$port;username='test';auth='private_key';identity_path=$server.encrypted_identity;jump_id='fixture';favorite=$false}
    )} |
        ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $dataDir 'connections.json')
    Start-App
    Open-Workspace
    Wait-Text '确认服务器身份'
    UI screenshot (Join-Path $directory 'host-confirm.png') | Out-Null
    UI press key=cancel-connection-prompt --settle | Out-Null
    Wait-Text '已取消'
    if (Test-Path (Join-Path $dataDir 'known_hosts')) { throw 'Canceled trust persisted' }

    UI press key=connect-pending --settle | Out-Null
    Wait-Text '确认服务器身份'
    UI press key=accept-connection-prompt --settle | Out-Null
    Wait-Text '输入服务器密码'
    UI fill key=connection-secret 'wrong-password' --settle | Out-Null
    if ((Snapshot).Contains('wrong-password')) { throw 'Password leaked in UI snapshot' }
    UI press key=accept-connection-prompt --settle | Out-Null
    Wait-Text '连接失败'
    UI press key=connect-pending --settle | Out-Null
    Wait-Text '输入服务器密码'
    UI fill key=connection-secret 'fixture-password-中文' --settle | Out-Null
    UI press key=remember-secret --settle | Out-Null
    if ((Snapshot).Contains('fixture-password-中文')) { throw 'Password leaked in UI snapshot' }
    UI screenshot (Join-Path $directory 'masked-password.png') | Out-Null
    UI press key=accept-connection-prompt --settle | Out-Null
    Wait-Text '本地安全 SSH · 已连接'
    $term = [regex]::Match((Snapshot), '(?m)^\s*(@e\d+) \[group\] "Terminal"').Groups[1].Value
    if (!$term) { throw 'Real terminal view missing' }
    UI click $term | Out-Null
    UI type 'SSH_UI_ECHO_中文_123' --settle | Out-Null
    UI key Enter --settle | Out-Null
    UI screenshot (Join-Path $directory 'terminal.png') | Out-Null
    UI press key=toggle-files --settle | Out-Null
    UI screenshot (Join-Path $directory 'terminal-resized.png') | Out-Null
    UI press key=toggle-files --settle | Out-Null
    $closeRef = [regex]::Match((Snapshot), '(?m)^\s*(@e\d+) \[button\] "关闭工作区 本地安全 SSH"').Groups[1].Value
    if (!$closeRef) { throw 'Session close button missing' }
    UI press $closeRef --settle | Out-Null
    Wait-Text '关闭终端会话？'
    UI key Escape --settle | Out-Null
    Wait-Text '本地安全 SSH · 已连接'
    UI press key=disconnect-session --settle | Out-Null
    Wait-Text '本地安全 SSH · 已断开'
    UI screenshot (Join-Path $directory 'disconnected.png') | Out-Null
    Stop-App

    Start-App
    Open-Workspace
    Wait-Text '本地安全 SSH · 已连接'
    if ((Snapshot).Contains('connection-secret')) { throw 'Remembered credential prompted again' }
    $appProcess.CloseMainWindow() | Out-Null
    Wait-Text '退出 SSHub？'
    UI key Escape --settle | Out-Null
    Wait-Text '本地安全 SSH · 已连接'
    UI press key=forget-credentials --settle | Out-Null
    Wait-Text '已忘记'
    UI press key=disconnect-session --settle | Out-Null
    UI press key=connect-pending --settle | Out-Null
    Wait-Text '输入服务器密码'
    UI press key=cancel-connection-prompt --settle | Out-Null
    Wait-Text '已取消'
    UI press key=manage-hosts --settle | Out-Null
    Wait-Text 'SHA256:'
    Press-Label '移除信任'
    Wait-Text '移除主机信任？'
    Press-Label '移除信任记录'
    Wait-Text '已移除主机信任记录'
    UI press key=connect-pending --settle | Out-Null
    Wait-Text '确认服务器身份'
    UI press key=cancel-connection-prompt --settle | Out-Null
    Wait-Text '已取消'

    UI press key=profile-encrypted --settle | Out-Null
    UI press key=edit-profile --settle | Out-Null
    if ((Snapshot) -notmatch 'key=profile-jump') { throw 'Jump selection missing' }
    UI press key=cancel-profile --settle | Out-Null
    UI press key=open-workspace --settle | Out-Null
    UI press key=connect-pending --settle | Out-Null
    Wait-Text '确认服务器身份'
    UI press key=accept-connection-prompt --settle | Out-Null
    Wait-Text '输入服务器密码'
    UI fill key=connection-secret 'fixture-password-中文' | Out-Null
    UI press key=accept-connection-prompt --settle | Out-Null
    Wait-Text '输入私钥口令'
    UI fill key=connection-secret 'fixture-passphrase' --settle | Out-Null
    if ((Snapshot).Contains('fixture-passphrase')) { throw 'Passphrase leaked' }
    UI press key=accept-connection-prompt --settle | Out-Null
    Wait-Text '加密私钥 SSH · 已连接'
    UI screenshot (Join-Path $directory 'jump-encrypted.png') | Out-Null
    $appProcess.CloseMainWindow() | Out-Null
    Wait-Text '退出 SSHub？'
    $quitRef = [regex]::Match((Snapshot), '(?m)^\s*(@e\d+) \[button\] "断开并退出"').Groups[1].Value
    if (!$quitRef) { throw 'Exit confirmation missing' }
    UI press $quitRef | Out-Null
    if (!$appProcess.WaitForExit(5000) -or $appProcess.ExitCode -ne 0) { throw 'Confirmed exit did not cleanly finish' }
    UI close | Out-Null
    $appProcess.Dispose(); $appProcess = $null

    $config = Get-Content -Raw (Join-Path $dataDir 'connections.json')
    if ($config -match 'fixture-password|wrong-password|fixture-passphrase') { throw 'Secret persisted in configuration' }
    Write-Host "Secure SSH UI checks passed: $directory"
} catch {
    if ($appProcess -and !$appProcess.HasExited) { try { Snapshot | Set-Content -Encoding utf8 (Join-Path $directory 'failure.txt') } catch {} }
    throw
} finally {
    if ($appProcess) { if (!$appProcess.HasExited) { $appProcess.Kill($true); $appProcess.WaitForExit() }; $appProcess.Dispose() }
    if ($serverProcess) {
        if (!$serverProcess.HasExited) { $serverProcess.StandardInput.Close(); if (!$serverProcess.WaitForExit(3000)) { $serverProcess.Kill($true); $serverProcess.WaitForExit() } }
        $serverProcess.Dispose()
    }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
    Pop-Location
}
