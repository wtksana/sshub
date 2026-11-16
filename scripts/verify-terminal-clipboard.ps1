param([string]$ActiveFixture = 'artifacts/integration-ui/active.json', [switch]$NativeCopy)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/ui-helpers.ps1"
$fixture = Get-Content -Raw (Join-Path $root $ActiveFixture) | ConvertFrom-Json
$cli = Join-Path $root 'build/tools/mygo-automation.exe'
$previousSession = $env:MYGO_AUTOMATION_SESSION
$previousClipboard = Get-Clipboard -Raw
$token = 'SSHub_CLIPBOARD_中文 空格_' + [guid]::NewGuid().ToString('N')
try {
    $env:MYGO_AUTOMATION_SESSION = $fixture.session
    Invoke-RawUI press key=profile-beta --settle | Out-Null
    Invoke-ProductUI press key=open-workspace --settle | Out-Null
    Invoke-ProductUI press key=connect-pending --settle | Out-Null
    Invoke-RawUI wait --text '集成服务器 B · 已连接' --timeout 8s --settle | Out-Null
    $tab = [int]([regex]::Matches((Get-ProductSnapshot), 'key=tab-(\d+)') | ForEach-Object { [int]$_.Groups[1].Value } | Measure-Object -Maximum).Maximum
    Invoke-RawUI wait "key=terminal-$tab" --timeout 8s --settle | Out-Null
    Set-Clipboard -Value $token
    Invoke-RawUI click "key=terminal-$tab" --settle | Out-Null
    Invoke-RawUI key Ctrl+Shift+V --settle | Out-Null
    Invoke-ProductUI press key=terminal-search --settle | Out-Null
    Invoke-RawUI fill "key=terminal-query-$tab" $token --settle | Out-Null
    Invoke-RawUI wait --text '1 处匹配' --timeout 8s --settle | Out-Null
    Invoke-RawUI press "key=search-close-$tab" --settle | Out-Null
    Invoke-RawUI click "key=terminal-$tab" --settle | Out-Null
    Invoke-RawUI key Ctrl+Shift+A --settle | Out-Null
    # A sentinel ensures the assertion cannot pass by reading the input clipboard again.
    Set-Clipboard -Value 'SSHub clipboard output has not been copied yet'
    if ($NativeCopy) {
        Invoke-RawUI click "key=terminal-$tab" --button right --settle | Out-Null
        Read-Host 'Choose Copy in the native terminal menu, then press Enter here' | Out-Null
    } else {
        # Debug builds reserve Ctrl+Shift+C for the MyGo inspector; Ctrl+Insert is the terminal's Windows copy binding.
        Invoke-RawUI key Ctrl+Insert --settle | Out-Null
    }
    $actual = (Get-Clipboard -Raw).Replace("`r`n", "`n").Trim()
    $expected = "SSHub local SSH fixture ready`n$token"
    if ($actual -cne $expected) { throw 'Terminal copy did not match the complete fixture output and pasted Unicode token' }
    Invoke-RawUI screenshot (Join-Path $fixture.directory 'clipboard-selection.png') | Out-Null
    @{passed=$true;scenario='Windows clipboard -> terminal paste -> SSH fixture echo -> output search -> select all -> Windows clipboard';unicode=$true;spaces=$true;outputMatchesExactly=$true;clipboardSentinelReplaced=$true;nativeCopyMenu=[bool]$NativeCopy} |
        ConvertTo-Json | Set-Content -Encoding utf8 (Join-Path $fixture.directory 'clipboard.json')
    Write-Output 'PASS: terminal paste, search, selection and copy through the Windows clipboard'
} finally {
    if ($null -eq $previousClipboard) { Set-Clipboard -Value '' } else { Set-Clipboard -Value $previousClipboard }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
}
