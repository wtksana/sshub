param([switch]$SkipBuild, [string]$BuildDirectory = 'build/p4', [switch]$TransferPickers, [switch]$LargeDirectory)
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
    if ($LargeDirectory) {
        $largeDir = Join-Path $remoteRoot 'large'
        New-Item -ItemType Directory $largeDir | Out-Null
        foreach ($i in 0..1999) { [IO.File]::WriteAllText((Join-Path $largeDir ('entry-{0:d4}.txt' -f $i)), 'large directory entry') }
    }
    @{version=1;profiles=@(@{id='sftp';name='本地 SFTP';host='127.0.0.1';port=[int]($server.address.Split(':')[-1]);username='test';auth='private_key';identity_path=$server.identity;favorite=$false})} |
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
    Wait-Text '远程.txt'
	$pathKey = [regex]::Match((Snapshot), 'key=(remote-path-\d+)').Groups[1].Value
	if (!$pathKey) { throw 'Remote path input missing' }
    File-Menu files-mkdir
    UI fill key=file-edit-value '中文 文件夹' --settle | Out-Null
    UI press key=file-edit-confirm --settle | Out-Null
    Wait-Text '中文 文件夹/'
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
    Write-Output "PASS: SFTP browser, mkdir, rename, path navigation, permission error, chmod, delete confirmation. Artifacts: $directory"
} finally {
    if ($appProcess) { if (!$appProcess.HasExited) { $appProcess.Kill(); $appProcess.WaitForExit() }; $appProcess.Dispose() }
    if ($serverProcess) { if (!$serverProcess.HasExited) { $serverProcess.StandardInput.WriteLine('quit'); if (!$serverProcess.WaitForExit(3000)) { $serverProcess.Kill() } }; $serverProcess.Dispose() }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
    Pop-Location
}
