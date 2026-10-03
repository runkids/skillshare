---
sidebar_position: 4
---

# trash

trash 디렉터리에서 제거된 skill과 agent를 관리합니다.

```bash
skillshare trash list                    # interactive TUI(TTY에서)
skillshare trash list --no-tui           # plain text 출력
skillshare trash restore my-skill        # trash에서 복원
skillshare trash restore my-skill -p     # project mode로 복원
skillshare trash delete my-skill         # trash에서 영구 삭제
skillshare trash empty                   # trash 비우기
skillshare trash agents list             # trash된 agent 나열
skillshare trash agents restore tutor    # trash에서 agent 복원
skillshare trash --all list              # trash된 skill + agent 나열
```

## 사용 시점

- 최근에(7일 이내) 제거한 skill이나 agent를 복구
- trash된 항목을 영구 삭제해 공간 확보
- 자동 만료되기 전에 trash에 무엇이 있는지 확인

## 인터랙티브 TUI

TTY에서 `trash list`는 휴지통 항목을 최신순으로 대화형 목록에 엽니다. 하나 이상을 선택해 복원하거나 영구 삭제할 수 있고, 휴지통 전체를 비울 수도 있으며, 모두 실행 전에 확인합니다. 항목을 열면 파일이 표시되어 복원 전에 내용을 확인할 수 있습니다. 키는 화면 아래쪽에 표시됩니다. `--all`을 사용하거나 종류를 지정하지 않으면 skill과 agent가 함께 표시됩니다.

일부 항목이 실패해도(예: 복원하려는 skill 이름이 source에 이미 있는 경우) 나머지 항목은 계속 처리되고, 결과에 실패한 항목이 표시됩니다.

TUI를 건너뛰고 plain text로 출력하려면 `--no-tui`를 사용하세요.

```bash
skillshare trash list --no-tui           # plain text 출력
skillshare trash list --no-tui | less    # 수동으로 pager에 pipe
```

## 종류 Filter

기본적으로 trash는 **skill**에 대해 동작합니다. agent를 대상으로 하려면 `agents` positional 키워드를, 둘 다 포함하려면 `--all`을 사용하세요.

```bash
skillshare trash list                    # skill만(기본값)
skillshare trash agents list             # agent만
skillshare trash --all list              # skill과 agent 모두
skillshare trash agents restore tutor    # trash된 agent 복원
skillshare trash agents empty            # agent trash만 비우기
```

## 하위 명령어

### list(alias: `ls`)

현재 trash에 있는 모든 항목을 표시합니다. 터미널에서는 interactive TUI를 실행하고, `--no-tui` 또는 non-TTY에서는 plain text를 출력합니다.

```bash
skillshare trash list
skillshare trash agents list
skillshare trash --all list --no-tui
```

Plain text 출력:

```
Trash
  my-skill      1.2 KB · 2d ago
  old-helper    800 B · 5d ago

2 items, 2.0 KB
  Each item is removed for good 7 days after it was trashed
```

### restore

가장 최근에 trash된 버전을 source 디렉터리로 복원합니다.

```bash
skillshare trash restore my-skill
skillshare trash agents restore tutor
```

```
✓ Restore   my-skill → ~/.config/skillshare/skills · trashed 2d ago

Next
  skillshare sync  link it into your targets again
```

agent의 경우, restore 힌트는 대신 `skillshare sync agents`를 제안합니다.

동일한 이름의 항목이 source에 이미 존재하면 restore는 실패합니다. 기존 항목을 먼저 제거하거나 다른 이름을 사용하세요.

### delete(alias: `rm`)

trash에서 단일 항목을 영구적으로 삭제합니다.

```bash
skillshare trash delete my-skill
skillshare trash agents delete tutor
```

```
✓ Permanently deleted my-skill
```

### empty

trash의 모든 항목을 영구적으로 삭제합니다(확인 프롬프트 포함).

```bash
skillshare trash empty
skillshare trash agents empty
```

```
! This will permanently delete 3 items from trash
? Continue? [y/N] y
✓ Emptied trash: 3 items permanently deleted · 0.1s
```

## Backup vs Trash

이 두 안전 장치는 서로 다른 대상을 보호합니다.

| | backup | trash |
|---|---|---|
| **보호 대상** | target 디렉터리(sync 스냅샷) | source skill과 agent(uninstall) |
| **위치** | `~/.local/share/skillshare/backups/` | `~/.local/share/skillshare/trash/`(skill), `.../trash/agents/`(agent) |
| **트리거** | `sync`, `target remove` | `uninstall` |
| **복원 방법** | `skillshare restore <target>` | `skillshare trash restore <name>` |
| **자동 정리** | 수동(`backup --cleanup`) | 7일 |

## 옵션

| Flag | 설명 |
|------|-------------|
| `agents` | Positional 키워드 — skill 대신 agent에 대해 동작 |
| `--all` | skill과 agent 모두 포함 |
| `--no-tui` | interactive TUI를 비활성화하고 plain text 출력 사용 |
| `--project, -p` | project 수준 trash(`.skillshare/trash/`) 사용 |
| `--global, -g` | global trash 사용 |
| `--help, -h` | 도움말 표시 |

## 자동 정리

만료된 trash 항목(7일 초과)은 `uninstall` 또는 `sync`를 실행할 때 자동으로 정리됩니다. cron이나 예약 작업이 필요하지 않습니다.

## 참고

- [uninstall](/docs/reference/commands/uninstall) — skill 제거(trash로 이동)
- [backup](/docs/reference/commands/backup) — target 디렉터리 backup
- [restore](/docs/reference/commands/restore) — backup에서 target 복원
