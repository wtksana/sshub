#requires -Version 7.5
param(
    [Parameter(Mandatory)][string]$OpenSSHDirectory,
    [string]$BuildDirectory = 'build/p6'
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/ui-helpers.ps1"
if ((Get-Service ssh-agent).Status -ne 'Running') { throw 'System ssh-agent must already be running; this script never changes service settings' }
$toolsDir = (Resolve-Path -LiteralPath $OpenSSHDirectory).Path
$sshAdd = Join-Path $toolsDir 'ssh-add.exe'
$keygen = Join-Path $toolsDir 'ssh-keygen.exe'
foreach ($path in @($sshAdd,$keygen,(Join-Path $root 'build/tools/ssh-fixture.exe'),(Join-Path $root 'build/tools/mygo-automation.exe'),(Join-Path $root "$BuildDirectory/windows-amd64/SSHub.exe"))) {
    if (!(Test-Path -LiteralPath $path)) { throw "Missing test prerequisite: $path" }
}
function Invoke-AgentTool([string[]]$Arguments, [switch]$AllowEmpty) {
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = $sshAdd; $start.UseShellExecute = $false; $start.CreateNoWindow = $true
    $start.RedirectStandardOutput = $true; $start.RedirectStandardError = $true
    $start.Environment.Remove('SSH_AUTH_SOCK') | Out-Null
    foreach ($arg in $Arguments) { $start.ArgumentList.Add($arg) }
    $process = [Diagnostics.Process]::Start($start)
    try {
        $output = $process.StandardOutput.ReadToEndAsync()
        $errors = $process.StandardError.ReadToEndAsync()
        if (!$process.WaitForExit(10000)) { $process.Kill(); $process.WaitForExit(); throw 'System Agent tool timed out' }
        $text = $output.GetAwaiter().GetResult()
        $errorText = $errors.GetAwaiter().GetResult()
        if ($process.ExitCode -ne 0 -and !($AllowEmpty -and $process.ExitCode -eq 1 -and $text -match 'no identities')) { throw "ssh-add failed ($($process.ExitCode)): $errorText" }
        return $text
    } finally { $process.Dispose() }
}
function Agent-Fingerprints {
    $text = Invoke-AgentTool -Arguments @('-l') -AllowEmpty
    return @([regex]::Matches($text, 'SHA256:[A-Za-z0-9+/]+') | ForEach-Object Value | Sort-Object -Unique)
}
$before = @(Agent-Fingerprints)
$directory = Join-Path $root ('artifacts/system-agent-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
$dataDir = Join-Path $directory 'data'
$serverDir = Join-Path $directory 'server'
New-Item -ItemType Directory -Force $dataDir | Out-Null
$appProcess = $null
$serverProcess = $null
$keyMayHaveBeenAdded = $false
$previousSession = $env:MYGO_AUTOMATION_SESSION
try {
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = Join-Path $root 'build/tools/ssh-fixture.exe'
    $start.UseShellExecute = $false; $start.CreateNoWindow = $true; $start.RedirectStandardInput = $true
    $start.ArgumentList.Add('-dir'); $start.ArgumentList.Add($serverDir)
    $serverProcess = [Diagnostics.Process]::Start($start)
    $timer = [Diagnostics.Stopwatch]::StartNew()
    while (!(Test-Path -LiteralPath (Join-Path $serverDir 'server.json'))) {
        if ($serverProcess.HasExited -or $timer.Elapsed.TotalSeconds -gt 10) { throw 'SSH fixture did not start' }
        Start-Sleep -Milliseconds 50
    }
    $server = Get-Content -Raw -Encoding utf8 (Join-Path $serverDir 'server.json') | ConvertFrom-Json
    $publicKey = & $keygen -y -f $server.identity
    if ($LASTEXITCODE -ne 0 -or !$publicKey) { throw 'Could not derive fixture public key' }
    $publicKey | Set-Content -Encoding utf8 ($server.identity + '.pub')
    $keyFingerprint = [regex]::Match((& $keygen -lf ($server.identity + '.pub')), 'SHA256:[^ ]+').Value
    if (!$keyFingerprint -or $before -contains $keyFingerprint) { throw 'Fixture identity is missing or already loaded; refusing to change preexisting identity' }
    $hostKeyFile = Join-Path $directory 'expected-host'
    $server.known_host | Set-Content -Encoding utf8 $hostKeyFile
    $hostFingerprint = [regex]::Match((& $keygen -lf $hostKeyFile), 'SHA256:[^ ]+').Value
    if (!$hostFingerprint) { throw 'Fixture host fingerprint missing' }
    $keyMayHaveBeenAdded = $true
    Invoke-AgentTool -Arguments @($server.identity) | Out-Null
    if (@(Agent-Fingerprints) -notcontains $keyFingerprint) { throw 'Temporary key was not loaded into the system Agent' }
    [IO.File]::WriteAllText((Join-Path $serverDir 'files/Agent 中文.txt'), 'System Agent SFTP proof', [Text.UTF8Encoding]::new($false))
    @{version=1;profiles=@(@{id='system-agent';name='系统 Agent 验收';host='127.0.0.1';port=[int]($server.address.Split(':')[-1]);username='test';auth='agent';favorite=$false})} |
        ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $dataDir 'connections.json')
    $cli = Join-Path $root 'build/tools/mygo-automation.exe'
    $socket = Join-Path $directory 'app.sock'
    $env:MYGO_AUTOMATION_SESSION = 'sshub-system-agent-' + [guid]::NewGuid().ToString('N')
    function UI { return Invoke-ProductUI @args }
    function Wait-Text([string]$text) { UI wait --text $text --timeout 15s --settle | Out-Null }
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = Join-Path $root "$BuildDirectory/windows-amd64/SSHub.exe"
    $start.UseShellExecute = $false; $start.CreateNoWindow = $true
    $start.Environment['MYGO_AUTOMATION'] = $socket
    $start.Environment.Remove('SSH_AUTH_SOCK') | Out-Null
    $start.ArgumentList.Add('-data-dir'); $start.ArgumentList.Add($dataDir)
    $appProcess = [Diagnostics.Process]::Start($start)
    $timer.Restart()
    while (!(Test-Path -LiteralPath $socket)) {
        if ($appProcess.HasExited -or $timer.Elapsed.TotalSeconds -gt 12) { throw 'Product did not start' }
        Start-Sleep -Milliseconds 50
    }
    UI open --socket $socket | Out-Null
    UI click key=profile-system-agent --clicks 2 --settle | Out-Null
    Wait-Text '确认服务器身份'
    if (!(Get-ProductSnapshot).Contains($hostFingerprint)) { throw 'Product host fingerprint mismatch' }
    UI press key=accept-connection-prompt --settle | Out-Null
    Wait-Text '系统 Agent 验收 · 已连接'
    Wait-Text 'Agent 中文.txt'
    $tab = [int]([regex]::Matches((Get-ProductSnapshot), 'key=tab-(\d+)') | ForEach-Object { [int]$_.Groups[1].Value } | Measure-Object -Maximum).Maximum
    UI click "key=terminal-$tab" --settle | Out-Null
    UI type 'SSHUB_SYSTEM_AGENT_中文' --settle | Out-Null
    UI key Enter --settle | Out-Null
    UI press key=terminal-search --settle | Out-Null
    UI fill "key=terminal-query-$tab" 'SSHUB_SYSTEM_AGENT_中文' --settle | Out-Null
    Wait-Text '1 处匹配'
    UI screenshot (Join-Path $directory 'system-agent.png') | Out-Null
    UI press key=window-close --settle | Out-Null
    Wait-Text '退出 SSHub？'
    $ref = [regex]::Match((Get-ProductSnapshot), '(?m)^\s*(@e\d+) \[button\] "断开并退出"').Groups[1].Value
    if (!$ref) { throw 'Exit confirmation missing' }
    UI press $ref | Out-Null
    if (!$appProcess.WaitForExit(5000) -or $appProcess.ExitCode -ne 0) { throw 'Product did not exit cleanly' }
} finally {
    if ($appProcess) { if (!$appProcess.HasExited) { $appProcess.Kill(); $appProcess.WaitForExit() }; $appProcess.Dispose() }
    if ($serverProcess) { if (!$serverProcess.HasExited) { $serverProcess.StandardInput.WriteLine('quit'); if (!$serverProcess.WaitForExit(3000)) { $serverProcess.Kill(); $serverProcess.WaitForExit() } }; $serverProcess.Dispose() }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
    if ($keyMayHaveBeenAdded) {
        Invoke-AgentTool -Arguments @('-d',($server.identity + '.pub')) | Out-Null
        $after = @(Agent-Fingerprints)
        if ($after -contains $keyFingerprint -or ($before -join '|') -cne ($after -join '|')) { throw 'Agent identities were not restored; inspect before further changes' }
    }
}
@{passed=$true;systemAgentService=$true;defaultNamedPipe=$true;hostFingerprintMatched=$true;temporaryIdentity=$keyFingerprint;preexistingIdentityCount=$before.Count;originalIdentitySetRestored=$true;sftpUnicodeListing=$true;terminalEcho=$true;cleanExit=$true;serviceSettingsChangedByScript=$false;sshAddSHA256=(Get-FileHash $sshAdd).Hash} |
    ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $directory 'summary.json')
Write-Output "PASS: system Agent signature, SFTP, terminal and temporary key cleanup. Evidence: $directory"
