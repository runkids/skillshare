---
sidebar_position: 2
---

# backup

target 디렉터리의 백업을 생성, 목록 조회, 관리합니다.

```bash
skillshare backup              # Backup all skill targets
skillshare backup claude       # Backup specific target
skillshare backup agents       # Backup all agent targets
skillshare backup --all        # Backup skills + agents
skillshare backup --list       # List all backups
skillshare backup --cleanup    # Remove old backups
skillshare backup --delete 2026-01-19_10-00-00  # Delete one backup
skillshare backup files        # Versions of single files skillshare rewrote
```

## 언제 사용하나요

- 위험한 변경 전에 수동 백업을 생성할 때
- 복구 옵션을 확인하기 위해 기존 백업 목록을 조회할 때
- 오래된 백업을 정리하거나 더 이상 필요 없는 백업 하나를 삭제할 때
- `AGENTS.md`나 `CLAUDE.md` 같은 파일의 이전 버전을 되돌릴 때

## 자동 백업

다음 작업 전에 백업이 **자동으로** 생성됩니다.
- `skillshare sync` (skill target과 agent target)
- `skillshare sync agents` (agent target만)
- `skillshare target remove`

위치: `~/.local/share/skillshare/backups/<timestamp>/` (global), `.skillshare/backups/` (project mode, agent 전용)

각 자동 백업 이후 `--cleanup`과 동일한 정책으로 보존 정책이 자동 적용됩니다. 스냅샷을 직접 정리할 필요는 없습니다.

## 명령

### 백업 생성

```bash
skillshare backup              # All targets
skillshare backup claude       # Specific target
skillshare backup --dry-run    # Preview
```

### 백업 목록 조회

```bash
skillshare backup --list
```

```
Backups  ~/.local/share/skillshare/backups
  2026-01-20_15-30-00  claude, cursor · 4.2 MB
  2026-01-19_10-00-00  claude · 2.1 MB
  2026-01-18_09-00-00  claude, cursor · 4.0 MB

3 backups, 10.3 MB

Next
  skillshare restore <target> --from <timestamp>  roll a target back
```

### 오래된 백업 정리

```bash
skillshare backup --cleanup           # Remove old backups
skillshare backup --cleanup --dry-run # Preview cleanup
```

기본 정리 정책:
- 최근 10개 백업 유지
- 30일보다 오래된 백업 제거
- 총 크기를 500 MB로 제한

개수와 크기 한도는 전역 config에서, 또는 대시보드 **대상 폴더** 탭의 보관 요약에서 바꿀 수 있습니다. `0`은 제한 없음입니다. 30일 한도는 고정이며, 프로젝트 스냅샷은 기본값을 따릅니다.

```yaml
backup:
  max_count: 20      # 기본값 10
  max_size_mb: 1000  # 기본값 500
```

가장 최신 스냅샷은 그 자체만으로 크기 제한을 초과하더라도 항상 유지됩니다 — 복원 지점 없이 남겨지는 일은 없습니다.

이 정책은 모든 `sync` 이후 자동으로 실행되므로, `--cleanup`은 필요할 때 수동으로 정리하는 용도로만 사용하면 됩니다.

### 백업 삭제

```bash
skillshare backup --delete 2026-01-19_10-00-00            # Delete one snapshot
skillshare backup --delete 2026-01-19_10-00-00 --dry-run  # Show what would be deleted
skillshare backup --delete 2026-01-19_10-00-00 -p         # From the project's .skillshare/backups/
```

timestamp는 `--list`에 표시되는 폴더 이름입니다. 스냅샷 전체가 그 안의 모든 target과 함께 삭제됩니다.

### 파일 이력 {#file-history}

