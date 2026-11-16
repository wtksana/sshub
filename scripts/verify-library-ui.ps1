param([switch]$SkipBuild)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
$process = $null
$previousSession = $env:MYGO_AUTOMATION_SESSION
try {
    if (!$SkipBuild) { & "$PSScriptRoot/build.ps1" -DebugBuild }
    go build -o build/tools/mygo-automation.exe github.com/egoist/mygo/plugins/automation/cmd/mygo-automation
    if ($LASTEXITCODE -ne 0) { throw 'Automation CLI build failed' }
    $cli = Join-Path $root 'build/tools/mygo-automation.exe'
    $exe = Join-Path $root 'build/windows-amd64/SSHub.exe'
    $directory = Join-Path $root ('artifacts/library-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $dataDir = Join-Path $directory 'data'
    New-Item -ItemType Directory -Force $dataDir | Out-Null
    $socket = Join-Path $directory 'app.sock'
    if ($socket.Length -gt 100) { throw 'Automation socket path is too long' }
    $env:MYGO_AUTOMATION_SESSION = 'sshub-library-' + [guid]::NewGuid().ToString('N')

    function Invoke-UI {
        $output = & $cli @args
        if ($LASTEXITCODE -ne 0) { throw "UI command failed: $args" }
        return $output
    }
    function Read-UI { return (Invoke-UI snapshot --full) -join "`n" }
    function Assert-Text([string]$text) {
        if (!(Read-UI).Contains($text)) { throw "Expected UI text: $text" }
    }
    function Assert-Absent([string]$text) {
        if ((Read-UI).Contains($text)) { throw "Unexpected UI text: $text" }
    }
    function Choose-Option([string]$key, [string]$label) {
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
        Invoke-UI fill key=profile-name $name | Out-Null
        Invoke-UI fill key=profile-host $hostName | Out-Null
        Invoke-UI fill key=profile-user deploy | Out-Null
        Invoke-UI fill key=profile-group $groupName | Out-Null
    }
    function Select-Profile([string]$id) { Invoke-UI press "key=profile-$id" --settle | Out-Null }

    Start-TestApp
    Assert-Text '0 / 0 条连接'
    Fill-NewProfile '香港生产服务器' '192.0.2.10' '生产'
    Invoke-UI fill key=profile-port '70000' | Out-Null
    Invoke-UI press key=save-profile --settle | Out-Null
    Assert-Text '端口必须为 1–65535'
    Invoke-UI fill key=profile-port '2222' | Out-Null
    Choose-Option 'profile-auth' '密码'
    Invoke-UI press key=profile-favorite | Out-Null
    Invoke-UI press key=save-profile --settle | Out-Null
    Assert-Text '认证：密码'
    Assert-Text 'deploy@192.0.2.10:2222'
    Assert-Text '取消收藏'
    Invoke-UI press key=open-workspace --settle | Out-Null
    Assert-Text '香港生产服务器 · 待连接'
    $snapshot = Read-UI
    if ($snapshot -notmatch 'key=connect-pending' -or $snapshot -match 'key=connect-pending disabled') { throw 'Connection action missing' }
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
    Assert-Text '分组：核心'
    Invoke-UI press key=favorite-profile --settle | Out-Null
    Invoke-UI press key=scope-favorites --settle | Out-Null
    Assert-Text '0 / 2 条连接'
    Invoke-UI press key=scope- --settle | Out-Null
    Invoke-UI press key=toggle-files --settle | Out-Null
    Assert-Absent '传输队列'
    Invoke-UI press key=toggle-files --settle | Out-Null
    Assert-Text '传输队列'
    Invoke-UI press key=toggle-connections --settle | Out-Null
    Assert-Absent 'key=search-profiles'
    Invoke-UI press key=toggle-connections --settle | Out-Null
    Stop-TestApp

    Start-TestApp
    Assert-Text '2 / 2 条连接'
    Select-Profile $firstID
    Assert-Text '香港主机（已修改）'
    Assert-Text '分组：核心'
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
    Assert-Text '无法读取连接库'
    Assert-Absent 'key=new-profile'
    Invoke-UI screenshot (Join-Path $directory 'invalid-config.png') | Out-Null
    Stop-TestApp
    if (![Linq.Enumerable]::SequenceEqual([byte[]][IO.File]::ReadAllBytes($configPath), [byte[]]$badBytes)) { throw 'Invalid file was changed' }

    Write-Host "Connection library UI checks passed: $directory"
} finally {
    if ($process -and !$process.HasExited) { $process.Kill($true); $process.WaitForExit() }
    if ($process) { $process.Dispose() }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
    Pop-Location
}
