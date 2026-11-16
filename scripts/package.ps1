param([switch]$SkipBuild)
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
Push-Location $root
try{
    if(!$SkipBuild){& "$PSScriptRoot/build.ps1"}
    $config=Get-Content -Raw 'mygo.json'|ConvertFrom-Json
    $name="SSHub-$($config.version)-windows-amd64"
    $stamp=Get-Date -Format 'yyyyMMdd-HHmmss'
    $out=Join-Path $root "build/releases/$stamp"
    $stage=Join-Path $out $name
    New-Item -ItemType Directory -Force $stage|Out-Null
    foreach($file in @('SSHub.exe','ghostty-vt.dll')){
        $source=Join-Path $root "build/windows-amd64/$file"
        if(!(Test-Path $source)){throw "Missing release resource: $file"}
        Copy-Item -LiteralPath $source -Destination $stage
    }
    $buildInfo=(go version -m (Join-Path $stage 'SSHub.exe')) -join "`n"
    if($LASTEXITCODE -ne 0 -or $buildInfo -notmatch 'mygo_noinspector' -or $buildInfo -match '-tags=.*e2e') {throw 'Packaging requires release EXE without automation'}
    $licenses = @(Get-Content -Raw 'docs/third-party/manifest.json' | ConvertFrom-Json)
    $modules = [regex]::Matches($buildInfo, '(?m)^\s*dep\s+(\S+)\s+(\S+)')
    if ($modules.Count -ne $licenses.Count) { throw 'Linked modules differ from license manifest; refresh third-party notices' }
    foreach ($module in $modules) {
        $license = @($licenses | Where-Object { $_.module -eq $module.Groups[1].Value -and $_.version -eq $module.Groups[2].Value })
        if ($license.Count -ne 1 -or !(Test-Path -LiteralPath (Join-Path 'docs/third-party' $license[0].license_file))) { throw "Missing license for $($module.Groups[1].Value)" }
    }
    $plugin=Get-Content -Raw ((go env GOMODCACHE)+'/github.com/egoist/mygo@v0.9.1/plugins/terminal/mygo-plugin.json')|ConvertFrom-Json
    $expected=$plugin.libraries[0].files.'windows-amd64'.sha256
    if((Get-FileHash (Join-Path $stage 'ghostty-vt.dll')).Hash.ToLowerInvariant() -ne $expected){throw 'Ghostty DLL hash mismatch'}
    Copy-Item -LiteralPath 'docs/portable-readme.md' -Destination (Join-Path $stage '使用说明.md')
    Copy-Item -LiteralPath 'scripts/verify-portable-runtime.ps1' -Destination $stage
    Copy-Item -LiteralPath 'docs/third-party' -Destination (Join-Path $stage 'licenses') -Recurse
    @{name='SSHub';version=$config.version;packaged_at=(Get-Date).ToString('o');packaging_commit=(git rev-parse HEAD);packaging_tree_dirty=([bool](git status --porcelain));go_build_info=$buildInfo;ghostty_source=$plugin.libraries[0].source;ghostty_dll_sha256=$expected;mygo='v0.9.1';platform='windows-amd64';signed=$false}|ConvertTo-Json -Depth 5|Set-Content -Encoding utf8 (Join-Path $stage 'build-info.json')
    $checksums=Get-ChildItem -LiteralPath $stage -File -Recurse|ForEach-Object {
        $relative=[IO.Path]::GetRelativePath($stage,$_.FullName).Replace('\','/')
        '{0}  {1}' -f (Get-FileHash -LiteralPath $_.FullName).Hash.ToLowerInvariant(),$relative
    }
    $checksums|Set-Content -Encoding utf8 (Join-Path $stage 'SHA256SUMS.txt')
    $zip=Join-Path $out "$name.zip"
    Compress-Archive -LiteralPath $stage -DestinationPath $zip -CompressionLevel Optimal
    '{0}  {1}' -f (Get-FileHash $zip).Hash.ToLowerInvariant(),(Split-Path -Leaf $zip)|Set-Content -Encoding utf8 (Join-Path $out 'SHA256SUMS.txt')
    Write-Host "Packaged: $zip"
}finally{Pop-Location}
