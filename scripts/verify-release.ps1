#requires -Version 7.5
param([Parameter(Mandatory)][string]$Archive, [string]$PreviousExecutable)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$archivePath = (Resolve-Path -LiteralPath $Archive).Path
$archiveSums = Join-Path (Split-Path -Parent $archivePath) 'SHA256SUMS.txt'
if (!(Test-Path -LiteralPath $archiveSums)) { throw 'Archive checksum file missing' }
$archiveRecord = @(Get-Content -LiteralPath $archiveSums | Where-Object { $_ -match ('  ' + [regex]::Escape([IO.Path]::GetFileName($archivePath)) + '$') })
if ($archiveRecord.Count -ne 1 -or $archiveRecord[0].Substring(0,64) -ne (Get-FileHash -LiteralPath $archivePath).Hash.ToLowerInvariant()) { throw 'Archive checksum mismatch' }
$directory = Join-Path $root ('artifacts/release-package/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
New-Item -ItemType Directory -Force $directory | Out-Null
Expand-Archive -LiteralPath $archivePath -DestinationPath (Join-Path $directory 'unpacked')
$program = @(Get-ChildItem -LiteralPath (Join-Path $directory 'unpacked') -Directory)
if ($program.Count -ne 1) { throw 'Archive must contain one application directory' }
$programDir = $program[0].FullName
foreach ($line in Get-Content -LiteralPath (Join-Path $programDir 'SHA256SUMS.txt')) {
    if ($line -notmatch '^([0-9a-f]{64})  (.+)$') { throw 'Invalid checksum record' }
    $expected, $relative = $Matches[1], $Matches[2]
    $file = [IO.Path]::GetFullPath((Join-Path $programDir $relative))
    if (!$file.StartsWith($programDir + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Checksum path escaped archive' }
    if ((Get-FileHash -LiteralPath $file).Hash.ToLowerInvariant() -ne $expected) { throw "Checksum mismatch: $relative" }
}
$roaming = Join-Path $directory 'roaming'
$dataDir = Join-Path $roaming 'SSHub'
New-Item -ItemType Directory -Force $dataDir | Out-Null
$config = Join-Path $dataDir 'connections.json'
$workspace = Join-Path $dataDir 'workspace.json'
$hosts = Join-Path $dataDir 'known_hosts'
@{version=1;preferences=@{theme='dark';font_family='Consolas';font_size=15};profiles=@(@{id='upgrade';name='升级保留 中文';host='192.0.2.1';port=22;username='test';auth='agent';group='验收';favorite=$true})} |
    ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 $config
@{version=1;root=@{id=1;tabs=@(@{id=2;profile_id='upgrade';font_size=17});active=2};active_pane=1;next_id=3;left_size=250;content_percent=70;hide_left=$false;hide_right=$true;window_width=1100;window_height=700} |
    ConvertTo-Json -Depth 7 | Set-Content -Encoding utf8 $workspace
[IO.File]::WriteAllText($hosts, "# isolated package verification`n")
$configHash = (Get-FileHash $config).Hash
$hostsHash = (Get-FileHash $hosts).Hash
$rows = @()

function Start-Isolated([string]$exe, [string]$label, [bool]$expectDiagnostics) {
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = $exe
    $start.WorkingDirectory = Split-Path -Parent $exe
    $start.UseShellExecute = $false
    $start.CreateNoWindow = $true
    $start.Environment['PATH'] = "$env:SystemRoot\System32;$env:SystemRoot"
    $start.Environment['APPDATA'] = $roaming
    $diagnostics = Join-Path $directory "$label.jsonl"
    $start.Environment['SSHUB_DIAGNOSTICS'] = $diagnostics
    foreach ($name in @('GOROOT','GOPATH','GOTOOLCHAIN','GOFLAGS','MYGO_AUTOMATION','MYGO_AUTOMATION_SESSION')) { $start.Environment.Remove($name) | Out-Null }
    $process = [Diagnostics.Process]::Start($start)
    try {
        $timer = [Diagnostics.Stopwatch]::StartNew()
        $ready = $null
        while ($true) {
            $process.Refresh()
            if ($process.HasExited -or $timer.Elapsed.TotalSeconds -gt 15) { throw "Package did not start: $label" }
            if ($expectDiagnostics -and (Test-Path $diagnostics)) {
                $line = Get-Content $diagnostics -Tail 1
                if ($line) { try { $ready = $line | ConvertFrom-Json -DateKind String } catch {} }
                if ($ready -and $ready.ready_at -notlike '0001*') { break }
            } elseif (!$expectDiagnostics -and $process.MainWindowHandle -ne 0 -and $process.Responding) { break }
            Start-Sleep -Milliseconds 50
        }
        if ($ready -and $ready.connected -ne 0) { throw 'Restored package unexpectedly connected to network' }
        if (!$process.CloseMainWindow() -or !$process.WaitForExit(5000) -or $process.ExitCode -ne 0) { throw "Package failed to close: $label" }
        if ((Get-FileHash $config).Hash -ne $configHash -or (Get-FileHash $hosts).Hash -ne $hostsHash) { throw 'User configuration or trust changed during replacement' }
        $layout = Get-Content -Raw $workspace | ConvertFrom-Json
        if ($layout.root.tabs[0].profile_id -ne 'upgrade' -or $layout.root.tabs[0].font_size -ne 17 -or !$layout.hide_right) { throw 'Workspace did not survive replacement' }
        return [pscustomobject]@{label=$label;exe=$exe;exe_sha256=(Get-FileHash $exe).Hash;clean_exit=$true;configuration_preserved=$true;layout_preserved=$true;child_path=$start.Environment['PATH'];appdata=$roaming;connected=$(if($ready){$ready.connected}else{$null})}
    } finally {
        if (!$process.HasExited) { $process.Kill($true); $process.WaitForExit() }
        $process.Dispose()
    }
}

if ($PreviousExecutable) {
    $old = (Resolve-Path -LiteralPath $PreviousExecutable).Path
    $oldDir = Join-Path $directory 'previous-program'
    New-Item -ItemType Directory $oldDir | Out-Null
    Copy-Item -LiteralPath $old -Destination (Join-Path $oldDir 'SSHub.exe')
    Copy-Item -LiteralPath (Join-Path (Split-Path -Parent $old) 'ghostty-vt.dll') -Destination $oldDir
    $rows += Start-Isolated (Join-Path $oldDir 'SSHub.exe') 'previous' $false
}
$rows += Start-Isolated (Join-Path $programDir 'SSHub.exe') 'extracted' $true
$replacement = Join-Path $directory 'replacement-program'
Copy-Item -LiteralPath $programDir -Destination $replacement -Recurse
$rows += Start-Isolated (Join-Path $replacement 'SSHub.exe') 'replaced-directory' $true
@{archive=$archivePath;archive_sha256=(Get-FileHash $archivePath).Hash;runs=$rows;limitation='Child PATH excludes development tools; Windows, fonts and user profile remain on the development machine. Not a clean VM.'} |
    ConvertTo-Json -Depth 7 | Set-Content -Encoding utf8 (Join-Path $directory 'summary.json')
Write-Host "PASS: ZIP checksums, isolated runtime, clean exit and program replacement. $directory"
