param([switch]$DebugBuild, [string]$Proxy = 'https://goproxy.cn,https://proxy.golang.org,direct', [string]$OutputDirectory = 'build')
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$previousProxy = $env:GOPROXY
$previousFlags = $env:GOFLAGS
Push-Location $root
try {
    $env:GOPROXY = $Proxy
    if ($DebugBuild) {
        $env:GOFLAGS = '-tags=mygo_nowebview,e2e'
        go tool mygo build -debug -skip-installer -o $OutputDirectory
    } else {
        $env:GOFLAGS = '-tags=mygo_nowebview'
        go tool mygo build -skip-installer -o $OutputDirectory
    }
    if ($LASTEXITCODE -ne 0) { throw "MyGo build failed ($LASTEXITCODE)" }
} finally {
    $env:GOPROXY = $previousProxy
    $env:GOFLAGS = $previousFlags
    Pop-Location
}
