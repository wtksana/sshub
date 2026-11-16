#requires -Version 7.5
param(
    [string]$Executable = 'build/windows-amd64/SSHub.exe',
    [ValidateRange(1, 30)][int]$Repeats = 3,
    [ValidateRange(12, 60)][int]$ArmSeconds = 15
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$exe = [IO.Path]::GetFullPath((Join-Path $root $Executable))
$directory = Join-Path $root ('artifacts/startup-interaction/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
New-Item -ItemType Directory -Force $directory | Out-Null
$profiles = @(0..99 | ForEach-Object {
    @{id="startup-$_";name="服务器-$_";host='192.0.2.1';port=22;username='test';auth='agent';favorite=$false;group='测试'}
})
$hash = (Get-FileHash $exe).Hash
$rows = @()
@{directory=$directory;exe=$exe;repeats=$Repeats} | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path (Split-Path $directory) 'active.json')
for ($i = 0; $i -lt $Repeats; $i++) {
    $trial = Join-Path $directory "trial-$i"
    $dataDir = Join-Path $trial 'data'
    New-Item -ItemType Directory -Force $dataDir | Out-Null
    @{version=1;profiles=$profiles} | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $dataDir 'connections.json')
    $diagnostics = Join-Path $trial 'diagnostics.jsonl'
    $observationPath = Join-Path $trial 'observation.json'
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = $exe; $start.UseShellExecute = $false; $start.CreateNoWindow = $true
    $start.Environment['SSHUB_DIAGNOSTICS'] = $diagnostics
    $start.ArgumentList.Add('-data-dir'); $start.ArgumentList.Add($dataDir)
    Write-Host "Trial $i armed for $ArmSeconds seconds. Observe native Ctrl+B sidebar response; write reviewed observation.json with observation_completed_at and sidebar_hidden=true to $trial"
    Start-Sleep -Seconds $ArmSeconds
    $launched = [DateTimeOffset]::UtcNow
    $process = [Diagnostics.Process]::Start($start)
    try {
        @{launched_at=$launched.ToString('o');process_id=$process.Id;exe_sha256=$hash} | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $trial 'launch.json')
        $timer = [Diagnostics.Stopwatch]::StartNew()
        while (!(Test-Path $observationPath)) {
            if ($process.HasExited -or $timer.Elapsed.TotalSeconds -gt 120) { throw 'No reviewed native input observation' }
            Start-Sleep -Milliseconds 100
        }
        $observation = Get-Content -Raw $observationPath | ConvertFrom-Json -DateKind String
        if ($observation.reviewed -ne $true -or $observation.sidebar_hidden -ne $true) { throw 'Native sidebar response was not visually confirmed' }
        $observed = [DateTimeOffset]::Parse($observation.observation_completed_at)
        $duration = ($observed - $launched).TotalMilliseconds
        if ($duration -le 0) { throw 'Invalid observation timestamp' }
        $samples = @(Get-Content $diagnostics | ForEach-Object { $_ | ConvertFrom-Json -DateKind String })
        if (!$samples.Count -or @($samples | Where-Object connected -ne 0).Count) { throw 'Missing diagnostics or unexpected network connection' }
        $rows += [pscustomobject]@{trial=$i;launch_to_observed_response_ms=$duration;exe_sha256=$hash}
        $process.CloseMainWindow() | Out-Null
        if (!$process.WaitForExit(5000) -or $process.ExitCode -ne 0) { throw 'Startup interaction trial failed to close' }
    } finally {
        if (!$process.HasExited) { $process.Kill($true); $process.WaitForExit() }
        $process.Dispose()
    }
}
$rows | Export-Csv -NoTypeInformation -Encoding utf8 (Join-Path $directory 'samples.csv')
$summary = @{
    at=(Get-Date).ToString('o');exe_sha256=$hash;trials=$Repeats;profiles=100
    max_observed_response_ms=($rows | Measure-Object launch_to_observed_response_ms -Maximum).Maximum
    mean_observed_response_ms=($rows | Measure-Object launch_to_observed_response_ms -Average).Average
    note='Conservative process-launch to screenshot observation completion; includes native keyboard automation and capture overhead. Requires visual confirmation of sidebar hidden by Ctrl+B. Cached process starts, not reboot cold start or hardware presentation latency.'
}
$summary | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $directory 'summary.json')
$summary | ConvertTo-Json
if ($summary.max_observed_response_ms -gt 1000) { throw 'Observed upper bound exceeds 1 second; target remains unproven, not a measured product-only latency' }
