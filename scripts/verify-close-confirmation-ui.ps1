param([switch]$SkipBuild, [string]$BuildDirectory = 'build/close-confirmation')
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
    $directory = Join-Path $root ('artifacts/close-confirmation-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $dataDir = Join-Path $directory 'data'
    $fixtureDir = Join-Path $directory 'server'
    New-Item -ItemType Directory -Force $dataDir | Out-Null
    $cli = Join-Path $root 'build/tools/mygo-automation.exe'
    $socket = Join-Path $directory 'app.sock'
    $env:MYGO_AUTOMATION_SESSION = 'sshub-close-' + [guid]::NewGuid().ToString('N')
    function UI { return Invoke-ProductUI @args }
    function Snapshot { return (UI snapshot --full) -join "`n" }
    function Wait-Text([string]$text) { UI wait --text $text --timeout 8s --settle | Out-Null }
    function Start-App {
        $start = [Diagnostics.ProcessStartInfo]::new()
        $start.FileName = Join-Path $root "$BuildDirectory/windows-amd64/SSHub.exe"
        $start.UseShellExecute = $false; $start.CreateNoWindow = $true
        $start.Environment['MYGO_AUTOMATION'] = $socket
        $start.ArgumentList.Add('-data-dir'); $start.ArgumentList.Add($dataDir)
        $script:appProcess = [Diagnostics.Process]::Start($start)
        $timer = [Diagnostics.Stopwatch]::StartNew()
        while (!(Test-Path $socket)) {
            if ($script:appProcess.HasExited -or $timer.Elapsed.TotalSeconds -gt 12) { throw 'App did not start' }
            Start-Sleep -Milliseconds 50
        }
        UI open --socket $socket | Out-Null
    }
    function Stop-App {
        UI press key=window-close | Out-Null
        if (!$script:appProcess.WaitForExit(5000) -or $script:appProcess.ExitCode -ne 0) { throw 'App did not exit cleanly' }
        UI close | Out-Null
        $script:appProcess.Dispose(); $script:appProcess = $null
    }
    function Connect-Tab {
        UI click key=profile-fixture --clicks 2 --settle | Out-Null
        if ((Snapshot).Contains('确认服务器身份')) { UI press key=accept-connection-prompt --settle | Out-Null }
        Wait-Text '关闭提示 SSH · 已连接'
        $script:tabKey = [regex]::Match((Snapshot), 'key=close-tab-(\d+)').Groups[1].Value
        if (!$script:tabKey) { throw 'Session tab missing' }
    }
    function Close-Tab {
        UI press "key=close-tab-$script:tabKey" --settle | Out-Null
    }
    function Assert-Closed {
        UI wait --text '关闭终端会话？' --gone --timeout 8s --settle | Out-Null
        if ((Snapshot) -match "key=close-tab-$script:tabKey\b") { throw 'Session still open' }
    }
    function Assert-Saved([bool]$expected) {
        $prefs = (Get-Content -Raw $configPath | ConvertFrom-Json).preferences
        if ([bool]$prefs.skip_close_confirmation -ne $expected) { throw "Expected skip_close_confirmation=$expected" }
    }
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = Join-Path $root 'build/tools/ssh-fixture.exe'
    $start.UseShellExecute = $false; $start.CreateNoWindow = $true; $start.RedirectStandardInput = $true
    $start.ArgumentList.Add('-dir'); $start.ArgumentList.Add($fixtureDir)
    $serverProcess = [Diagnostics.Process]::Start($start)
    $timer = [Diagnostics.Stopwatch]::StartNew()
    while (!(Test-Path (Join-Path $fixtureDir 'server.json'))) {
        if ($serverProcess.HasExited -or $timer.Elapsed.TotalSeconds -gt 8) { throw 'Fixture did not start' }
        Start-Sleep -Milliseconds 50
    }
    $server = Get-Content -Raw (Join-Path $fixtureDir 'server.json') | ConvertFrom-Json
    $configPath = Join-Path $dataDir 'connections.json'
    @{version=1;profiles=@(@{id='fixture';name='关闭提示 SSH';host='127.0.0.1';port=[int]($server.address.Split(':')[-1]);username='test';auth='private_key';identity_path=$server.identity;favorite=$false})} |
        ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 $configPath
    Start-App
    Connect-Tab
    Close-Tab
    Wait-Text '以后不再提示'
    UI screenshot (Join-Path $directory 'close-confirmation.png') | Out-Null
    UI press key=close-session-remember --settle | Out-Null
    UI press key=close-session-cancel --settle | Out-Null
    Assert-Saved $false
    Close-Tab
    if ((Snapshot) -match '\[checkbox\].*key=close-session-remember.*checked') { throw 'Canceled checkbox remained checked' }
    UI press key=close-session-remember --settle | Out-Null
    UI key Escape --settle | Out-Null
    Assert-Saved $false
    Close-Tab
    UI press key=close-session-accept --settle | Out-Null
    Assert-Closed
    Assert-Saved $false
    Connect-Tab
    Close-Tab
    UI press key=close-session-remember --settle | Out-Null
    UI press key=close-session-accept --settle | Out-Null
    Assert-Closed
    Assert-Saved $true
    Connect-Tab
    UI press key=window-close --settle | Out-Null
    Wait-Text '退出 SSHub？'
    UI key Escape --settle | Out-Null
    Close-Tab
    Assert-Closed
    Stop-App
    Start-App
    Connect-Tab
    Close-Tab
    Assert-Closed
    Connect-Tab
    UI press key=app-menu --settle | Out-Null
    UI press 'key={settings}' --settle | Out-Null
    UI act key=settings-skip-close-confirmation scrollIntoView --settle | Out-Null
    UI screenshot (Join-Path $directory 'settings.png') | Out-Null
    UI press key=settings-skip-close-confirmation --settle | Out-Null
    UI press key=settings-save --settle | Out-Null
    Wait-Text '设置已保存'
    Assert-Saved $false
    Close-Tab
    Wait-Text '以后不再提示'
    # Force an actual atomic-save failure in the isolated fixture directory.
    Move-Item -LiteralPath $configPath -Destination "$configPath.bak"
    New-Item -ItemType Directory -Path $configPath | Out-Null
    try {
        UI press key=close-session-remember --settle | Out-Null
        UI press key=close-session-accept --settle | Out-Null
        Wait-Text '无法保存提示偏好'
        Snapshot | Set-Content -Encoding utf8 (Join-Path $directory 'save-failure.txt')
        UI screenshot (Join-Path $directory 'save-failure.png') | Out-Null
    } finally {
        Remove-Item -LiteralPath $configPath
        Move-Item -LiteralPath "$configPath.bak" -Destination $configPath
    }
    UI press key=close-session-cancel --settle | Out-Null
    if ((Snapshot) -notmatch "key=close-tab-$script:tabKey\b") { throw 'Save failure closed the session' }
    Close-Tab
    UI press key=close-session-accept --settle | Out-Null
    Assert-Closed
    Assert-Saved $false
    Stop-App
    Write-Output "PASS: cancel/Escape, one-time close, remember/restart, settings restore, save failure and quit confirmation. Artifacts: $directory"
} finally {
    if ($appProcess) { if (!$appProcess.HasExited) { $appProcess.Kill(); $appProcess.WaitForExit() }; $appProcess.Dispose() }
    if ($serverProcess) { if (!$serverProcess.HasExited) { $serverProcess.StandardInput.WriteLine('quit'); if (!$serverProcess.WaitForExit(3000)) { $serverProcess.Kill() } }; $serverProcess.Dispose() }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
    Pop-Location
}
