---
sidebar_position: 1
---

# doctor

skillshare 설정 환경을 확인하고 문제를 진단합니다.

```bash
skillshare doctor
skillshare doctor -p        # Project mode (.skillshare/config.yaml)
skillshare doctor -g        # global mode 강제
skillshare doctor --json    # CI용 구조화된 JSON 출력
```

```text
skillshare doctor

Environment
✓ Config       ~/.config/skillshare/config.yaml
  Config dir   ~/.config/skillshare
  Data         ~/.local/share/skillshare
  State        ~/.local/state/skillshare
✓ Source       ~/.config/skillshare/skills · 43 skills
✓ Agents       ~/.config/skillshare/agents · 2 agents
  Skillignore  not configured
✓ Links        supported
! Git          not initialized (recommended for backup)
✓ Integrity    27/27 skills verified

Targets
✓ claude    skills  merged · merge · 43 shared
✓           agents  synced · merge · 2/2 linked
✓ cursor    skills  merged · merge · 43 shared, 1 local
✓           agents  synced · merge · 2/2 linked
✓ gemini    skills  merged · merge · 43 shared
…
! gemini will see content from: universal
  ~/.agents/skills ← universal
  suggestion: …
…
✗ claude: 1 broken symlink: frontend__css-review
…

Extras
✓ rules     2 files · 2/2 targets OK
✓ commands  1 file · 1/1 targets OK
✓ team      1 file · 4/4 targets OK

Storage
  Backups      last 2026-09-28_12-41-50 · 10m ago
  Trash        1 item, 247 B · oldest under a day

✗ 6 errors, 4 warnings · 1.2s

Next
  skillshare sync  bring the targets up to date
```

## 사용 시점

- 뭔가 작동하지 않는데 원인을 모를 때
- skillshare 또는 OS를 업그레이드한 후
- 모든 target, git, symlink가 정상인지 확인
- 버그를 신고하기 전 첫 진단 단계

## 확인 항목

```text
skillshare doctor

Environment
✓ Config       ~/.config/skillshare/config.yaml
  Config dir   ~/.config/skillshare
  Data         ~/.local/share/skillshare
  State        ~/.local/state/skillshare
✓ Source       ~/.config/skillshare/skills · 12 skills
✓ Agents       ~/.config/skillshare/agents · 8 agents
✓ Skillignore  2 patterns, 1 skill ignored
✓ Links        supported
✓ Git          initialized with remote
✓ Integrity    12/12 skills verified

Targets
✓ claude    skills  merged · merge · 8 shared, 2 local
✓           agents  synced · merge · 8/8 linked
✓ codex     skills  merged · merge · 8 shared
✓ cursor    skills  copied · copy · 8 managed
✓           agents  synced · merge · 8/8 linked

Extras
✓ commands  3 files · 1/1 targets OK
✓ rules     4 files · 1/1 targets OK

MCP, hooks and plugins
✓ MCP          all 2 servers OK
✓ Hooks        all 1 hook in sync
  Plugins      none configured

Storage
  Backups      last 2026-01-18_09-00-00 · 3d ago
  Trash        empty

Version
✓ CLI          0.23.5
✓ Skill        0.23.5

✓ All checks passed · 0.4s
```

## 수행되는 검사

### Environment

| 검사 항목 | 확인 내용 |
|-------|-----------------|
| Config | config 파일 존재 여부 및 유효성 |
| Source | source 디렉터리 존재 여부 및 읽기 가능 여부 |
| Agents | agents source 디렉터리 존재 여부 (구성된 경우) |
| Skillignore | `.skillignore` (및 `.skillignore.local`) 활성 패턴과 무시된 skill 수 |
| Source link | skills source 바로 아래의 symlink 또는 Windows junction마다 info 한 줄. discovery가 따라가지 않으므로 그 내용은 skillshare에 보이지 않습니다 |
| Links | 시스템이 symlink를 생성할 수 있는지 여부 |
| Git | 저장소 상태 및 remote 구성 |

Source link 검사는 global과 project mode 모두에서 수행됩니다. source 루트는 discovery와 같은 방식으로 해석되며, 바로 아래 항목만 검사합니다. 링크를 따라가거나 내용을 읽지 않습니다. 해당 링크가 없으면 출력이 추가되지 않습니다. 각 링크는 `doctor --json`에서 status `info`인 `undeclared_source_links` 검사로도 나타납니다.

### Targets

