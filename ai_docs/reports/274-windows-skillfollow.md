# 274 Windows verification: .skillfollow

- Date: 2026-10-04
- Commit: `0da66fde` (built as `head-0da66fde` by `scripts/windows/utm.sh build 0da66fde arm64`)
- Guest: Windows 11 Home ARM64 (10.0.26200), UTM, desktop user logged in
- Developer Mode: off (`devMode=` empty in the probe and in both reports)
- Script: `scripts/windows/e2e-skillfollow.ps1 -Extended`; runbook: `ai_docs/tests/windows_skillfollow_runbook.md`
- Scope: global mode only

| Token | Link kinds run | Result |
|---|---|---|
| full (`sstest-full`, administrator token, `SeCreateSymbolicLinkPrivilege` present) | junction, directory symlink | 80 pass, 0 fail, 0 skip |
| basic (`sstest-basic`, `runas /trustlevel:0x20000`, no symlink right) | junction | 40 pass, 0 fail, 1 skip (directory symlinks cannot be created) |

## Full token (ARM64, `0da66fde`)

```text
=== environment ===
exe=C:\Users\Public\sstest\ss.exe
version=skillshare head-0da66fde
whoami=win-uutr26f90sd\willie
arch(native)=ARM64 os=Microsoft Windows NT 10.0.26200.0
devMode=
SeCreateSymbolicLinkPrivilege: SeCreateSymbolicLinkPrivilege             Create symbolic links                                              Disabled
directory symlink probe: CREATED

PASS [junction] fixture: entries are junction links (0xa0000003)
PASS [junction] 1 list --json shows no skills before declaring
PASS [junction] 1 status --json has no source.skillfollow
PASS [junction] 2 list --json names (repo and nested-repo .skillignore applied, undeclared hidden)
PASS [junction] 2x plain list shows the followed repo with its resolved path
PASS [junction] 3 status --json counts
PASS [junction] 3 status --json entry states
PASS [junction] 3 _team resolved_target is the external repo
PASS [junction] 3x doctor --json has skillfollow checks
PASS [junction] 3x doctor --json reports the undeclared link
PASS [junction] 4 sync --json linked=4
PASS [junction] 4 target contents
PASS [junction] 4 group__alpha is a junction (0xa0000003)
PASS [junction] 4 group__alpha stores the logical source path, not the external tree
PASS [junction] 4 SKILL.md readable through the target link
PASS [junction] 4 nested-repo skill readable through the target link
PASS [junction] 4 ignored skills not synced
PASS [junction] 4x second sync is idempotent (updated=0, junction not recreated)
PASS [junction] 4x status --json counts the followed links as merged, not local
PASS [junction] 5 sync prints prune paused for _off
PASS [junction] 5 stale link kept while paused
PASS [junction] 5 sync --json details prune_paused names _off
PASS [junction] 5 status --json _off is missing
PASS [junction] 5 status --json prune_paused recovery message
PASS [junction] 5 diff names the blocking entry
PASS [junction] 6 sync no longer pauses
PASS [junction] 6 stale link pruned
PASS [junction] 6 restored entry linked
PASS [junction] 7 update --force --dry-run refused
PASS [junction] 7 update exits non-zero
PASS [junction] 7 HEAD unchanged
PASS [junction] 8 sync --json pruned=1
PASS [junction] 8 target contents after unfollow
PASS [junction] 8 external repo untouched
PASS [junction] 9 status --json local_active, duplicate collapsed
PASS [junction] 9 list includes the local-only entry
PASS [junction] 9 sync links the local-only entry
PASS [junction] 10 regular file entry is invalid-target
PASS [junction] 10 junction link to a file is invalid-target
PASS [junction] 10 sync pauses prune for invalid-target entries
PASS [symlink] fixture: entries are symlink links (0xa000000c)
PASS [symlink] 1 list --json shows no skills before declaring
PASS [symlink] 1 status --json has no source.skillfollow
PASS [symlink] 2 list --json names (repo and nested-repo .skillignore applied, undeclared hidden)
PASS [symlink] 2x plain list shows the followed repo with its resolved path
PASS [symlink] 3 status --json counts
PASS [symlink] 3 status --json entry states
PASS [symlink] 3 _team resolved_target is the external repo
PASS [symlink] 3x doctor --json has skillfollow checks
PASS [symlink] 3x doctor --json reports the undeclared link
PASS [symlink] 4 sync --json linked=4
PASS [symlink] 4 target contents
PASS [symlink] 4 group__alpha is a junction (0xa0000003)
PASS [symlink] 4 group__alpha stores the logical source path, not the external tree
PASS [symlink] 4 SKILL.md readable through the target link
PASS [symlink] 4 nested-repo skill readable through the target link
PASS [symlink] 4 ignored skills not synced
PASS [symlink] 4x second sync is idempotent (updated=0, junction not recreated)
PASS [symlink] 4x status --json counts the followed links as merged, not local
PASS [symlink] 5 sync prints prune paused for _off
PASS [symlink] 5 stale link kept while paused
PASS [symlink] 5 sync --json details prune_paused names _off
PASS [symlink] 5 status --json _off is missing
PASS [symlink] 5 status --json prune_paused recovery message
PASS [symlink] 5 diff names the blocking entry
PASS [symlink] 6 sync no longer pauses
PASS [symlink] 6 stale link pruned
PASS [symlink] 6 restored entry linked
PASS [symlink] 7 update --force --dry-run refused
PASS [symlink] 7 update exits non-zero
PASS [symlink] 7 HEAD unchanged
PASS [symlink] 8 sync --json pruned=1
PASS [symlink] 8 target contents after unfollow
PASS [symlink] 8 external repo untouched
PASS [symlink] 9 status --json local_active, duplicate collapsed
PASS [symlink] 9 list includes the local-only entry
PASS [symlink] 9 sync links the local-only entry
PASS [symlink] 10 regular file entry is invalid-target
PASS [symlink] 10 symlink link to a file is invalid-target
PASS [symlink] 10 sync pauses prune for invalid-target entries
SUMMARY pass=80 fail=0 skip=0
DONE
```

