param(
    [Parameter(Mandatory)][string]$QemuDirectory,
    [Parameter(Mandatory)][string]$Image,
    [Parameter(Mandatory)][string]$ExpectedImageSHA512,
    [Parameter(Mandatory)][string]$PythonModulesDirectory,
    [string]$Python = 'python'
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$qemu = (Resolve-Path -LiteralPath (Join-Path $QemuDirectory 'qemu-system-x86_64.exe')).Path
$imagePath = (Resolve-Path -LiteralPath $Image).Path
$modules = (Resolve-Path -LiteralPath $PythonModulesDirectory).Path
if ((Get-FileHash -LiteralPath $imagePath -Algorithm SHA512).Hash -ine $ExpectedImageSHA512) { throw 'Linux image SHA512 mismatch' }
$directory = Join-Path $root ('artifacts/linux-fixture/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
New-Item -ItemType Directory -Force $directory | Out-Null
ssh-keygen -q -t ed25519 -N '' -f (Join-Path $directory 'user_ed25519')
if ($LASTEXITCODE -ne 0) { throw 'Temporary key generation failed' }
$seed = [Diagnostics.ProcessStartInfo]::new()
$seed.FileName = $Python; $seed.UseShellExecute = $false; $seed.CreateNoWindow = $true
$seed.Environment['PYTHONPATH'] = $modules
$seed.ArgumentList.Add((Join-Path $PSScriptRoot 'linux-fixture-seed.py')); $seed.ArgumentList.Add($directory)
$seedProcess = [Diagnostics.Process]::Start($seed)
try { $seedProcess.WaitForExit(); if ($seedProcess.ExitCode -ne 0) { throw 'NoCloud ISO generation failed' } } finally { $seedProcess.Dispose() }
function Free-Port {
    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    try { $listener.Start(); return $listener.LocalEndpoint.Port } finally { $listener.Stop() }
}
$port = Free-Port
$qmpPort = Free-Port
while ($qmpPort -eq $port) { $qmpPort = Free-Port }
$serial = Join-Path $directory 'serial.log'
$start = [Diagnostics.ProcessStartInfo]::new()
$start.FileName = $qemu; $start.UseShellExecute = $false; $start.CreateNoWindow = $true
foreach ($arg in @(
    '-L',(Join-Path (Split-Path $qemu) 'share'),'-accel','tcg,thread=multi','-machine','q35',
    '-smbios','type=1,serial=ds=nocloud','-m','512','-smp','2','-display','none',
    '-serial',"file:$serial",'-drive',"file=$imagePath,if=virtio,snapshot=on",'-cdrom',(Join-Path $directory 'seed.iso'),
    '-nic',"user,model=virtio-net-pci,restrict=on,hostfwd=tcp:127.0.0.1:$port-:22",
    '-qmp',"tcp:127.0.0.1:$qmpPort,server=on,wait=off"
)) { $start.ArgumentList.Add($arg) }
$process = [Diagnostics.Process]::Start($start)
try {
    $timer = [Diagnostics.Stopwatch]::StartNew()
    do {
        if ($process.HasExited -or $timer.Elapsed.TotalSeconds -gt 120) { throw "Linux guest did not bootstrap; inspect $serial" }
        Start-Sleep -Milliseconds 250
        $log = if (Test-Path $serial) { Get-Content -Raw $serial } else { '' }
    } until ($log.Contains('SSHUB_LINUX_READY'))
    $fingerprints = @([regex]::Matches($log, 'SHA256:[A-Za-z0-9+/]+') | ForEach-Object Value | Sort-Object -Unique)
    if (!$fingerprints.Count) { throw 'Guest did not publish host fingerprints' }
    $metadata = @{process_id=$process.Id;port=$port;qmp_port=$qmpPort;directory=$directory;identity=(Join-Path $directory 'user_ed25519');host_fingerprints=$fingerprints;image_sha512=$ExpectedImageSHA512;qemu_sha256=(Get-FileHash $qemu).Hash}
    $metadata | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $directory 'active.json')
    $metadata | ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path (Split-Path $directory) 'active.json')
    $metadata | ConvertTo-Json
} catch {
    if (!$process.HasExited) { $process.Kill($true); $process.WaitForExit() }
    throw
} finally { $process.Dispose() }
