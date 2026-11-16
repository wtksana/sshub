param([string]$DataDir='')
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
$exe=Join-Path $root 'build/windows-amd64/SSHub.exe'
if (!(Test-Path -LiteralPath $exe)) { & "$PSScriptRoot/build.ps1" }
$appArgs=@()
if ($DataDir) { $appArgs+=@('-data-dir',$DataDir) }
& $exe @appArgs
if ($LASTEXITCODE -ne 0) { throw "SSHub exited ($LASTEXITCODE)" }
