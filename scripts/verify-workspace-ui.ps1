param([switch]$SkipBuild)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/ui-helpers.ps1"
Push-Location $root
$appProcess = $null
$servers = [Collections.Generic.List[Diagnostics.Process]]::new()
$previousSession = $env:MYGO_AUTOMATION_SESSION
try {
    if (!$SkipBuild) { & "$PSScriptRoot/build.ps1" -DebugBuild }
    go build -o build/tools/mygo-automation.exe github.com/egoist/mygo/plugins/automation/cmd/mygo-automation
    if ($LASTEXITCODE -ne 0) { throw 'Automation build failed' }
    go build -tags e2e -o build/tools/ssh-fixture.exe ./internal/testssh/cmd
    if ($LASTEXITCODE -ne 0) { throw 'SSH fixture build failed' }
    $directory = Join-Path $root ('artifacts/workspace-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $dataDir = Join-Path $directory 'data'
    New-Item -ItemType Directory -Force $dataDir | Out-Null
    $cli = Join-Path $root 'build/tools/mygo-automation.exe'
    $socket = Join-Path $directory 'app.sock'
    $env:MYGO_AUTOMATION_SESSION = 'sshub-workspace-' + [guid]::NewGuid().ToString('N')
    function UI { return Invoke-ProductUI @args }
    function Snapshot { return (UI snapshot --full) -join "`n" }
    function Wait-Text([string]$text) { UI wait --text $text --timeout 8s --settle | Out-Null }
    function Assert-Text([string]$text) { if (!(Snapshot).Contains($text)) { throw "Expected text: $text" } }
    function Press-Label([string]$label, [switch]$Exits) {
        $ref = [regex]::Match((Snapshot), '(?m)^\s*(@e\d+) \[button\] "' + [regex]::Escape($label) + '"').Groups[1].Value
        if (!$ref) { throw "Missing button: $label" }
        if ($Exits) { UI press $ref | Out-Null } else { UI press $ref --settle | Out-Null }
    }
    function Start-Server([string]$name) {
        $fixtureDir = Join-Path $directory $name
        $start = [Diagnostics.ProcessStartInfo]::new()
        $start.FileName = Join-Path $root 'build/tools/ssh-fixture.exe'
        $start.UseShellExecute = $false; $start.CreateNoWindow = $true; $start.RedirectStandardInput = $true
        $start.ArgumentList.Add('-dir'); $start.ArgumentList.Add($fixtureDir)
        $server = [Diagnostics.Process]::Start($start); $servers.Add($server)
        $timer = [Diagnostics.Stopwatch]::StartNew()
        $path = Join-Path $fixtureDir 'server.json'
        while (!(Test-Path -LiteralPath $path)) {
            if ($server.HasExited -or $timer.Elapsed.TotalSeconds -gt 8) { throw 'SSH server did not start' }
            Start-Sleep -Milliseconds 50
        }
        return Get-Content -Raw $path | ConvertFrom-Json
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
    function Stop-App([switch]$Connected) {
        $script:appProcess.CloseMainWindow() | Out-Null
        if ($Connected) { Wait-Text '退出 SSHub？'; Press-Label '断开并退出' -Exits }
        if (!$script:appProcess.WaitForExit(5000) -or $script:appProcess.ExitCode -ne 0) { throw 'App did not exit cleanly' }
        UI close | Out-Null
        $script:appProcess.Dispose(); $script:appProcess = $null
    }
    $trusted = @{}
    $tabs = [Collections.Generic.List[object]]::new()
    function Open-Session([string]$profileID, [string]$name, [string]$token) {
        if ($tabs.Count -eq 0) {
            UI click "key=profile-$profileID" --clicks 2 --settle | Out-Null
        } else {
            UI press "key=profile-$profileID" --settle | Out-Null
            UI press key=open-workspace --settle | Out-Null
        }
        if ($tabs.Count -ne 0) { UI press key=connect-pending --settle | Out-Null }
        if (!$trusted.ContainsKey($profileID)) {
            Wait-Text '确认服务器身份'
            UI press key=accept-connection-prompt --settle | Out-Null
            $trusted[$profileID] = $true
        }
        Wait-Text '输入服务器密码'
        UI fill key=connection-secret 'fixture-password-中文' | Out-Null
        UI press key=accept-connection-prompt --settle | Out-Null
        Wait-Text "$name · 已连接"
        $id = ([regex]::Matches((Snapshot), 'key=tab-(\d+)') | ForEach-Object { [int]$_.Groups[1].Value } | Measure-Object -Maximum).Maximum
        UI wait "key=terminal-$id" --timeout 8s --settle | Out-Null
        UI click "key=terminal-$id" | Out-Null
        UI type $token --settle | Out-Null
        UI key Enter --settle | Out-Null
        $tabs.Add(@{id=[int]$id;name=$name;token=$token})
    }
    function Assert-FileServer([string]$name) {
        $pattern = '(?m)^.*\[text\] "' + [regex]::Escape($name) + ' · 已连接" key=file-server\b'
        if ((Snapshot) -notmatch $pattern) { throw "File panel did not follow: $name" }
    }

    $serverA = Start-Server 'server-a'; $serverB = Start-Server 'server-b'
    @{version=1;profiles=@(
        @{id='alpha';name='开发服务器';host='127.0.0.1';port=[int]($serverA.address.Split(':')[-1]);username='test';auth='password';favorite=$false},
        @{id='beta';name='测试服务器';host='127.0.0.1';port=[int]($serverB.address.Split(':')[-1]);username='test';auth='password';favorite=$false}
    )} | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $dataDir 'connections.json')
    Start-App
    Open-Session 'alpha' '开发服务器' 'SESSION_ALPHA_ONE'
    Open-Session 'alpha' '开发服务器' 'SESSION_ALPHA_TWO'
    UI press key=split-horizontal --settle | Out-Null
    Open-Session 'beta' '测试服务器' 'SESSION_BETA_ONE'
    Open-Session 'beta' '测试服务器' 'SESSION_BETA_TWO'
    UI press key=split-vertical --settle | Out-Null
    Open-Session 'alpha' '开发服务器' 'SESSION_ALPHA_THREE'
    Open-Session 'beta' '测试服务器' 'SESSION_BETA_THREE'
    UI press "key=tab-$($tabs[0].id)" --settle | Out-Null
    UI press key=split-vertical --settle | Out-Null
    Open-Session 'alpha' '开发服务器' 'SESSION_ALPHA_FOUR'
    Open-Session 'beta' '测试服务器' 'SESSION_BETA_FOUR'
    Assert-WorkspaceCount 4 8
    Open-PaneMenu
    if ((Snapshot) -notmatch 'key="{split-horizontal}" disabled') { throw 'Four-pane control limit missing' }
    Invoke-RawUI key Escape --settle | Out-Null
    foreach ($tab in $tabs) {
        UI press "key=tab-$($tab.id)" --settle | Out-Null
        UI press key=terminal-search --settle | Out-Null
        UI fill "key=terminal-query-$($tab.id)" $tab.token --settle | Out-Null
        Wait-Text '1 处匹配'
        UI press "key=search-close-$($tab.id)" --settle | Out-Null
    }
    # Right-click a background tab: search must belong to that tab, not the current pane.
    UI click "key=tab-$($tabs[0].id)" --button right --settle | Out-Null
    UI screenshot (Join-Path $directory 'tab-context-menu.png') | Out-Null
    Invoke-RawUI press 'key={terminal-search}' --settle | Out-Null
    UI fill "key=terminal-query-$($tabs[0].id)" $tabs[0].token --settle | Out-Null
    Wait-Text '1 处匹配'
    Assert-FileServer $tabs[0].name
    UI screenshot (Join-Path $directory 'tab-context-search.png') | Out-Null
    UI press "key=search-close-$($tabs[0].id)" --settle | Out-Null
    UI click "key=terminal-$($tabs[0].id)" --button right --settle | Out-Null
    if ((Snapshot) -notmatch '\[menuItem\] "(Paste|粘贴)"') { throw 'Terminal clipboard menu missing' }
    Invoke-RawUI key Escape --settle | Out-Null
    UI click "key=terminal-$($tabs[0].id)" --settle | Out-Null
    UI key Ctrl+Shift+F --settle | Out-Null
    UI wait "key=terminal-query-$($tabs[0].id)" --settle | Out-Null
    UI press "key=search-close-$($tabs[0].id)" --settle | Out-Null
    UI press "key=tab-$($tabs[1].id)" --settle | Out-Null
    if ((Snapshot) -match 'key=(connect-pending|split-horizontal|font-larger|edit-profile)') { throw 'Persistent workspace actions remain' }
    UI screenshot (Join-Path $directory 'four-panes-eight-sessions.png') | Out-Null

    # A pointer click in a terminal must select its pane without sending a command elsewhere.
    foreach ($index in @(1,3,5,7)) {
        $tab = $tabs[$index]
        UI click "key=terminal-$($tab.id)" --settle | Out-Null
        Assert-FileServer $tab.name
    }
    UI focus "key=terminal-$($tabs[1].id)" --settle | Out-Null
    Assert-FileServer $tabs[1].name
    UI focus "key=terminal-$($tabs[7].id)" --settle | Out-Null
    Assert-FileServer $tabs[7].name
    $searchTab = $tabs[7]
    UI press key=terminal-search --settle | Out-Null
    UI fill "key=terminal-query-$($searchTab.id)" $searchTab.token --settle | Out-Null
    Wait-Text '1 处匹配'
    UI press "key=search-next-$($searchTab.id)" --settle | Out-Null
    UI press "key=search-previous-$($searchTab.id)" --settle | Out-Null
    UI press key=font-larger --settle | Out-Null
    Assert-FontSize 14
    UI press key=font-reset --settle | Out-Null
    Assert-FontSize 13
    UI press key=font-larger --settle | Out-Null
    UI press key=font-smaller --settle | Out-Null
    Assert-FontSize 13
    UI press key=font-larger --settle | Out-Null
    UI screenshot (Join-Path $directory 'search-and-zoom.png') | Out-Null
    UI press "key=search-close-$($searchTab.id)" --settle | Out-Null
    UI press key=move-tab --settle | Out-Null
    UI wait "key=terminal-$($searchTab.id)" --settle | Out-Null
    Assert-FileServer $searchTab.name
    Assert-WorkspaceCount 4 8
    UI press key=merge-pane --settle | Out-Null
    Assert-WorkspaceCount 3 8

    # Same-profile sessions retain distinct IDs; closing one cannot close its sibling.
    UI press "key=tab-$($tabs[0].id)" --settle | Out-Null
    UI press "key=close-tab-$($tabs[0].id)" --settle | Out-Null
    Wait-Text '关闭终端会话？'
    Press-Label '关闭会话'
    Assert-WorkspaceCount 3 7
    UI press "key=tab-$($tabs[1].id)" --settle | Out-Null
    UI wait "key=terminal-$($tabs[1].id)" --settle | Out-Null
    Assert-FileServer '开发服务器'
    UI press key=terminal-search --settle | Out-Null
    UI fill "key=terminal-query-$($tabs[1].id)" 'SESSION_ALPHA_TWO' --settle | Out-Null
    Wait-Text '1 处匹配'
    UI press "key=search-close-$($tabs[1].id)" --settle | Out-Null
    $servers[0].StandardInput.WriteLine('disconnect')
    Wait-Text '开发服务器 · 已断开'
    UI press key=terminal-search --settle | Out-Null
    UI fill "key=terminal-query-$($tabs[1].id)" 'SESSION_ALPHA_TWO' --settle | Out-Null
    Wait-Text '1 处匹配'
    UI press "key=search-close-$($tabs[1].id)" --settle | Out-Null
    UI screenshot (Join-Path $directory 'network-disconnected-output.png') | Out-Null
    UI press key=connect-pending --settle | Out-Null
    Wait-Text '输入服务器密码'
    UI fill key=connection-secret 'fixture-password-中文' | Out-Null
    UI press key=accept-connection-prompt --settle | Out-Null
    UI wait "key=terminal-$($tabs[1].id)" --timeout 8s --settle | Out-Null
    UI press key=terminal-search --settle | Out-Null
    Wait-Text '0 处匹配'
    UI fill "key=terminal-query-$($tabs[1].id)" 'local SSH fixture ready' --settle | Out-Null
    Wait-Text '1 处匹配'
    UI press "key=search-close-$($tabs[1].id)" --settle | Out-Null
    UI screenshot (Join-Path $directory 'after-close-and-merge.png') | Out-Null
    $splitters = [regex]::Matches((Snapshot), '(?m)^\s*(@e\d+) \[splitter\].*bounds=([\d.]+),([\d.]+),([\d.]+)x([\d.]+)')
    $divider = $splitters | Where-Object { [double]$_.Groups[2].Value -gt 300 -and [double]$_.Groups[4].Value -lt 3 -and [double]$_.Groups[5].Value -gt 150 } | Select-Object -First 1
    if (!$divider) { throw 'Pane divider missing' }
    UI focus $divider.Groups[1].Value | Out-Null
    UI key Right Right Right --settle | Out-Null
    UI press key=toggle-files --settle | Out-Null
    Stop-App -Connected

    $saved = Get-Content -Raw (Join-Path $dataDir 'workspace.json')
    if ($saved -match 'fixture-password|SESSION_|identity_path|已连接') { throw 'Layout persisted non-restorable data' }
    $layout = $saved | ConvertFrom-Json
    if ($layout.root.ratio -eq 50) { throw 'Pane divider position was not saved' }
    if ($layout.window_width -lt 900 -or $layout.window_height -lt 620) { throw 'Window dimensions were not saved' }
    Start-App
    Assert-WorkspaceCount 3 7
    if ((Snapshot) -match '\[group\] "Terminal"|· 已连接|connection-secret|确认服务器身份') { throw 'Restart restored network sessions or output' }
    if ((Snapshot).Contains('key=file-server')) { throw 'Hidden file panel preference not restored' }
    UI press key=toggle-files --settle | Out-Null
    Wait-Text '待连接'
    UI press "key=tab-$($searchTab.id)" --settle | Out-Null
    Assert-FontSize 14
    UI screenshot (Join-Path $directory 'restored-pending.png') | Out-Null
    Stop-App

    $smallLayout = Get-Content -Raw (Join-Path $dataDir 'workspace.json') | ConvertFrom-Json
    $smallLayout.window_width = 900; $smallLayout.window_height = 620
    $smallLayout.hide_left = $true; $smallLayout.hide_right = $true
    $smallLayout | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8 (Join-Path $dataDir 'workspace.json')
    Start-App
    Assert-WorkspaceCount 3 7
    UI press key=merge-pane --settle | Out-Null
    Assert-WorkspaceCount 2 7
    UI screenshot (Join-Path $directory 'minimum-window.png') | Out-Null
    Stop-App

    # External edits are reported without overwriting the external file.
    Start-App
    $external = '{external-workspace-change'
    [IO.File]::WriteAllText((Join-Path $dataDir 'workspace.json'), $external, [Text.UTF8Encoding]::new($false))
    UI press key=toggle-files --settle | Out-Null
    Wait-Text '保存工作区布局失败'
    Stop-App
    if ([IO.File]::ReadAllText((Join-Path $dataDir 'workspace.json')) -ne $external) { throw 'External layout overwritten' }

    # A damaged layout is recoverable and remains byte-for-byte unchanged.
    $path = Join-Path $dataDir 'workspace.json'; $bad = '{broken-workspace'
    [IO.File]::WriteAllText($path, $bad, [Text.UTF8Encoding]::new($false))
    Start-App
    Wait-Text '无法读取工作区布局'
    UI press key=toggle-connections --settle | Out-Null
    if (!(Snapshot).Contains('2 / 2 条连接')) { UI press key=toggle-connections --settle | Out-Null }
    Assert-Text '2 / 2 条连接'
    Stop-App
    if ([IO.File]::ReadAllText($path) -ne $bad) { throw 'Damaged layout overwritten' }
    Write-Host "Workspace UI checks passed: $directory"
} catch {
    if ($appProcess -and !$appProcess.HasExited) {
        try { Snapshot | Set-Content -Encoding utf8 (Join-Path $directory 'failure.txt'); UI screenshot (Join-Path $directory 'failure.png') | Out-Null } catch {}
    }
    throw
} finally {
    if ($appProcess) { if (!$appProcess.HasExited) { $appProcess.Kill($true); $appProcess.WaitForExit() }; $appProcess.Dispose() }
    foreach ($server in $servers) {
        if (!$server.HasExited) { $server.StandardInput.Close(); if (!$server.WaitForExit(3000)) { $server.Kill($true); $server.WaitForExit() } }
        $server.Dispose()
    }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
    Pop-Location
}
