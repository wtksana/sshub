param([switch]$SkipBuild, [string]$BuildDirectory = 'build/log-highlighting-01')
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/ui-helpers.ps1"
Push-Location $root
$appProcess = $null
$serverProcess = $null
$previousSession = $env:MYGO_AUTOMATION_SESSION
try {
    if (!$SkipBuild) { & "$PSScriptRoot/build.ps1" -DebugBuild -OutputDirectory $BuildDirectory }
    $cli = Join-Path $root 'build/tools/mygo-automation.exe'
    go build -o $cli github.com/egoist/mygo/plugins/automation/cmd/mygo-automation
    if ($LASTEXITCODE -ne 0) { throw 'Automation build failed' }
    go build -tags e2e -o build/tools/ssh-fixture.exe ./internal/testssh/cmd
    if ($LASTEXITCODE -ne 0) { throw 'Fixture build failed' }
    $directory = Join-Path $root ('artifacts/log-highlighting-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $dataDir = Join-Path $directory 'data'
    New-Item -ItemType Directory -Force $dataDir | Out-Null
    $socket = Join-Path $directory 'app.sock'
    $env:MYGO_AUTOMATION_SESSION = 'sshub-highlight-' + [guid]::NewGuid().ToString('N')

    function UI { return Invoke-ProductUI @args }
    function Snapshot { return Get-ProductSnapshot }
    function Wait-Text([string]$text) { UI wait --text $text --timeout 10s --settle | Out-Null }
    function Start-App {
        $start = [Diagnostics.ProcessStartInfo]::new()
        $start.FileName = Join-Path $root "$BuildDirectory/windows-amd64/SSHub.exe"
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
        UI press key=window-close --settle | Out-Null
        if ((Snapshot).Contains('退出 SSHub？')) {
            $quit = [regex]::Match((Snapshot), '(?m)^\s*(@e\d+) \[button\] "断开并退出"').Groups[1].Value
            if (!$quit) { throw 'Exit confirmation missing' }
            UI press $quit | Out-Null
        }
        if (!$script:appProcess.WaitForExit(5000) -or $script:appProcess.ExitCode -ne 0) { throw 'App did not exit cleanly' }
        UI close | Out-Null
        $script:appProcess.Dispose(); $script:appProcess = $null
    }
    function Open-Session {
        UI press key=profile-fixture --settle | Out-Null
        UI press key=connect-profile --settle | Out-Null
        Wait-Text '高亮专项 · 已连接'
        $tabs = [regex]::Matches((Snapshot), 'key=tab-(\d+)(?= |$)')
        return [int]$tabs[$tabs.Count-1].Groups[1].Value
    }
    function Assert-Highlight([int]$tab, [bool]$enabled) {
        UI press "key=tab-$tab" --settle | Out-Null
        Open-PaneMenu
        $line = ((Snapshot) -split "`n" | Where-Object { $_ -match 'key="\{log-highlighting\}"' }) -join ''
        if (!$line -or $line.Contains(' checked') -ne $enabled) { throw "Unexpected highlighter state for tab ${tab}: $line" }
        UI key Escape --settle | Out-Null
    }
    function Toggle-Highlight([int]$tab) {
        UI click "key=terminal-$tab" --settle | Out-Null
        UI key Ctrl+Shift+H --settle | Out-Null
    }
    function Assert-Search([int]$tab, [string]$query) {
        UI press "key=tab-$tab" --settle | Out-Null
        UI press key=terminal-search --settle | Out-Null
        UI fill "key=terminal-query-$tab" $query --settle | Out-Null
        $snapshot = Snapshot
        if ($snapshot -notmatch '([1-9]\d*) 处匹配') { throw "Search did not preserve ${query}: $snapshot" }
        UI screenshot (Join-Path $directory ("search-$query.png")) | Out-Null
        UI press "key=search-close-$tab" --settle | Out-Null
    }

    $fixtureDir = Join-Path $directory 'server'
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = Join-Path $root 'build/tools/ssh-fixture.exe'
    $start.UseShellExecute = $false; $start.CreateNoWindow = $true; $start.RedirectStandardInput = $true
    $start.ArgumentList.Add('-dir'); $start.ArgumentList.Add($fixtureDir)
    $serverProcess = [Diagnostics.Process]::Start($start)
    $metadata = Join-Path $fixtureDir 'server.json'
    $timer = [Diagnostics.Stopwatch]::StartNew()
    while (!(Test-Path -LiteralPath $metadata)) {
        if ($serverProcess.HasExited -or $timer.Elapsed.TotalSeconds -gt 8) { throw 'Fixture did not start' }
        Start-Sleep -Milliseconds 50
    }
    $server = Get-Content -Raw $metadata | ConvertFrom-Json
    @{version=1;profiles=@(@{id='fixture';name='高亮专项';host='127.0.0.1';port=[int]($server.address.Split(':')[-1]);username='test';auth='private_key';identity_path=$server.identity})} |
        ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8NoBOM (Join-Path $dataDir 'connections.json')
    # Pin the exact host key created by this isolated local fixture.
    $server.known_host | Set-Content -Encoding utf8NoBOM (Join-Path $dataDir 'known_hosts')
    Start-App
    $first = Open-Session
    Assert-Highlight $first $false
    Toggle-Highlight $first
    Assert-Highlight $first $true
    $second = Open-Session
    Assert-Highlight $second $false
    Assert-Highlight $first $true
    Assert-Highlight $second $false
    UI press "key=tab-$first" --settle | Out-Null
    UI press key=disconnect-session --settle | Out-Null
    Wait-Text '高亮专项 · 已断开'
    UI press key=connect-pending --settle | Out-Null
    Wait-Text '高亮专项 · 已连接'
    Assert-Highlight $first $false
    Toggle-Highlight $first
    Assert-Highlight $first $true
    UI press key=split-horizontal --settle | Out-Null
    UI press "key=tab-$first" --settle | Out-Null
    $serverProcess.StandardInput.WriteLine('load on')
    Start-Sleep -Seconds 3
    $serverProcess.StandardInput.WriteLine('load off')
    Start-Sleep -Milliseconds 300
    UI screenshot (Join-Path $directory 'continuous-logs-wrapped.png') | Out-Null
    Assert-Search $first 'INFO'
    Assert-Search $first '性能'
    Assert-Highlight $first $true
    Assert-Highlight $second $false
    Stop-App
    Start-App
    if ((Snapshot).Contains('· 已连接')) { throw 'Restored tabs auto-connected' }
    foreach ($tab in @($first, $second)) {
        UI press "key=tab-$tab" --settle | Out-Null
        UI press key=connect-pending --settle | Out-Null
        Wait-Text '高亮专项 · 已连接'
        Assert-Highlight $tab $false
    }
    UI screenshot (Join-Path $directory 'restored-disabled.png') | Out-Null
    Stop-App
    Write-Host "Log highlighting UI lifecycle checks passed: $directory"
} catch {
    if ($appProcess -and !$appProcess.HasExited) { try { Snapshot | Set-Content -Encoding utf8NoBOM (Join-Path $directory 'failure.txt') } catch {} }
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
