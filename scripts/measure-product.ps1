#requires -Version 7.5
param([int]$Seconds = 600, [ValidateSet(1,8)][int]$Sessions = 8, [switch]$SkipBuild, [string]$Executable = 'build/windows-amd64/SSHub.exe')
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
$appProcess = $null
$serverProcess = $null
try {
    if (!$SkipBuild) { & "$PSScriptRoot/build.ps1" }
    go build -tags e2e -o build/tools/ssh-fixture.exe ./internal/testssh/cmd
    if ($LASTEXITCODE -ne 0) { throw 'Fixture build failed' }
    $directory = Join-Path $root ('artifacts/product-performance/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $dataDir = Join-Path $directory 'data'
    $serverDir = Join-Path $directory 'server'
    New-Item -ItemType Directory -Force $dataDir,$serverDir | Out-Null
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = Join-Path $root 'build/tools/ssh-fixture.exe'
    $start.UseShellExecute = $false; $start.CreateNoWindow = $true
    $start.RedirectStandardInput = $true
    $start.ArgumentList.Add('-dir'); $start.ArgumentList.Add($serverDir)
    $serverProcess = [Diagnostics.Process]::Start($start)
    $timer = [Diagnostics.Stopwatch]::StartNew()
    $metadata = Join-Path $serverDir 'server.json'
    while (!(Test-Path $metadata)) {
        if ($serverProcess.HasExited -or $timer.Elapsed.TotalSeconds -gt 12) { throw 'Fixture unavailable' }
        Start-Sleep -Milliseconds 100
    }
    $server = Get-Content -Raw $metadata | ConvertFrom-Json
    $portNumber = [int]$server.address.Split(':')[-1]
    $profiles = @(0..99 | ForEach-Object {
        @{id="perf-$_";name=('性能服务器-{0:d3}' -f $_);host='127.0.0.1';port=$portNumber;username='test';auth='private_key';identity_path=$server.identity;group=$(if ($_ % 2) {'生产'} else {'测试'});favorite=($_ % 10 -eq 0)}
    })
    @{version=1;profiles=$profiles} | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 (Join-Path $dataDir 'connections.json')
    # Explicitly pinned public key from the isolated fixture, never user servers.
    $server.known_host | Set-Content -Encoding utf8 (Join-Path $dataDir 'known_hosts')
    if ($Sessions -eq 8) {
        $panes = @(0..3 | ForEach-Object { $n=$_; @{id=10+$n;tabs=@(@{id=20+2*$n;profile_id="perf-$($n*2)";font_size=13},@{id=21+2*$n;profile_id="perf-$($n*2+1)";font_size=13});active=21+2*$n} })
        $tree = @{id=1;axis='horizontal';ratio=50;first=@{id=2;axis='vertical';ratio=50;first=$panes[0];second=$panes[1]};second=@{id=3;axis='vertical';ratio=50;first=$panes[2];second=$panes[3]}}
    } else { $tree=@{id=10;tabs=@(@{id=20;profile_id='perf-0';font_size=13});active=20} }
    @{version=1;root=$tree;active_pane=10;next_id=40;left_size=230;content_percent=76;hide_left=$false;hide_right=$false;window_width=1280;window_height=800} |
        ConvertTo-Json -Depth 10 | Set-Content -Encoding utf8 (Join-Path $dataDir 'workspace.json')
    $exe = [IO.Path]::GetFullPath((Join-Path $root $Executable))
    $diagnosticPath = Join-Path $directory 'diagnostics.jsonl'
    $environment = @{at=(Get-Date).ToString('o');commit=(git rev-parse HEAD);exe_sha256=(Get-FileHash $exe).Hash;dll_sha256=(Get-FileHash (Join-Path (Split-Path $exe) 'ghostty-vt.dll')).Hash;go=(go version);os=(Get-CimInstance Win32_OperatingSystem | Select-Object Caption,Version,TotalVisibleMemorySize);cpu=(Get-CimInstance Win32_Processor | Select-Object Name,NumberOfLogicalProcessors);sessions=$Sessions;seconds=$Seconds;window_dip='1280x800';font='Cascadia Mono 13';scrollback_bytes=2097152;line_bytes=101;lines_per_second_per_session=1000;visible_panes=$(if ($Sessions -eq 8) {4} else {1})}
    $environment | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 (Join-Path $directory 'environment.json')
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName=$exe; $start.UseShellExecute=$false; $start.CreateNoWindow=$true
    $start.Environment['SSHUB_DIAGNOSTICS']=$diagnosticPath
    $start.ArgumentList.Add('-data-dir'); $start.ArgumentList.Add($dataDir)
    $launched = [DateTimeOffset]::Now
    $appProcess=[Diagnostics.Process]::Start($start)
    @{directory=$directory;pid=$appProcess.Id;sessions=$Sessions;status='awaiting_connections'} | ConvertTo-Json | Set-Content -Encoding utf8 'artifacts/product-performance/active.json'
    Write-Host "Connect $Sessions restored tabs using the actual release UI; fixture key is pinned. $directory"
    $timer.Restart()
    $phase='connecting';$phaseStarted=[DateTimeOffset]::Now
    $lastCPU=$appProcess.TotalProcessorTime.TotalSeconds;$lastAt=[DateTimeOffset]::Now
    $samples=[Collections.Generic.List[object]]::new()
    while ($true) {
        if ($appProcess.HasExited) {throw 'Application exited before measurement completed'}
        $appProcess.Refresh()
        $now=[DateTimeOffset]::Now
        $cpu=$appProcess.TotalProcessorTime.TotalSeconds
        $latest=$null
        if (Test-Path $diagnosticPath) {
            $lines=@(Get-Content $diagnosticPath -Tail 1)
            if ($lines.Count -gt 0) { try {$latest=$lines[-1] | ConvertFrom-Json -DateKind String} catch {} }
        }
        $received=0
        if ($latest) {foreach ($bytes in $latest.received_bytes) {$received += [long]$bytes}}
        $sample=[pscustomobject]@{at=$now.ToString('o');elapsed_seconds=($now-$launched).TotalSeconds;phase=$phase;phase_seconds=($now-$phaseStarted).TotalSeconds;working_set_bytes=$appProcess.WorkingSet64;private_bytes=$appProcess.PrivateMemorySize64;cpu_one_core_percent=100*($cpu-$lastCPU)/[Math]::Max(.001,($now-$lastAt).TotalSeconds);connected=$(if ($latest){$latest.connected}else{0});received_bytes=$received;heap_bytes=$(if($latest){$latest.heap_bytes}else{0});goroutines=$(if($latest){$latest.goroutines}else{0})}
        $samples.Add($sample)
        $samples | Select-Object -Last 1 | Export-Csv -NoTypeInformation -Append -Encoding utf8 (Join-Path $directory 'samples.csv')
        $lastCPU=$cpu;$lastAt=$now
        if ($phase -eq 'connecting' -and $latest -and $latest.connected -eq $Sessions) {
            $phase='idle';$phaseStarted=$now
            Write-Host "All $Sessions connected; idle sampling for 15 seconds"
        } elseif ($phase -eq 'idle' -and ($now-$phaseStarted).TotalSeconds -ge 15) {
            if ($Sessions -eq 1) {break}
            $serverProcess.StandardInput.WriteLine('load on');$serverProcess.StandardInput.Flush()
            $phase='load';$phaseStarted=$now
            Write-Host "Started $Seconds second load"
            @{directory=$directory;pid=$appProcess.Id;sessions=$Sessions;status='load';started=$now.ToString('o')} | ConvertTo-Json | Set-Content -Encoding utf8 'artifacts/product-performance/active.json'
        } elseif ($phase -eq 'load' -and ($now-$phaseStarted).TotalSeconds -ge $Seconds) {break}
        if ($phase -eq 'connecting' -and $timer.Elapsed.TotalSeconds -gt 600) {throw 'Timed out awaiting release UI connections'}
        Start-Sleep -Milliseconds 1000
    }
    $serverProcess.StandardInput.WriteLine('load off');$serverProcess.StandardInput.Flush()
    $phaseRows=@($samples | Where-Object phase -eq $(if($Sessions -eq 1){'idle'}else{'load'}))
    $firstDiag=Get-Content $diagnosticPath | ForEach-Object {$_ | ConvertFrom-Json -DateKind String} | Where-Object { $_.ready_at -and $_.ready_at -notlike '0001*' } | Select-Object -First 1
    $summary=@{directory=$directory;startup_show_ms=([DateTimeOffset]::Parse($firstDiag.ready_at)-$launched).TotalMilliseconds;sessions=$Sessions;duration_seconds=($phaseRows[-1].phase_seconds);max_working_set_mib=($phaseRows | Measure-Object working_set_bytes -Maximum).Maximum/1MB;mean_working_set_mib=($phaseRows | Measure-Object working_set_bytes -Average).Average/1MB;max_private_mib=($phaseRows | Measure-Object private_bytes -Maximum).Maximum/1MB;mean_cpu_one_core_percent=($phaseRows | Measure-Object cpu_one_core_percent -Average).Average;received_delta_bytes=$phaseRows[-1].received_bytes-$phaseRows[0].received_bytes;received_per_session_line_rate=($phaseRows[-1].received_bytes-$phaseRows[0].received_bytes)/101/$Sessions/($phaseRows[-1].phase_seconds-$phaseRows[0].phase_seconds)}
    $summary | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $directory 'summary.json')
    $summary | ConvertTo-Json -Depth 5 | Write-Host
    # Stop the loopback server so release exits without an online-session modal.
    $serverProcess.StandardInput.WriteLine('quit');$serverProcess.StandardInput.Flush()
    $serverProcess.WaitForExit(5000) | Out-Null
    Start-Sleep -Milliseconds 500
    $appProcess.CloseMainWindow() | Out-Null
    if (!$appProcess.WaitForExit(5000) -or $appProcess.ExitCode -ne 0) {throw 'Release did not exit cleanly'}
    @{directory=$directory;status='complete'} | ConvertTo-Json | Set-Content -Encoding utf8 'artifacts/product-performance/active.json'
} finally {
    if ($serverProcess) {if(!$serverProcess.HasExited){$serverProcess.StandardInput.Close();if(!$serverProcess.WaitForExit(2000)){$serverProcess.Kill($true)}};$serverProcess.Dispose()}
    if ($appProcess) {if(!$appProcess.HasExited){$appProcess.Kill($true);$appProcess.WaitForExit()};$appProcess.Dispose()}
    Pop-Location
}
