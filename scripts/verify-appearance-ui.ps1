param([string]$BuildDirectory = 'build/appearance')
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/ui-helpers.ps1"
$directory = Join-Path $root ('artifacts/appearance-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
$dataDir = Join-Path $directory 'data'
New-Item -ItemType Directory -Force $dataDir | Out-Null
$cli = Join-Path $root 'build/tools/mygo-automation.exe'
$socket = Join-Path $directory 'app.sock'
$previousSession = $env:MYGO_AUTOMATION_SESSION
$env:MYGO_AUTOMATION_SESSION = 'sshub-appearance-' + [guid]::NewGuid().ToString('N')
$process = $null
try {
    @{version=1;preferences=@{theme='dark';font_family='Consolas';font_size=13;ui_font_family='Microsoft YaHei UI';ui_font_size=20};profiles=@()} | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 (Join-Path $dataDir 'connections.json')
    @{version=1;root=@{id=1;tabs=@();active=0};active_pane=1;next_id=2;left_size=260;content_percent=66;hide_left=$false;hide_right=$true;window_width=900;window_height=620} | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 (Join-Path $dataDir 'workspace.json')
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = Join-Path $root "$BuildDirectory/windows-amd64/SSHub.exe"
    $start.UseShellExecute = $false; $start.CreateNoWindow = $true
    $start.Environment['MYGO_AUTOMATION'] = $socket
    $start.ArgumentList.Add('-data-dir'); $start.ArgumentList.Add($dataDir)
    $process = [Diagnostics.Process]::Start($start)
    $timer = [Diagnostics.Stopwatch]::StartNew()
    while (!(Test-Path $socket)) { if ($process.HasExited -or $timer.Elapsed.TotalSeconds -gt 12) { throw 'App did not start' }; Start-Sleep -Milliseconds 50 }
    Invoke-RawUI open --socket $socket | Out-Null
    Invoke-RawUI press key=app-menu --settle | Out-Null
    if ((Get-ProductSnapshot) -match 'key="{toggle-') { throw 'Panel toggles still in menu' }
    Invoke-RawUI press 'key={settings}' --settle | Out-Null
    Invoke-RawUI screenshot (Join-Path $directory 'settings-900-dark-20.png') | Out-Null
    Invoke-RawUI fill key=settings-ui-size '0' | Out-Null
    Invoke-RawUI press key=settings-save --settle | Out-Null
    if (!(Get-ProductSnapshot).Contains('10–20 的整数')) { throw 'Invalid UI size accepted' }
    Invoke-RawUI fill key=settings-ui-size '20' | Out-Null
    Invoke-RawUI act key=settings-reset-tabs scrollIntoView --settle | Out-Null
    Invoke-RawUI screenshot (Join-Path $directory 'settings-scrolled-900.png') | Out-Null
    Invoke-RawUI press key=settings-cancel --settle | Out-Null
    Invoke-RawUI press key=new-profile --settle | Out-Null
    Invoke-RawUI screenshot (Join-Path $directory 'new-connection-900-dark-20.png') | Out-Null
    Invoke-RawUI fill key=profile-name '字体检查' | Out-Null
    Invoke-RawUI fill key=profile-host 'example.com' | Out-Null
    Invoke-RawUI fill key=profile-user 'test' | Out-Null
    Invoke-RawUI press key=save-profile --settle | Out-Null
    Invoke-RawUI wait --text '字体检查' --settle | Out-Null
    Invoke-RawUI press key=toggle-connections --settle | Out-Null
    if ((Get-ProductSnapshot).Contains('key=search-profiles')) { throw 'Connection toggle failed' }
    Invoke-RawUI press key=toggle-connections --settle | Out-Null
    Invoke-RawUI press key=toggle-files --settle | Out-Null
    Invoke-RawUI wait --text '连接后浏览远程文件' --settle | Out-Null
    Invoke-RawUI screenshot (Join-Path $directory 'panels-900-dark-20.png') | Out-Null
    Invoke-RawUI press key=app-menu --settle | Out-Null
    Invoke-RawUI press 'key={settings}' --settle | Out-Null
    Invoke-RawUI fill key=settings-ui-size '13' | Out-Null
    Invoke-RawUI press key=theme-light | Out-Null
    Invoke-RawUI press key=settings-save --settle | Out-Null
    $closeNotice = [regex]::Match((Get-ProductSnapshot), '(?m)^\s*(@e\d+) \[button\] "关闭通知"').Groups[1].Value
    if (!$closeNotice) { throw 'Saved notification close button missing' }
    Invoke-RawUI press $closeNotice --settle | Out-Null
    Invoke-RawUI press key=app-menu --settle | Out-Null
    Invoke-RawUI press 'key={settings}' --settle | Out-Null
    Invoke-RawUI screenshot (Join-Path $directory 'settings-900-light-13.png') | Out-Null
    Invoke-RawUI press key=settings-cancel --settle | Out-Null
    $prefs = (Get-Content -Raw (Join-Path $dataDir 'connections.json') | ConvertFrom-Json).preferences
    if ($prefs.font_family -ne 'Consolas' -or $prefs.font_size -ne 13 -or $prefs.ui_font_size -ne 13) { throw 'UI settings modified terminal font' }
    Invoke-RawUI press key=window-close | Out-Null
    if (!$process.WaitForExit(5000) -or $process.ExitCode -ne 0) { throw 'App did not exit cleanly' }
    Write-Output "PASS: separate fonts, GitHub themes, panel toggles, forms at 900x620 and UI size 20. Artifacts: $directory"
} finally {
    if ($process) { if (!$process.HasExited) { $process.Kill(); $process.WaitForExit() }; $process.Dispose() }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
}