## Basic-user token (ARM64, `0da66fde`)

```text
=== environment ===
exe=C:\Users\Public\sstest\ss.exe
version=skillshare head-0da66fde
whoami=win-uutr26f90sd\willie
arch(native)=ARM64 os=Microsoft Windows NT 10.0.26200.0
devMode=
SeCreateSymbolicLinkPrivilege: absent
directory symlink probe: FAILED (You do not have sufficient privilege to perform this operation.)

PASS [junction] fixture: entries are junction links (0xa0000003)
PASS [junction] 1 list --json shows no skills before declaring
PASS [junction] 1 status --json has no source.skillfollow
PASS [junction] 2 list --json names (repo and nested-repo .skillignore applied, undeclared hidden)
PASS [junction] 2x plain list shows the followed repo with its resolved path
PASS [junction] 3 status --json counts
PASS [junction] 3 status --json entry states
PASS [junction] 3 _team resolved_target is the external repo
PASS [junction] 3x doctor --json has skillfollow checks
PASS [junction] 3x doctor --json reports the undeclared link
PASS [junction] 4 sync --json linked=4
PASS [junction] 4 target contents
PASS [junction] 4 group__alpha is a junction (0xa0000003)
PASS [junction] 4 group__alpha stores the logical source path, not the external tree
PASS [junction] 4 SKILL.md readable through the target link
PASS [junction] 4 nested-repo skill readable through the target link
PASS [junction] 4 ignored skills not synced
PASS [junction] 4x second sync is idempotent (updated=0, junction not recreated)
PASS [junction] 4x status --json counts the followed links as merged, not local
PASS [junction] 5 sync prints prune paused for _off
PASS [junction] 5 stale link kept while paused
PASS [junction] 5 sync --json details prune_paused names _off
PASS [junction] 5 status --json _off is missing
PASS [junction] 5 status --json prune_paused recovery message
PASS [junction] 5 diff names the blocking entry
PASS [junction] 6 sync no longer pauses
PASS [junction] 6 stale link pruned
PASS [junction] 6 restored entry linked
PASS [junction] 7 update --force --dry-run refused
PASS [junction] 7 update exits non-zero
PASS [junction] 7 HEAD unchanged
PASS [junction] 8 sync --json pruned=1
PASS [junction] 8 target contents after unfollow
PASS [junction] 8 external repo untouched
PASS [junction] 9 status --json local_active, duplicate collapsed
PASS [junction] 9 list includes the local-only entry
PASS [junction] 9 sync links the local-only entry
PASS [junction] 10 regular file entry is invalid-target
PASS [junction] 10 junction link to a file is invalid-target
PASS [junction] 10 sync pauses prune for invalid-target entries
SKIP [symlink] all directory-symlink scenarios
  evidence: this token cannot create directory symlinks (no SeCreateSymbolicLinkPrivilege, Developer Mode off)
SUMMARY pass=40 fail=0 skip=1
DONE
```

## Key raw evidence (full token)

Sync-created target link when the followed entry is a directory symlink. It is a junction to the logical path:

```text
C:\Users\Public\sstest\sf-full\symlink\home\.claude\skills\group__alpha
  LinkType=Junction Target=C:\Users\Public\sstest\sf-full\symlink\home\src\skills\group\alpha Attributes=Directory, ReparsePoint
  reparseTag=0xa0000003 readSKILL=yes: --- | name: alpha | description: Fixture alpha | --- | # alpha |
```

Prune pause and update refusal:

```text
prune paused: _off is missing; restore or fix C:\Users\Public\sstest\sf-full\junction\home\src\skills\_off, or remove _off from .skillfollow[.local], to resume cleanup
! claude: prune paused; unavailable .skillfollow entry: _off (missing)
✗ followed repository update refused: --force is not allowed for followed repository _team; resolve in `C:\Users\Public\sstest\sf-full\junction\ext\team`
rc=1
```

## FAIL classification

The final runs have no FAIL. Earlier development runs failed on two script bugs, which were fixed before the runs above:

| FAIL | Class | Cause and fix |
|---|---|---|
| `isolation: config under the test root` | script bug | The check grepped `doctor` for `Config:`. Current `doctor` prints `Config` with no colon and folds the home to `~`. The script now compares `status --json` `source.path` with the configured source. |
| `4x status shows claude linked 4/4 without drift` | script bug | The check expected the old `4/4` text; `status` prints `✓ 4 linked`. The script now reads `status --json` (`status=merged`, `synced_count=4`). |

No product bug was found, so there is no `fix(skillfollow)` commit.

## Follow-ups

- Not verified on Windows: project mode's relative links, the Developer Mode relative-symlink branch of `createLink` (proposal §3 case 13, needs Developer Mode on), a linked source root or target parent, the dashboard, and source Git staging.
- `scripts/windows/e2e-file-links.ps1` has the same stale `doctor` `Config:` isolation check, so it now aborts before running. It needs the same `status --json` check. Not changed here.
