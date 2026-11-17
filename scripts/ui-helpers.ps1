# Drive the product's actual menus; all fixtures continue to use isolated data.
function Invoke-RawUI {
    $output = & $cli @args
    if ($LASTEXITCODE -ne 0) { throw "UI command failed: $args" }
    return $output
}

function Get-ProductSnapshot { return (Invoke-RawUI snapshot --full) -join "`n" }

function Get-CurrentPaneID {
    $line = (Get-ProductSnapshot) -split "`n" | Where-Object { $_ -match '\[group\] "终端窗格".*current=true' } | Select-Object -First 1
    $id = [regex]::Match($line, 'key=pane-(\d+)').Groups[1].Value
    if (!$id) { throw 'Current terminal pane missing' }
    return $id
}

function Open-ProfileMenu([string]$id) {
    if (!$id) {
        $line = (Get-ProductSnapshot) -split "`n" | Where-Object { $_ -match '\[button\]' -and $_ -match 'key=profile-' -and $_ -match ' checked' } | Select-Object -First 1
        $id = [regex]::Match($line, 'key=profile-([^ ]+)').Groups[1].Value
    }
    if (!$id) { throw 'Selected connection missing' }
    Invoke-RawUI click "key=profile-$id" --button right --settle | Out-Null
}

function Open-PaneMenu {
    $id = Get-CurrentPaneID
    Invoke-RawUI press "key=pane-menu-$id" --settle | Out-Null
}

function Invoke-ProductUI {
    $commandArgs = @($args)
    if ($commandArgs.Count -ge 2 -and $commandArgs[0] -eq 'press' -and $commandArgs[1] -like 'key=*') {
        $key = $commandArgs[1].Substring(4)
        if ($key -match '^(connect-profile|edit-profile|copy-profile|favorite-profile|forget-profile-credentials|delete-profile)$') {
            Open-ProfileMenu
        } elseif ($key -match '^(manage-hosts)$') {
            Invoke-RawUI press key=app-menu --settle | Out-Null
        } elseif ($key -like 'scope-*') {
            Invoke-RawUI press key=connection-filter --settle | Out-Null
        } elseif ($key -match '^(connect-pending|cancel-connect|disconnect-session|terminal-search|font-larger|font-smaller|font-reset|split-horizontal|split-vertical|move-tab|merge-pane|forget-credentials)$') {
            Open-PaneMenu
            if ($key -like 'font-*') { Open-UISubmenu '字号' }
        } else {
            return Invoke-RawUI @commandArgs
        }
        $commandArgs[1] = "key={$key}"
    }
    return Invoke-RawUI @commandArgs
}

function Open-UISubmenu([string]$label) {
    $ref = [regex]::Match((Get-ProductSnapshot), '(?m)^\s*(@e\d+) \[menuItem\] "' + [regex]::Escape($label) + '"').Groups[1].Value
    if (!$ref) { throw "Submenu missing: $label" }
    Invoke-RawUI press $ref --settle | Out-Null
}

function Select-UIGroup([string]$label) {
    Invoke-RawUI press key=connection-filter --settle | Out-Null
    Open-UISubmenu '分组'
    $ref = [regex]::Match((Get-ProductSnapshot), '(?m)^\s*(@e\d+) \[menuItemRadio\] "' + [regex]::Escape($label) + '"').Groups[1].Value
    if (!$ref) { throw "Group missing: $label" }
    Invoke-RawUI press $ref --settle | Out-Null
}

function Assert-WorkspaceCount([int]$panes, [int]$tabs) {
    $snapshot = Get-ProductSnapshot
    if ([regex]::Matches($snapshot, 'key=pane-\d+\b').Count -ne $panes -or [regex]::Matches($snapshot, 'key=tab-\d+\b').Count -ne $tabs) {
        throw "Expected $panes panes and $tabs sessions"
    }
}

function Assert-FontSize([int]$size) {
    Open-PaneMenu
    Open-UISubmenu '字号'
    if (!(Get-ProductSnapshot).Contains("当前字号：$size")) { throw "Expected font size: $size" }
    Invoke-RawUI key Escape Escape --settle | Out-Null
}