각 target은 **skills**와 **agents**(agent가 구성된 경우)에 대한 하위 항목을 보여줍니다:
- Skills: 경로, sync 모드, sync 상태, shared/local 개수
- Agents: sync mode, linked 개수, drift 탐지. Developer Mode가 없는 Windows에서는 `merge`가 `copy`로 표시되며, 최신 상태의 관리되는 복사본은 linked로 집계됩니다. skillshare가 소유하지 않는 내용이 같은 로컬 파일은 유지됩니다. copy fallback에서 agent 개수는 이를 `local preserved`로 따로 표시합니다(예: `0/1 linked, 1 local preserved`).
- 깨진 symlink 없음
- 의도치 않은 local 충돌에 대한 중복 skill 검사:
  - `merge` 모드: 건너뜀 (local skill은 예상된 것)
  - `copy` 모드: manifest로 관리되는 복사본은 무시하며, local 충돌 복사본만 경고
- 유효한 include/exclude glob 패턴
- 해당하는 경우 target별 호환성 힌트 (info 수준) (예시 target 우선순위: `cursor` → `antigravity` → `copilot` → `opencode`; 이 target들이 없으면 힌트 없음)

### Path Overlap

Doctor는 런타임 피커에 도달하기 전에 두 가지 종류의 중복 skill 위험을 표시합니다:

**`shared_target_paths`** — 두 개 이상의 활성화된 target이 동일한 기본 경로로 귀결될 때 발생합니다. 흔한 원인: `universal`과 `~/.agents/skills`에 쓰는 도구(예: `warp`, `witsy`)를 동시에 활성화한 경우.

```text
! Shared path ~/.agents/skills ← universal, warp
```

해결 방법: 중복된 target 중 하나를 비활성화하거나, `skillshare target <name> --path <dir>`로 별도의 경로를 설정하십시오.

경로를 공유하는 target들의 `include`, `exclude` 필터가 서로 다르면, 동기화할 때마다 한 target이 원하는 항목이 추가되고 다른 target이 필터로 제외한 항목이 삭제됩니다. 그래서 폴더가 안정되지 않고 `sync`에 같은 대기 중 변경이 계속 표시됩니다. Doctor는 이 경우를 표시하고, target을 제거하는 대신 하나(`universal`이 포함되어 있으면 그것)만 남기고 나머지는 Skills 동기화를 끄도록 제안합니다:

```text
! Shared path ~/.agents/skills ← codex, universal (different filters, so they undo each other on every sync)
  suggestion: Keep universal syncing skills to ~/.agents/skills and stop the rest with `skillshare target codex --skills=false`.
```

`sync`도 같은 target과 실행할 명령을 출력하며, 대시보드의 동기화 페이지에는 뺄 target의 Skills 동기화를 중지하는 버튼이 있습니다. 설정이 같은 target들이 경로를 공유하는 경우에는 위의 해결 방법이 그대로 적용됩니다.

**`cross_target_discovery`** — 활성화된 target의 런타임이 다른 활성화된 target이 쓰는 디렉터리도 스캔하도록 문서화되어 있을 때 발생합니다. 예를 들어 이전 설정이 남아 있어 `codex`가 여전히 legacy 경로인 `~/.codex/skills`를 가리키는 반면, `universal`은 `~/.agents/skills`에 쓰고 Codex는 이 경로도 읽습니다. 둘 다 활성화하면 Codex가 자신의 콘텐츠에 더해 universal의 콘텐츠까지 보게 됩니다.

```text
! codex will see content from: universal
  ~/.agents/skills ← universal
```

해결 방법: 먼저 스캔하는 쪽 target(위 예시의 `codex`)을 제거하십시오. 해당 런타임은 이미 공유 디렉터리를 읽고 있으며, 다른 도구에는 영향을 주지 않습니다. `skillshare target remove codex --dry-run`으로 미리 확인할 수 있습니다. 대신 writer(`universal`)를 제거하면 `~/.agents/skills`를 읽는 다른 도구에서도 해당 skill이 보이지 않게 됩니다. 스캔하는 쪽 target에 writer가 필터링한 skill이 있는 경우에만 둘 다 유지하고, 런타임 피커에 중복 항목이 나타나는 것을 감수하십시오.

두 검사 모두 순수한 메타데이터 기반입니다 — 구성된 경로와 내장된 `also_scans` 테이블을 읽을 뿐, 파일시스템을 직접 프로빙하지 않습니다.

### Version

- CLI 버전
- skillshare skill 버전 (더 새로운 skill이 공개되면 경고, `skillshare upgrade --skill`)
- 사용 가능한 업데이트 확인

### Skill Integrity

파일 해시 메타데이터가 있는 설치된 skill에 대해, doctor는 설치 이후 파일이 변조되지 않았는지 검증합니다:

- 현재 SHA-256 해시와 저장된 해시를 비교
- skill별로 수정, 누락, 추가된 파일을 보고
- Local skill(`.metadata.json`에 없는 것)은 조용히 건너뜀 — 정상적인 동작
- 메타데이터는 있지만 `file_hashes`가 없는 설치된 skill은 이름과 함께 표시됨

