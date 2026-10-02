---
sidebar_position: 3
---

# check

변경 사항을 적용하지 않고, 추적 중인 저장소와 설치된 skill에 사용 가능한 업데이트가 있는지 확인합니다.

```bash
skillshare check                      # Check all repos and skills
skillshare check my-skill             # Check a single skill
skillshare check a b c                # Check multiple skills
skillshare check --group frontend     # Check all skills in frontend/
skillshare check x -G backend         # Mix names and groups
skillshare check --json               # Machine-readable output
```

## 언제 사용하나요

### 업데이트 전에

`update`를 실행하기 전에 무엇이 변경될지 미리 확인합니다.

```bash
skillshare check         # See what has updates
skillshare update --all  # Apply updates
skillshare sync          # Distribute changes
```

### CI/CD 파이프라인

CI에서 오래된 skill을 확인합니다.

```bash
result=$(skillshare check --json)
# Parse JSON to detect outdated skills
```

## 동작 내용

`check`는 source 디렉터리를 검사하여 다음 항목의 업데이트 상태를 보고합니다.

1. **추적 중인 저장소** — origin에서 fetch하고, 몇 커밋 뒤처져 있는지 표시
2. **메타데이터가 있는 설치된 skill** — 설치된 버전을 remote HEAD와 비교
3. **오래된(stale) skill** — 업스트림 저장소에서 하위 디렉터리가 삭제된 skill 감지
4. **로컬 경로 설치** — 설치한 경로의 파일을 설치 시 기록된 파일과 비교([로컬 경로 설치](#local-path-installs) 참고)
5. **로컬 skill** — 설치 메타데이터가 없는 skill을 "local source"로 표시(비교할 대상 없음)
6. **skill 수준 `targets` 검증** — SKILL.md의 `targets` frontmatter 필드에 있는 알 수 없는 target 이름에 대해 경고

`update`와 달리 `check`는 어떤 파일도 수정하지 않습니다.

## 로컬 경로 설치 {#local-path-installs}

디스크의 디렉터리에서 설치한 skill(`skillshare install /path/to/skill`)은 해당 경로와 복사한 모든 파일의 해시를 기록합니다. `check`는 그 경로의 파일을 다시 해시하여 다음과 같이 보고합니다.

- **up to date** — 파일이 설치 시점과 일치함
- **update available** — 원본 경로에서 파일이 변경, 추가 또는 삭제됨
- **error** — 원본 경로가 더 이상 존재하지 않음(`local source not found: <path>`)

앱 번들 안의 skill처럼 다른 애플리케이션이 배포하고 업데이트하는 skill에 유용합니다.

```bash
skillshare install /Applications/Surge.app/Contents/Resources/Skills/surge

# 앱 업데이트 후:
skillshare check surge     # → Update available
skillshare update surge    # 경로에서 다시 복사하고 보안 감사를 실행
skillshare sync
```

파일 해시가 기록되기 전에 설치된 skill은 update하거나 다시 설치할 때까지 "local source" 상태로 남습니다. Project mode에서 상대 경로(`./vendor/my-skill` 등)로 설치한 skill은 비교하지 않습니다.

## 출력 예시

```
skillshare check

  Tracked Repos
  ─────────────────────────────────────────
  ✓ _team-skills       up to date
  ⬇ _shared-rules      3 commits behind
  ! _design-system     has uncommitted changes

  Installed Skills (remote)
  ─────────────────────────────────────────
  ✓ pdf                up to date          anthropics/skills
  ⬇ commit             update available    anthropics/skills
  ⚠ old-helper         stale (deleted upstream)
  • local-skill        local source

  ⚠ 1 skill(s) stale (deleted upstream) — run 'skillshare update --all --prune' to remove
  Summary: 1 repo + 1 skill have updates available
  Run 'skillshare update <name>' or 'skillshare update --all'
```

## 특정 skill 확인

전체를 스캔하는 대신 이름으로 하나 이상의 skill을 확인할 수 있습니다.

```bash
skillshare check my-skill                # Single skill
skillshare check skill-a skill-b         # Multiple skills
```

group 디렉터리 안의 업데이트 가능한 모든 skill을 확인하려면 `--group` / `-G`를 사용하세요.

```bash
skillshare check --group frontend        # All skills under frontend/
skillshare check -G frontend -G backend  # Multiple groups
skillshare check my-skill -G frontend    # Mix names and groups
```

위치 인자로 지정한 이름이 (저장소나 skill 자체가 아닌) group 디렉터리와 일치하면 자동으로 확장됩니다.

```bash
skillshare check frontend               # Auto-detected as group
```

메타데이터가 없는(로컬 전용) skill은 group을 확장할 때 건너뜁니다.

## 옵션

| Flag | Description |
|------|-------------|
| `--group`, `-G` `<name>` | group 안의 업데이트 가능한 모든 skill 확인(반복 가능) |
| `--project`, `-p` | project 수준 skill 확인(`.skillshare/`) |
| `--global`, `-g` | global skill 확인(`~/.config/skillshare`) |
| `--json` | JSON으로 출력(스크립팅/CI용) |
| `--help`, `-h` | 도움말 표시 |

:::tip Auto-detection
`--project`와 `--global` 중 어느 것도 지정하지 않으면 skillshare가 자동으로 감지합니다: 현재 디렉터리에 `.skillshare/config.yaml`이 존재하면 project mode를, 그렇지 않으면 global mode를 기본값으로 사용합니다.
:::

## JSON 출력

```bash
skillshare check --json
```

```json
{
  "tracked_repos": [
    {"name": "_team-skills", "status": "up_to_date", "behind": 0, "branch": "main"},
    {"name": "_shared-rules", "status": "behind", "behind": 3, "branch": "develop"}
  ],
  "skills": [
    {"name": "pdf", "source": "anthropics/skills", "version": "a1b2c3d",
     "status": "up_to_date", "installed_at": "2024-06-01T10:00:00Z"},
    {"name": "commit", "source": "anthropics/skills", "version": "x9y8z7w",
     "status": "update_available", "installed_at": "2024-05-15T08:30:00Z"},
    {"name": "old-helper", "source": "anthropics/skills", "version": "d4e5f6g",
     "status": "stale", "installed_at": "2024-03-10T09:00:00Z"},
    {"name": "local-skill", "source": "", "version": "",
     "status": "local", "installed_at": "2024-04-20T12:00:00Z"}
  ]
}
```

`"status": "error"`인 skill에는 원인을 알 수 있을 때 `message` 필드가 포함됩니다. 예: `"message": "local source not found: /path/to/skill"`.

## 상태 표시자

| Icon | Meaning |
|------|---------|
| `✓` | 최신 상태 |
| `⬇` | 업데이트 사용 가능(추적 저장소: 뒤처진 커밋 수; skill: 새 버전) |
| `⚠` | Stale — 업스트림에서 하위 디렉터리가 삭제되거나 이름이 변경됨 |
| `!` | 커밋되지 않은 변경 사항 있음 |
| `•` | 로컬 source(비교할 설치 메타데이터 없음) |

:::info Stale skills
skill의 하위 디렉터리가 업스트림에서 이름이 바뀌거나 삭제된 경우, `check`는 이를 **stale**로 보고합니다. stale skill을 정리하려면 `update --prune`을 사용하세요.
:::

:::tip Monorepo awareness
하위 디렉터리에서 설치된 skill의 경우, `check`는 해당 특정 디렉터리가 변경되었을 때만 "update available"로 보고합니다 — 저장소의 관련 없는 부분에 새 커밋이 있는 경우는 해당하지 않습니다.
:::

## Project Mode

```bash
skillshare check -p                    # Check all project skills
skillshare check -p my-skill           # Check specific project skill
skillshare check -p --group frontend   # Check project group
skillshare check -p --json             # JSON output for project
```

## Agent 지원

`skillshare check agents`는 확인 범위를 agent로 한정하여, agent source 디렉터리의 `.md` 파일에 대한 drift와 업데이트 상태를 보고합니다.

```bash
skillshare check agents              # Check all agents
skillshare check agents --json       # JSON output for agents
skillshare check agents -p           # Check project agents
```

`agents` 인자 없이 실행하면 `check`는 skill에 대해서만 동작합니다(기본 동작). 배경 지식은 [Agents](/docs/understand/agents)를 참고하세요.

## 참고 항목

- [update](/docs/reference/commands/update) — 업데이트 적용
- [list](/docs/reference/commands/list) — 설치된 skill 확인
- [status](/docs/reference/commands/status) — 동기화 상태 표시
- [Agents](/docs/understand/agents) — agent 개념
