param([string]$Metadata = 'artifacts/linux-fixture/active.json')
$ErrorActionPreference = 'Stop'
$fixture = Get-Content -Raw -LiteralPath $Metadata | ConvertFrom-Json
$process = Get-Process -Id $fixture.process_id -ErrorAction Stop
if ($process.ProcessName -ne 'qemu-system-x86_64') { throw 'Fixture PID no longer identifies QEMU; refusing to stop it' }
$owners = @(Get-NetTCPConnection -LocalPort $fixture.qmp_port -State Listen -ErrorAction Stop | ForEach-Object OwningProcess)
if ($owners -notcontains $process.Id) { throw 'QMP listener does not belong to the recorded fixture process' }
$client = [Net.Sockets.TcpClient]::new()
try {
    $client.Connect([Net.IPAddress]::Loopback, [int]$fixture.qmp_port)
    $stream = $client.GetStream(); $stream.ReadTimeout = 5000
    $reader = [IO.StreamReader]::new($stream)
    $writer = [IO.StreamWriter]::new($stream); $writer.AutoFlush = $true
    $greeting = $reader.ReadLine() | ConvertFrom-Json
    if (!$greeting.QMP) { throw 'Expected a QMP greeting' }
    $writer.WriteLine('{"execute":"qmp_capabilities"}')
    $capabilities = $reader.ReadLine() | ConvertFrom-Json
    if ($null -eq $capabilities.return) { throw 'QMP negotiation failed' }
    $writer.WriteLine('{"execute":"quit"}')
    if (!$process.WaitForExit(5000)) { throw 'QEMU did not stop after QMP quit' }
    Write-Output 'Stopped isolated Linux guest; temporary snapshot discarded'
} finally { $client.Dispose(); $process.Dispose() }
