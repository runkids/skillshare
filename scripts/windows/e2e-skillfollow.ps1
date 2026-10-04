# Windows E2E for .skillfollow (proposal 274): followed first-level junctions and directory symlinks.
# For each link kind the current token can create (junction always, directory symlink only with the
# symlink right), it runs one skillshare binary in an isolated home under -Root\<kind> and writes a
# report to -Out with one PASS/FAIL/SKIP line per check plus the raw evidence.
# Usage: powershell -NoProfile -ExecutionPolicy Bypass -File e2e-skillfollow.ps1 `
#          -Exe C:\Users\Public\sstest\ss.exe -Root C:\Users\Public\sstest\sf-full -Out C:\Users\Public\sstest\sf-full.txt [-Extended]
# -Extended adds: doctor checks, plain list with the resolved path, sync idempotency, .skillfollow.local
# union, and declarations naming a file (invalid-target).
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

$script:Kind = '-'
$script:Counts = @{ PASS = 0; FAIL = 0; SKIP = 0 }
function Log([string]$s) { $s | Out-File -Encoding utf8 -Append $Out }
function Section([string]$s) { Log ""; Log "=== [$script:Kind] $s ===" }
function Result([string]$status, [string]$name, [string]$evidence) {
  $script:Counts[$status]++
  Log "$status [$script:Kind] $name"
  if ($evidence) { Log "  evidence: $evidence" }
}
function Check([string]$name, [bool]$ok, [string]$evidence) { Result $(if ($ok) { 'PASS' } else { 'FAIL' }) $name $evidence }
function Run([string[]]$CliArgs) {
  Log "> skillshare $($CliArgs -join ' ')"
  $res = & $Exe @CliArgs 2>&1 | ForEach-Object { "$_" }
  $script:LastRc = $LASTEXITCODE
  $script:LastOut = ($res | Out-String)
  Log $script:LastOut.TrimEnd()
  Log "rc=$script:LastRc"
}
# stdout only, parsed as JSON; stderr is logged separately so warnings cannot break the parse.
function RunJson([string[]]$CliArgs) {
  Log "> skillshare $($CliArgs -join ' ')"
  $errFile = Join-Path $env:TEMP 'stderr.txt'
  $res = & $Exe @CliArgs 2> $errFile
  $script:LastRc = $LASTEXITCODE
  $txt = ($res | Out-String)
  Log $txt.TrimEnd()
  $err = (Get-Content -Raw $errFile -ErrorAction SilentlyContinue)
  if ($err) { Log "stderr: $($err.TrimEnd())" }
  Log "rc=$script:LastRc"
  try { return ($txt | ConvertFrom-Json) } catch { Log "JSON parse error: $($_.Exception.Message)"; return $null }
}
function Git([string]$cmdline) {
  $o = cmd /c "git -c user.email=e2e@example.com -c user.name=e2e -c core.autocrlf=false $cmdline 2>&1"
  Log "> git $cmdline"; if ($o) { Log (($o | Out-String).TrimEnd()) }
  return $o
}
function Tag([string]$p) {
  $fs = (cmd /c "fsutil reparsepoint query `"$p`" 2>&1") | Out-String
  if ($fs -match 'Tag value: (0x[0-9a-fA-F]{8})') { return $Matches[1].ToLower() }
  if ($fs -match '0x[0-9a-fA-F]{8}') { return $Matches[0].ToLower() }
  return 'none'
}
# A dangling junction or symlink still exists; Test-Path follows it and says False.
function Exists([string]$p) { ($null -ne (Get-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue)) -or ((Tag $p) -ne 'none') }
function Inspect([string]$p) {
  $i = Get-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue
  if (-not $i -and (Tag $p) -eq 'none') { Log "$p : MISSING"; return }
  $read = 'n/a'
  try { $c = Get-Content -LiteralPath "$p\SKILL.md" -Raw -ErrorAction Stop; $read = "yes: " + ($c -replace "`r?`n", ' | ').Trim() }
  catch { $read = "NO: " + $_.Exception.Message }
  Log ("{0}`n  LinkType={1} Target={2} Attributes={3}`n  reparseTag={4} readSKILL={5}" -f `
    $p, $i.LinkType, ($i.Target -join ','), $i.Attributes, (Tag $p), $read)
}
# kind: junction (mklink /J) or symlink (mklink /D; mklink for a file target).
function NewLink([string]$kind, [string]$link, [string]$target, [switch]$File) {
  $flag = if ($kind -eq 'junction') { '/J' } elseif ($File) { '' } else { '/D' }
  $o = cmd /c "mklink $flag `"$link`" `"$target`" 2>&1"
  Log "> mklink $flag $link -> $target : $(($o | Out-String).Trim())"
}
function Skill([string]$dir, [string]$name) {
  New-Item -ItemType Directory -Force $dir | Out-Null
  "---`nname: $name`ndescription: Fixture $name`n---`n# $name`n" | Set-Content -NoNewline "$dir\SKILL.md"
}
function Names($list) { (@($list | Where-Object { $_.disabled -ne $true } | ForEach-Object { $_.name }) | Sort-Object) -join ',' }
function TargetNames([string]$t) { (@(Get-ChildItem -LiteralPath $t -Force -ErrorAction SilentlyContinue | Where-Object { $_.Name -notlike '.*' } | ForEach-Object { $_.Name }) | Sort-Object) -join ',' }
function Detail($j) { @($j.details | Where-Object { $_.name -eq 'claude' })[0] }
function Entries($sf) { (@($sf.entries | ForEach-Object { "$($_.name)=$($_.state)" }) | Sort-Object) -join ',' }

