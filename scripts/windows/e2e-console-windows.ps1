# Console windows from child processes (issue #339). Launches ss.exe under four console states,
# runs `pull` and `sync plugins --no-tui`, and records every descendant process plus every new
# visible top-level window. Run as the desktop user through `utm.sh task`; see
# ai_docs/tests/windows_console_windows_runbook.md.
# With -UiZip/-VersionFile (utm.sh build output) it also checks cancellation across the
# hidden-console relaunch and that a background `ui start` server outlives its launcher.
# Usage: e2e-console-windows.ps1 -Exe <ss.exe> -Root <empty dir> -Out <report.txt> [-UiZip <zip> -VersionFile <txt>]
param([string]$Exe, [string]$Root, [string]$Out, [string]$UiZip, [string]$VersionFile)
$ErrorActionPreference = 'Continue'
$log = New-Object System.Collections.Generic.List[string]
function L($s) { $log.Add([string]$s) }

Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;

public static class ConsoleProbe {
  [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
  struct STARTUPINFO { public int cb; public string r, d, t; public int x, y, w, h, cx, cy, fill, flags; public short show, r2; public IntPtr r3, hin, hout, herr; }
  [StructLayout(LayoutKind.Sequential)]
  struct PI { public IntPtr hp, ht; public int pid, tid; }
  [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
  struct PE32 { public int size, usage, pid; public IntPtr heap; public int module, threads, ppid, prio, flags; [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 260)] public string exe; }
  [StructLayout(LayoutKind.Sequential)]
  struct SA { public int n; public IntPtr sd; public bool inherit; }
  delegate bool EnumProc(IntPtr h, IntPtr p);

  [DllImport("kernel32", CharSet = CharSet.Unicode, SetLastError = true)] static extern bool CreateProcessW(string app, StringBuilder cmd, IntPtr pa, IntPtr ta, bool inherit, uint flags, IntPtr env, string cwd, ref STARTUPINFO si, out PI pi);
  [DllImport("kernel32", CharSet = CharSet.Unicode, SetLastError = true)] static extern IntPtr CreateFileW(string n, uint acc, uint share, ref SA sa, uint disp, uint fl, IntPtr t);
  [DllImport("kernel32")] static extern uint WaitForSingleObject(IntPtr h, uint ms);
  [DllImport("kernel32")] static extern bool GetExitCodeProcess(IntPtr h, out uint c);
  [DllImport("kernel32")] static extern bool CloseHandle(IntPtr h);
  [DllImport("kernel32", SetLastError = true)] static extern IntPtr CreateToolhelp32Snapshot(uint f, uint p);
  [DllImport("kernel32", CharSet = CharSet.Unicode)] static extern bool Process32FirstW(IntPtr s, ref PE32 e);
  [DllImport("kernel32", CharSet = CharSet.Unicode)] static extern bool Process32NextW(IntPtr s, ref PE32 e);
  [DllImport("user32")] static extern bool EnumWindows(EnumProc cb, IntPtr p);
  [DllImport("user32")] static extern bool IsWindowVisible(IntPtr h);
  [DllImport("user32")] static extern uint GetWindowThreadProcessId(IntPtr h, out int pid);
  [DllImport("user32", CharSet = CharSet.Unicode)] static extern int GetClassNameW(IntPtr h, StringBuilder s, int n);
  [DllImport("user32", CharSet = CharSet.Unicode)] static extern int GetWindowTextW(IntPtr h, StringBuilder s, int n);

  public static Dictionary<int, string> Procs = new Dictionary<int, string>();   // pid -> "ppid|exe|ms"
  public static Dictionary<long, string> Wins = new Dictionary<long, string>();  // hwnd -> "pid|class|title|ms"
  static HashSet<long> baseline = new HashSet<long>();
  static HashSet<int> baseProcs = new HashSet<int>();
  static volatile bool stop;
  static System.Diagnostics.Stopwatch sw;

  static void SnapProcs(bool isBase) {
    IntPtr s = CreateToolhelp32Snapshot(2, 0);
    PE32 e = new PE32(); e.size = Marshal.SizeOf(typeof(PE32));
    if (Process32FirstW(s, ref e)) do {
      if (isBase) baseProcs.Add(e.pid);
      else if (!baseProcs.Contains(e.pid) && !Procs.ContainsKey(e.pid)) Procs[e.pid] = e.ppid + "|" + e.exe + "|" + sw.ElapsedMilliseconds;
    } while (Process32NextW(s, ref e));
    CloseHandle(s);
  }
  static void SnapWins(bool isBase) {
    EnumWindows(delegate (IntPtr h, IntPtr p) {
      if (!IsWindowVisible(h)) return true;
      long k = h.ToInt64();
      if (isBase) { baseline.Add(k); return true; }
      if (baseline.Contains(k) || Wins.ContainsKey(k)) return true;
      int pid; GetWindowThreadProcessId(h, out pid);
      StringBuilder c = new StringBuilder(256), t = new StringBuilder(256);
      GetClassNameW(h, c, 256); GetWindowTextW(h, t, 256);
      Wins[k] = pid + "|" + c + "|" + t + "|" + sw.ElapsedMilliseconds;
      return true;
    }, IntPtr.Zero);
  }

  // Starts without waiting; returns the PID (0 on failure).
  public static int Start(string exe, string args, string cwd, uint flags, string outFile) {
    SA sa = new SA(); sa.n = Marshal.SizeOf(typeof(SA)); sa.inherit = true;
    IntPtr fh = CreateFileW(outFile, 0x40000000, 3, ref sa, 2, 0x80, IntPtr.Zero);
    STARTUPINFO si = new STARTUPINFO(); si.cb = Marshal.SizeOf(typeof(STARTUPINFO));
    si.flags = 0x100; si.hout = fh; si.herr = fh;
    PI pi;
    bool ok = CreateProcessW(exe, new StringBuilder("\"" + exe + "\" " + args), IntPtr.Zero, IntPtr.Zero, true, flags, IntPtr.Zero, cwd, ref si, out pi);
    CloseHandle(fh);
    if (!ok) return 0;
    CloseHandle(pi.hp); CloseHandle(pi.ht);
    return pi.pid;
  }

  public static string Run(string exe, string args, string cwd, uint flags, string outFile) {
    Procs.Clear(); Wins.Clear(); baseline.Clear(); baseProcs.Clear(); stop = false;
    sw = System.Diagnostics.Stopwatch.StartNew();
    SnapProcs(true); SnapWins(true);
    Thread mon = new Thread(delegate () { while (!stop) { SnapProcs(false); SnapWins(false); Thread.Sleep(5); } });
    mon.Start();

    SA sa = new SA(); sa.n = Marshal.SizeOf(typeof(SA)); sa.inherit = true;
    IntPtr fh = CreateFileW(outFile, 0x40000000, 3, ref sa, 2, 0x80, IntPtr.Zero);
    STARTUPINFO si = new STARTUPINFO(); si.cb = Marshal.SizeOf(typeof(STARTUPINFO));
    si.flags = 0x100; si.hout = fh; si.herr = fh;   // STARTF_USESTDHANDLES; stdin left null
    PI pi;
    string res;
    if (!CreateProcessW(exe, new StringBuilder("\"" + exe + "\" " + args), IntPtr.Zero, IntPtr.Zero, true, flags, IntPtr.Zero, cwd, ref si, out pi)) {
      res = "pid=0 createError=" + Marshal.GetLastWin32Error();
    } else {
      WaitForSingleObject(pi.hp, 180000);
      uint code; GetExitCodeProcess(pi.hp, out code);
      res = "pid=" + pi.pid + " exit=" + code + " ms=" + sw.ElapsedMilliseconds;
      CloseHandle(pi.hp); CloseHandle(pi.ht);
    }
    CloseHandle(fh);
    Thread.Sleep(1500);   // let late windows appear
    stop = true; mon.Join();
    return res;
  }
}
'@

# Isolated profile: nothing below touches the real user's config.
$realCfg = Join-Path $env:APPDATA 'skillshare'
$homeDir = Join-Path $Root 'home'
foreach ($d in 'home', 'home\AppData\Roaming', 'home\AppData\Local', 'remote.git', 'out') { New-Item -ItemType Directory -Force (Join-Path $Root $d) | Out-Null }
$env:USERPROFILE = $homeDir; $env:HOME = $homeDir
$env:APPDATA = "$homeDir\AppData\Roaming"; $env:LOCALAPPDATA = "$homeDir\AppData\Local"
$env:XDG_CONFIG_HOME = "$homeDir\.config"; $env:XDG_DATA_HOME = "$homeDir\.local\share"; $env:XDG_STATE_HOME = "$homeDir\.local\state"; $env:XDG_CACHE_HOME = "$homeDir\.cache"
$env:GIT_CONFIG_NOSYSTEM = '1'
$realBefore = if (Test-Path $realCfg) { (Get-ChildItem $realCfg -Recurse -Force | Measure-Object -Property LastWriteTime -Maximum).Maximum } else { 'absent' }

# Setup (inside this hidden console, before any measurement): source repo tracking a local bare remote.
$src = Join-Path $Root 'src'
git init -q --bare (Join-Path $Root 'remote.git') 2>&1 | Out-Null
$null | & $Exe init --source $src --remote (Join-Path $Root 'remote.git') --no-copy --no-targets --no-skill --git 2>&1 | ForEach-Object { L "init: $_" }
L "initExit=$LASTEXITCODE"
git -C $src -c user.name=t -c user.email=t@t add -A 2>&1 | Out-Null
git -C $src -c user.name=t -c user.email=t@t commit -q --allow-empty -m init 2>&1 | Out-Null
git -C $src push -q -u origin HEAD 2>&1 | ForEach-Object { L "push: $_" }
L "configDir=$(Test-Path "$homeDir\.config\skillshare\config.yaml") branch=$(git -C $src branch --show-current) upstream=$(git -C $src rev-parse --abbrev-ref '@{u}' 2>&1)"
L "sessionConsole=$((Get-Process -Id $PID).MainWindowHandle) parentHiddenConsole=yes(task uses -WindowStyle Hidden)"

$modes = [ordered]@{
  'INHERIT_HIDDEN' = 0x0          # child of this hidden console (wrapper workaround)
  'CREATE_NO_WINDOW' = 0x08000000 # windowless console
  'DETACHED' = 0x00000008         # ss.exe has no console at all (pythonw-like parent)
  'NEW_CONSOLE' = 0x00000010      # what Task Scheduler gives a console app it starts directly
}
# 'unknown' exits 1: the exit code must survive the hidden-console relaunch.
$cmds = [ordered]@{ 'pull' = 'pull'; 'sync-plugins' = 'sync plugins --no-tui'; 'unknown' = 'no-such-command' }

foreach ($m in $modes.Keys) {
  foreach ($c in $cmds.Keys) {
    $of = Join-Path $Root "out\$m-$c.txt"
    $r = [ConsoleProbe]::Run($Exe, $cmds[$c], $src, [uint32]$modes[$m], $of)
    L ""
    L "=== mode=$m cmd=$($cmds[$c]) $r"
    $ssPid = [int](($r -split ' ')[0] -replace 'pid=', '')
    # Descendants of ss.exe in creation order.
    $tree = @{}; foreach ($k in [ConsoleProbe]::Procs.Keys) { $p = [ConsoleProbe]::Procs[$k] -split '\|'; $tree[$k] = $p }
    $desc = New-Object System.Collections.Generic.HashSet[int]; [void]$desc.Add($ssPid)
    do { $grew = $false; foreach ($k in $tree.Keys) { if (-not $desc.Contains($k) -and $desc.Contains([int]$tree[$k][0])) { [void]$desc.Add($k); $grew = $true } } } while ($grew)
    foreach ($k in ($tree.Keys | Sort-Object { [int]$tree[$_][2] })) {
      $tag = if ($desc.Contains($k)) { 'desc' } else { 'other' }
      if ($tag -eq 'desc' -or $tree[$k][1] -match 'conhost|OpenConsole|WindowsTerminal') { L ("  proc[$tag] pid=$k ppid=$($tree[$k][0]) exe=$($tree[$k][1]) t=$($tree[$k][2])ms") }
    }
    if ([ConsoleProbe]::Wins.Count -eq 0) { L "  windows: none" }
    foreach ($k in [ConsoleProbe]::Wins.Keys) {
      $w = [ConsoleProbe]::Wins[$k] -split '\|'
      $owner = (Get-Process -Id ([int]$w[0]) -ErrorAction SilentlyContinue).ProcessName
      if (-not $owner -and $tree.ContainsKey([int]$w[0])) { $owner = $tree[[int]$w[0]][1] }
      L ("  WINDOW pid=$($w[0]) owner=$owner class=$($w[1]) title='$($w[2])' t=$($w[3])ms")
    }
    L ("  output: " + ((Get-Content $of -ErrorAction SilentlyContinue | Select-Object -First 6) -join ' / '))
    # Close any windows we caused so the next run starts clean.
    foreach ($k in [ConsoleProbe]::Wins.Keys) { $w = [ConsoleProbe]::Wins[$k] -split '\|'; if (-not $tree.ContainsKey([int]$w[0])) { continue }; $pn = (Get-Process -Id ([int]$w[0]) -ErrorAction SilentlyContinue).ProcessName; if ($pn -match 'WindowsTerminal|conhost|OpenConsole') { Stop-Process -Id ([int]$w[0]) -Force -ErrorAction SilentlyContinue } }
    Start-Sleep -Milliseconds 800
  }
}

if ($UiZip) {
  $ver = (Get-Content $VersionFile -Raw).Trim()
  Expand-Archive $UiZip "$env:XDG_CACHE_HOME\skillshare\ui\$ver" -Force
  # A caller that kills the PID it started (DETACHED launcher) must stop the relaunched work.
  $launcher = [ConsoleProbe]::Start($Exe, 'ui --no-open --port 19451', $src, 0x8, (Join-Path $Root 'out\cancel-fg.txt'))
  Start-Sleep 5
  $inner = (Get-CimInstance Win32_Process -Filter "ParentProcessId=$launcher AND Name='ss.exe'").ProcessId
  $before = [bool](Get-NetTCPConnection -LocalPort 19451 -State Listen -ErrorAction SilentlyContinue)
  Stop-Process -Id $launcher -Force
  Start-Sleep 2
  $innerAlive = [bool]($inner -and (Get-Process -Id $inner -ErrorAction SilentlyContinue))
  $after = [bool](Get-NetTCPConnection -LocalPort 19451 -State Listen -ErrorAction SilentlyContinue)
  L ""
  L "=== cancel foreground ui (DETACHED): launcher=$launcher inner=$inner listeningBefore=$before innerAliveAfterKill=$innerAlive listeningAfterKill=$after"
  if ($innerAlive) { Stop-Process -Id $inner -Force }
  # A background server started by `ui start` must outlive the launcher that started it.
  $r = [ConsoleProbe]::Run($Exe, 'ui start --no-open --port 19452', $src, 0x8, (Join-Path $Root 'out\ui-start.txt'))
  Start-Sleep 3
  $bg = [bool](Get-NetTCPConnection -LocalPort 19452 -State Listen -ErrorAction SilentlyContinue)
  L "=== background ui start (DETACHED): $r serverListeningAfterLauncherExit=$bg"
  & $Exe ui stop 2>&1 | Out-Null
  Start-Sleep 2
  L "  ui stop exit=$LASTEXITCODE stillListening=$([bool](Get-NetTCPConnection -LocalPort 19452 -State Listen -ErrorAction SilentlyContinue))"
}

$realAfter = if (Test-Path $realCfg) { (Get-ChildItem $realCfg -Recurse -Force | Measure-Object -Property LastWriteTime -Maximum).Maximum } else { 'absent' }
L ""
L "realProfileUnchanged=$("$realBefore" -eq "$realAfter") ($realBefore)"
L "DONE"
$log | Out-File -Encoding ascii $Out
