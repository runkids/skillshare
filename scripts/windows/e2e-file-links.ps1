# Windows E2E for file links (skills = junctions, file modes = symlink or copy fallback).
# Runs one skillshare binary in an isolated home under -Root and writes a report to -Out.
# Usage: powershell -NoProfile -ExecutionPolicy Bypass -File e2e-file-links.ps1 `
#          -Exe C:\Users\Public\sstest\ss.exe -Root C:\Users\Public\sstest\run1 -Out C:\Users\Public\sstest\run1.txt [-Extended]
# -Extended adds: source edit, agent prune, broken-junction repair, idempotent resync, status/doctor/list.
# The last line of -Out is "DONE" when the script finishes.
param(
  [Parameter(Mandatory)] [string]$Exe,
  [Parameter(Mandatory)] [string]$Root,
  [Parameter(Mandatory)] [string]$Out,
  [switch]$Extended
)
$ErrorActionPreference = 'Continue'
try { [Console]::OutputEncoding = [Text.Encoding]::UTF8 } catch {}
$OutputEncoding = [Text.Encoding]::UTF8

function Log([string]$s) { $s | Out-File -Encoding utf8 -Append $Out }
function Section([string]$s) { Log ""; Log "=== $s ===" }
function Run([string[]]$CliArgs) {
  Log "> skillshare $($CliArgs -join ' ')"
  $res = & $Exe @CliArgs 2>&1 | ForEach-Object { "$_" }
  $script:LastOut = ($res | Out-String)
  Log $script:LastOut.TrimEnd()
  Log "rc=$LASTEXITCODE"
}
function DriftCount {
  @(Get-ChildItem -Recurse -File -Filter *.bak "$env:APPDATA\skillshare\extras\backups" -ErrorAction SilentlyContinue |
    Where-Object { $_.FullName -like '*\drift\*' }).Count
}
function Inspect([string]$p) {
  $i = Get-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue
  if (-not $i) { Log "$p : MISSING"; return }
  $fs = (cmd /c "fsutil reparsepoint query `"$p`" 2>&1") | Out-String
  $tag = if ($fs -match '0x[0-9a-fA-F]{8}') { $Matches[0] } else { 'none' }
  $read = 'n/a'
  if ($p -like '*.md') {
    try { $c = Get-Content -LiteralPath $p -Raw -ErrorAction Stop; $read = "yes: " + ($c -replace "`r?`n", ' | ').Trim() }
    catch { $read = "NO: " + $_.Exception.Message }
  }
  Log ("{0}`n  LinkType={1} Target={2} PSIsContainer={3} Attributes={4}`n  reparseTag={5} read={6}" -f `
    $p, $i.LinkType, ($i.Target -join ','), $i.PSIsContainer, $i.Attributes, $tag, $read)
}

# --- isolation ---
if (Test-Path $Root) { Remove-Item -LiteralPath $Root -Recurse -Force }
$H = Join-Path $Root 'home'
$env:USERPROFILE = $H; $env:HOME = $H
$env:APPDATA = "$H\AppData\Roaming"; $env:LOCALAPPDATA = "$H\AppData\Local"
$env:TEMP = "$Root\tmp"; $env:TMP = "$Root\tmp"
foreach ($v in 'XDG_CONFIG_HOME','XDG_DATA_HOME','XDG_STATE_HOME','XDG_CACHE_HOME','SKILLSHARE_CONFIG') { Remove-Item "env:$v" -ErrorAction SilentlyContinue }
$env:NO_COLOR = '1'
New-Item -ItemType Directory -Force $env:APPDATA, $env:LOCALAPPDATA, $env:TEMP | Out-Null
Set-Location $H

Section 'environment'
Log "exe=$Exe"
Log "whoami=$(whoami)"
Log "arch=$env:PROCESSOR_ARCHITECTURE os=$([Environment]::OSVersion.VersionString)"
$priv = (whoami /priv | Select-String 'SeCreateSymbolicLinkPrivilege') -join ''
Log ("SeCreateSymbolicLinkPrivilege: " + $(if ($priv) { $priv.Trim() } else { 'absent' }))
$pt = "$Root\tmp\probe-target.txt"; 'x' | Out-File $pt
try { New-Item -ItemType SymbolicLink -Path "$Root\tmp\probe-link.txt" -Target $pt -ErrorAction Stop | Out-Null; Log 'file symlink probe: CREATED' }
catch { Log "file symlink probe: FAILED ($($_.Exception.Message.Trim()))" }

# --- fixture ---
$S = "$H\src"
New-Item -ItemType Directory -Force "$S\skills\demo-skill", "$S\agents", "$S\extras\rules", "$S\extras\personal", "$H\.claude", "$H\.codex", "$env:APPDATA\skillshare" | Out-Null
"---`nname: demo-skill`ndescription: Demo skill`n---`nskill body`n" | Set-Content -NoNewline "$S\skills\demo-skill\SKILL.md"
"---`nname: demo-agent`ndescription: Demo agent`n---`nagent v1`n" | Set-Content -NoNewline "$S\agents\demo-agent.md"
"rule v1`n" | Set-Content -NoNewline "$S\extras\rules\style.md"
"shared v1`n" | Set-Content -NoNewline "$S\extras\personal\AGENTS.md"
@"
source: '$S\skills'
agents_source: '$S\agents'
extras_source: '$S\extras'
mode: merge
targets:
  claude:
    skills:
      path: '$H\.claude\skills'
    agents:
      path: '$H\.claude\agents'