if (Test-Path $Root) { cmd /c "rmdir /s /q `"$Root`"" }
Remove-Item -LiteralPath $Out -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $Root | Out-Null

Log "=== environment ==="
Log "exe=$Exe"
Log "version=$((& $Exe version 2>&1 | Out-String).Trim())"
Log "whoami=$(whoami)"
Log "arch(native)=$((Get-ItemProperty 'HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Environment').PROCESSOR_ARCHITECTURE) os=$([Environment]::OSVersion.VersionString)"
Log "devMode=$((Get-ItemProperty HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock -ErrorAction SilentlyContinue).AllowDevelopmentWithoutDevLicense)"
$priv = (whoami /priv | Select-String 'SeCreateSymbolicLinkPrivilege') -join ''
Log ("SeCreateSymbolicLinkPrivilege: " + $(if ($priv) { $priv.Trim() } else { 'absent' }))
New-Item -ItemType Directory -Force "$Root\probe\dir" | Out-Null
$probe = cmd /c "mklink /D `"$Root\probe\dlink`" `"$Root\probe\dir`" 2>&1"
$canSymlink = (Tag "$Root\probe\dlink") -eq '0xa000000c'
Log "directory symlink probe: $(if ($canSymlink) { 'CREATED' } else { 'FAILED (' + (($probe | Out-String).Trim()) + ')' })"
cmd /c "rmdir /s /q `"$Root\probe`""

function Scenario([string]$kind) {
  $script:Kind = $kind
  $KR = "$Root\$kind"; $KH = "$KR\home"
  $env:USERPROFILE = $KH; $env:HOME = $KH
  $env:APPDATA = "$KH\AppData\Roaming"; $env:LOCALAPPDATA = "$KH\AppData\Local"
  $env:TEMP = "$KR\tmp"; $env:TMP = "$KR\tmp"
  foreach ($v in 'XDG_CONFIG_HOME','XDG_DATA_HOME','XDG_STATE_HOME','XDG_CACHE_HOME','SKILLSHARE_CONFIG') { Remove-Item "env:$v" -ErrorAction SilentlyContinue }
  $env:NO_COLOR = '1'
  $S = "$KH\src\skills"; $T = "$KH\.claude\skills"; $E = "$KR\ext"
  New-Item -ItemType Directory -Force $env:APPDATA, $env:LOCALAPPDATA, $env:TEMP, $S, $T, "$env:APPDATA\skillshare" | Out-Null
  Set-Location $KH
@"
source: '$S'
mode: merge
targets:
  claude:
    skills:
      path: '$T'
"@ | Set-Content "$env:APPDATA\skillshare\config.yaml"

  # status --json reports the source from the loaded config; it matches only the config written above.
  Section 'isolation check (status --json source.path)'
  $st = RunJson @('status', '--json')
  if (-not $st -or ($st.source.path.TrimEnd('\') -ine $S)) { Result 'FAIL' 'isolation: config under the test root' "source.path=$($st.source.path)"; return }

  # External trees, outside the source and the target.
  Skill "$E\group\alpha" alpha
  Skill "$E\group\beta" beta
  Skill "$E\group\sub\_nested\keep" keep
  Skill "$E\group\sub\_nested\drop" drop
  "drop`n" | Set-Content -NoNewline "$E\group\sub\_nested\.skillignore"
  Git "init -q `"$E\group\sub\_nested`"" | Out-Null
  Skill "$E\team\review" review
  Skill "$E\team\wip" wip
  "wip`n" | Set-Content -NoNewline "$E\team\.skillignore"
  Git "init -q `"$E\team`"" | Out-Null
  Git "-C `"$E\team`" add -A" | Out-Null
  Git "-C `"$E\team`" commit -q -m init" | Out-Null
  Skill "$E\hidden\secret" secret
  Skill "$E\extra\gamma" gamma
  "not a directory`n" | Set-Content -NoNewline "$E\note.txt"

  NewLink $kind "$S\group" "$E\group"
  NewLink $kind "$S\_team" "$E\team"
  NewLink $kind "$S\undeclared" "$E\hidden"
  foreach ($p in "$S\group", "$S\_team") { Log "entry $p reparseTag=$(Tag $p)" }
  $wantTag = if ($kind -eq 'junction') { '0xa0000003' } else { '0xa000000c' }
  Check "fixture: entries are $kind links ($wantTag)" (((Tag "$S\group") -eq $wantTag) -and ((Tag "$S\_team") -eq $wantTag)) "group=$(Tag "$S\group") _team=$(Tag "$S\_team")"

  Section '1. undeclared first-level links stay invisible'
  $l = RunJson @('list', '--json')
  Check '1 list --json shows no skills before declaring' (($script:LastRc -eq 0) -and (Names $l) -eq '') "rc=$script:LastRc names=[$(Names $l)]"
  $st = RunJson @('status', '--json')
  Check '1 status --json has no source.skillfollow' ($null -ne $st -and $null -eq $st.source.skillfollow) "skillfollow=$($st.source.skillfollow | ConvertTo-Json -Compress)"

  Section '2. declaring the links makes their skills visible'
  "group`n_team`n" | Set-Content "$S\.skillfollow"   # Set-Content writes CRLF; the parser must trim it
  $l = RunJson @('list', '--json')
  $want = '_team__review,group__alpha,group__beta,group__sub___nested__keep'
  Check '2 list --json names (repo and nested-repo .skillignore applied, undeclared hidden)' ((Names $l) -eq $want) "got=[$(Names $l)] want=[$want]"
  if ($Extended) {
    Run @('list', '--no-tui')
    Check '2x plain list shows the followed repo with its resolved path' ($script:LastOut -match '_team' -and $script:LastOut -match [regex]::Escape('ext\team')) ''
  }

  Section '3. status reports the declaration'
  $st = RunJson @('status', '--json')
  $sf = $st.source.skillfollow
  $counts = "active=$($sf.active) local_active=$($sf.local_active) entry_count=$($sf.entry_count) followed_count=$($sf.followed_count) skipped_count=$($sf.skipped_count)"
  Check '3 status --json counts' ($counts -eq 'active=True local_active=False entry_count=2 followed_count=2 skipped_count=0') $counts
  Check '3 status --json entry states' ((Entries $sf) -eq '_team=followed,group=followed') (Entries $sf)
  $rt = @($sf.entries | Where-Object { $_.name -eq '_team' })[0].resolved_target
  Check '3 _team resolved_target is the external repo' ($rt -and ($rt.TrimEnd('\') -ieq "$E\team")) "resolved_target=$rt"
  if ($Extended) {
    $d = RunJson @('doctor', '--json')
    $sfChecks = @($d.checks | Where-Object { $_.name -eq 'skillfollow' })
    Check '3x doctor --json has skillfollow checks' ($sfChecks.Count -ge 1) "count=$($sfChecks.Count) status=$(($sfChecks | ForEach-Object { $_.status }) -join ',')"
    $und = (@($d.checks | Where-Object { $_.name -eq 'undeclared_source_links' } | ForEach-Object { $_.message }) -join ' ')
    Check '3x doctor --json reports the undeclared link' ($und -match 'undeclared') $und
  }

  Section '4. sync links followed skills through logical paths'
  $j = RunJson @('sync', '--json')
  $dt = Detail $j
  Check '4 sync --json linked=4' ($dt.linked -eq 4) "linked=$($dt.linked) error=$($dt.error)"
  Check '4 target contents' ((TargetNames $T) -eq $want) "got=[$(TargetNames $T)]"
  foreach ($n in 'group__alpha', '_team__review', 'group__sub___nested__keep') { Inspect "$T\$n" }
  $a = Get-Item -LiteralPath "$T\group__alpha" -Force -ErrorAction SilentlyContinue
  $aTarget = ($a.Target -join ',')
  Check '4 group__alpha is a junction (0xa0000003)' ((Tag "$T\group__alpha") -eq '0xa0000003') "LinkType=$($a.LinkType) tag=$(Tag "$T\group__alpha")"
  Check '4 group__alpha stores the logical source path, not the external tree' (($aTarget -ilike "*\src\skills\group\alpha") -and ($aTarget -notlike '*\ext\*')) "Target=$aTarget"
  $body = Get-Content -LiteralPath "$T\group__alpha\SKILL.md" -Raw -ErrorAction SilentlyContinue
  Check '4 SKILL.md readable through the target link' ($body -match 'name: alpha') ($body -replace "`r?`n", ' | ')
  $body = Get-Content -LiteralPath "$T\group__sub___nested__keep\SKILL.md" -Raw -ErrorAction SilentlyContinue
  Check '4 nested-repo skill readable through the target link' ($body -match 'name: keep') ($body -replace "`r?`n", ' | ')
  Check '4 ignored skills not synced' (-not (Exists "$T\_team__wip") -and -not (Exists "$T\group__sub___nested__drop")) ''
  if ($Extended) {
    $before = $a.CreationTimeUtc.Ticks
    $j = RunJson @('sync', '--json')
    $dt = Detail $j
    $after = (Get-Item -LiteralPath "$T\group__alpha" -Force).CreationTimeUtc.Ticks
    Check '4x second sync is idempotent (updated=0, junction not recreated)' (($dt.updated -eq 0) -and ($before -eq $after)) "updated=$($dt.updated) recreated=$($before -ne $after)"
    $st = RunJson @('status', '--json')
    $tg = @($st.targets | Where-Object { $_.name -eq 'claude' })[0]
    Check '4x status --json counts the followed links as merged, not local' (($tg.status -eq 'merged') -and ($tg.synced_count -eq 4)) "status=$($tg.status) synced_count=$($tg.synced_count)"
  }

  Section '5. a missing entry pauses prune'
  "group`n_team`n_off`n" | Set-Content "$S\.skillfollow"
  NewLink $kind "$S\_off" "$E\offline"
  # A stale managed-looking target link into the missing entry (sync would normally prune it).
  NewLink 'junction' "$T\_off__c" "$S\_off\c"
  Run @('sync')
  Check '5 sync prints prune paused for _off' ($script:LastOut -match [regex]::Escape('prune paused; unavailable .skillfollow entry: _off')) ''
  Check '5 stale link kept while paused' (Exists "$T\_off__c") "tag=$(Tag "$T\_off__c")"
  $j = RunJson @('sync', '--json')
  Check '5 sync --json details prune_paused names _off' ((@((Detail $j).prune_paused) -join ',') -match '_off') "prune_paused=$(@((Detail $j).prune_paused) -join ',')"
  $st = RunJson @('status', '--json')
  $sf = $st.source.skillfollow
  Check '5 status --json _off is missing' ((Entries $sf) -eq '_off=missing,_team=followed,group=followed') (Entries $sf)
  $pp = @($sf.prune_paused)[0]
  Check '5 status --json prune_paused recovery message' ($pp -match '^prune paused: _off is missing; restore or fix .*_off, or remove _off from \.skillfollow\[\.local\], to resume cleanup$') $pp
  Run @('diff')
  Check '5 diff names the blocking entry' ($script:LastOut -match [regex]::Escape('prune paused; unavailable .skillfollow entry: _off (missing)')) ''

  Section '6. restoring the entry resumes prune'
  Skill "$E\offline\d" d
  Run @('sync')
  Check '6 sync no longer pauses' ($script:LastOut -notmatch 'prune paused') ''
  Check '6 stale link pruned' (-not (Exists "$T\_off__c")) "tag=$(Tag "$T\_off__c")"
  Check '6 restored entry linked' (Exists "$T\_off__d") "tag=$(Tag "$T\_off__d")"

  Section '7. a followed repository refuses --force updates'
  $headBefore = ((Git "-C `"$E\team`" rev-parse HEAD") | Out-String).Trim()
  Run @('update', '_team', '--force', '--dry-run')
  $headAfter = ((Git "-C `"$E\team`" rev-parse HEAD") | Out-String).Trim()
  Check '7 update --force --dry-run refused' (($script:LastOut -match 'followed repository update refused') -and ($script:LastOut -match [regex]::Escape('--force is not allowed for followed repository _team'))) ''
  Check '7 update exits non-zero' ($script:LastRc -ne 0) "rc=$script:LastRc"
  Check '7 HEAD unchanged' ($headBefore -and $headBefore -eq $headAfter) "$headBefore -> $headAfter"

  Section '8. unfollowing prunes the managed links'
  "group`n_off`n" | Set-Content "$S\.skillfollow"
  $j = RunJson @('sync', '--json')
  Check '8 sync --json pruned=1' ((Detail $j).pruned -eq 1) "pruned=$((Detail $j).pruned)"
  $want8 = '_off__d,group__alpha,group__beta,group__sub___nested__keep'
  Check '8 target contents after unfollow' ((TargetNames $T) -eq $want8) "got=[$(TargetNames $T)]"
  Check '8 external repo untouched' (Test-Path "$E\team\review\SKILL.md") ''

  if ($Extended) {
    Section '9. .skillfollow.local forms a union'
    NewLink $kind "$S\localgrp" "$E\extra"
    "localgrp`ngroup`n" | Set-Content "$S\.skillfollow.local"
    $st = RunJson @('status', '--json')
    $sf = $st.source.skillfollow
    Check '9 status --json local_active, duplicate collapsed' (($sf.local_active -eq $true) -and ($sf.entry_count -eq 3)) "local_active=$($sf.local_active) entry_count=$($sf.entry_count) entries=$(Entries $sf)"
    $l = RunJson @('list', '--json')
    Check '9 list includes the local-only entry' ((Names $l) -match 'localgrp__gamma') "names=[$(Names $l)]"
    Run @('sync')
    Check '9 sync links the local-only entry' (Exists "$T\localgrp__gamma") "tag=$(Tag "$T\localgrp__gamma")"

    Section '10. a declaration naming a file is invalid-target'
    "plain file`n" | Set-Content -NoNewline "$S\plainfile"
    NewLink $kind "$S\filelink" "$E\note.txt" -File
    Log "filelink reparseTag=$(Tag "$S\filelink")"
    "group`n_off`nplainfile`nfilelink`n" | Set-Content "$S\.skillfollow"
    $st = RunJson @('status', '--json')
    $sf = $st.source.skillfollow
    $states = Entries $sf
    Check '10 regular file entry is invalid-target' ($states -match 'plainfile=invalid-target') $states
    if ((Tag "$S\filelink") -ne 'none') {
      Check "10 $kind link to a file is invalid-target" ($states -match 'filelink=invalid-target') $states
    } else {
      Result 'SKIP' "10 $kind link to a file is invalid-target" 'link to a file could not be created with this token'
    }
    Run @('sync')
    Check '10 sync pauses prune for invalid-target entries' ($script:LastOut -match 'prune paused; unavailable .skillfollow entry: .*plainfile') ''
    "group`n_off`n" | Set-Content "$S\.skillfollow"
  }
}

Scenario 'junction'
if ($canSymlink) { Scenario 'symlink' }
else { $script:Kind = 'symlink'; Result 'SKIP' 'all directory-symlink scenarios' 'this token cannot create directory symlinks (no SeCreateSymbolicLinkPrivilege, Developer Mode off)' }

Log ''
Log "SUMMARY pass=$($script:Counts.PASS) fail=$($script:Counts.FAIL) skip=$($script:Counts.SKIP)"
Log 'DONE'
