# Run pinned Pi regressions as an interactive desktop user in an isolated UTM kit.
param(
  [Parameter(Mandatory)][string]$Root,
  [Parameter(Mandatory)][ValidateSet('full','basic')][string]$Token,
  [string]$RunName = $Token,
  [string]$NativeRoot = $Root,
  [switch]$InstallNative
)
$ErrorActionPreference = 'Stop'
foreach ($path in @($Root,$NativeRoot)) {
  if (![IO.Path]::GetFullPath($path).StartsWith('C:\Users\Public\sstest\',[StringComparison]::OrdinalIgnoreCase)) { throw 'Use an isolated sstest directory.' }
}
if ($RunName -notmatch '^[a-z0-9-]+$') { throw 'Invalid run name.' }
$run = Join-Path $Root $RunName
$out = Join-Path $Root ('result-' + $RunName + '.txt')
if (Test-Path $run) { throw 'The token test directory already exists; use a fresh kit.' }
New-Item -ItemType Directory -Path $run | Out-Null
Start-Transcript -Path $out -Force | Out-Null
$failed = $false
try {
  Write-Output ('commit=' + (Get-Content (Join-Path $Root 'commit.txt')).Trim())
  Write-Output ('token=' + $Token)
  Write-Output ('nativeArch=' + (Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Environment').PROCESSOR_ARCHITECTURE)
  & whoami.exe /priv
  foreach ($name in @('USERPROFILE','HOME','APPDATA','LOCALAPPDATA','TEMP','TMP')) {
    $path = Join-Path $run $name
    New-Item -ItemType Directory -Path $path | Out-Null
    [Environment]::SetEnvironmentVariable($name,$path,'Process')
  }
  $env:SKILLSHARE_CONFIG = Join-Path $run 'config\config.yaml'
  $env:PI_CODING_AGENT_DIR = Join-Path $run 'agent'
  $env:npm_config_cache = Join-Path $run 'npm-cache'
  $npmUser = Join-Path $run 'npmrc'
  $npmGlobal = Join-Path $run 'npm-globalrc'
  Set-Content -Path $npmUser -Value '' -Encoding ascii
  Set-Content -Path $npmGlobal -Value '' -Encoding ascii
  Remove-Item Env:PI_ROOT -ErrorAction SilentlyContinue
  Set-Location (Join-Path $Root 'src\internal\plugin')
  $test = Join-Path $Root 'plugin.test.exe'
  & $test '-test.v' '-test.count=1' '-test.run=^TestPiNativeLock|^TestPiCLIIsNative|^TestPiExtensionsAccountLauncherGate|^TestPiFilteredImport|^TestPiImport|^TestPiRegistrationWindows' *> (Join-Path $run 'regressions.log')
  Write-Output ('REGRESSIONS_EXIT=' + $LASTEXITCODE)
  if ($LASTEXITCODE -ne 0) { $failed = $true }
  foreach ($version in @('0.99.2','1.0.0')) {
    $prefix = Join-Path $NativeRoot ('native-' + $version)
    if ($InstallNative) {
      New-Item -ItemType Directory -Path $prefix | Out-Null
      & npm.cmd install --prefix $prefix --userconfig $npmUser --globalconfig $npmGlobal --registry https://registry.npmjs.org --ignore-scripts --no-audit --no-fund --save-exact ('@earendil-works/pi-coding-agent@' + $version)
      Write-Output ('INSTALL_' + $version + '_EXIT=' + $LASTEXITCODE)
      if ($LASTEXITCODE -ne 0) { $failed = $true; continue }
    }
    $env:PI_ROOT = Join-Path $prefix 'node_modules\@earendil-works\pi-coding-agent'
    if (!(Test-Path (Join-Path $env:PI_ROOT 'package.json'))) { Write-Output ('NATIVE_' + $version + '=MISSING'); $failed = $true; continue }
    & (Join-Path $prefix 'node_modules\.bin\pi.cmd') --version *> (Join-Path $run ('launcher-' + $version + '.log'))
    Write-Output ('LAUNCHER_' + $version + '_EXIT=' + $LASTEXITCODE)
    if ($LASTEXITCODE -ne 0) { $failed = $true }
    & $test '-test.v' '-test.count=1' '-test.run=^TestPiNativeLockHoldsAgainstPi$|^TestPiProjectOverridesResolveInPi$' *> (Join-Path $run ('native-' + $version + '.log'))
    Write-Output ('NATIVE_' + $version + '_EXIT=' + $LASTEXITCODE)
    if ($LASTEXITCODE -ne 0) { $failed = $true }
  }
} catch {
  $failed = $true
  Write-Output ('ERROR=' + $_)
} finally {
  # Preserve original child streams; export UTF-8 copies for host-side retrieval.
  foreach ($log in Get-ChildItem $run -Filter '*.log') {
    [IO.File]::WriteAllText(($log.FullName + '.utf8'),(Get-Content $log.FullName -Raw),[Text.Encoding]::UTF8)
  }
  Write-Output 'DONE'
  Stop-Transcript | Out-Null
}
if ($failed) { exit 1 }
