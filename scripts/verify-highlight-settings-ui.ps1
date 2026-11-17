param([switch]$SkipBuild, [string]$BuildDirectory = 'build/highlight-settings')
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/ui-helpers.ps1"
Push-Location $root
$appProcess = $null
$serverProcess = $null
$previousSession = $env:MYGO_AUTOMATION_SESSION
try {
    if (!$SkipBuild) { & "$PSScriptRoot/build.ps1" -DebugBuild -OutputDirectory $BuildDirectory }
    New-Item -ItemType Directory -Force 'build/tools' | Out-Null
    $cli = Join-Path $root 'build/tools/mygo-automation.exe'
    go build -o $cli github.com/egoist/mygo/plugins/automation/cmd/mygo-automation
    if ($LASTEXITCODE -ne 0) { throw 'Automation build failed' }
    go build -tags e2e -o build/tools/ssh-fixture.exe ./internal/testssh/cmd
    if ($LASTEXITCODE -ne 0) { throw 'Fixture build failed' }
    $directory = Join-Path $root ('artifacts/highlight-settings-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $dataDir = Join-Path $directory 'data'
    New-Item -ItemType Directory -Force $dataDir | Out-Null
    $socket = Join-Path $directory 'app.sock'
    $config = Join-Path $dataDir 'connections.json'
    $env:MYGO_AUTOMATION_SESSION = 'sshub-highlight-settings-' + [guid]::NewGuid().ToString('N')

    function UI { return Invoke-ProductUI @args }
    function Snapshot { return Get-ProductSnapshot }
    function Wait-Text([string]$text) { UI wait --text $text --timeout 12s --settle | Out-Null }
    function Menu([string]$key) {
        Invoke-RawUI press key=app-menu --settle | Out-Null
        Invoke-RawUI press "key={$key}" --settle | Out-Null
    }
    function Read-Config { return Get-Content -Raw $config | ConvertFrom-Json }
    function Preset([string]$id) { return (Read-Config).preferences.log_highlight.builtins | Where-Object id -eq $id }
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
        if ((Snapshot).Contains('· 已连接')) {
            UI press key=window-close --settle | Out-Null
            Wait-Text '退出 SSHub？'
            $quit = [regex]::Match((Snapshot), '(?m)^\s*(@e\d+) \[button\] "断开并退出"').Groups[1].Value
            UI press $quit | Out-Null
        } else { $script:appProcess.CloseMainWindow() | Out-Null }
        if (!$script:appProcess.WaitForExit(5000) -or $script:appProcess.ExitCode -ne 0) { throw 'App did not exit cleanly' }
        UI close | Out-Null
        $script:appProcess.Dispose(); $script:appProcess = $null
    }
    function Open-Session {
        UI press key=profile-fixture --settle | Out-Null
        UI press key=connect-profile --settle | Out-Null
        Wait-Text '规则专项 · 已连接'
        $tabs = [regex]::Matches((Snapshot), 'key=tab-(\d+)(?= |$)')
        return [int]$tabs[$tabs.Count-1].Groups[1].Value
    }
    function Send-Line([int]$tab, [string]$text) {
        UI press "key=tab-$tab" --settle | Out-Null
        UI click "key=terminal-$tab" | Out-Null
        UI type "$text END" | Out-Null
        UI key Enter Ctrl+J --settle | Out-Null
    }
    function Assert-Highlight([int]$tab, [bool]$enabled) {
        UI press "key=tab-$tab" --settle | Out-Null
        Open-PaneMenu
        $line = ((Snapshot) -split "`n" | Where-Object { $_ -match 'key="\{log-highlighting\}"' }) -join ''
        if (!$line -or $line.Contains(' checked') -ne $enabled) { throw "Unexpected highlight state: $line" }
        UI key Escape --settle | Out-Null
    }
    function Capture-Search([int]$tab, [string]$query, [string]$name) {
        UI press "key=tab-$tab" --settle | Out-Null
        UI press key=terminal-search --settle | Out-Null
        UI fill "key=terminal-query-$tab" $query --settle | Out-Null
        if ((Snapshot) -notmatch '([1-9]\d*) 处匹配') { throw "Search did not preserve $query" }
        UI screenshot (Join-Path $directory "$name.png") | Out-Null
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
    @{version=1;profiles=@(@{id='fixture';name='规则专项';host='127.0.0.1';port=[int]($server.address.Split(':')[-1]);username='test';auth='private_key';identity_path=$server.identity})} |
        ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8NoBOM $config
    $server.known_host | Set-Content -Encoding utf8NoBOM (Join-Path $dataDir 'known_hosts')
    Start-App
    $first = Open-Session
    UI click "key=terminal-$first" | Out-Null
    UI key Ctrl+Shift+H --settle | Out-Null
    $second = Open-Session
    Assert-Highlight $first $true
    Assert-Highlight $second $false
    Send-Line $first 'BEFORE INFO ERROR 2026-11-17T12:34:56Z 中文 {"id":42}'
    UI screenshot (Join-Path $directory 'default-presets.png') | Out-Null

    Menu 'settings'
    UI fill key=settings-highlight-error-light '#AA00AA' | Out-Null
    UI press key=settings-highlight-info-enabled | Out-Null
    UI press key=settings-cancel --settle | Out-Null
    if ((Read-Config).preferences.log_highlight) { throw 'Canceled rules were persisted' }
    Send-Line $first 'CANCELLED INFO ERROR 2026/11/17 12:34:56'
    Menu 'settings'
    UI fill key=settings-highlight-error-light 'invalid' | Out-Null
    UI press key=settings-save --settle | Out-Null
    Wait-Text '日志高亮颜色必须为'
    UI fill key=settings-highlight-error-light '#AA00AA' | Out-Null
    UI fill key=settings-highlight-error-dark '#00FFFF' | Out-Null
    UI fill key=settings-highlight-date-light '#AA5500' | Out-Null
    UI fill key=settings-highlight-date-dark '#FFAA00' | Out-Null
    UI press key=settings-highlight-info-enabled | Out-Null
    # Verify the reused native color well can be opened by keyboard.
    UI act key=settings-highlight-error-light-picker scrollIntoView --settle | Out-Null
    UI focus key=settings-highlight-error-light-picker | Out-Null
    UI key Enter --settle | Out-Null
    if ((Snapshot) -notmatch 'key=settings-highlight-error-light-picker expanded') { throw 'Color well did not open by keyboard' }
    UI screenshot (Join-Path $directory 'color-picker.png') | Out-Null
    $red = [regex]::Match((Snapshot), '(?m)^\s*(@e\d+) \[button\] "Red"').Groups[1].Value
    UI press $red --settle | Out-Null
    UI key Escape --settle | Out-Null
    if ((Snapshot) -notmatch 'value="#EF4444" key=settings-highlight-error-light') { throw 'Color well did not update draft' }
    UI fill key=settings-highlight-error-light '#AA00AA' | Out-Null
    UI press key=settings-save --settle | Out-Null
    Wait-Text '设置已保存'
    if ((Preset 'error').light_color -ne '#AA00AA' -or (Preset 'info').enabled) { throw 'Saved presets differ' }
    Send-Line $first 'SAVED INFO ERROR 2026-11-17T12:34:56.123Z'
    UI screenshot (Join-Path $directory 'saved-and-old-history.png') | Out-Null
    Assert-Highlight $first $true
    Assert-Highlight $second $false
    Send-Line $second 'DISABLED INFO ERROR 2026-11-17T12:34:56Z'
    UI screenshot (Join-Path $directory 'disabled-session.png') | Out-Null
    Menu 'settings'
    UI press key=theme-dark | Out-Null
    UI press key=settings-save --settle | Out-Null
    Wait-Text '设置已保存'
    # First tab is hidden while the theme changes, then receives the new palette.
    Send-Line $first 'DARK INFO ERROR 2026-11-17T12:34:56Z'
    UI screenshot (Join-Path $directory 'dark-and-old-history.png') | Out-Null
    $serverProcess.StandardInput.WriteLine('load on')
    Start-Sleep -Milliseconds 200
    $serverProcess.StandardInput.WriteLine('load off')
    Start-Sleep -Milliseconds 200
    UI screenshot (Join-Path $directory 'continuous-ansi.png') | Out-Null
    Capture-Search $first 'BEFORE' 'history-before'
    Capture-Search $first '2026-11-17' 'date-search'
    # Copy only fixture output and paste into the terminal search box.
    UI click "key=terminal-$first" | Out-Null
    UI key Ctrl+Shift+A Ctrl+C | Out-Null
    UI press key=terminal-search --settle | Out-Null
    UI fill "key=terminal-query-$first" '' | Out-Null
    UI key Ctrl+V --settle | Out-Null
    $copied = Snapshot
    if (!$copied.Contains('BEFORE INFO ERROR') -or $copied.Contains('[38;2;')) { throw 'Copy did not preserve original fixture text' }
    UI press "key=search-close-$first" --settle | Out-Null

    $export = Join-Path $directory 'export.json'
    Menu 'export-config'
    UI fill key=migration-path $export | Out-Null
    UI press key=export-save --settle | Out-Null
    Wait-Text '配置已导出到'
    $exported = Get-Content -Raw $export | ConvertFrom-Json
    if (($exported.preferences.log_highlight.builtins | Where-Object id -eq 'error').dark_color -ne '#00FFFF') { throw 'Export lost palette' }
    $exported.profiles = @()
    ($exported.preferences.log_highlight.builtins | Where-Object id -eq 'info').enabled = $true
    $incoming = Join-Path $directory 'incoming.json'
    $exported | ConvertTo-Json -Depth 15 | Set-Content -Encoding utf8NoBOM $incoming
    Menu 'import-config'
    UI fill key=migration-path $incoming | Out-Null
    UI press key=import-preview --settle | Out-Null
    UI press key=import-preferences --settle | Out-Null
    UI press key=import-confirm --settle | Out-Null
    Wait-Text '已导入 0 个连接'
    Send-Line $first 'IMPORTED INFO ERROR 2026-11-17T12:34:56Z'
    UI screenshot (Join-Path $directory 'imported-palette.png') | Out-Null
    if (!(Preset 'info').enabled) { throw 'Import did not publish presets' }

    Menu 'settings'
    UI fill key=settings-highlight-error-light '#123456' | Out-Null
    $before = [IO.File]::ReadAllText($config)
    [IO.File]::WriteAllText($config, $before + "`n")
    UI press key=settings-save --settle | Out-Null
    Wait-Text '配置文件已被外部修改'
    UI press key=settings-cancel --settle | Out-Null
    if ((Preset 'error').light_color -ne '#AA00AA') { throw 'Failed save changed presets' }
    Stop-App
    Start-App
    Menu 'settings'
    UI act key=settings-highlight-error-light scrollIntoView --settle | Out-Null
    if (!(Snapshot).Contains('#AA00AA')) { throw 'Rules not restored' }
    UI press key=settings-cancel --settle | Out-Null
    Menu 'settings'
    UI fill key=settings-ui-size '20' | Out-Null
    UI press key=settings-save --settle | Out-Null
    Wait-Text '设置已保存'
    Stop-App
    $layoutPath = Join-Path $dataDir 'workspace.json'
    $layout = Get-Content -Raw $layoutPath | ConvertFrom-Json
    $layout.window_width = 900; $layout.window_height = 620
    $layout | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8NoBOM $layoutPath
    Start-App
    Menu 'settings'
    UI act key=settings-highlight-error-light scrollIntoView --settle | Out-Null
    UI screenshot (Join-Path $directory 'settings-900x620-ui20.png') | Out-Null
    UI fill key=settings-highlight-error-light '#654321' | Out-Null
    UI press key=settings-cancel --settle | Out-Null
    if ((Preset 'error').light_color -ne '#AA00AA') { throw 'Narrow dialog canceled changes persisted' }
    Stop-App
    @{result='passed';source='isolated local SSH fixture';sessions=2;rules=7;narrow_window='900x620, UI size 20';checks='presets, cancel, validation, save, theme, history, ANSI, search, copy, migration, external conflict, restart'} |
        ConvertTo-Json | Set-Content -Encoding utf8NoBOM (Join-Path $directory 'result.json')
    Write-Host "Highlight settings UI checks passed: $directory"
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