```text
! Integrity    5/6 skills verified
!              _team-repo__api-helper: 1 modified, 1 missing
!              1 skill missing file hashes: _old-repo__legacy-skill
```

### Extras

extras가 구성된 경우 다음을 검증합니다:
- 각 extra의 설정이 유효한지(mode, `flatten`, `as`), 두 extra가 같은 파일을 차지하지 않는지
- 각 extra에 대한 source 디렉터리 존재 여부
- target 디렉터리 접근 가능 여부
- 디렉터리 target 안의 깨진 심볼릭 링크(error)
- source와 다른 파일. `skillshare diff`와 같은 방식으로 판단합니다(warning). `flatten` 또는 `extension`을 설정한 target은 비교하지 않습니다.

```text
✗ rules     → ~/.claude/rules: broken symlink gone.md
!           → ~/.claude/rules: 1 file out of sync (a.md missing in target)
```

### MCP

[`mcp check`](./mcp.md)의 정적 검사를 실행합니다. 참조된 환경 변수가 설정되어 있는지, `command`를 `PATH`에서 찾을 수 있는지, 클라이언트 규칙이 서버를 허용하는지, 모든 항목이 동기화되었는지 확인합니다. Doctor는 호스트를 확인하거나 서버를 시작하지 않습니다. 필요하면 `skillshare mcp check` 또는 `skillshare mcp check --live`를 실행하세요. 구성된 서버가 없으면 `info`로 표시합니다.

```text
✗ MCP          docs: command no-such-mcp-binary was not found on PATH
!              docs → claude: not synced yet; run skillshare sync mcp
```

### Hooks

파일을 쓰지 않고 `skillshare sync hooks`를 미리 봅니다. 미리 보기 실패는 error입니다. sync가 아직 추가, 업데이트 또는 제거할 항목, 네이티브 hooks와의 충돌, 참고용 경고(예: Agent가 문서화하지 않은 이벤트 이름)는 warning입니다. 구성된 hook이 없으면 `info`로 표시합니다.

```text
! Hooks        bash-log → claude: not synced (add)
```

### Plugins

소스를 가져오지 않고 `skillshare sync plugins`를 미리 봅니다. Doctor는 플러그인 패키지가 있을 때만 바인딩된 각 Agent의 네이티브 CLI에 설치된 항목을 묻습니다. 차단된 바인딩(예: Agent의 CLI가 설치되지 않음)과 아직 sync가 필요한 바인딩은 warning입니다. 소스의 새 릴리스 확인은 계속 `skillshare plugin check`가 담당합니다. 구성된 패키지가 없으면 `info`로 표시합니다.

### 기타

- `SKILL.md` 파일이 없는 skill
- Skill 수준 `targets:` 필드 검증 (알 수 없는 target 이름에 대해 경고)
- 마지막 backup 타임스탬프 (global mode)
- Trash 상태 (항목 수, 총 용량, 가장 오래된 항목의 경과 시간)
- target 내 깨진 symlink

:::note Project Mode
project에 `.skillshare/config.yaml`이 있으면 `skillshare doctor`는 자동으로 project mode로 실행됩니다.

Project mode에서는:
- Config/source 검사가 `.skillshare/config.yaml`과 `.skillshare/skills`를 사용
- Trash 상태가 `.skillshare/trash`를 사용
- Backup은 `not used in project mode`로 표시
:::

## 자주 발생하는 문제

### "Needs sync"

Target 모드는 변경되었지만 아직 적용되지 않음:

```bash
skillshare sync
```

### "Not synced"

Target이 source보다 linked skill 수가 적음 (예: 새 skill 설치 후):

```bash
skillshare sync
```

### "Has uncommitted changes"

Tracked repo에 local 변경 사항이 있음:

```bash
cd ~/.config/skillshare/skills/_team-repo
git status
# 변경 사항을 커밋하거나 버림
```

### "Broken symlink"

Source에서 skill이 제거되었지만 symlink는 남아있음:

```bash
skillshare sync  # 고아 symlink를 정리함
```

### "Skills without SKILL.md"

필수 파일이 없는 skill 폴더:

```bash
# 각 skill에 SKILL.md를 추가하거나, 폴더를 제거
skillshare new my-skill  # 올바른 구조 생성
```

### "Link not supported"

`doctor`는 시스템 임시 디렉터리(Windows에서는 `%TEMP%`, 그 외에는 `$TMPDIR` 또는 `/tmp`)에 테스트 폴더 링크를 만듭니다. Windows에서 이 링크는 NTFS junction이며 관리자 권한도 Developer Mode도 필요하지 않으므로, Developer Mode를 켜도 이 오류는 해결되지 않습니다. 메시지의 `junction error:` 줄에 Windows가 거부한 이유가 표시됩니다. 임시 디렉터리가 다음 조건을 만족하는지 확인하세요:

