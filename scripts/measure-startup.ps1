#requires -Version 7.5
param([int]$Repeats=10,[string]$Executable='build/windows-amd64/SSHub.exe')
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
$exe=[IO.Path]::GetFullPath((Join-Path $root $Executable))
$directory=Join-Path $root ('artifacts/startup/'+(Get-Date -Format 'yyyyMMdd-HHmmss'))
$dataDir=Join-Path $directory 'data'
New-Item -ItemType Directory -Force $dataDir | Out-Null
$profiles=@(0..99|ForEach-Object {@{id="startup-$_";name="服务器-$_";host='192.0.2.1';port=22;username='test';auth='agent';favorite=$false;group='测试'}})
@{version=1;profiles=$profiles}|ConvertTo-Json -Depth 5|Set-Content -Encoding utf8 (Join-Path $dataDir 'connections.json')
$rows=@()
for($i=0;$i -lt $Repeats;$i++){
    $diagnostics=Join-Path $directory "startup-$i.jsonl"
    $start=[Diagnostics.ProcessStartInfo]::new()
    $start.FileName=$exe;$start.UseShellExecute=$false;$start.CreateNoWindow=$true
    $start.Environment['SSHUB_DIAGNOSTICS']=$diagnostics
    $start.ArgumentList.Add('-data-dir');$start.ArgumentList.Add($dataDir)
    $launched=[DateTimeOffset]::Now
    $process=[Diagnostics.Process]::Start($start)
    try{
        $timer=[Diagnostics.Stopwatch]::StartNew()
        $sample=$null
        while($true){
            if($process.HasExited -or $timer.Elapsed.TotalSeconds -gt 10){throw 'No first-frame diagnostic'}
            if(Test-Path $diagnostics){
                $line=Get-Content $diagnostics -Tail 1
                if($line){try{$sample=$line|ConvertFrom-Json -DateKind String}catch{}}
                if($sample -and $sample.ready_at -notlike '0001*'){break}
            }
            Start-Sleep -Milliseconds 10
        }
        $rows += [pscustomobject]@{trial=$i;ready_show_ms=([DateTimeOffset]::Parse($sample.ready_at)-$launched).TotalMilliseconds;observed_ready_ms=$timer.Elapsed.TotalMilliseconds;connected=$sample.connected;exe_sha256=(Get-FileHash $exe).Hash}
        if($sample.connected -ne 0){throw 'Startup unexpectedly connected to network'}
        $process.CloseMainWindow()|Out-Null
        if(!$process.WaitForExit(5000) -or $process.ExitCode -ne 0){throw 'Startup trial failed to close'}
    }finally{
        if(!$process.HasExited){$process.Kill($true);$process.WaitForExit()};$process.Dispose()
    }
}
$rows|Export-Csv -NoTypeInformation -Encoding utf8 (Join-Path $directory 'samples.csv')
@{at=(Get-Date).ToString('o');exe=$exe;exe_sha256=(Get-FileHash $exe).Hash;trials=$Repeats;max_show_ms=($rows|Measure-Object ready_show_ms -Maximum).Maximum;mean_show_ms=($rows|Measure-Object ready_show_ms -Average).Average;note='System caches retained; process launch to ready-to-show after first frame; not reboot cold start or hardware presentation latency'}|ConvertTo-Json|Set-Content -Encoding utf8 (Join-Path $directory 'summary.json')
Get-Content (Join-Path $directory 'summary.json')
if(($rows|Measure-Object ready_show_ms -Maximum).Maximum -gt 1000){throw 'Cached startup exceeds 1 second target'}