extras:
  - name: rules
    targets:
      - path: '$H\.claude\rules'
  - name: personal
    file: AGENTS.md
    targets:
      - path: '$H\.codex'
"@ | Set-Content "$env:APPDATA\skillshare\config.yaml"

# status --json reports the source from the loaded config; it matches only the config written above.
# (doctor prints the config path shortened to ~, so its text cannot prove the path is under $Root.)
Section 'isolation check (status --json source.path)'
$st = try { (& $Exe status --json 2> "$env:TEMP\stderr.txt" | Out-String) | ConvertFrom-Json } catch { $null }
Log "source.path=$($st.source.path)"
if (-not $st -or ($st.source.path.TrimEnd('\') -ine "$S\skills")) { Log 'ABORT: config is not under the test root'; Log 'DONE'; exit 1 }

$paths = @("$H\.claude\skills\demo-skill", "$H\.claude\agents\demo-agent.md", "$H\.claude\rules\style.md", "$H\.codex\AGENTS.md")
$manifests = @("$H\.claude\skills", "$H\.claude\agents", "$H\.claude\rules", "$H\.codex")
function InspectAll {
  foreach ($p in $paths) { Inspect $p }
  foreach ($d in $manifests) { Log ("manifest {0}: {1}" -f $d, (Test-Path "$d\.skillshare-manifest.json")) }
}

Section 'sync --all'
Run @('sync', '--all')
Section 'state after first sync'
InspectAll

if ($Extended) {
  Section '(a) edit shared AGENTS.md source twice, sync each time'
  $driftBefore = DriftCount
  $backedUp = $false
  foreach ($v in 'shared v1b', 'shared v2') {
    "$v`n" | Set-Content -NoNewline "$S\extras\personal\AGENTS.md"
    Run @('sync', '--all')
    if ($script:LastOut -match 'backed up') { $backedUp = $true }
  }
  Inspect "$H\.codex\AGENTS.md"
  Log "drift backups before/after source edits: $driftBefore/$(DriftCount)"
  Log "backed-up message after source edits: $backedUp"

  Section '(b) remove source agent; untracked user agent must stay'
  "user agent`n" | Set-Content -NoNewline "$H\.claude\agents\my-own.md"
  Remove-Item "$S\agents\demo-agent.md"
  Run @('sync', '--all')
  Inspect "$H\.claude\agents\demo-agent.md"
  Inspect "$H\.claude\agents\my-own.md"
  Log ("manifest agents: " + $(if (Test-Path "$H\.claude\agents\.skillshare-manifest.json") { Get-Content -Raw "$H\.claude\agents\.skillshare-manifest.json" } else { 'absent' }))

  Section '(c) broken junction repair'
  Remove-Item -LiteralPath "$H\.codex\AGENTS.md" -Force
  cmd /c "mklink /J `"$H\.codex\AGENTS.md`" `"$S\extras\personal\AGENTS.md`"" 2>&1 | ForEach-Object { Log "$_" }
  Inspect "$H\.codex\AGENTS.md"
  Run @('sync', '--all')
  Inspect "$H\.codex\AGENTS.md"

  Section '(d) skills resync is a no-op'
  $before = (Get-Item -LiteralPath "$H\.claude\skills\demo-skill" -Force).CreationTimeUtc.Ticks
  Run @('sync')
  Run @('sync')
  $after = (Get-Item -LiteralPath "$H\.claude\skills\demo-skill" -Force).CreationTimeUtc.Ticks
  Log "skill junction recreated: $($before -ne $after)"
  Inspect "$H\.claude\skills\demo-skill"

  Section '(e) re-add agent, then status / doctor / list / extras list'
  "---`nname: demo-agent`ndescription: Demo agent`n---`nagent v3`n" | Set-Content -NoNewline "$S\agents\demo-agent.md"
  Run @('sync', 'agents')
  Inspect "$H\.claude\agents\demo-agent.md"
  Log "extras backups:"
  Get-ChildItem -Recurse -File "$env:APPDATA\skillshare\extras\backups" -ErrorAction SilentlyContinue | ForEach-Object { Log "  $($_.FullName)" }
  Run @('status')
  Log ("status agents line: " + (($script:LastOut -split "`r?`n" | Select-String '^\s*agents\s') -join ' || ').Trim())
  Run @('doctor')
  Log ("doctor agents line: " + (($script:LastOut -split "`r?`n" | Select-String '^\s*agents\s') -join ' || ').Trim())
  Run @('list', '--all', '--no-tui')
  Run @('extras', 'list', '--no-tui')
}

Log ''
Log 'DONE'
