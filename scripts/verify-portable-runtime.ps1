#requires -Version 5.1
param(
    [string]$ProgramDirectory = $PSScriptRoot,
    [string]$OutputDirectory = (Join-Path ([IO.Path]::GetTempPath()) ('SSHub-runtime-' + [guid]::NewGuid().ToString('N')))
)
$ErrorActionPreference = 'Stop'
$program = (Resolve-Path -LiteralPath $ProgramDirectory).Path
$output = [IO.Path]::GetFullPath($OutputDirectory)
if ($output.StartsWith($program + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Output must be outside the package directory' }
if (Test-Path -LiteralPath $output) { throw 'Output directory already exists; choose a fresh directory' }
$utf8 = [Text.UTF8Encoding]::new($false)
foreach ($name in @('SSHub.exe','ghostty-vt.dll','build-info.json','SHA256SUMS.txt','licenses')) {
    if (!(Test-Path -LiteralPath (Join-Path $program $name))) { throw "Missing package resource: $name" }
}
$build = Get-Content -Raw -Encoding UTF8 -LiteralPath (Join-Path $program 'build-info.json') | ConvertFrom-Json
if ($build.go_build_info -notmatch 'mygo_noinspector' -or $build.go_build_info -match '-tags=.*e2e') { throw 'Expected a release build without automation' }
$checkedFiles = @()
foreach ($line in Get-Content -Encoding UTF8 -LiteralPath (Join-Path $program 'SHA256SUMS.txt')) {
    if ($line -notmatch '^([0-9a-f]{64})  (.+)$') { throw 'Invalid checksum record' }
    $expected, $relative = $Matches[1], $Matches[2]
    $path = [IO.Path]::GetFullPath((Join-Path $program $relative))
    if (!$path.StartsWith($program + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Checksum path escaped package' }
    if ((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ine $expected) { throw "Package checksum mismatch: $relative" }
    $checkedFiles += $relative
}
foreach ($required in @('SSHub.exe','ghostty-vt.dll','build-info.json')) {
    if ($checkedFiles -notcontains $required) { throw "Required file has no checksum: $required" }
}
New-Item -ItemType Directory -Path $output | Out-Null
$roaming = Join-Path $output 'roaming'
$data = Join-Path $roaming 'SSHub'
New-Item -ItemType Directory -Path $data -Force | Out-Null
$configuration = Join-Path $data 'connections.json'
$hosts = Join-Path $data 'known_hosts'
$workspace = Join-Path $data 'workspace.json'
$unicodeName = 'Runtime Unicode ' + [char]0x4e2d + [char]0x6587
@{version=1;preferences=@{theme='dark';font_family='Consolas';font_size=15};profiles=@(@{id='runtime';name=$unicodeName;host='192.0.2.1';port=22;username='test';auth='agent';group='Runtime';favorite=$true})} |
    ConvertTo-Json -Depth 5 | ForEach-Object { [IO.File]::WriteAllText($configuration, $_, $utf8) }
@{version=1;root=@{id=1;tabs=@(@{id=2;profile_id='runtime';font_size=17});active=2};active_pane=1;next_id=3;left_size=250;content_percent=70;hide_left=$false;hide_right=$true;window_width=1100;window_height=700} |
    ConvertTo-Json -Depth 7 | ForEach-Object { [IO.File]::WriteAllText($workspace, $_, $utf8) }
[IO.File]::WriteAllText($hosts, "# isolated runtime verification`n", $utf8)
$configurationHash = (Get-FileHash -LiteralPath $configuration).Hash
$hostsHash = (Get-FileHash -LiteralPath $hosts).Hash
$toolsFound = @('go','node','python','python3' | ForEach-Object {
    $command = Get-Command $_ -CommandType Application -ErrorAction SilentlyContinue
    if ($command) { $command | ForEach-Object { @{name=$_.Name;path=$_.Source} } }
})
function Run-Package([string]$directory, [string]$label) {
    $exe = Join-Path $directory 'SSHub.exe'
    $diagnostics = Join-Path $output ($label + '.jsonl')
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = $exe; $start.WorkingDirectory = $directory
    $start.UseShellExecute = $false; $start.CreateNoWindow = $true
    $start.EnvironmentVariables['PATH'] = "$env:SystemRoot\System32;$env:SystemRoot"
    $start.EnvironmentVariables['APPDATA'] = $roaming
    $start.EnvironmentVariables['SSHUB_DIAGNOSTICS'] = $diagnostics
    foreach ($name in @('GOROOT','GOPATH','GOTOOLCHAIN','GOFLAGS','MYGO_AUTOMATION','MYGO_AUTOMATION_SESSION','SSH_AUTH_SOCK')) { $start.EnvironmentVariables.Remove($name) }
    $process = [Diagnostics.Process]::Start($start)
    try {
        $timer = [Diagnostics.Stopwatch]::StartNew()
        $ready = $null
        while ($true) {
            if ($process.HasExited -or $timer.Elapsed.TotalSeconds -gt 20) { throw "Package did not become ready: $label" }
            if (Test-Path -LiteralPath $diagnostics) {
                foreach ($line in @(Get-Content -Encoding UTF8 -LiteralPath $diagnostics)) {
                    if ($line) {
                        try { $sample = $line | ConvertFrom-Json } catch { continue }
                        if ($sample.connected -ne 0) { throw 'Restored package unexpectedly connected' }
                        if ($sample.ready_at -and $sample.ready_at -notlike '0001*') { $ready = $sample }
                    }
                }
                if ($ready) { break }
            }
            Start-Sleep -Milliseconds 50
        }
        if (!$process.CloseMainWindow() -or !$process.WaitForExit(5000) -or $process.ExitCode -ne 0) { throw "Package did not exit cleanly: $label" }
        if ((Get-FileHash -LiteralPath $configuration).Hash -ne $configurationHash -or (Get-FileHash -LiteralPath $hosts).Hash -ne $hostsHash) { throw 'Configuration or trust changed' }
        $layout = Get-Content -Raw -Encoding UTF8 -LiteralPath $workspace | ConvertFrom-Json
        if ($layout.root.tabs[0].profile_id -ne 'runtime' -or $layout.root.tabs[0].font_size -ne 17 -or !$layout.hide_right) { throw 'Workspace was not preserved' }
        return @{label=$label;exe_sha256=(Get-FileHash -LiteralPath $exe).Hash;clean_exit=$true;configuration_preserved=$true;trust_preserved=$true;workspace_preserved=$true;connected=$ready.connected;child_path=$start.EnvironmentVariables['PATH']}
    } finally {
        if (!$process.HasExited) { $process.Kill(); $process.WaitForExit() }
        $process.Dispose()
    }
}
$runs = @(Run-Package $program 'original')
$replacement = Join-Path $output 'replacement'
Copy-Item -LiteralPath $program -Destination $replacement -Recurse
$runs += Run-Package $replacement 'replacement'
$os = Get-CimInstance Win32_OperatingSystem
$summary = @{
    passed=$true;at=[DateTimeOffset]::Now.ToString('o');powershell=$PSVersionTable.PSVersion.ToString()
    os=@{caption=$os.Caption;version=$os.Version;architecture=$os.OSArchitecture;last_boot=$os.LastBootUpTime.ToUniversalTime().ToString('o')}
    checked_file_count=$checkedFiles.Count;runs=$runs;discoverable_development_tools=$toolsFound
    limitation='Records this Windows environment and discoverable tools; PATH isolation alone does not establish a clean VM. Ready/show is not input responsiveness. This script does not test SSH, terminal DLL loading, system drag-and-drop, or reboot cold startup.'
}
[IO.File]::WriteAllText((Join-Path $output 'summary.json'), ($summary | ConvertTo-Json -Depth 8), $utf8)
Write-Output "PASS: package hashes, isolated runtime, normal exit and replacement. Evidence: $output"
