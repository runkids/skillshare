---
sidebar_position: 7
---

# status

skillshare의 현재 상태(source, tracked repository, target, 버전)를 표시합니다.

```bash
skillshare status
```

`follow_source_links: true`이면 사용할 수 없는 최상위 소스 링크의 이름을 경고로 표시하고 정상 스킬은 목록에 유지합니다. `--json`에서는 경고를 stderr로 보내고 stdout은 유효한 JSON으로 유지합니다.

## 사용 시점

- 변경 사항을 적용한 후 모든 target이 sync 상태인지 확인
- 어떤 target에 `sync` 실행이 필요한지 확인
- tracked repo가 최신 상태인지 확인
- 활성 audit policy(profile, threshold, dedupe mode)를 확인
- CLI 또는 skill 업데이트 여부 확인

## 출력 예시

```
Source
  skills    ~/.config/skillshare/skills  43 skills
  agents    ~/.config/skillshare/agents  2 agents

Tracked repositories
✓ _superpowers  15 skills

Targets                        skills                 agents
  claude     ~/.claude/skills  ✓ 43 linked            ✓ 2
  cursor     ~/.cursor/skills  ✓ 43 linked · 1 local  ✓ 2
  gemini     ~/.gemini/skills  ✓ 43 linked            —
  universal  ~/.agents/skills  ✓ 43 linked            —
  all use merge

Extras
  rules     ~/.claude/rules     2 files · merge
  rules     ~/.cursor/rules     2 files · merge
  commands  ~/.claude/commands  1 file · merge
  team      ~/.codex            1 file · symlink
  team      ~/.claude           1 file · import

Audit    default · blocks critical
Version  CLI 0.24.0 · skill 0.21.12
! Skill 0.21.13 is available — run skillshare upgrade --skill && skillshare sync
```

## 섹션

### Source

skills 폴더와 그 안의 skill 수를 표시합니다. agents 폴더가 있으면 두 번째 줄에 폴더와 agent 수를 표시합니다. `.skillignore`가 적용 중이면 패턴 수와 제외된 skill 수를 보여 주는 줄이 추가됩니다.

### Tracked Repositories

`--track`으로 설치한 git 저장소와 각 저장소의 skill 수를 나열합니다. `✓`는 커밋되지 않은 변경이 없다는 뜻입니다. `!`에는 `uncommitted changes`가 붙거나, git status를 읽을 수 없으면 그 오류가 붙습니다.

### Targets

각 target을 한 줄로 표시합니다: 이름, skills 폴더, skills와 agents의 상태입니다. 표 아래 줄에는 사용 중인 sync mode가 표시됩니다.

```
Targets                     skills                agents
  claude  ~/.claude/skills  ✓ 8 linked · 2 local  ✓ 8
  cursor  ~/.cursor/skills  ! 6/8 copied          ! 7/8
  copy: cursor · merge: claude
! 2 skills not synced — run skillshare sync
```

**skills 열:**

| 표시 | 의미 |
|------|------|
| `✓ 8 linked` / `✓ 8 copied` | 예상되는 skill이 모두 있습니다. merge와 copy는 `include`/`exclude`로 거른 뒤의 skill을 셉니다 |
| `· 2 local` | 그 폴더에 있는 직접 만든 skill로, sync가 건드리지 않습니다 |
| `! 6/8 linked` | 일부 skill이 아직 sync되지 않았습니다. status 마지막에 개수와 `sync` 명령이 표시됩니다. `sync`가 의도적으로 건너뛰는 skill(`standard`/`prefixed` naming에서 잘못된 이름이거나 이름 충돌)은 세지 않습니다. `sync`를 다시 실행해도 추가할 수 없기 때문이며, `sync`가 해당 skill을 알려 줍니다 |
| `✓ symlinked` | symlink mode: 폴더 전체가 source에 연결되어 있습니다 |
| `! needs sync` | mode가 바뀌었습니다. `sync`를 실행해 적용하세요 |
| `! has files` / `! not synced yet` | 이 target은 아직 한 번도 sync되지 않았습니다 |
| `✗ links to …` / `✗ broken link` | 폴더가 다른 곳이나 존재하지 않는 곳을 가리키는 링크입니다 |
| `skills off` | 이 target의 skills가 꺼져 있습니다 |

**agents 열:** `✓ 8`은 연결된 agent 수입니다(최신 복사본도 연결된 것으로 셉니다). `! 7/8`은 일부가 빠졌다는 뜻이므로 `skillshare sync agents`를 실행하세요. 이 target이 sync하는 agent만 셉니다. 즉 `.agentignore`, target의 agents include/exclude, 각 agent의 `targets` frontmatter를 거친 뒤 남은 agent입니다. copy fallback에서는 skillshare가 소유하지 않은 동일한 로컬 file을 보존하며 `· 1 local`로 셉니다. `—`는 그 target에 agents 폴더가 없다는 뜻입니다. agents source가 없으면 이 열은 생략됩니다.

