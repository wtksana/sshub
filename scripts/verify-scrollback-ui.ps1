param([switch]$SkipBuild, [string]$BuildDirectory = 'build/scrollback')
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/ui-helpers.ps1"
Push-Location $root
$appProcess = $null
$serverProcess = $null
$proxy = $null
$previousSession = $env:MYGO_AUTOMATION_SESSION
try {
    if (!$SkipBuild) { & "$PSScriptRoot/build.ps1" -DebugBuild -OutputDirectory $BuildDirectory }
    $tools = Join-Path $root 'build/tools'
    New-Item -ItemType Directory -Force $tools | Out-Null
    $cli = Join-Path $tools 'mygo-automation.exe'
    go build -o $cli github.com/egoist/mygo/plugins/automation/cmd/mygo-automation
    if ($LASTEXITCODE -ne 0) { throw 'Automation build failed' }
    go build -tags e2e -o (Join-Path $tools 'ssh-fixture.exe') ./internal/testssh/cmd
    if ($LASTEXITCODE -ne 0) { throw 'Fixture build failed' }
    $directory = Join-Path $root ('artifacts/scrollback-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $dataDir = Join-Path $directory 'data'
    New-Item -ItemType Directory -Force $dataDir | Out-Null
    $socket = Join-Path $directory 'app.sock'
    $config = Join-Path $dataDir 'connections.json'
    $env:MYGO_AUTOMATION_SESSION = 'sshub-scrollback-' + [guid]::NewGuid().ToString('N')

    # Delay only the local fixture's handshake to verify an in-flight attempt's snapshot.
    Add-Type -TypeDefinition @'
using System;
using System.Net;
using System.Net.Sockets;
using System.Threading.Tasks;
public sealed class ScrollbackHandshakeProxy : IDisposable {
    readonly TcpListener listener = new TcpListener(IPAddress.Loopback, 0);
    readonly TaskCompletionSource<bool> gate = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
    TcpClient inbound, outbound;
    public int Port { get { return ((IPEndPoint)listener.LocalEndpoint).Port; } }
    public volatile bool Accepted;
    public ScrollbackHandshakeProxy(int targetPort) {
        listener.Start();
        Task.Run(async () => {
            try {
                inbound = await listener.AcceptTcpClientAsync();
                Accepted = true;
                await gate.Task;
                outbound = new TcpClient();
                await outbound.ConnectAsync(IPAddress.Loopback, targetPort);
                await Task.WhenAny(inbound.GetStream().CopyToAsync(outbound.GetStream()), outbound.GetStream().CopyToAsync(inbound.GetStream()));
            } catch (Exception) { } finally { inbound?.Dispose(); outbound?.Dispose(); }
        });
    }
    public void Release() { gate.TrySetResult(true); }
    public void Dispose() { listener.Stop(); gate.TrySetCanceled(); inbound?.Dispose(); outbound?.Dispose(); }
}
'@
    function UI { return Invoke-ProductUI @args }
    function Wait-Text([string]$value) { UI wait --text $value --timeout 12s --settle | Out-Null }
    function Menu([string]$key) {
        Invoke-RawUI press key=app-menu --settle | Out-Null
        Invoke-RawUI press "key={$key}" --settle | Out-Null
    }
    function Read-Config { Get-Content -Raw $config | ConvertFrom-Json }
    function Select-Capacity([int]$capacity) {
        Menu 'settings'
        UI press "key=settings-scrollback-$capacity" --settle | Out-Null
        UI press key=settings-save --settle | Out-Null
        Wait-Text '设置已保存'
        if ((Read-Config).preferences.scrollback_mib -ne $capacity) { throw 'Scrollback not persisted' }
    }
    function Open-Session([string]$id, [string]$name, [switch]$Pending) {
        UI press "key=profile-$id" --settle | Out-Null
        UI press key=connect-profile --settle | Out-Null
        if (!$Pending) { Wait-Text "$name · 已连接" }
        $all = [regex]::Matches((Get-ProductSnapshot), 'key=tab-(\d+)(?= |$)')
        return [int]$all[$all.Count-1].Groups[1].Value
    }
    function Assert-History([int]$tab, [int]$count) {
        UI press "key=tab-$tab" --settle | Out-Null
        UI press key=terminal-search --settle | Out-Null
        UI fill "key=terminal-query-$tab" 'SSHub local SSH fixture ready' --settle | Out-Null
        Wait-Text "$count 处匹配"
        UI press "key=search-close-$tab" --settle | Out-Null
    }
    function Start-App {
        $start = [Diagnostics.ProcessStartInfo]::new()
        $start.FileName = Join-Path $root "$BuildDirectory/windows-amd64/SSHub.exe"
        $start.UseShellExecute = $false
        $start.CreateNoWindow = $true
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
        $connected = (Get-ProductSnapshot).Contains('· 已连接')
        if ($connected) {
            UI press key=window-close --settle | Out-Null
            Wait-Text '退出 SSHub？'
            $quit = [regex]::Match((Get-ProductSnapshot), '(?m)^\s*(@e\d+) \[button\] "断开并退出"').Groups[1].Value
            UI press $quit | Out-Null
        } else {
            $script:appProcess.CloseMainWindow() | Out-Null
        }
        if (!$script:appProcess.WaitForExit(5000) -or $script:appProcess.ExitCode -ne 0) { throw 'App did not exit cleanly' }
        UI close | Out-Null
        $script:appProcess.Dispose(); $script:appProcess = $null
    }

    $fixtureDir = Join-Path $directory 'server'
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = Join-Path $tools 'ssh-fixture.exe'
    $start.UseShellExecute = $false
    $start.CreateNoWindow = $true
    $start.RedirectStandardInput = $true
    $start.ArgumentList.Add('-dir'); $start.ArgumentList.Add($fixtureDir)
    $serverProcess = [Diagnostics.Process]::Start($start)
    $timer = [Diagnostics.Stopwatch]::StartNew()
    while (!(Test-Path (Join-Path $fixtureDir 'server.json'))) {
        if ($serverProcess.HasExited -or $timer.Elapsed.TotalSeconds -gt 8) { throw 'Fixture did not start' }
        Start-Sleep -Milliseconds 50
    }
    $server = Get-Content -Raw (Join-Path $fixtureDir 'server.json') | ConvertFrom-Json
    $port = [int]($server.address.Split(':')[-1])
    $proxy = [ScrollbackHandshakeProxy]::new($port)
    $profiles = @(
        @{id='old';name='原有 2 MiB';host='127.0.0.1';port=$port;username='test';auth='private_key';identity_path=$server.identity},
        @{id='pending';name='连接中 2 MiB';host='127.0.0.1';port=$proxy.Port;username='test';auth='private_key';identity_path=$server.identity},
        @{id='new';name='新增 8 MiB';host='127.0.0.1';port=$port;username='test';auth='private_key';identity_path=$server.identity}
    )
    @{version=1;profiles=$profiles} | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 $config
    $known = $server.known_host + "`n[127.0.0.1]:$($proxy.Port) " + $server.known_host.Split(' ',2)[1] + "`n"
    [IO.File]::WriteAllText((Join-Path $dataDir 'known_hosts'), $known)
    Start-App
    Menu 'settings'
    if (!(Get-ProductSnapshot).Contains('仅新建或重新连接的终端生效')) { throw 'Application scope missing' }
    UI press key=settings-scrollback-64 | Out-Null
    UI press key=settings-cancel --settle | Out-Null
    if ((Read-Config).preferences) { throw 'Canceled draft was persisted' }
    $oldTab = Open-Session 'old' '原有 2 MiB'
    $pendingTab = Open-Session 'pending' '连接中 2 MiB' -Pending
    if (!$proxy.Accepted) { throw 'Handshake was not held' }
    Select-Capacity 8
    $proxy.Release()
    Wait-Text '连接中 2 MiB · 已连接'
    $newTab = Open-Session 'new' '新增 8 MiB'
    foreach ($tab in @($oldTab,$pendingTab,$newTab)) { Assert-History $tab 1 }
    $serverProcess.StandardInput.WriteLine('load on')
    Start-Sleep -Seconds 6
    $serverProcess.StandardInput.WriteLine('load off')
    Start-Sleep -Milliseconds 500
    Assert-History $oldTab 0
    Assert-History $pendingTab 0
    Assert-History $newTab 1
    UI screenshot (Join-Path $directory 'retained-8mib.png') | Out-Null
    UI press "key=tab-$oldTab" --settle | Out-Null
    UI press key=disconnect-session --settle | Out-Null
    UI press key=connect-pending --settle | Out-Null
    Wait-Text '原有 2 MiB · 已连接'
    $serverProcess.StandardInput.WriteLine('load on')
    Start-Sleep -Seconds 6
    $serverProcess.StandardInput.WriteLine('load off')
    Start-Sleep -Milliseconds 500
    Assert-History $oldTab 1
    UI screenshot (Join-Path $directory 'reconnected-8mib.png') | Out-Null
    Select-Capacity 32
    Select-Capacity 64
    Select-Capacity 8

    $export = Join-Path $directory 'export.json'
    Menu 'export-config'
    UI fill key=migration-path $export | Out-Null
    UI press key=export-save --settle | Out-Null
    Wait-Text '配置已导出到'
    if ((Get-Content -Raw $export | ConvertFrom-Json).preferences.scrollback_mib -ne 8) { throw 'Export lost capacity' }
    $incoming = Join-Path $directory 'incoming.json'
    $prefs = (Read-Config).preferences
    $prefs.scrollback_mib = 64
    @{format='sshub';version=1;profiles=@();preferences=$prefs} | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 $incoming
    Menu 'import-config'
    UI fill key=migration-path $incoming | Out-Null
    UI press key=import-preview --settle | Out-Null
    Wait-Text '预览：0 个连接'
    UI press key=import-preferences --settle | Out-Null
    UI press key=import-confirm --settle | Out-Null
    Wait-Text '已导入 0 个连接'
    if ((Read-Config).preferences.scrollback_mib -ne 64) { throw 'Import lost capacity' }
    Select-Capacity 8

    Menu 'settings'
    UI press key=settings-scrollback-64 | Out-Null
    $before = [IO.File]::ReadAllText($config)
    [IO.File]::WriteAllText($config, $before + "`n")
    UI press key=settings-save --settle | Out-Null
    Wait-Text '配置文件已被外部修改'
    UI press key=settings-cancel --settle | Out-Null
    if ((Read-Config).preferences.scrollback_mib -ne 8) { throw 'Failed save changed capacity' }
    Stop-App
    Start-App
    Menu 'settings'
    $snapshot = Get-ProductSnapshot
    if ($snapshot -notmatch '"8 MiB".*checked') { throw 'Scrollback selection not restored' }
    UI screenshot (Join-Path $directory 'settings-restored.png') | Out-Null
    UI fill key=settings-ui-size '20' | Out-Null
    UI press key=settings-save --settle | Out-Null
    Wait-Text '设置已保存'
    Stop-App
    $layoutPath = Join-Path $dataDir 'workspace.json'
    $layout = Get-Content -Raw $layoutPath | ConvertFrom-Json
    $layout.window_width = 900; $layout.window_height = 620
    $layout | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8 $layoutPath
    Start-App
    Menu 'settings'
    UI act key=settings-scrollback-64 scrollIntoView --settle | Out-Null
    UI screenshot (Join-Path $directory 'settings-900x620-ui20.png') | Out-Null
    UI press key=settings-scrollback-64 --settle | Out-Null
    UI press key=settings-cancel --settle | Out-Null
    if ((Read-Config).preferences.scrollback_mib -ne 8) { throw 'Narrow dialog cancel changed capacity' }
    Stop-App
    @{old_tab=$oldTab;pending_tab=$pendingTab;new_tab=$newTab;old_capacity_mib=2;pending_capacity_mib=2;new_capacity_mib=8;load_seconds=6;reconnect_capacity_mib=8;settings_migration_restart='passed';narrow_window='900x620, UI size 20';result='passed'} |
        ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $directory 'result.json')
    Write-Host "Scrollback UI checks passed: $directory"
} catch {
    if ($appProcess -and !$appProcess.HasExited) { try { Get-ProductSnapshot | Set-Content -Encoding utf8 (Join-Path $directory 'failure.txt') } catch {} }
    throw
} finally {
    if ($appProcess -and !$appProcess.HasExited) { $appProcess.Kill($true); $appProcess.WaitForExit() }
    if ($appProcess) { $appProcess.Dispose() }
    if ($proxy) { $proxy.Dispose() }
    if ($serverProcess -and !$serverProcess.HasExited) {
        $serverProcess.StandardInput.WriteLine('stop')
        if (!$serverProcess.WaitForExit(3000)) { $serverProcess.Kill($true); $serverProcess.WaitForExit() }
    }
    if ($serverProcess) { $serverProcess.Dispose() }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
    Pop-Location
}
