#requires -Version 7.5
param(
    [string]$Fixture = 'artifacts/sftp-ui/active.json',
    [switch]$PrepareOnly,
    [switch]$CheckOnly
)
$ErrorActionPreference = 'Stop'
if ($PrepareOnly -and $CheckOnly) { throw 'Choose preparation or verification, not both' }
$active = Get-Content -Raw -Encoding utf8 -LiteralPath $Fixture | ConvertFrom-Json
$source = Join-Path $active.directory 'native-drop-source'
$remote = $active.remote
if (!$CheckOnly) {
    if (Test-Path -LiteralPath $source) { throw 'Drop source already exists; use a fresh SFTP fixture' }
    $nested = Join-Path $source '拖入 文件夹/子目录'
    New-Item -ItemType Directory -Force $nested,(Join-Path $source '拖入 文件夹/空目录') | Out-Null
    $utf8 = [Text.UTF8Encoding]::new($false)
    [IO.File]::WriteAllText((Join-Path $source '拖入 中文.txt'), 'Native drop file 中文 ' + [guid]::NewGuid().ToString('N'), $utf8)
    [IO.File]::WriteAllText((Join-Path $nested '空格 内容.txt'), 'Recursive native drop 中文 ' + [guid]::NewGuid().ToString('N'), $utf8)
    @{source=$source;remote=$remote;session=$active.session;socket=$active.socket} | ConvertTo-Json |
        Set-Content -Encoding utf8 (Join-Path $active.directory 'native-drop.json')
    Write-Output "Prepared: $source"
    Write-Output 'With the product remote path at /, drag BOTH the file and folder from Explorer into the right file list. Wait until both transfers succeed. This script does not inject or upload files.'
    if ($PrepareOnly) { return }
    Read-Host 'Perform the native drag, wait for completed tasks, then press Enter to verify' | Out-Null
}
if (!(Test-Path -LiteralPath $source)) { throw 'Native drop source is missing; prepare it first' }
$rows = @()
foreach ($file in Get-ChildItem -LiteralPath $source -File -Recurse) {
    $relative = [IO.Path]::GetRelativePath($source, $file.FullName)
    $target = Join-Path $remote $relative
    if (!(Test-Path -LiteralPath $target -PathType Leaf)) { throw "Dropped file is missing remotely: $relative" }
    $expected = (Get-FileHash -LiteralPath $file.FullName).Hash
    if ((Get-FileHash -LiteralPath $target).Hash -ne $expected) { throw "Dropped content differs: $relative" }
    $rows += @{relative=$relative.Replace('\','/');sha256=$expected}
}
foreach ($folder in Get-ChildItem -LiteralPath $source -Directory -Recurse) {
    $relative = [IO.Path]::GetRelativePath($source, $folder.FullName)
    if (!(Test-Path -LiteralPath (Join-Path $remote $relative) -PathType Container)) { throw "Dropped directory is missing remotely: $relative" }
}
@{contents_verified=$true;files=$rows;empty_directory_verified=$true;native_gesture_observed_by_script=$false;note='File hashes prove delivery only. Record actual Explorer drag and product task results separately; do not treat this check as proof of native OLE input.'} |
    ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $active.directory 'native-drop-result.json')
Write-Output 'PASS: dropped file, recursive Unicode folder and empty directory contents. Native gesture still requires direct observation.'