1. FAT32, exFAT, 네트워크 공유가 아닌 로컬 NTFS 드라이브에 있을 것 (junction은 NTFS에서만 동작합니다)
2. 현재 계정에 쓰기 권한이 있고, 백신이나 보안 소프트웨어가 막고 있지 않을 것

이 검사는 파일 링크를 테스트하지 않습니다. Developer Mode가 없으면 단일 파일을 링크하는 agents와 extras는 대신 복사됩니다. 자세한 내용은 [Windows 문제 해결](../../troubleshooting/windows.md#file-links-need-windows-developer-mode-copying-instead)을 참고하세요.

## 문제가 있는 경우의 출력 예시

```
Environment
✓ Config       ~/.config/skillshare/config.yaml
✓ Source       ~/.config/skillshare/skills · 12 skills
✓ Agents       ~/.config/skillshare/agents · 8 agents
✓ Links        supported
! Git          3 uncommitted changes
! Integrity    5/6 skills verified
!              _team-repo__api-helper: 1 modified
! Skills without SKILL.md: test-dir, temp

Targets
✓ claude    skills  merged · merge · 8 shared, 2 local
✓           agents  synced · merge · 8/8 linked
! codex     skills  linked · merge · needs sync
✓ cursor    skills  merged · merge · 6 shared
! claude    1 skill not synced · 2/3 linked
✗ cursor: 2 broken symlinks: old-skill, removed-skill

Storage
  Backups      last 2026-01-18_09-00-00 · 3d ago
  Trash        2 items, 45.2 KB · oldest 3 days

Version
✓ CLI          1.2.0
✓ Skill        0.16.0
  Update       v1.2.0 → v1.3.0 available

✗ 1 error, 5 warnings · 0.6s

Next
  skillshare sync          bring the targets up to date
  brew upgrade skillshare  update to v1.3.0
```

## JSON 출력

CI 파이프라인과 자동화를 위한 기계 판독 가능한 출력에는 `--json`을 사용하십시오:

```bash
skillshare doctor --json
```

```json
{
  "checks": [
    { "name": "source", "status": "pass", "message": "Source: ~/.config/skillshare/skills (12 skills)" },
    { "name": "skillignore", "status": "pass", "message": ".skillignore: 3 patterns, 2 skills ignored", "details": ["test-*", "vendor/", "!important", "---", "test-draft", "vendor/lib"] },
    { "name": "sync_drift", "status": "warning", "message": "claude: 1 skill(s) not synced (7/8 linked)", "details": ["new-skill"] },
    { "name": "shared_target_paths", "status": "warning", "message": "1 shared target path(s) — enabled targets writing to the same directory may produce duplicate skills in runtime pickers", "details": ["~/.agents/skills ← universal, warp"], "suggestions": ["Choose one authoritative target for ~/.agents/skills; preview removing duplicate targets with `skillshare target remove <name> --global --dry-run` (currently: universal, warp)."] },
    { "name": "broken_symlinks", "status": "error", "message": "cursor: 1 broken symlink(s)", "details": ["old-skill"] }
  ],
  "summary": { "total": 14, "pass": 12, "warnings": 1, "errors": 1, "info": 0 },
  "version": { "current": "0.17.4", "latest": "0.18.0", "update_available": true }
}
```

검사 상태: `pass`, `warning`, `error`, `info`. `info` 상태는 통과도 실패도 아닌 정보성 검사(예: `.skillignore`를 찾을 수 없는 경우)에 사용됩니다. Info 검사는 `total`에는 포함되지만 `pass`, `warnings`, `errors`에는 포함되지 않습니다.

일부 warning 검사(예: `shared_target_paths`, `cross_target_discovery`)에는 실행 가능한 해결 단계를 담은 선택적 `suggestions` 배열도 포함됩니다. 제안할 내용이 없으면 이 필드는 생략됩니다.

### Exit Codes

| 조건 | Exit Code |
|-----------|-----------|
| 모든 검사 통과 (또는 warning만 있음) | `0` |
| `error` 상태인 검사가 있음 | `1` |

### CI 예시

```bash
# doctor에서 오류가 발견되면 파이프라인 실패 처리
skillshare doctor --json | jq -e '.summary.errors == 0'

# 알림을 위해 warning 추출
skillshare doctor --json | jq '[.checks[] | select(.status == "warning")]'
```

:::tip Web Dashboard
web dashboard(`skillshare ui`)의 **Health Check** 페이지는 필터 토글과 펼칠 수 있는 상세 정보가 포함된 `doctor --json`의 시각화 버전을 제공합니다.
:::

## 참고

- [status](/docs/reference/commands/status) — 빠른 상태 확인
- [sync](/docs/reference/commands/sync) — sync 문제 해결
- [upgrade](/docs/reference/commands/upgrade) — CLI 및 skill 업데이트
