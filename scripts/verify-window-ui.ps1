param([switch]$SkipBuild, [string]$BuildDirectory = 'build')
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
. "$PSScriptRoot/ui-helpers.ps1"
Push-Location $root
$appProcess = $null
$previousSession = $env:MYGO_AUTOMATION_SESSION
try {
    if (!$SkipBuild) { & "$PSScriptRoot/build.ps1" -DebugBuild -OutputDirectory $BuildDirectory }
    go build -o build/tools/mygo-automation.exe github.com/egoist/mygo/plugins/automation/cmd/mygo-automation
    if ($LASTEXITCODE -ne 0) { throw 'Automation build failed' }
    $cli = Join-Path $root 'build/tools/mygo-automation.exe'
    $directory = Join-Path $root ('artifacts/window-ui/' + (Get-Date -Format 'yyyyMMdd-HHmmss'))
    $dataDir = Join-Path $directory 'data'
    New-Item -ItemType Directory -Force $dataDir | Out-Null
    $socket = Join-Path $directory 'app.sock'
    $env:MYGO_AUTOMATION_SESSION = 'sshub-window-' + [guid]::NewGuid().ToString('N')
    Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class SSHubWindowTest {
    [StructLayout(LayoutKind.Sequential)] public struct Rect { public int Left, Top, Right, Bottom; }
    [StructLayout(LayoutKind.Sequential)] public struct Point { public int X, Y; }
    [DllImport("user32.dll")] public static extern IntPtr SetThreadDpiAwarenessContext(IntPtr context);
    [StructLayout(LayoutKind.Sequential)] public struct MonitorInfo { public int Size; public Rect Monitor, Work; public int Flags; }
    [DllImport("user32.dll")] public static extern IntPtr MonitorFromWindow(IntPtr w, int flags);
    [DllImport("user32.dll")] public static extern bool GetMonitorInfoW(IntPtr monitor, ref MonitorInfo info);
    public static Rect WorkArea(IntPtr w) {
        MonitorInfo i = new MonitorInfo { Size = Marshal.SizeOf<MonitorInfo>() };
        if (!GetMonitorInfoW(MonitorFromWindow(w, 2), ref i)) throw new InvalidOperationException("No monitor work area");
        return i.Work;
    }
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr w, out Rect r);
    [DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr w, out Rect r);
    [DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr w, ref Point p);
    [DllImport("user32.dll")] public static extern bool IsZoomed(IntPtr w);
    [DllImport("user32.dll")] public static extern bool IsIconic(IntPtr w);
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr w, int command);
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr w);
    [DllImport("user32.dll")] public static extern IntPtr ChildWindowFromPointEx(IntPtr w, Point p, uint flags);
    [DllImport("user32.dll")] public static extern IntPtr SendMessageW(IntPtr w, uint message, IntPtr wp, IntPtr lp);
    [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr w, IntPtr after, int x, int y, int width, int height, uint flags);
    [DllImport("user32.dll")] public static extern uint GetDpiForWindow(IntPtr w);
    [DllImport("user32.dll")] public static extern bool GetCursorPos(out Point p);
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
    [DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint dx, uint dy, uint data, UIntPtr extra);
    public static Rect Bounds(IntPtr w) { Rect r; GetWindowRect(w, out r); return r; }
    public static Rect Client(IntPtr w) { Rect r; GetClientRect(w, out r); return r; }
    public static Point Origin(IntPtr w) { Point p = new Point(); ClientToScreen(w, ref p); return p; }
    public static int Hit(IntPtr w, int x, int y) {
        Point origin = Origin(w);
        Point p = new Point { X = x - origin.X, Y = y - origin.Y };
        IntPtr child = ChildWindowFromPointEx(w, p, 1);
        return (int)SendMessageW(child == IntPtr.Zero ? w : child, 0x84, IntPtr.Zero, (IntPtr)((y << 16) | (x & 65535)));
    }
}
'@
    function Start-App {
        $start = [Diagnostics.ProcessStartInfo]::new()
        $start.FileName = Join-Path $root "$BuildDirectory/windows-amd64/SSHub.exe"
        $start.UseShellExecute = $false; $start.CreateNoWindow = $true
        $start.Environment['MYGO_AUTOMATION'] = $socket
        $start.ArgumentList.Add('-data-dir'); $start.ArgumentList.Add($dataDir)
        $script:appProcess = [Diagnostics.Process]::Start($start)
        $timer = [Diagnostics.Stopwatch]::StartNew()
        while (!(Test-Path -LiteralPath $socket)) {
            if ($appProcess.HasExited -or $timer.Elapsed.TotalSeconds -gt 12) { throw 'App did not start' }
            Start-Sleep -Milliseconds 100
        }
        Invoke-RawUI open --socket $socket | Out-Null
        Invoke-RawUI wait key=window-close --settle | Out-Null
        $appProcess.Refresh()
        $script:windowHandle = $appProcess.MainWindowHandle
        if (!$windowHandle) { throw 'Native window missing' }
    }
    function Wait-Native([scriptblock]$condition, [string]$description) {
        $timer = [Diagnostics.Stopwatch]::StartNew()
        while (!(& $condition)) {
            if ($appProcess.HasExited -or $timer.Elapsed.TotalSeconds -gt 5) { throw "Window check failed: $description" }
            Start-Sleep -Milliseconds 50
        }
    }
    function Close-App {
        Invoke-RawUI press key=window-close | Out-Null
        if (!$appProcess.WaitForExit(5000) -or $appProcess.ExitCode -ne 0) { throw 'Custom close did not exit cleanly' }
        Invoke-RawUI close | Out-Null
        $appProcess.Dispose(); $script:appProcess = $null
    }
    $previousDpiContext = [SSHubWindowTest]::SetThreadDpiAwarenessContext([IntPtr]::new(-4))
    Start-App
    $origin = [SSHubWindowTest]::Origin($windowHandle)
    $bounds = [SSHubWindowTest]::Bounds($windowHandle)
    $scale = [SSHubWindowTest]::GetDpiForWindow($windowHandle) / 96.0
    # The client begins at the top of the window: no separate system caption.
    if ([Math]::Abs($origin.Y - $bounds.Top) -gt 2 * $scale) { throw 'System title bar still takes vertical space' }
    if ([SSHubWindowTest]::Hit($windowHandle, $origin.X + 100, $origin.Y + 1) -ne 12) { throw "Top resize edge missing: bounds=$($bounds | ConvertTo-Json -Compress) origin=$($origin | ConvertTo-Json -Compress) hit=$([SSHubWindowTest]::Hit($windowHandle, $origin.X + 100, $origin.Y + 1))" }
    $client = [SSHubWindowTest]::Client($windowHandle)
    if ([SSHubWindowTest]::Hit($windowHandle, $origin.X + $client.Right - [int](23 * $scale), $origin.Y + [int](17 * $scale)) -ne 1) { throw 'Close button is still system chrome' }
    if ([SSHubWindowTest]::Hit($windowHandle, $origin.X + $client.Right - [int](69 * $scale), $origin.Y + [int](17 * $scale)) -ne 9) { throw 'Custom maximize does not expose Windows snap hit target' }
    Invoke-RawUI press key=toggle-files --settle | Out-Null
    Invoke-RawUI press key=toggle-files --settle | Out-Null
    Invoke-RawUI screenshot (Join-Path $directory 'custom-titlebar.png') | Out-Null
    Invoke-RawUI press key=window-maximize --settle | Out-Null
    Wait-Native { [SSHubWindowTest]::IsZoomed($windowHandle) } 'maximize'
    if (!(Get-ProductSnapshot).Contains('还原窗口')) { throw 'Restore action label missing' }
    Invoke-RawUI screenshot (Join-Path $directory 'maximized.png') | Out-Null
    Invoke-RawUI press key=window-maximize --settle | Out-Null
    Wait-Native { ![SSHubWindowTest]::IsZoomed($windowHandle) } 'restore'
    Invoke-RawUI click key=titlebar-drag --clicks 2 --settle | Out-Null
    Wait-Native { [SSHubWindowTest]::IsZoomed($windowHandle) } 'double-click maximize'
    Invoke-RawUI click key=titlebar-drag --clicks 2 --settle | Out-Null
    Wait-Native { ![SSHubWindowTest]::IsZoomed($windowHandle) } 'double-click restore'
    Invoke-RawUI press key=window-minimize | Out-Null
    Wait-Native { [SSHubWindowTest]::IsIconic($windowHandle) } 'minimize'
    [SSHubWindowTest]::ShowWindow($windowHandle, 9) | Out-Null
    Wait-Native { ![SSHubWindowTest]::IsIconic($windowHandle) } 'restore minimized window'
    Invoke-RawUI wait key=window-close --settle | Out-Null

    # Exercise the real OS drag loop; automation's synthetic pointer cannot
    # release the native move loop once it has taken the UI thread.
    [SSHubWindowTest]::SetForegroundWindow($windowHandle) | Out-Null
    $beforeDrag = [SSHubWindowTest]::Bounds($windowHandle)
    $origin = [SSHubWindowTest]::Origin($windowHandle)
    function Move-WindowPointer([int]$x, [int]$y, [int]$dx, [int]$dy) {
        $cursor = [SSHubWindowTest+Point]::new()
        [SSHubWindowTest]::GetCursorPos([ref]$cursor) | Out-Null
        try {
            [SSHubWindowTest]::SetCursorPos($x, $y) | Out-Null
            [SSHubWindowTest]::mouse_event(2, 0, 0, 0, [UIntPtr]::Zero)
            Start-Sleep -Milliseconds 80
            foreach ($step in 1..5) {
                [SSHubWindowTest]::SetCursorPos(($x + [int]($dx * $step / 5)), ($y + [int]($dy * $step / 5))) | Out-Null
                Start-Sleep -Milliseconds 40
            }
        } finally {
            [SSHubWindowTest]::mouse_event(4, 0, 0, 0, [UIntPtr]::Zero)
            [SSHubWindowTest]::SetCursorPos($cursor.X, $cursor.Y) | Out-Null
        }
    }
    Move-WindowPointer ($origin.X + [int](300 * $scale)) ($origin.Y + [int](17 * $scale)) 50 30
    Wait-Native { $afterDrag = [SSHubWindowTest]::Bounds($windowHandle); $afterDrag.Left -ne $beforeDrag.Left -or $afterDrag.Top -ne $beforeDrag.Top } 'title bar drag'
    $beforeResize = [SSHubWindowTest]::Bounds($windowHandle)
    $origin = [SSHubWindowTest]::Origin($windowHandle)
    Move-WindowPointer ($origin.X + [int](300 * $scale)) ($origin.Y + 1) 0 40
    Wait-Native { $afterResize = [SSHubWindowTest]::Bounds($windowHandle); ($afterResize.Bottom - $afterResize.Top) -ne ($beforeResize.Bottom - $beforeResize.Top) } 'top edge resize'
    # Resize through the native window API and check the application reflows.
    [SSHubWindowTest]::SetWindowPos($windowHandle, [IntPtr]::Zero, 40, 40, [int](900 * $scale), [int](620 * $scale), 4) | Out-Null
    Invoke-RawUI wait key=window-close --settle | Out-Null
    Invoke-RawUI screenshot (Join-Path $directory 'minimum-window.png') | Out-Null
    Invoke-RawUI press key=window-maximize --settle | Out-Null
    Close-App
    $saved = Get-Content -Raw (Join-Path $dataDir 'workspace.json') | ConvertFrom-Json
    if ($saved.window_width -ne 900 -or $saved.window_height -ne 620) { throw 'Maximized exit replaced the normal window size' }
    Start-App
    Invoke-RawUI screenshot (Join-Path $directory 'restored-normal-size.png') | Out-Null
    if ([SSHubWindowTest]::IsZoomed($windowHandle)) { throw 'Restart should restore the normal window' }
    Close-App
    $saved.window_width = 8192
    $saved.window_height = 8192
    $saved | ConvertTo-Json -Depth 8 | Set-Content -Encoding utf8 (Join-Path $dataDir 'workspace.json')
    Start-App
    $work = [SSHubWindowTest]::WorkArea($windowHandle)
    $bounds = [SSHubWindowTest]::Bounds($windowHandle)
    if ($bounds.Left -lt $work.Left - 2 -or $bounds.Top -lt $work.Top - 2 -or $bounds.Right -gt $work.Right + 2 -or $bounds.Bottom -gt $work.Bottom + 2) {
        throw "Oversized saved window did not fit: bounds=$($bounds | ConvertTo-Json -Compress) work=$($work | ConvertTo-Json -Compress)"
    }
    Close-App
    Write-Host "Custom title bar UI checks passed: $directory"
} catch {
    if ($appProcess -and !$appProcess.HasExited) {
        try { Get-ProductSnapshot | Set-Content -Encoding utf8 (Join-Path $directory 'failure.txt') } catch {}
    }
    throw
} finally {
    if ($appProcess) { if (!$appProcess.HasExited) { $appProcess.Kill($true); $appProcess.WaitForExit() }; $appProcess.Dispose() }
    if ($previousDpiContext) { [SSHubWindowTest]::SetThreadDpiAwarenessContext($previousDpiContext) | Out-Null }
    $env:MYGO_AUTOMATION_SESSION = $previousSession
    Pop-Location
}