skillshare는 단일 파일, 즉 `AGENTS.md`나 `CLAUDE.md` 같은 지침 파일 또는 [공유 파일](/docs/how-to/daily-tasks/sharing-instructions#backups)의 위치를 다시 쓰거나 교체하기 전에 이전 내용을 저장합니다. `backup files`는 이 버전들을 나열하고 복원합니다.

```bash
skillshare backup files                                   # Files with saved versions
skillshare backup files show ~/.claude/CLAUDE.md          # Versions of one file, newest first
skillshare backup files restore ~/.claude/CLAUDE.md origin
skillshare backup files restore ./CLAUDE.md 1769000000000000000.shim --dry-run
```

```
Versions  ~/.claude/CLAUDE.md
  1769000000000000000.edit        2026-01-21 12:53:20  history/edit          2.1 KB  # Team rules
  drift:1768900000000000000.mode  2026-01-20 09:06:40  drift/mode            1.9 KB  # Team rules
  origin                          2026-01-10 08:00:00  origin                1.2 KB  # My notes
```

각 버전에는 ID가 있습니다.

| ID | Kind | Meaning |
|----|------|---------|
| `<time>[.<reason>]` | `history` | skillshare가 파일을 쓰기 전에 저장됨 |
| `drift:<time>[.<reason>]` | `drift` | skillshare가 교체한 사용자의 편집 내용 |
| `origin` | `origin` | 공유 파일을 처음 연결했을 때의 파일이며, 그 위치를 제거하면 자동으로 복원됩니다. 파일이 없었다면 이를 복원하면 현재 파일이 삭제됩니다 |

reason은 skillshare가 하려던 작업을 나타냅니다.

| Kind | Reason | Saved before |
|------|--------|--------------|
| `history` | `convert` | 파일을 `AGENTS.md`로 변환하거나 이름을 바꾸기 전 |
| `history` | `shim` | 프로젝트 파일에 `@AGENTS.md`를 추가하기 전 |
| `history` | `edit` | 대시보드에서 편집하기 전 |
| `history` | `collect` | target의 변경 사항을 공유 파일로 수집하기 전 |
| `history` | `attach` | 처음 연결할 때 공유 파일이 이를 교체하기 전 |
| `history` | `restore` | 이전 버전을 복원하기 전 |
| `history` | `migrate` | 0.23.0에서 폐지된 MCP 설정을 뺀 config를 sync가 저장하기 전. [0.22에서 Pi 업그레이드하기](/docs/reference/commands/mcp#pi-migration) 참고 |
| `drift` | `overwrite` | 파일을 직접 편집한 뒤 **공유 파일로 덮어쓰기**를 선택했을 때 |
| `drift` | `mode` | 위치의 mode를 바꾸기 전 |
| `drift` | `restore` | 위치를 복원하기 전 |

이전 릴리스에서 저장된 버전에는 reason이 없습니다. 파일마다 kind별로 최근 10개가 보관됩니다.

`restore`는 먼저 현재 내용을 reason이 `restore`인 새 버전으로 저장한 뒤, 선택한 버전을 씁니다. 경로가 symlink이면 `--unlink`를 추가하지 않는 한 거부하며, `--unlink`는 링크를 일반 파일로 교체합니다.

`backup files`는 mode를 따릅니다. 프로젝트 안에서(또는 `-p`와 함께) 실행하면 그 프로젝트 안의 파일만 나열하고, `show` / `restore`는 프로젝트 밖의 경로를 거부합니다. `-g`는 모든 파일을 대상으로 합니다. `files`는 하위 명령이므로, 이름이 정확히 `files`인 target을 백업하려면 `skillshare backup -t files`를 사용하세요.

## 대시보드 {#dashboard}

[`skillshare ui`](/docs/reference/commands/ui)의 **설정 › 백업**에는 세 개의 탭이 있습니다.

- **대상 폴더** — 위의 스냅샷을 날짜별로 묶어 보여 줍니다. target 또는 **agents만**으로 필터링할 수 있습니다. 스냅샷을 펼치면 폴더마다 파일 수와 크기가 표시되고, 그중 하나를 **복원**하거나(skill과 agent 항목 모두) **경로 복사**, **이 백업 삭제**를 할 수 있습니다. **지금 백업**과 **오래된 백업 정리**는 `backup` 및 `--cleanup`과 같습니다. 옆의 보관 요약에서 개수와 크기 한도를 바꿀 수 있고, **모두 삭제**는 확인 후 모든 스냅샷을 삭제합니다. 파일, MCP, Hooks 백업은 영향을 받지 않습니다.
- **파일** — 위의 파일 이력입니다. 파일을 고르면 reason과 함께 버전이 표시되며, **미리 보고 복원**은 현재 파일과의 diff 또는 버전 전체를 보여 줍니다. 링크된 위치는 **복원하고 링크 끊기**를 확인한 뒤에만 일반 파일로 교체됩니다.
- **MCP** — MCP 설정을 쓸 때마다 만들어진 백업으로, Agent 설정별로 묶이고 각 백업이 추가, 변경, 제거한 서버가 표시됩니다. **미리 보고 복원**은 **MCP** 페이지와 같은 복원 대화상자를 엽니다(명령줄에서는 [`mcp restore`](/docs/reference/commands/mcp)).

![설정 › 백업 › 파일: 복원 전에 이전 CLAUDE.md 버전 미리 보기](/img/backup-files-preview.png)

project mode에서는 이 페이지가 프로젝트만 다룹니다. `.skillshare/backups/`의 agent 스냅샷, 프로젝트 안의 파일, 프로젝트 MCP 설정의 백업입니다. 삭제된 skill과 agent는 여기에 없으며, **Skills**와 **Agents**의 **휴지통** 탭으로 이동합니다.

## 옵션

| Flag | Description |
|------|-------------|
| `--all` | skill과 agent 모두 백업 |
| `--project, -p` | project mode 사용(`.skillshare/backups/`); **agent 전용** |
| `--global, -g` | global mode 사용(skill의 기본값) |
| `--list, -l` | 모든 백업 목록 조회 (`-p`는 프로젝트 백업) |
| `--cleanup, -c` | 오래된 백업 제거 (`-p`는 프로젝트 백업) |
| `--delete <timestamp>` | 백업 하나를 삭제. `-p`와 함께 사용하면 `.skillshare/backups/`에서 삭제 |
| `--target, -t <name>` | 특정 백업 대상 지정(위치 인자의 대안) |
| `--dry-run, -n` | 변경 없이 미리보기 |

`backup files`에는 자체 옵션이 있습니다: `--project, -p`, `--global, -g`, 그리고 `restore`용 `--unlink`와 `--dry-run, -n`. [파일 이력](#file-history)을 참고하세요.

`backup`은 위치 인자로 kind도 받습니다: `skillshare backup agents`는 백업 범위를 agent target으로 한정합니다.

## 백업 구조

```
~/.local/share/skillshare/backups/
├── 2026-01-20_15-30-00/
│   ├── claude/
│   │   ├── skill-a/
│   │   └── skill-b/
│   └── cursor/
│       ├── skill-a/
│       └── skill-b/
└── 2026-01-19_10-00-00/
    └── claude/
        └── ...
```

존재하는 skill 디렉터리는 target의 mode에 따라 다릅니다. [What Gets Backed Up](#what-gets-backed-up)을 참고하세요.

## 백업 대상 {#what-gets-backed-up}

백업은 `sync`가 파괴할 수 있는 것, 즉 **target에는 존재하지만 source에는 없는 로컬 콘텐츠**만 보호합니다.

- target 안의 일반 파일과 디렉터리는 백업됩니다
- merge-mode target의 skill별 symlink는 백업에서 **제외**됩니다 — source를 가리키는 symlink는 이미 안전한 단일 source of truth이기 때문입니다. `skillshare sync`가 이를 다시 생성합니다

이는 다음을 의미합니다.
- merge mode: 로컬(symlink되지 않은) skill만 백업됩니다. 동기화된 skill은 source에 존재합니다
- copy mode: 관리되는 모든 skill 디렉터리가 백업됩니다(실제 파일이므로)
- symlink mode: 아무것도 백업되지 않습니다(전체 디렉터리가 하나의 symlink이므로)

target에 symlink만 존재하는 경우 백업이 생성되지 않으며, `backup`은 할 일이 없다고 보고합니다 — 비어 있는 복원 지점은 쓸모가 없기 때문입니다.

## 백업과 디스크 공간 {#backups--disk-space}

백업은 source를 복사하지 않으므로 크기가 작게 유지됩니다. 다음 세 가지 메커니즘은 혼동하기 쉬우니 구분해서 이해하세요.

| Mechanism | Scope | What it controls |
|-----------|-------|------------------|
| `.gitignore` in your source | Git 전용 | Git이 추적하는 항목. 무시된 파일도 디스크에는 그대로 존재함 |
| `ignore:` in `config.yaml` | `sync` | `sync`가 target에 복사하는 파일(주로 copy mode). [sync](/docs/reference/commands/sync) 참고 |
| Backup | Snapshot | 로컬 target 콘텐츠만 해당 — symlink 및 그에 따른 source artifact는 제외됨 |

symlink된 skill은 따라가지 않으므로, source skill 안에 있는 무거운 아티팩트(모델 가중치, `.venv`, 브라우저 프로필, 미디어)는 `.gitignore`나 `ignore:`에 언급되었는지 여부와 관계없이 스냅샷에 **절대** 복사되지 않습니다.

보존 정책은 아래 기본 정책으로 모든 `sync` 이후 자동 실행됩니다. 사용량을 직접 확인하려면:

```bash
du -sh ~/.local/share/skillshare/backups   # Total size on disk
skillshare backup --list                   # Per-snapshot sizes
skillshare backup --cleanup --dry-run      # Preview what retention would remove
```

copy-mode target은 스냅샷이 여전히 커질 수 있는 유일한 경우입니다: 실제 파일이므로 skill 디렉터리 아래의 모든 것이 복사됩니다. 런타임 캐시와 대용량 아티팩트를 skill 트리 밖에 두거나, `ignore:`로 제외하여 애초에 target에 도달하지 않도록 하세요.

## Agent 백업 {#agent-backup}

Agent는 skill 백업과 함께 실행되는 자체 백업 흐름을 가지며, 알아둘 만한 두 가지 차이점이 있습니다.

**항목 이름.** Agent 백업은 각 timestamp 디렉터리 안에서 skill 백업과 나란히 `<target>-agents/`에 저장됩니다. 예를 들어 `skillshare backup --all` 실행 후 레이아웃은 다음과 같습니다.

```
~/.local/share/skillshare/backups/2026-01-20_15-30-00/
├── claude/          # Skills backup for claude
├── claude-agents/   # Agents backup for claude
└── cursor/
```

**project mode는 skill과 반대입니다.** project mode(`-p`)에서 `backup`은 skill target 백업을 거부하지만 agent target은 백업**합니다**. `agents` 필터를 잊으면 다음과 같은 오류가 표시됩니다.

```
backup is not supported in project mode (except for agents)
```

따라서 project mode에서는 `skillshare backup -p agents` 또는 `skillshare backup -p --all` 중 하나를 명시해야 합니다. `--list -p`와 `--cleanup -p`는 지정할 필요 없이 `.skillshare/backups/`를 대상으로 합니다.

```bash
skillshare backup agents                  # All agent targets (global)
skillshare backup agents claude           # Only claude's agents
skillshare backup agents -p               # Project agent targets
skillshare backup --all                   # Skills + agents in one shot
```

agent 리소스 모델은 [Agents](/docs/understand/agents)를, 복구는 [restore](/docs/reference/commands/restore)를 참고하세요.

## 참고 항목

- [restore](/docs/reference/commands/restore) — 백업에서 복원
- [sync](/docs/reference/commands/sync) — 자동으로 백업 생성
- [target remove](/docs/reference/commands/target) — 자동으로 백업 생성
