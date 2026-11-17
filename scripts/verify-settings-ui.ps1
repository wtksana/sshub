param([switch]$SkipBuild, [string]$BuildDirectory = 'build/p5', [switch]$NativePickers)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/ui-helpers.ps1"
Push-Location $root
$process = $null
$previousSession = $env:MYGO_AUTOMATION_SESSION
try {
    if (!$SkipBuild) { & "$PSScriptRoot/build.ps1" -DebugBuild -OutputDirectory $BuildDirectory }
    $cli = Join-Path $root 'build/tools/mygo-automation.exe'
    if (!(Test-Path $cli)) {
        go build -o $cli github.com/egoist/mygo/plugins/automation/cmd/mygo-automation
        if ($LASTEXITCODE -ne 0) { throw 'Automation build failed' }
    }
    $exe = Join-Path $root "$BuildDirectory/windows-amd64/SSHub.exe"
    $directory = Join-Path $root ('artifacts/settings-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $dataDir = Join-Path $directory 'data'
    New-Item -ItemType Directory -Force $dataDir | Out-Null
    $socket = Join-Path $directory 'app.sock'
    $config = Join-Path $dataDir 'connections.json'
    $export = Join-Path $directory 'export.json'
    $incoming = Join-Path $directory 'incoming.json'
    $bad = Join-Path $directory 'bad.json'
    $env:MYGO_AUTOMATION_SESSION = 'sshub-settings-' + [guid]::NewGuid().ToString('N')
    @{version=1;profiles=@(@{id='original';name='服务器';host='example.com';port=22;username='test';auth='password';group='生产';favorite=$true})} | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 $config
    [IO.File]::WriteAllText($bad, '{broken')

    function Assert-Text([string]$value) { if (!(Get-ProductSnapshot).Contains($value)) { throw "Missing UI text: $value" } }
    function Wait-Text([string]$value) { Invoke-RawUI wait --text $value --settle | Out-Null }
    function Menu([string]$key) {
        Invoke-RawUI press key=app-menu --settle | Out-Null
        Invoke-RawUI press "key={$key}" --settle | Out-Null
    }
    function Choose([string]$key, [string]$label) {
        Invoke-RawUI press "key=$key" --settle | Out-Null
        $ref = [regex]::Match((Get-ProductSnapshot), '(?m)^\s*(@e\d+) \[listItem\] "' + [regex]::Escape($label) + '"').Groups[1].Value
        if (!$ref) { throw "Missing option: $label" }
        Invoke-RawUI press $ref --settle | Out-Null
    }
    function Read-Config { Get-Content -Raw $config | ConvertFrom-Json }
    function Start-App {
        $start = [Diagnostics.ProcessStartInfo]::new()
        $start.FileName = $exe
        $start.UseShellExecute = $false
        $start.CreateNoWindow = $true
        $start.Environment['MYGO_AUTOMATION'] = $socket
        foreach ($argument in @('-data-dir', $dataDir)) { $start.ArgumentList.Add($argument) }
        $script:process = [Diagnostics.Process]::Start($start)
        $timer = [Diagnostics.Stopwatch]::StartNew()
        while (!(Test-Path -LiteralPath $socket)) {
            if ($script:process.HasExited -or $timer.Elapsed.TotalSeconds -gt 12) { throw 'App did not start' }
            Start-Sleep -Milliseconds 100
        }
        Invoke-RawUI open --socket $socket | Out-Null
        Wait-Text 'SSHub'
    }
    function Stop-App {
        Invoke-RawUI close | Out-Null
        $script:process.CloseMainWindow() | Out-Null
        if (!$script:process.WaitForExit(5000)) { throw 'App did not close' }
        if ($script:process.ExitCode -ne 0) { throw 'Nonzero app exit' }
        $script:process.Dispose(); $script:process = $null
    }
    function Begin-Import([string]$path) {
        Menu 'import-config'
        Invoke-RawUI fill key=migration-path $path | Out-Null
        Invoke-RawUI press key=import-preview --settle | Out-Null
    }
    function Write-Incoming {
        @{format='sshub';version=1;preferences=@{theme='light';font_family='Consolas';font_size=15};profiles=@(
            @{id='incoming';name='服务器';host='new.example.com';port=22;username='test';auth='password';group='生产';favorite=$false},
            @{id='key';name='私钥服务器';host='key.example.com';port=22;username='test';auth='private_key';identity_path='C:\migrated\id_ed25519';group='生产';favorite=$false}
        )} | ConvertTo-Json -Depth 6 | Set-Content -Encoding utf8 $incoming
    }

    Start-App
    Menu 'settings'
    Invoke-RawUI fill key=settings-size '99' | Out-Null
    Invoke-RawUI press key=settings-save --settle | Out-Null
    Assert-Text '8–32'
    Invoke-RawUI press key=theme-dark | Out-Null
    Invoke-RawUI fill key=settings-ui-font 'Microsoft YaHei UI' | Out-Null
    Invoke-RawUI fill key=settings-ui-size '15' | Out-Null
    Invoke-RawUI fill key=settings-font 'Consolas' | Out-Null
    Invoke-RawUI fill key=settings-size '16' | Out-Null
    Invoke-RawUI press key=settings-save --settle | Out-Null
    Wait-Text '设置已保存'
    if ((Read-Config).preferences.ui_font_size -ne 15 -or (Read-Config).preferences.ui_font_family -ne 'Microsoft YaHei UI') { throw 'UI fonts not persisted separately' }
    if ((Read-Config).preferences.theme -ne 'dark') { throw 'Theme not persisted' }
    Invoke-RawUI screenshot (Join-Path $directory 'dark.png') | Out-Null
    Invoke-RawUI press key=profile-original --settle | Out-Null
    Invoke-ProductUI press key=open-workspace --settle | Out-Null
    Assert-FontSize 16
    Stop-App
    Start-App
    Menu 'settings'
    Assert-Text 'value="Consolas"'
    Assert-Text 'value="16"'
    Invoke-RawUI screenshot (Join-Path $directory 'settings.png') | Out-Null
    Invoke-RawUI press key=settings-cancel --settle | Out-Null

    Menu 'export-config'
    Invoke-RawUI fill key=migration-path $export | Out-Null
    Invoke-RawUI press key=export-save --settle | Out-Null
    Wait-Text '配置已导出到'
    $bundle = Get-Content -Raw $export | ConvertFrom-Json
    if ($bundle.format -ne 'sshub' -or $bundle.profiles.Count -ne 1 -or $bundle.preferences.font_size -ne 16) { throw 'Export data incorrect' }
    $beforeExport = [IO.File]::ReadAllText($export)
    Menu 'export-config'
    Invoke-RawUI fill key=migration-path $export | Out-Null
    Invoke-RawUI press key=export-save --settle | Out-Null
    Wait-Text '目标文件已存在，确认覆盖？'
    Invoke-RawUI press key=migration-cancel --settle | Out-Null
    if ([IO.File]::ReadAllText($export) -ne $beforeExport) { throw 'Canceled export changed file' }
    Menu 'export-config'
    Invoke-RawUI fill key=migration-path $export | Out-Null
    Invoke-RawUI press key=export-save --settle | Out-Null
    Wait-Text '目标文件已存在，确认覆盖？'
    Invoke-RawUI press key=export-overwrite --settle | Out-Null
    Wait-Text '配置已导出到'

    $before = [IO.File]::ReadAllText($config)
    Begin-Import $bad
    Wait-Text '迁移文件格式无效'
    Invoke-RawUI press key=migration-cancel --settle | Out-Null
    if ([IO.File]::ReadAllText($config) -ne $before) { throw 'Invalid import changed data' }
    Write-Incoming
    Begin-Import $incoming
    Wait-Text '预览：2 个连接'
    Invoke-RawUI screenshot (Join-Path $directory 'preview.png') | Out-Null
    Invoke-RawUI press key=migration-cancel --settle | Out-Null
    if ([IO.File]::ReadAllText($config) -ne $before) { throw 'Canceled preview changed data' }
    Begin-Import $incoming
    Wait-Text '预览：2 个连接'
    Invoke-RawUI press key=import-confirm --settle | Out-Null
    Assert-Text '请处理全部重名冲突'
    Choose 'import-choice-0' '另存'
    Invoke-RawUI fill key=import-name-0 '服务器副本' | Out-Null
    Invoke-RawUI press key=import-preferences --settle | Out-Null
    Invoke-RawUI press key=import-confirm --settle | Out-Null
    Wait-Text '已导入 2 个连接'
    $doc = Read-Config
    if ($doc.profiles.Count -ne 3 -or $doc.preferences.theme -ne 'light') { throw 'Import not applied' }
    $keyID = ($doc.profiles | Where-Object name -eq '私钥服务器').id
    if (!$keyID -or $keyID -eq 'key' -or !($doc.profiles | Where-Object id -eq $keyID).identity_needs_review) { throw 'Imported private key identity unsafe' }
    Invoke-RawUI press "key=profile-$keyID" --settle | Out-Null
    Invoke-ProductUI press key=open-workspace --settle | Out-Null
    Invoke-ProductUI press key=connect-pending --settle | Out-Null
    Assert-Text '导入的私钥路径尚未核对'
    Open-ProfileMenu $keyID
    Invoke-RawUI press 'key={edit-profile}' --settle | Out-Null
    Invoke-RawUI press key=save-profile --settle | Out-Null
    Assert-Text '请核对本机私钥路径并勾选确认'
    Invoke-RawUI press key=identity-reviewed --settle | Out-Null
    Invoke-RawUI press key=save-profile --settle | Out-Null
    if ((Read-Config).profiles | Where-Object { $_.id -eq $keyID -and $_.identity_needs_review }) { throw 'Review not saved' }

    Begin-Import $incoming
    Wait-Text '预览：2 个连接'
    Choose 'import-choice-0' '替换'
    Choose 'import-choice-1' '跳过'
    Invoke-RawUI press key=import-confirm --settle | Out-Null
    Wait-Text '已导入 1 个连接'
    $doc = Read-Config
    $replaced = $doc.profiles | Where-Object name -eq '服务器'
    if ($replaced.id -eq 'original' -or $replaced.id -eq 'incoming' -or $replaced.host -ne 'new.example.com') { throw 'Replacement identity unsafe' }
    # Original tab still uses its original snapshot during this process.
    Assert-WorkspaceCount 1 2
    Assert-Text 'test@example.com · 字号 16'
    Invoke-RawUI screenshot (Join-Path $directory 'light.png') | Out-Null
    Stop-App
    Start-App
    Assert-WorkspaceCount 1 2
    Assert-Text 'test@new.example.com · 字号 16'
    if ((Read-Config).preferences.theme -ne 'light') { throw 'Imported preference lost on restart' }
    if ($NativePickers) {
        @{directory=$directory;socket=$socket;session=$env:MYGO_AUTOMATION_SESSION;export=(Join-Path $directory '原生选择器 导出.json')} |
            ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $root 'artifacts/settings-ui/active.json')
        Write-Host "Drive export and import native pickers, cancel the import preview, then resume: $directory"
        Read-Host 'Native picker checks' | Out-Null
        $picked = Get-Content -Raw (Join-Path $directory '原生选择器 导出.json') | ConvertFrom-Json
        if ($picked.format -ne 'sshub' -or $picked.profiles.Count -ne 3) { throw 'Native picker export invalid' }
        if ((Read-Config).profiles.Count -ne 3) { throw 'Canceled picker import changed configuration' }
    }
    Stop-App
    Write-Host "Settings and migration UI checks passed: $directory"
} catch {
    if ($process -and !$process.HasExited) { try { Get-ProductSnapshot | Set-Content -Encoding utf8 (Join-Path $directory 'failure.txt') } catch {} }
    throw
} finally {
    if ($process -and !$process.HasExited) { $process.Kill($true); $process.WaitForExit() }
    if ($process) { $process.Dispose() }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
    Pop-Location
}
