param([switch]$SkipBuild, [string]$BuildDirectory = 'build')
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/ui-helpers.ps1"
Push-Location $root
$process = $null
$previousSession = $env:MYGO_AUTOMATION_SESSION
try {
    if (!$SkipBuild) { & "$PSScriptRoot/build.ps1" -DebugBuild -OutputDirectory $BuildDirectory }
    go build -o build/tools/mygo-automation.exe github.com/egoist/mygo/plugins/automation/cmd/mygo-automation
    if ($LASTEXITCODE -ne 0) { throw 'Automation CLI build failed' }
    $cli = Join-Path $root 'build/tools/mygo-automation.exe'
    $exe = Join-Path $root "$BuildDirectory/windows-amd64/SSHub.exe"
    $directory = Join-Path $root ('artifacts/library-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $dataDir = Join-Path $directory 'data'
    New-Item -ItemType Directory -Force $dataDir | Out-Null
    $socket = Join-Path $directory 'app.sock'
    if ($socket.Length -gt 100) { throw 'Automation socket path is too long' }
    $env:MYGO_AUTOMATION_SESSION = 'sshub-library-' + [guid]::NewGuid().ToString('N')

    function Invoke-UI { return Invoke-ProductUI @args }
    function Read-UI { return (Invoke-UI snapshot --full) -join "`n" }
    function Assert-Text([string]$text) {
        if (!(Read-UI).Contains($text)) { throw "Expected UI text: $text" }
    }
    function Assert-Absent([string]$text) {
        if ((Read-UI).Contains($text)) { throw "Unexpected UI text: $text" }
    }
    function Choose-Option([string]$key, [string]$label) {
        if ($key -eq 'group-filter') { Select-UIGroup $label; return }
        Invoke-UI press "key=$key" --settle | Out-Null
        $snapshot = Read-UI
        $pattern = '(?m)^\s*(@e\d+) \[listItem\] "' + [regex]::Escape($label) + '"'
        $match = [regex]::Match($snapshot, $pattern)
        if (!$match.Success) { throw "Option not found: $label" }
        Invoke-UI press $match.Groups[1].Value --settle | Out-Null
    }
    function Start-TestApp {
        $start = [Diagnostics.ProcessStartInfo]::new()
        $start.FileName = $exe
        $start.UseShellExecute = $false
        $start.CreateNoWindow = $true
        $start.Environment['MYGO_AUTOMATION'] = $socket
        foreach ($argument in @('-data-dir', $dataDir)) { $start.ArgumentList.Add($argument) }
        $script:process = [Diagnostics.Process]::Start($start)
        $timer = [Diagnostics.Stopwatch]::StartNew()
        while (!(Test-Path -LiteralPath $socket)) {
            if ($script:process.HasExited -or $timer.Elapsed.TotalSeconds -gt 12) { throw 'App automation did not start' }
            Start-Sleep -Milliseconds 100
        }
        Invoke-UI open --socket $socket | Out-Null
        Invoke-UI wait --text 'SSHub' --settle | Out-Null
    }
    function Stop-TestApp {
        Invoke-UI close | Out-Null
        $script:process.CloseMainWindow() | Out-Null
        if (!$script:process.WaitForExit(5000)) { throw 'App did not close' }
        if ($script:process.ExitCode -ne 0) { throw "App exited: $($script:process.ExitCode)" }
        $script:process.Dispose()
        $script:process = $null
    }
    function Fill-NewProfile([string]$name, [string]$hostName, [string]$groupName) {
        Invoke-UI press key=new-profile --settle | Out-Null
        Assert-Text '密码'
        Invoke-UI fill key=profile-name $name | Out-Null
        Invoke-UI fill key=profile-host $hostName | Out-Null
        Invoke-UI fill key=profile-user deploy | Out-Null
        Invoke-UI fill key=profile-group $groupName | Out-Null
    }
    function Select-Profile([string]$id) { Invoke-UI press "key=profile-$id" --settle | Out-Null }

    Start-TestApp
    Assert-Text '0 / 0 条连接'
    Fill-NewProfile '香港生产服务器' '127.0.0.1' '生产'
    Invoke-UI fill key=profile-port '70000' | Out-Null
    Invoke-UI press key=save-profile --settle | Out-Null
    Assert-Text '端口必须为 1–65535'
    Invoke-UI fill key=profile-port '1' | Out-Null
    Choose-Option 'profile-auth' '密码'
    Invoke-UI press key=profile-favorite | Out-Null
    Invoke-UI press key=save-profile --settle | Out-Null
    Assert-Absent 'key=edit-profile'
    Assert-Text 'deploy@127.0.0.1:1'
    Open-ProfileMenu
    Assert-Text '取消收藏'
    Assert-Text '复制连接'
    Assert-Text '删除凭据'
    Assert-Absent '打开工作区'
    Invoke-UI screenshot (Join-Path $directory 'connection-menu.png') | Out-Null
    Invoke-RawUI key Escape --settle | Out-Null
    Invoke-UI press key=connect-profile --settle | Out-Null
    Invoke-UI wait --text '连接失败' --timeout 8s --settle | Out-Null
    Open-PaneMenu
    if ((Get-ProductSnapshot) -notmatch '\[menuItem\] "连接"') { throw 'Connection menu missing' }
    Invoke-RawUI key Escape --settle | Out-Null
    $snapshot = Read-UI
    if ($snapshot -match 'key=(connect-pending|split-horizontal|font-larger|edit-profile)') { throw 'Workspace shows persistent action controls' }
    $snapshot | Set-Content -Encoding utf8 (Join-Path $directory 'workspace.txt')
    Invoke-UI screenshot (Join-Path $directory 'workspace.png') | Out-Null

    Fill-NewProfile '测试机' '192.0.2.20' '测试'
    Choose-Option 'profile-auth' '私钥文件'
    Invoke-UI fill key=profile-identity 'C:\test keys\id_ed25519' | Out-Null
    Invoke-UI press key=save-profile --settle | Out-Null
    Assert-Text '2 / 2 条连接'
    $configPath = Join-Path $dataDir 'connections.json'
    $doc = Get-Content -Raw $configPath | ConvertFrom-Json
    $firstID = ($doc.profiles | Where-Object name -eq '香港生产服务器').id
    $secondID = ($doc.profiles | Where-Object name -eq '测试机').id
    if (!$firstID -or !$secondID) { throw 'Saved connection IDs missing' }
    Open-ProfileMenu $secondID
    Invoke-RawUI press 'key={copy-profile}' --settle | Out-Null
    Assert-Text '新建连接'
    Assert-Text 'value="测试机 副本"'
    Assert-Text 'value="192.0.2.20"'
    Assert-Text '密码'
    Assert-Absent 'key=profile-identity'
    Invoke-UI screenshot (Join-Path $directory 'copy-connection.png') | Out-Null
    Invoke-UI press key=save-profile --settle | Out-Null
    $copied = (Get-Content -Raw $configPath | ConvertFrom-Json).profiles | Where-Object name -eq '测试机 副本'
    if (!$copied -or $copied.id -eq $secondID -or $copied.auth -ne 'password' -or $copied.identity_path) { throw 'Copied connection inherited authentication' }
    Open-ProfileMenu $copied.id
    Invoke-RawUI press 'key={delete-profile}' --settle | Out-Null
    $deleteRef = [regex]::Match((Read-UI), '(?m)^\s*(@e\d+) \[button\] "删除配置"').Groups[1].Value
    Invoke-UI press $deleteRef --settle | Out-Null
    Assert-Text '2 / 2 条连接'
    Open-ProfileMenu $secondID
    Open-UISubmenu '分组'
    $groupRef = [regex]::Match((Read-UI), '(?m)^\s*(@e\d+) \[menuItemRadio\] "生产"').Groups[1].Value
    if (!$groupRef) { throw 'Existing group option missing' }
    Invoke-RawUI press $groupRef --settle | Out-Null
    if (((Get-Content -Raw $configPath | ConvertFrom-Json).profiles | Where-Object id -eq $secondID).group -ne '生产') { throw 'Context group move failed' }
    Open-ProfileMenu $secondID
    Open-UISubmenu '分组'
    Invoke-RawUI press 'key={profile-group-new}' --settle | Out-Null
    Invoke-UI fill key=quick-group '新建分组' --settle | Out-Null
    Invoke-UI press key=quick-group-save --settle | Out-Null
    if (((Get-Content -Raw $configPath | ConvertFrom-Json).profiles | Where-Object id -eq $secondID).group -ne '新建分组') { throw 'New context group failed' }
    Open-ProfileMenu $secondID
    Open-UISubmenu '分组'
    Invoke-RawUI press 'key={profile-group-none}' --settle | Out-Null
    if (((Get-Content -Raw $configPath | ConvertFrom-Json).profiles | Where-Object id -eq $secondID).group) { throw 'Ungroup failed' }
    Open-ProfileMenu $secondID
    Invoke-RawUI press 'key={edit-profile}' --settle | Out-Null
    Invoke-UI press key=profile-group-options --settle | Out-Null
    Assert-Text '[listItem] "生产"'
    Invoke-UI screenshot (Join-Path $directory 'group-options.png') | Out-Null
    Choose-Option 'profile-group' '生产'
    Invoke-UI screenshot (Join-Path $directory 'group-combobox.png') | Out-Null
    Invoke-UI fill key=profile-group '测试' --settle | Out-Null
    Invoke-UI press key=save-profile --settle | Out-Null
    if (((Get-Content -Raw $configPath | ConvertFrom-Json).profiles | Where-Object id -eq $secondID).group -ne '测试') { throw 'Typed group was not saved' }
    # Editing an unselected row by right-click must target that row.
    Open-ProfileMenu $firstID
    Invoke-RawUI press 'key={edit-profile}' --settle | Out-Null
    Assert-Text 'value="香港生产服务器"'
    Invoke-UI press key=cancel-profile --settle | Out-Null
    Invoke-UI fill key=search-profiles '香港' --settle | Out-Null
    Assert-Text '1 / 2 条连接'
    Assert-Absent '选择连接 测试机'
    Invoke-UI fill key=search-profiles '' --settle | Out-Null
    Invoke-UI press key=scope-favorites --settle | Out-Null
    Assert-Text '1 / 2 条连接'
    Invoke-UI press key=scope-recent --settle | Out-Null
    Assert-Text '1 / 2 条连接'
    Assert-Text '选择连接 香港生产服务器'
    Invoke-UI press key=scope- --settle | Out-Null
    Choose-Option 'group-filter' '分组：测试'
    Assert-Text '1 / 2 条连接'
    Assert-Text '选择连接 测试机'
    Choose-Option 'group-filter' '全部分组'
    Select-Profile $firstID
    Invoke-UI press key=edit-profile --settle | Out-Null
    Invoke-UI fill key=profile-name '香港主机（已修改）' | Out-Null
    Invoke-UI fill key=profile-group '核心' | Out-Null
    Invoke-UI press key=save-profile --settle | Out-Null
    if ((Get-Content -Raw (Join-Path $dataDir 'connections.json') | ConvertFrom-Json).profiles[0].group -ne '核心') { throw 'Edited group not saved' }
    Invoke-UI press key=favorite-profile --settle | Out-Null
    Invoke-UI press key=scope-favorites --settle | Out-Null
    Assert-Text '0 / 2 条连接'
    Invoke-UI press key=scope- --settle | Out-Null
    Invoke-UI press key=toggle-files --settle | Out-Null
    Assert-Text '传输队列'
    Invoke-UI press key=toggle-files --settle | Out-Null
    Assert-Absent '传输队列'
    Invoke-UI press key=toggle-connections --settle | Out-Null
    Assert-Absent 'key=search-profiles'
    Invoke-UI press key=toggle-connections --settle | Out-Null
    Invoke-UI key Ctrl+B --settle | Out-Null
    Assert-Absent 'key=search-profiles'
    Invoke-UI key Ctrl+B --settle | Out-Null
    Assert-Text 'key=search-profiles'
    Invoke-UI key Ctrl+Shift+E --settle | Out-Null
    Assert-Text '传输队列'
    Invoke-UI key Ctrl+Shift+E --settle | Out-Null
    Assert-Absent '传输队列'
    Stop-TestApp

    Start-TestApp
    Assert-Text '2 / 2 条连接'
    Select-Profile $firstID
    Assert-Text '香港主机（已修改）'
    if ((Get-Content -Raw (Join-Path $dataDir 'connections.json') | ConvertFrom-Json).profiles[0].group -ne '核心') { throw 'Edited group not saved' }
    Invoke-UI press key=scope-recent --settle | Out-Null
    Assert-Text '1 / 2 条连接'
    Invoke-UI press key=scope- --settle | Out-Null
    Select-Profile $secondID
    Invoke-UI press key=edit-profile --settle | Out-Null
    Assert-Text 'value="C:\\test keys\\id_ed25519"'
    Invoke-UI press key=cancel-profile --settle | Out-Null
    Invoke-UI press key=delete-profile --settle | Out-Null
    Invoke-UI key Escape --settle | Out-Null
    Assert-Text '2 / 2 条连接'
    Invoke-UI press key=delete-profile --settle | Out-Null
    $deleteRef = [regex]::Match((Read-UI), '(?m)^\s*(@e\d+) \[button\] "删除配置"').Groups[1].Value
    if (!$deleteRef) { throw 'Delete confirmation missing' }
    Invoke-UI press $deleteRef --settle | Out-Null
    Assert-Text '1 / 1 条连接'
    Stop-TestApp

    # Use a separate fixture with 100 saved profiles, never the user's data.
    $profiles = @(0..99 | ForEach-Object {
        @{id="fixture-$_";name=('服务器-{0:d3}' -f $_);host="192.0.2.$($_ + 1)";port=22;username='deploy';auth='agent';group=$(if ($_ % 2) {'生产'} else {'测试'});favorite=($_ % 10 -eq 0)}
    })
    @{version=1;profiles=$profiles} | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 $configPath
    Start-TestApp
    Assert-Text '100 / 100 条连接'
    Invoke-UI fill key=search-profiles '服务器-099' --settle | Out-Null
    Assert-Text '1 / 100 条连接'
    Assert-Text '选择连接 服务器-099'
    Invoke-UI fill key=search-profiles '' --settle | Out-Null
    Invoke-UI press key=scope-favorites --settle | Out-Null
    Assert-Text '10 / 100 条连接'
    Invoke-UI press key=scope- --settle | Out-Null
    Choose-Option 'group-filter' '分组：生产'
    Assert-Text '50 / 100 条连接'
    Invoke-UI screenshot (Join-Path $directory '100-profiles.png') | Out-Null
    Stop-TestApp

    $badBytes = [Text.Encoding]::UTF8.GetBytes('{broken-config')
    [IO.File]::WriteAllBytes($configPath, $badBytes)
    Start-TestApp
    Assert-Text '无法读取连接配置'
    Assert-Absent 'key=new-profile'
    Invoke-UI screenshot (Join-Path $directory 'invalid-config.png') | Out-Null
    Stop-TestApp
    if (![Linq.Enumerable]::SequenceEqual([byte[]][IO.File]::ReadAllBytes($configPath), [byte[]]$badBytes)) { throw 'Invalid file was changed' }

    Write-Host "Connection library UI checks passed: $directory"
} catch {
    if ($process -and !$process.HasExited) { try { Read-UI | Set-Content -Encoding utf8 (Join-Path $directory 'failure.txt') } catch {} }
    throw
} finally {
    if ($process -and !$process.HasExited) { $process.Kill($true); $process.WaitForExit() }
    if ($process) { $process.Dispose() }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
    Pop-Location
}
