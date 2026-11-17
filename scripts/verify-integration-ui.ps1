param([switch]$SkipBuild, [string]$BuildDirectory = 'build/p6', [switch]$NativeActions)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/ui-helpers.ps1"
if (!$NativeActions) { throw 'Use -NativeActions in an interactive terminal to drive the actual Windows picker' }
Push-Location $root
$appProcess = $null
$servers = [Collections.Generic.List[Diagnostics.Process]]::new()
$previousSession = $env:MYGO_AUTOMATION_SESSION
try {
    if (!$SkipBuild) { & "$PSScriptRoot/build.ps1" -DebugBuild -OutputDirectory $BuildDirectory }
    go build -tags e2e -o build/tools/ssh-fixture.exe ./internal/testssh/cmd
    if ($LASTEXITCODE -ne 0) { throw 'Fixture build failed' }
    $directory = Join-Path $root ('artifacts/integration-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $dataDir = Join-Path $directory 'data'
    $localDir = Join-Path $directory 'local'
    New-Item -ItemType Directory -Force $dataDir,$localDir | Out-Null
    $cli = Join-Path $root 'build/tools/mygo-automation.exe'
    if (!(Test-Path -LiteralPath $cli)) {
        go build -o $cli github.com/egoist/mygo/plugins/automation/cmd/mygo-automation
        if ($LASTEXITCODE -ne 0) { throw 'Automation build failed' }
    }
    $socket = Join-Path $directory 'app.sock'
    $env:MYGO_AUTOMATION_SESSION = 'sshub-integration-' + [guid]::NewGuid().ToString('N')
    function UI { return Invoke-ProductUI @args }
    function Wait-Text([string]$text) { UI wait --text $text --timeout 12s --settle | Out-Null }
    function Start-Server([string]$name) {
        $fixtureDir = Join-Path $directory $name
        $start = [Diagnostics.ProcessStartInfo]::new()
        $start.FileName = Join-Path $root 'build/tools/ssh-fixture.exe'
        $start.UseShellExecute = $false; $start.CreateNoWindow = $true; $start.RedirectStandardInput = $true
        foreach ($value in @('-dir', $fixtureDir, '-sftp-delay', '80ms')) { $start.ArgumentList.Add($value) }
        $server = [Diagnostics.Process]::Start($start); $servers.Add($server)
        $timer = [Diagnostics.Stopwatch]::StartNew()
        $path = Join-Path $fixtureDir 'server.json'
        while (!(Test-Path -LiteralPath $path)) {
            if ($server.HasExited -or $timer.Elapsed.TotalSeconds -gt 8) { throw 'Fixture did not start' }
            Start-Sleep -Milliseconds 50
        }
        return Get-Content -Raw $path | ConvertFrom-Json
    }
    function Open-Session([string]$id, [string]$name, [switch]$First) {
        if ($First) { UI click "key=profile-$id" --clicks 2 --settle | Out-Null }
        else {
            UI press "key=profile-$id" --settle | Out-Null
            UI press key=open-workspace --settle | Out-Null
            UI press key=connect-pending --settle | Out-Null
        }
        Wait-Text '确认服务器身份'
        UI press key=accept-connection-prompt --settle | Out-Null
        Wait-Text "$name · 已连接"
        return [int]([regex]::Matches((Get-ProductSnapshot), 'key=tab-(\d+)') | ForEach-Object { [int]$_.Groups[1].Value } | Measure-Object -Maximum).Maximum
    }
    function Navigate([int]$tab, [string]$path) {
        UI fill "key=remote-path-$tab" $path --settle | Out-Null
        UI key Enter --settle | Out-Null
        Wait-Text '空目录'
    }
    $serverA = Start-Server 'server-a'; $serverB = Start-Server 'server-b'
    $remoteA = Join-Path $directory 'server-a/files'
    $remoteB = Join-Path $directory 'server-b/files'
    New-Item -ItemType Directory (Join-Path $remoteA 'inbox'),(Join-Path $remoteB 'other') | Out-Null
    $sources = @((Join-Path $localDir '持续上传 中文.bin'), (Join-Path $localDir '排队上传.txt'))
    $stream = [IO.File]::Create($sources[0])
    try {
        $block = [Text.Encoding]::UTF8.GetBytes(('SSHub isolated transfer payload 中文 ' * 2048))
        while ($stream.Length -lt 16MB) { $stream.Write($block) }
    } finally { $stream.Dispose() }
    [IO.File]::WriteAllText($sources[1], 'Queued upload remains on server A /inbox', [Text.UTF8Encoding]::new($false))
    @{version=1;profiles=@(
        @{id='alpha';name='集成服务器 A';host='127.0.0.1';port=[int]($serverA.address.Split(':')[-1]);username='test';auth='private_key';identity_path=$serverA.identity;favorite=$false},
        @{id='beta';name='集成服务器 B';host='127.0.0.1';port=[int]($serverB.address.Split(':')[-1]);username='test';auth='private_key';identity_path=$serverB.identity;favorite=$false}
    )} | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $dataDir 'connections.json')
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
    $tabA = Open-Session 'alpha' '集成服务器 A' -First
    Navigate $tabA '/inbox'
    $tabB = Open-Session 'beta' '集成服务器 B'
    Navigate $tabB '/other'
    UI press "key=tab-$tabA" --settle | Out-Null
    @{directory=$directory;socket=$socket;session=$env:MYGO_AUTOMATION_SESSION;sources=$sources;remoteA=$remoteA;remoteB=$remoteB;tabA=$tabA;tabB=$tabB} |
        ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $root 'artifacts/integration-ui/active.json')
    UI press key=files-menu --settle | Out-Null
    UI press 'key={files-upload}' --settle | Out-Null
    Read-Host 'Select both generated sources in the native picker, then immediately press Enter here' | Out-Null
    Wait-Text '传输任务 上传 持续上传 中文.bin'
    Wait-Text '传输任务 上传 排队上传.txt'
    $before = Get-ProductSnapshot
    if ($before -notmatch '集成服务器 A · 传输中' -or $before -notmatch '集成服务器 A · 等待中') { throw 'Expected both an active and a queued upload at focus switch; rerun with slower fixture' }
    $before | Set-Content -Encoding utf8 (Join-Path $directory 'before-focus-switch.txt')
    UI press "key=tab-$tabB" --settle | Out-Null
    if ((Get-ProductSnapshot) -notmatch '\[text\] "集成服务器 B · 已连接" key=file-server') { throw 'File panel did not follow B' }
    UI press "key=close-tab-$tabA" --settle | Out-Null
    Wait-Text '关闭终端会话？'
    $ref = [regex]::Match((Get-ProductSnapshot), '(?m)^\s*(@e\d+) \[button\] "关闭终端并继续传输"').Groups[1].Value
    if (!$ref) { throw 'Close-session confirmation missing' }
    UI press $ref --settle | Out-Null
    UI wait "key=tab-$tabA" --gone --timeout 8s --settle | Out-Null
    $timer.Restart()
    do {
        $snapshot = Get-ProductSnapshot
        if ($snapshot -match '集成服务器 A · (失败|已取消)') { throw 'Upload failed after closing its terminal' }
        if ([regex]::Matches($snapshot, '集成服务器 A · 已完成').Count -eq 2) { break }
        if ($timer.Elapsed.TotalSeconds -gt 120) { throw 'Uploads did not finish' }
        Start-Sleep -Milliseconds 250
    } while ($true)
    $results = foreach ($source in $sources) {
        $name = [IO.Path]::GetFileName($source)
        $destination = Join-Path $remoteA "inbox/$name"
        $expected = (Get-FileHash -LiteralPath $source).Hash
        if ((Get-FileHash -LiteralPath $destination).Hash -ne $expected) { throw "Upload content mismatch: $name" }
        foreach ($wrong in @((Join-Path $remoteA $name), (Join-Path $remoteB $name), (Join-Path $remoteB "other/$name"))) {
            if (Test-Path -LiteralPath $wrong) { throw "Upload reached wrong destination: $wrong" }
        }
        @{file=$name;sha256=$expected;destination="server-a:/inbox/$name"}
    }
    UI screenshot (Join-Path $directory 'transfers-after-close.png') | Out-Null
    @{passed=$true;scenario='Native multi-file picker; active upload and queued upload retain A:/inbox after switching to B:/other and closing A terminal';files=@($results)} |
        ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $directory 'transfers.json')
    Read-Host 'Perform clipboard or other native integration checks, then press Enter to exit' | Out-Null
    UI press key=window-close --settle | Out-Null
    Wait-Text '退出 SSHub？'
    $ref = [regex]::Match((Get-ProductSnapshot), '(?m)^\s*(@e\d+) \[button\] "断开并退出"').Groups[1].Value
    if (!$ref) { throw 'Exit confirmation missing' }
    UI press $ref | Out-Null
    if (!$appProcess.WaitForExit(5000) -or $appProcess.ExitCode -ne 0) { throw 'App failed to exit cleanly' }
    Write-Output "PASS: cross-server UI target binding and terminal-close lifecycle. Artifacts: $directory"
} catch {
    if ($appProcess -and !$appProcess.HasExited) {
        try { Get-ProductSnapshot | Set-Content -Encoding utf8 (Join-Path $directory 'failure.txt'); UI screenshot (Join-Path $directory 'failure.png') | Out-Null } catch {}
    }
    throw
} finally {
    if ($appProcess) { if (!$appProcess.HasExited) { $appProcess.Kill(); $appProcess.WaitForExit() }; $appProcess.Dispose() }
    foreach ($server in $servers) {
        if (!$server.HasExited) { $server.StandardInput.WriteLine('quit'); if (!$server.WaitForExit(3000)) { $server.Kill(); $server.WaitForExit() } }
        $server.Dispose()
    }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
    Pop-Location
}