### Extras

extras가 설정되어 있으면 extra target마다 한 줄씩 표시합니다:

```
Extras
  rules     .cursor/rules     4 files · merge
  commands  .claude/commands  3 files · merge
```

각 줄에는 extra 이름, target 폴더, file 수, 그리고 file이 실제로 sync되는 mode가 표시됩니다: 개발자 모드가 꺼진 Windows에서는 file을 연결하는 target이 `copy`로 표시됩니다.

### Audit

활성 audit policy(CLI flag, project 설정, global 설정에서 결정)를 한 줄로 표시합니다: profile(`default`, `strict`, `permissive`)과 설치를 차단하는 최저 심각도(기본값 `critical`)입니다. dedupe mode와 analyzer는 기본값(`global`과 모든 analyzer)과 다를 때만 표시됩니다.

### Version

CLI와 skill 버전을 표시합니다. 새 skill이 출시되면 업데이트 방법을 알려 주는 줄이 추가됩니다. (global mode 전용)

## 옵션

| Flag | 설명 |
|------|-------------|
| `--json` | JSON으로 출력(scripting/CI용) |
| `--project, -p` | project mode 사용 |
| `--global, -g` | global mode 사용 |
| `--help, -h` | 도움말 표시 |

## JSON 출력

```bash
skillshare status --json
```

```json
{
  "source": {
    "path": "~/.config/skillshare/skills",
    "exists": true,
    "skillignore": {
      "active": true,
      "files": [".skillignore", "_team-skills/.skillignore"],
      "patterns": ["test-*", "vendor/"],
      "ignored_count": 2,
      "ignored_skills": ["test-draft", "vendor/lib"]
    }
  },
  "skill_count": 12,
  "tracked_repos": [
    {"name": "_team-skills", "skill_count": 5, "dirty": false},
    {"name": "_personal-repo", "skill_count": 3, "dirty": true}
  ],
  "targets": [
    {
      "name": "claude",
      "path": "~/.claude/skills",
      "mode": "merge",
      "status": "merged",
      "synced_count": 8,
      "include": [],
      "exclude": []
    }
  ],
  "agents": {
    "source": "~/.config/skillshare/agents",
    "exists": true,
    "count": 8,
    "targets": [
      {"name": "claude", "path": "~/.claude/agents", "expected": 8, "linked": 8, "drift": false}
    ]
  },
  "audit": {
    "profile": "DEFAULT",
    "threshold": "CRITICAL",
    "dedupe": "GLOBAL",
    "analyzers": []
  },
  "version": "0.17.0"
}
```

`warning` 문자열은 sync가 해당 target을 거부할 때만 붙습니다(예: copy 이외의 mode에서 `prefixed` naming). 문구에 해결 방법이 포함됩니다.

git status를 읽을 수 없는 tracked repo는 `"status": "unknown"`이 되고 `message`에 오류가 담깁니다. 이때 `dirty`는 false이며 의미가 없습니다.

`source.skillignore` 필드는 `.skillignore` 또는 `.skillignore.local` file이 하나 이상 존재할 때만 나타납니다. 없을 경우: `"skillignore": { "active": false }`. `files` 배열은 존재하는 경우 `.skillignore.local` 경로를 포함합니다. text mode에서는 `.skillignore.local`이 적용 중이면 `.skillignore` 줄에 `(.local active)`가 표시됩니다.

JSON 출력은 global mode와 project mode 모두에서 지원됩니다.

## Project Mode

project 디렉터리에서는 status가 project의 source, targets, extras를 project 루트 기준 상대 경로로 표시합니다.

```bash
skillshare status        # .skillshare/가 존재하면 자동 감지
skillshare status -p     # 명시적 project mode
```

### 출력 예시

```
Source
  skills    .skillshare/skills  3 skills
  agents    .skillshare/agents  4 agents
  .skillignore: 3 patterns, 0 skills ignored

Targets                   skills      agents
  claude  .claude/skills  ✓ 3 linked  ✓ 4
  cursor  .cursor/skills  ✓ 3 linked  ✓ 4
  all use merge

Extras
  rules     .cursor/rules     4 files · merge
  commands  .claude/commands  3 files · merge

Audit    default · blocks critical
```

Project status는 Tracked Repositories나 Version 섹션을 표시하지 않습니다(이들은 global 전용 기능입니다).

## 참고

- [sync](/docs/reference/commands/sync) — target으로 skill sync
- [diff](/docs/reference/commands/diff) — 상세한 차이점 표시
- [doctor](/docs/reference/commands/doctor) — 문제 진단
- [Project Skills](/docs/understand/project-skills) — project mode 개념
