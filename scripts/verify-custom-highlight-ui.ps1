param([switch]$SkipBuild, [string]$BuildDirectory = 'build/custom-highlight')
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
    $directory = Join-Path $root ('artifacts/custom-highlight-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
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
    Send-Line $first 'BEFORE INFO 性能 message {"message":"error","id":42}'
    Menu 'settings'
    function Add-Rule([string]$pattern,[string]$color,[string]$kind='keyword') {
        UI press key=settings-custom-add --settle | Out-Null
        $n=[regex]::Matches((Snapshot),'key=settings-custom-\d+-pattern').Count-1
        UI fill "key=settings-custom-$n-pattern" $pattern | Out-Null
        UI fill "key=settings-custom-$n-color" $color | Out-Null
        if($kind -eq 'regex'){ UI press "key=settings-custom-$n-regex" --settle | Out-Null }
    }
    Add-Rule '性能' '#AA00AA'
    Add-Rule 'message' '#D29922'
    Add-Rule '"message"' '#0099CC' 'regex'
    Add-Rule '"id":[0-9]+' '#123456' 'regex'
    UI screenshot (Join-Path $directory 'custom-editor.png') | Out-Null
    UI press key=settings-custom-3-delete --settle | Out-Null
    UI fill key=settings-custom-2-pattern '(?=x)' | Out-Null
    UI press key=settings-save --settle | Out-Null
    Wait-Text '自定义规则 3'
    UI fill key=settings-custom-2-pattern '"message"' | Out-Null
    UI press key=settings-save --settle | Out-Null
    Wait-Text '设置已保存'
    if ((Read-Config).preferences.log_highlight.custom.Count -ne 3) { throw 'Delete or save failed' }
    Send-Line $first 'SAVED INFO 性能 message {"message":"error","id":42}'
    UI screenshot (Join-Path $directory 'custom-before-reorder.png') | Out-Null
    Assert-Highlight $first $true
    Assert-Highlight $second $false
    Send-Line $second 'DISABLED INFO 性能 message {"message":"error","id":42}'
    UI screenshot (Join-Path $directory 'custom-disabled.png') | Out-Null
    Menu 'settings'
    UI act key=settings-custom-2-up scrollIntoView --settle | Out-Null
    UI focus key=settings-custom-2-up | Out-Null
    UI key Enter --settle | Out-Null
    UI press key=settings-save --settle | Out-Null
    Wait-Text '设置已保存'
    $rules=(Read-Config).preferences.log_highlight.custom
    if ($rules[1].kind -ne 'regex' -or $rules[1].pattern -ne '"message"' -or $rules[1].color -ne '#0099CC' -or $rules[2].pattern -ne 'message' -or $rules[2].color -ne '#D29922') { throw 'Reorder did not retain each rule' }
    Send-Line $first 'REORDERED INFO 性能 message {"message":"error","id":42}'
    UI screenshot (Join-Path $directory 'custom-after-reorder-old-history.png') | Out-Null
    Menu 'settings'
    UI fill key=settings-custom-1-color '#0088CC' | Out-Null
    UI press key=settings-save --settle | Out-Null
    Wait-Text '设置已保存'
    $rules=(Read-Config).preferences.log_highlight.custom
    if ($rules[1].color -ne '#0088CC' -or $rules[1].pattern -ne '"message"' -or $rules[2].color -ne '#D29922') { throw 'Editing reordered rule changed another entry' }
    Menu 'settings'
    UI fill key=settings-custom-0-pattern 'cancelled' | Out-Null
    UI press key=settings-custom-0-enabled | Out-Null
    UI press key=settings-cancel --settle | Out-Null
    if ((Read-Config).preferences.log_highlight.custom[0].pattern -ne '性能') { throw 'Cancel leaked draft' }
    $serverProcess.StandardInput.WriteLine('load on')
    Start-Sleep -Milliseconds 300
    $serverProcess.StandardInput.WriteLine('load off')
    Start-Sleep -Milliseconds 200
    UI screenshot (Join-Path $directory 'custom-continuous-ansi.png') | Out-Null
    Capture-Search $first '性能' 'custom-search-chinese'
    Capture-Search $first 'REORDERED' 'custom-history-search'
    UI click "key=terminal-$first" | Out-Null
    UI key Ctrl+Shift+A Ctrl+C | Out-Null
    UI press key=terminal-search --settle | Out-Null
    UI fill "key=terminal-query-$first" '' | Out-Null
    UI key Ctrl+V --settle | Out-Null
    $copied = Snapshot
    if (!$copied.Contains('BEFORE INFO 性能 message') -or !$copied.Contains('REORDERED INFO 性能 message') -or $copied.Contains('[38;2;')) { throw 'Copy lost original text' }
    UI press "key=search-close-$first" --settle | Out-Null
    $export=Join-Path $directory 'export.json'
    Menu 'export-config'; UI fill key=migration-path $export | Out-Null;UI press key=export-save --settle | Out-Null
    Wait-Text '配置已导出到'
    $exported=Get-Content -Raw $export | ConvertFrom-Json
    if ($exported.preferences.log_highlight.custom[1].pattern -ne '"message"') { throw 'Export lost order' }
    $exported.profiles=@();$exported.preferences.log_highlight.custom[0].color='#006600'
    $incoming=Join-Path $directory 'incoming.json';$exported | ConvertTo-Json -Depth 15 | Set-Content -Encoding utf8NoBOM $incoming
    Menu 'import-config'; UI fill key=migration-path $incoming | Out-Null;UI press key=import-preview --settle | Out-Null
    UI press key=import-preferences --settle | Out-Null;UI press key=import-confirm --settle | Out-Null
    Wait-Text '已导入 0 个连接'
    Send-Line $first 'IMPORTED INFO 性能 message {"message":"error"}'
    UI screenshot (Join-Path $directory 'custom-imported.png') | Out-Null
    Menu 'settings';UI fill key=settings-custom-0-color '#123456' | Out-Null
    $before=[IO.File]::ReadAllText($config);[IO.File]::WriteAllText($config,$before+"`n")
    UI press key=settings-save --settle | Out-Null;Wait-Text '配置文件已被外部修改'
    UI press key=settings-cancel --settle | Out-Null
    if ((Read-Config).preferences.log_highlight.custom[0].color -ne '#006600') { throw 'Failed save applied' }
    Stop-App;Start-App
    Menu 'settings'
    UI act key=settings-custom-0-pattern scrollIntoView --settle | Out-Null
    if (!(Snapshot).Contains('性能')) { throw 'Custom restart lost content' }
    UI press key=settings-cancel --settle | Out-Null
    Menu 'settings';UI fill key=settings-ui-size '20' | Out-Null;UI press key=settings-save --settle | Out-Null
    Wait-Text '设置已保存';Stop-App
    $layoutPath=Join-Path $dataDir 'workspace.json';$layout=Get-Content -Raw $layoutPath | ConvertFrom-Json
    $layout.window_width=900;$layout.window_height=620;$layout | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8NoBOM $layoutPath
    Start-App;Menu 'settings'
    UI act key=settings-custom-1-pattern scrollIntoView --settle | Out-Null
    UI screenshot (Join-Path $directory 'custom-settings-900x620-ui20.png') | Out-Null
    UI fill key=settings-custom-1-pattern 'discard' | Out-Null
    UI press key=settings-cancel --settle | Out-Null
    Stop-App
    @{result='passed';source='isolated local SSH fixture';sessions=2;custom_rules=3;narrow_window='900x620, UI size 20';checks='custom add/delete/edit/reorder, validation, save/cancel/conflict, independent switches, old history, Chinese search/copy, ANSI, migration, restart'} | ConvertTo-Json | Set-Content -Encoding utf8NoBOM (Join-Path $directory 'result.json')
    Write-Host "Custom highlight UI checks passed: $directory"
} catch {
    if ($appProcess -and !$appProcess.HasExited) { try { Snapshot | Set-Content -Encoding utf8NoBOM (Join-Path $directory 'failure.txt') } catch {} }
    throw
} finally {
    if ($appProcess) { if (!$appProcess.HasExited) { $appProcess.Kill($true);$appProcess.WaitForExit() };$appProcess.Dispose() }
    if ($serverProcess) { if (!$serverProcess.HasExited) { $serverProcess.StandardInput.Close();if (!$serverProcess.WaitForExit(3000)) {$serverProcess.Kill($true);$serverProcess.WaitForExit()} };$serverProcess.Dispose() }
    $env:MYGO_AUTOMATION_SESSION=$previousSession
    Pop-Location
}