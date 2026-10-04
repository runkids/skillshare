---
sidebar_position: 4
---

# .skillfollow（실험적）

Skills source 첫 번째 계층의 symlink 또는 Windows junction을 선언해 외부 그룹/repo를 논리 경로로 검색합니다. 외부 작업 저장소를 원래 위치에 둘 수 있지만, 선언은 skillshare에 파일 소유권이나 쓰기 권한을 주지 않습니다.

## 설정

파일을 **설정된 skills source 루트**에 둡니다. 보통 `~/.config/skillshare/skills/`, Windows는 `%AppData%\skillshare\skills\`, project mode는 `.skillshare/skills/`이며 사용자 지정 `sources.skills`도 지원합니다. Agents/extras에는 적용하지 않고 중첩 repo 루트에서는 읽지 않습니다.

외부 repo에 `.git`과 `review/SKILL.md`가 있지만 루트 자체에는 `SKILL.md`가 없는 예:

```text
~/work/team-skills/
~/.config/skillshare/skills/
├── _team-skills -> ~/work/team-skills/
├── .skillfollow
└── .gitignore
```

첫 계층 링크를 직접 만드세요. macOS/Linux:

```bash
ln -s "$HOME/work/team-skills" "$HOME/.config/skillshare/skills/_team-skills"
```

Windows Command Prompt에서는 symlink 권한 없이 directory junction을 만들 수 있습니다（외부 경로 교체）:

```text
mklink /J "%AppData%\skillshare\skills\_team-skills" "C:\work\team-skills"
```

외부 경로 대신 **항목 이름**을 적으세요:

```text title=".skillfollow"
# One direct child of the skills source per line
_team-skills
```

Skills source `.gitignore`에 루트 기준이며 **끝에 `/`가 없는** 줄을 추가하세요:

```text title=".gitignore"
/_team-skills
/.skillfollow.local
```

Git은 symlink를 디렉터리가 아닌 파일로 저장하므로 `/_team-skills/`로는 부족합니다. `.skillfollow`는 commit하되 링크와 `.skillfollow.local`은 추적하지 마세요. 이미 index에 있으면 경로를 확인한 후 skills source에서 실행하세요. Index 항목만 제거하고 작업 링크는 남깁니다:

```bash
git rm --cached -- '_team-skills'
```

`skillshare doctor`, `skillshare list --no-tui`, `skillshare sync --dry-run`으로 확인하고 올바르면 `skillshare sync`하세요. `-g`/`-p`로 범위를 선택하세요. 선언/ignore는 수동 편집이며 discovery, status, doctor, dry run이 자동 생성/복구하지 않습니다. 아직 `follow`/`unfollow` 명령은 없습니다.

`_` 접두사와 `.git`을 가진 항목은 tracked repo, 그 외는 그룹입니다. Skills는 `_team-skills/review`（flat name `_team-skills__review`）같은 논리 경로를 유지합니다. Source-root/repo `.skillignore`는 계속 적용되며(followed 그룹 안에 중첩된 tracked repo 포함), 중첩된 tracked repo(`--track --into`로 설치한 것 포함)는 자신의 skills를 소유합니다(`list`에 repo 이름, `status`와 Dashboard 집계, `.metadata.json` target override 적용, Dashboard는 단일 skill uninstall 거부). 미선언 첫 계층 링크는 보이지 않습니다.

## 형식

`.skillfollow.local`은 base 파일 옆에서 머신별 이름을 추가합니다. 두 파일은 합집합이며 base 다음 local, 중복은 합칩니다. `.skillignore.local`과 달리 부정/덮어쓰기가 없습니다.

- 앞뒤 공백을 제거하고 빈 줄과 `#`로 시작하는 줄을 무시합니다. 주석은 별도 줄에 쓰세요.
- 직접 자식 이름만 허용합니다. `.`, `..`, 절대 경로, `C:` 같은 drive/volume, UNC, `/`, `\`, path cleaning으로 바뀌는 이름은 거부합니다.
- Glob/부정은 없습니다. `*`, `?`, `[`, `]`, `{`, `}`, `!`, NUL은 거부합니다. 잘못된 줄은 경고하며 followed 항목이 되지 않습니다.
- 선언된 첫 계층 링크만 따라가고 그 트리 안의 중첩 링크는 순회하지 않습니다.

## 상태 및 복구 {#states}

Canonical path로 안전성을 검사하며 처음 해당되는 상태를 사용합니다. 항목 중복은 선언 순서와 무관하게 양쪽을 거부합니다.

| 상태 | 의미 및 조치 |
|---|---|
| `missing` | 없거나 끊어진 링크, 읽기 불가, 안전 경계 해석 실패. 순회 중 읽기 실패도 해당. 드라이브/링크/읽기 권한과 경계를 복구하거나 폐기 선언 제거 |
| `not-link` | 실제 디렉터리로 정상 검색. 일반 소유권은 유지되며 복구 불필요 |
| `invalid-target` | 대상이 디렉터리가 아니거나 항목이 링크/디렉터리가 아님. 디렉터리 링크로 고치거나 선언 제거 |
| `cycle` | 대상이 source 자체, 내부 또는 상위 경로. 별도 외부 디렉터리로 변경 |
| `target-overlap` | 활성 skills target과 같거나 서로 포함. 입력/출력 디렉터리 분리 |
| `inside-git-root` | 유효 Git staging tree 내부 대상. 외부 트리를 밖으로 이동. 링크 ignore로 실제 파일을 숨길 수 없음 |
| `entry-overlap` | 선언 대상이 같거나 서로 포함. 선언 제거/변경으로 중복 해결 |
| `single-skill` | 대상 루트에 `SKILL.md`. 아직 미지원. 상위 그룹/repo 사용 또는 선언 제거 |
| `followed` | 안전하고 읽을 수 있는 그룹/tracked repo. 검색/sync 가능 |
| `undeclared-link` | 두 파일에 없는 첫 계층 링크. 숨겨 두거나 선언과 ignore 추가 |

`followed`/`not-link`는 doctor pass, 다른 선언 상태는 warning이며 정리를 중지합니다. `undeclared-link`는 info만 표시하고 중지하지 않습니다. Parser 경고는 별도입니다.

`.skillfollow` 또는 `.skillfollow.local`이 있지만 읽을 수 없으면 discovery는 불완전한 결과로 계속하지 않고 중단합니다. `sync`는 거부하고 기존 target을 유지하며, `check`와 `status`는 빈 카운트 대신 읽기 오류를 보고하며, `doctor`는 `skillfollow`로 보고하고 discovered skills가 필요한 검사(`skills_validity`, `skill_integrity`, `skill_targets_field`, `sync_drift`)를 빈 source로 판정하지 않고 skipped로 표시하며, 모든 `update`（CLI, Dashboard, `install --update`）는 `--force`로도 거부되고 Dashboard update-all은 전체가 실패하며, source Git staging도 거부됩니다. 파일의 읽기 권한을 복구하거나 파일을 제거하세요. 같은 규칙이 followed 항목 안에도 적용됩니다. 그 아래 그룹을 선택하는 `check`와 `update`(`--group <name>` 또는 위치 인자 그룹 이름)는 그룹 아래 디렉터리를 읽을 수 없으면 읽을 수 있는 부분만 처리하지 않고 `incomplete discovery of <entry>: <read error>`로 거부합니다.

## 명령 표시 {#visibility}

- **Status**: `.skillfollow: N entries, M skipped`, local 활성 시 `(.local active)`, 각 prune 중지 복구 메시지. JSON `source.skillfollow`에는 `active`, `local_active`, `entry_count`, `followed_count`, `skipped_count`, 선언 `entries`（`name`, `state`, 선택 `resolved_target`, `reason`）, 선택 `warnings`/`prune_paused`가 있습니다. 선언/선언 경고가 없으면 생략합니다.
- **Doctor**: `skillfollow`는 선언 상태, `skillfollow_prune`은 정리 차단. 미선언 링크는 `undeclared_source_links` info. Git repo에서는 indexed/`not-ignored` 링크와 안전하지 않은 local 파일도 검사하지만 파일을 수정하지 않습니다.
- **`list --no-tui`**: followed tracked repo에 `→ <resolved>` 추가（홈 경로는 `~`로 축약 가능）. Skills는 논리 경로, JSON 형식은 그대로입니다.
- **Diff**: sync와 같은 규칙으로 미리 봅니다. 선언 항목을 사용할 수 없는 동안 제거를 보고하지 않고 `<target>: prune paused; unavailable .skillfollow entry: <name> (<state>)`를 표시하며, sync가 유지할 standard naming managed copy를 **Kept**로 표시합니다. `diff --json`은 target별 `prune_paused`와 `keep` 항목을 추가합니다. Dashboard diff는 `prune_paused`를 추가하고 유지되는 copy를 `skip`으로 표시합니다. followed orphan link도 sync의 prune과 같은 판단으로 미리 봅니다. followed 항목의 resolved 위치를 가리키는 managed merge link는 해당 skill이 discovery에서 빠지면 `prune`으로, 같은 곳을 가리키는 직접 만든 link는 `local`로 표시됩니다.
- **Dashboard**: Skills, Overview, Check, Update, Audit, Hub에서 논리 경로 표시(audit는 resolved root를 통해 followed skill을 스캔). 내용 편집, uninstall, 토글, target 덮어쓰기, source URL 변경은 거부합니다. 외부 트리를 직접 편집하고 숨기려면 **source-root `.skillignore`**를 사용하세요. 선언 전용 편집기는 아직 없습니다. Dashboard sync는 CLI와 같은 prune/copy 안전 정책이며 target별 `prune_paused`/`kept` 및 경고를 표시합니다. Targets는 managed followed link를 local이 아닌 linked로 계산합니다.

실제 진단 문자열:

```text
_team-skills: not-ignored; add "/_team-skills" to <source>/.gitignore
_team-skills: indexed; run git rm --cached -- '_team-skills' and add "/_team-skills" to <source>/.gitignore
.skillfollow.local: tracked; run git rm --cached -- .skillfollow.local
.skillfollow.local: not-ignored; add "/.skillfollow.local" to <source>/.gitignore
```

## 정리 안전성 {#cleanup}

**어떤 선언 항목이라도** 사용할 수 없으면（`followed`/`not-link` 외）merge/copy의 모든 skills target prune을 중지하며 `sync --force`도 우회하지 못하며, source를 pull한 직후의 init 첫 sync에서도 같습니다. 새 링크/복사본은 생성 가능합니다. Standard naming의 기존 managed copy는 출처를 증명할 수 없으면 교체하지 않고 flat naming은 계속할 수 있습니다. Merge link는 교체할 수 있지만 항목 복귀 시 이름 충돌 가능성을 경고합니다.

Status/doctor는 차단마다 표시합니다:

```text
prune paused: <name> is <state>; restore or fix <path>, or remove <name> from .skillfollow[.local], to resume cleanup
```

복구하거나 이름이 있는 **모든 선언 파일**에서 제거한 후 sync하세요. 폐기된 선언을 남기면 무기한 중지됩니다. 선언 제거는 외부 트리를 삭제하지 않습니다. 논리 source를 통한 managed orphan link는 prune할 수 있지만 follow 해제된 외부 경로를 직접 가리키는 managed link는 보존하며 `managed link resolves outside the source after unfollow; remove it or re-run with --force`로 경고합니다.

## 업데이트 안전성 {#updates}

CLI, Dashboard（all/streaming 포함）, `install --update`는 같은 followed tracked repo 정책을 씁니다. 깨끗한 트리와 **fast-forward-only** pull（`--ff-only --no-rebase`）이 필요합니다. 명시적 `--force`는 dry run에서도 거부합니다. Dirty, status-check error, fast-forward 실패（분기 포함）는 항목별 실패로 `resolve in`과 실제 해결 경로를 표시하고 다른 batch 항목은 계속합니다. 외부 repo에서 해결하고 force로 재시도하지 마세요. 일반 installed repo 정책은 유지됩니다. Agent repo는 이 정책 밖에 있습니다. `.skillfollow`는 skills source의 파일이므로 Dashboard에서 repo 기반 agent를 update할 때는 skills 선언을 읽을 수 없어도 참조하지 않습니다.

followed entry 아래의 일반 skill은 재설치되지 않습니다. `update`는 모든 선택 방식（`--all`, 이름, glob, group, project mode, dry run）과 Dashboard 단일 업데이트·update-all에서 각 항목을 `followed repository update refused: skill <path> is inside followed entry <name>`로 실패 처리하고 다른 항목은 계속합니다. 위 정책을 따르는 followed repository 자체만 업데이트됩니다.

**Audit 실패 시 pull 이전 commit으로 hard-reset합니다.** Resolved root를 스캔하고 논리 경로로 보고하며 scan error도 업데이트를 차단합니다. 업데이트 중 편집, 링크 변경, 다른 Git 실행을 하지 마세요. 검사는 snapshot이지 lock이 아니며 rollback이 동시 변경을 잃게 할 수 있습니다. Skillshare 외부 pull/편집은 자동 audit되지 않으니 `skillshare audit`를 직접 실행하세요.

## Source Git 안전성 {#git-safety}

`commit`, `push`, dry run, Dashboard staging, init source commit은 Git이 도달하는 선언 링크가 indexed/미 ignore이면 거부합니다. Doctor의 정확한 끝 `/` 없는 ignore 줄과 `git rm --cached` 지시를 따르세요. 자동으로 추적 해제하지 않습니다. 물리적 Git 도달성을 사용하므로 skills 선언만으로 무관한 agents/extras repo를 차단하지 않습니다.

Source **pull/reset/checkout**은 indexed 선언（없지만 indexed인 링크 포함）이나 작업 트리 링크 구성 요소를 포함하는 incoming path를 거부합니다. **선언 여부와 무관합니다**. Ignore만으로 Git의 링크 교체를 막을 수 없습니다. 오류의 commit/path를 보고 indexed이면 추적 해제와 ignore, 또는 remote 수정 후 재시도하세요. Pull은 fetch 후 고정 revision을 검사합니다. Dashboard checkout은 선택한 기존 local/remote-tracking revision을 검사하며 암묵적 fetch를 추가하지 않습니다. Dashboard discard는 ignored followed link를 남깁니다.

이 guard는 skillshare 작업만 보호하며 직접 실행한 Git은 보호하지 않습니다.

## 제한

단일 skill, `follow`/`unfollow`, 선언 편집기는 미래 작업입니다. 중첩 링크는 따라가지 않습니다. Developer Mode가 꺼진 Windows 11 ARM64에서 global mode의 discovery, status, sync, prune 일시 중지와 재개, update 거부, unfollow, `.skillfollow.local`, `invalid-target`을 팔로우한 junction(관리자 및 basic-user token)과 directory symlink(관리자 token)로 검증했습니다. project mode의 상대 링크, Developer Mode의 상대 symlink, 링크된 source root나 target 상위 디렉터리, dashboard는 Windows에서 **미검증**입니다.

## 함께 보기

- [필터링](./filtering.md#skillignore)
- [Source 및 Targets](../understand/source-and-targets.md)
- [Update](./commands/update.md)
- [Sync](./commands/sync.md)
