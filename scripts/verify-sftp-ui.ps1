param([switch]$SkipBuild, [string]$BuildDirectory = 'build/p4', [switch]$TransferPickers, [switch]$LargeDirectory, [switch]$NarrowColumns)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/ui-helpers.ps1"
Push-Location $root
$appProcess = $null
$serverProcess = $null
$previousSession = $env:MYGO_AUTOMATION_SESSION
try {
    if (!$SkipBuild) { & "$PSScriptRoot/build.ps1" -DebugBuild -OutputDirectory $BuildDirectory }
    go build -tags e2e -o build/tools/ssh-fixture.exe ./internal/testssh/cmd
    if ($LASTEXITCODE -ne 0) { throw 'Fixture build failed' }
    $directory = Join-Path $root ('artifacts/sftp-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $dataDir = Join-Path $directory 'data'
    $fixtureDir = Join-Path $directory 'server'
    $localDir = Join-Path $directory 'local'
    $downloadDir = Join-Path $directory 'downloads'
    New-Item -ItemType Directory -Force $dataDir,$localDir,$downloadDir | Out-Null
    'SFTP UI 中文 content' | Set-Content -Encoding utf8 (Join-Path $localDir '上传 文本.txt')
    $cli = Join-Path $root 'build/tools/mygo-automation.exe'
    $socket = Join-Path $directory 'app.sock'
    $env:MYGO_AUTOMATION_SESSION = 'sshub-sftp-' + [guid]::NewGuid().ToString('N')
    function UI { return Invoke-ProductUI @args }
    function Snapshot { return (UI snapshot --full) -join "`n" }
    function Wait-Text([string]$text) { UI wait --text $text --timeout 8s --settle | Out-Null }
    function File-Menu([string]$key) {
        UI press key=files-menu --settle | Out-Null
        UI press "key={$key}" --settle | Out-Null
    }
    function Entry-Menu([string]$name, [string]$key) {
        $ref = [regex]::Match((Snapshot), '(?m)^\s*(@e\d+) \[button\] "' + [regex]::Escape($name) + '".*key=remote-entry-').Groups[1].Value
        if (!$ref) { throw "Missing remote entry: $name" }
        UI click $ref --button right --settle | Out-Null
        UI press "key={$key}" --settle | Out-Null
    }
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = Join-Path $root 'build/tools/ssh-fixture.exe'
    $start.UseShellExecute = $false; $start.CreateNoWindow = $true; $start.RedirectStandardInput = $true
    $start.ArgumentList.Add('-dir'); $start.ArgumentList.Add($fixtureDir)
    if ($TransferPickers) { $start.ArgumentList.Add('-sftp-delay'); $start.ArgumentList.Add('30ms') }
    $serverProcess = [Diagnostics.Process]::Start($start)
    $timer = [Diagnostics.Stopwatch]::StartNew()
    while (!(Test-Path (Join-Path $fixtureDir 'server.json'))) {
        if ($serverProcess.HasExited -or $timer.Elapsed.TotalSeconds -gt 8) { throw 'Fixture did not start' }
        Start-Sleep -Milliseconds 50
    }
    $server = Get-Content -Raw (Join-Path $fixtureDir 'server.json') | ConvertFrom-Json
    $remoteRoot = Join-Path $fixtureDir 'files'
    'Remote 中文' | Set-Content -Encoding utf8 (Join-Path $remoteRoot '远程.txt')
    $sortDir = Join-Path $remoteRoot 'sorting'
    New-Item -ItemType Directory $sortDir | Out-Null
    foreach ($item in @(@{name='Alpha.txt';size=30;minutes=3},@{name='beta.txt';size=10;minutes=1},@{name='中文.txt';size=20;minutes=2})) {
        $target = Join-Path $sortDir $item.name
        [IO.File]::WriteAllBytes($target, [byte[]]::new($item.size))
        [IO.File]::SetLastWriteTimeUtc($target, [datetime]::new(2026,11,17,8,0,0,[DateTimeKind]::Utc).AddMinutes($item.minutes))
    }
    if ($LargeDirectory) {
        $largeDir = Join-Path $remoteRoot 'large'
        New-Item -ItemType Directory $largeDir | Out-Null
        foreach ($i in 0..1999) { [IO.File]::WriteAllText((Join-Path $largeDir ('entry-{0:d4}.txt' -f $i)), 'large directory entry') }
    }
    $document = @{version=1;profiles=@(@{id='sftp';name='本地 SFTP';host='127.0.0.1';port=[int]($server.address.Split(':')[-1]);username='test';auth='private_key';identity_path=$server.identity;favorite=$false})}
    if ($NarrowColumns) {
        $document.preferences = @{theme='dark';font_family='Consolas';font_size=13;ui_font_size=20}
        @{version=1;root=@{id=1;tabs=@();active=0};active_pane=1;next_id=2;left_size=260;content_percent=66;hide_left=$false;hide_right=$true;window_width=900;window_height=620} |
            ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 (Join-Path $dataDir 'workspace.json')
    }
    $document |
        ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $dataDir 'connections.json')
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = Join-Path $root "$BuildDirectory/windows-amd64/SSHub.exe"
    $start.UseShellExecute = $false; $start.CreateNoWindow = $true
    $start.Environment['MYGO_AUTOMATION'] = $socket
    $start.ArgumentList.Add('-data-dir'); $start.ArgumentList.Add($dataDir)
    $appProcess = [Diagnostics.Process]::Start($start)
    $timer.Restart()
    while (!(Test-Path -LiteralPath $socket)) {
        if ($appProcess.HasExited -or $timer.Elapsed.TotalSeconds -gt 12) { throw 'App did not start' }
        Start-Sleep -Milliseconds 50
    }
    UI open --socket $socket | Out-Null
    UI click key=profile-sftp --clicks 2 --settle | Out-Null
    Wait-Text '确认服务器身份'
    UI press key=accept-connection-prompt --settle | Out-Null
    Wait-Text '本地 SFTP · 已连接'
    if ((Snapshot).Contains('key=file-server')) { throw 'File panel opened automatically' }
    Start-Sleep -Milliseconds 1200
    $stats = Get-Content -Raw (Join-Path $fixtureDir 'stats.json') | ConvertFrom-Json
    if ($stats.sftp_requests -ne 0) { throw 'SSH-only connection opened SFTP' }
    UI press key=toggle-files --settle | Out-Null
    Wait-Text '远程.txt'
    Start-Sleep -Milliseconds 1200
    $before = (Get-Content -Raw (Join-Path $fixtureDir 'stats.json') | ConvertFrom-Json).sftp_requests
    if ($before -lt 1) { throw 'Opening file panel did not initialize SFTP' }
    UI press key=toggle-files --settle | Out-Null
    UI press key=toggle-files --settle | Out-Null
    Wait-Text '远程.txt'
    Start-Sleep -Milliseconds 1200
    if ((Get-Content -Raw (Join-Path $fixtureDir 'stats.json') | ConvertFrom-Json).sftp_requests -ne $before) { throw 'Reopening initialized panel reconnected SFTP' }
    UI screenshot (Join-Path $directory 'lazy-sftp.png') | Out-Null
	$pathKey = [regex]::Match((Snapshot), 'key=(remote-path-\d+)').Groups[1].Value
	if (!$pathKey) { throw 'Remote path input missing' }
    $headerSnapshot = Snapshot
    $serverLine = ($headerSnapshot -split "`n" | Where-Object { $_ -match 'key=file-server' })[0]
    $summaryLine = ($headerSnapshot -split "`n" | Where-Object { $_ -match 'key=files-summary' })[0]
    $serverY = [double]([regex]::Match($serverLine,'bounds=[^,]+,([^,]+),').Groups[1].Value)
    $summaryY = [double]([regex]::Match($summaryLine,'bounds=[^,]+,([^,]+),').Groups[1].Value)
    if ([Math]::Abs($serverY-$summaryY) -gt 5) { throw 'Server and item count are on separate rows' }
    UI fill "key=$pathKey" '/sorting' --settle | Out-Null
    UI key Enter --settle | Out-Null
    Wait-Text 'Alpha.txt'
    function Assert-TerminalCommand([string]$command) {
        UI press key=terminal-search --settle | Out-Null
        $searchKey = [regex]::Match((Snapshot), 'key=(terminal-query-\d+)').Groups[1].Value
        if (!$searchKey) { throw 'Terminal search missing' }
        UI fill "key=$searchKey" $command --settle | Out-Null
        Wait-Text '1 处匹配'
        $closeKey = $searchKey.Replace('terminal-query-', 'search-close-')
        UI press "key=$closeKey" --settle | Out-Null
    }
    Entry-Menu 'Alpha.txt' file-cd
    Assert-TerminalCommand "cd '/sorting'"
    function Assert-FileOrder([string[]]$expected) {
        $actual = [regex]::Matches((Snapshot),'(?m)^\s*@e\d+ \[button\] "([^"]+)".*key=remote-entry-') | ForEach-Object { $_.Groups[1].Value }
        if (($actual -join '|') -ne ($expected -join '|')) { throw "Wrong file order: $actual; expected $expected" }
    }
    $default = @('Alpha.txt','beta.txt','中文.txt')
    $byValue = @('beta.txt','中文.txt','Alpha.txt')
    Assert-FileOrder $default
    foreach ($column in @('name','modified','size')) {
        if ($NarrowColumns) { UI act "key=files-sort-$column" scrollIntoView --settle | Out-Null }
        UI press "key=files-sort-$column" --settle | Out-Null
        if ($column -eq 'name') { Assert-FileOrder $default } else { Assert-FileOrder $byValue }
        if ((Snapshot) -notmatch 'sortAscending') { throw 'Ascending state missing from header' }
        if ($column -eq 'size') {
            File-Menu files-refresh
            Wait-Text 'Alpha.txt'
            Assert-FileOrder $byValue
            Entry-Menu 'beta.txt' file-properties
            Wait-Text '属性与权限'
            if (!(Snapshot).Contains('/sorting/beta.txt')) { throw 'Sorted row targeted another file' }
            UI press key=file-edit-cancel --settle | Out-Null
        }
        UI screenshot (Join-Path $directory "columns-$column.png") | Out-Null
        UI press "key=files-sort-$column" --settle | Out-Null
        if ($column -eq 'name') { Assert-FileOrder @('中文.txt','beta.txt','Alpha.txt') } else { Assert-FileOrder @('Alpha.txt','中文.txt','beta.txt') }
        if ((Snapshot) -notmatch 'sortDescending') { throw 'Descending state missing from header' }
        UI press "key=files-sort-$column" --settle | Out-Null
        Assert-FileOrder $default
    }
    File-Menu files-home
    Wait-Text '远程.txt'

    File-Menu files-mkdir
    UI fill key=file-edit-value '中文 文件夹' --settle | Out-Null
    UI press key=file-edit-confirm --settle | Out-Null
    Wait-Text '中文 文件夹/'
    Entry-Menu '中文 文件夹/' file-cd
    Assert-TerminalCommand "cd '/中文 文件夹'"
    Entry-Menu '中文 文件夹/' file-rename
    UI fill key=file-edit-value '重命名目录' --settle | Out-Null
    UI press key=file-edit-confirm --settle | Out-Null
    Wait-Text '重命名目录/'
    UI fill "key=$pathKey" '/重命名目录' --settle | Out-Null
    UI key Enter --settle | Out-Null
    Wait-Text '空目录'
    UI fill "key=$pathKey" '/denied' --settle | Out-Null
    UI key Enter --settle | Out-Null
    Wait-Text 'permission denied'
    File-Menu files-home
    Wait-Text '远程.txt'
    Entry-Menu '远程.txt' file-properties
    Wait-Text '属性与权限'
    UI fill key=file-edit-value '600' --settle | Out-Null
    UI press key=file-edit-confirm --settle | Out-Null
    Wait-Text '远程.txt'
    if ($LargeDirectory) {
        UI fill "key=$pathKey" '/large' --settle | Out-Null
        UI key Enter --settle | Out-Null
        Wait-Text 'entry-0000.txt'
        $snapshot = Snapshot
        if ([regex]::Matches($snapshot, 'key=remote-entry-').Count -ge 100) { throw 'Remote list built excessive rows' }
        UI wheel key=remote-files 0 100000 --settle | Out-Null
        Wait-Text 'entry-1999.txt'
        UI screenshot (Join-Path $directory 'large-directory.png') | Out-Null
        File-Menu files-home
        Wait-Text '远程.txt'
    }
    Entry-Menu '重命名目录/' file-delete
    UI press key=file-edit-cancel --settle | Out-Null
    if (!(Test-Path (Join-Path $remoteRoot '重命名目录'))) { throw 'Cancel deleted directory' }
    Entry-Menu '重命名目录/' file-delete
    UI press key=file-edit-confirm --settle | Out-Null
    UI wait --text '重命名目录/' --gone --timeout 8s --settle | Out-Null
    if (Test-Path (Join-Path $remoteRoot '重命名目录')) { throw 'Delete not applied' }
    UI screenshot (Join-Path $directory 'files.png') | Out-Null
    if ($TransferPickers) {
        $large = Join-Path $localDir '生命周期.bin'
        $stream = [IO.File]::Create($large)
        try { $stream.SetLength(8MB) } finally { $stream.Dispose() }
        @{directory=$directory;socket=$socket;session=$env:MYGO_AUTOMATION_SESSION;source=(Join-Path $localDir '上传 文本.txt');large=$large;downloads=$downloadDir;remote=$remoteRoot} |
            ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $root 'artifacts/sftp-ui/active.json')
        Write-Output "File management passed. Transfer fixture: $directory"
        Read-Host 'Drive native transfer pickers and lifecycle checks, then press Enter' | Out-Null
    }
    if (!$appProcess.HasExited) {
        UI press key=window-close --settle | Out-Null
        Wait-Text '退出 SSHub？'
        $ref = [regex]::Match((Snapshot), '(?m)^\s*(@e\d+) \[button\] "断开并退出"').Groups[1].Value
        UI press $ref | Out-Null
    }
    if (!$appProcess.WaitForExit(5000) -or $appProcess.ExitCode -ne 0) { throw 'App failed to exit cleanly' }
    Write-Output "PASS: file header layout, three-state column sorting, SFTP browser, mkdir, rename, path navigation, permission error, chmod, delete confirmation. Artifacts: $directory"
} finally {
    if ($appProcess) { if (!$appProcess.HasExited) { $appProcess.Kill(); $appProcess.WaitForExit() }; $appProcess.Dispose() }
    if ($serverProcess) { if (!$serverProcess.HasExited) { $serverProcess.StandardInput.WriteLine('quit'); if (!$serverProcess.WaitForExit(3000)) { $serverProcess.Kill() } }; $serverProcess.Dispose() }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
    Pop-Location
}
