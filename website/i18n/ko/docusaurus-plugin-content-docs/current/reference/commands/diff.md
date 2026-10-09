---
sidebar_position: 2
---

# diff

Source와 target 간의 차이점을 표시합니다.

```bash
skillshare diff              # 모든 target(interactive TUI)
skillshare diff claude       # 특정 target
skillshare diff agents       # agent target만
skillshare diff --stat       # file 단위 변경 사항
skillshare diff --patch      # 전체 unified diff
```

```text
skillshare diff --no-tui

claude, claude-work, gemini, opencode, universal
  New       remotion-captions

cursor
  Local only  cursor-shortcuts
  New         remotion-captions

Extras
✓ commands  ~/.claude/commands · in sync
✓ rules     ~/.claude/rules · in sync
✓ rules     ~/.cursor/rules · in sync
✓ team      ~/.codex · in sync
✓ team      ~/.claude · in sync
✓ team      ~/.gemini · in sync
✓ team      ~/notes · in sync

! 6 targets: 6 to sync

Next
  skillshare sync     apply the changes
  skillshare collect  copy local-only skills into source
```

## 인터랙티브 TUI

TTY에서 `diff`는 대화형 화면을 엽니다. 왼쪽에는 target, 오른쪽에는 선택한 target의 차이가 표시되며 파일 단위 diff까지 볼 수 있습니다. 키는 화면 아래쪽에 표시됩니다. 일반 텍스트로 보려면 `--no-tui`를 사용하거나 출력을 파이프하세요.

## 사용 시점

- sync 전에 source와 target 사이에 정확히 무엇이 다른지 확인
- target에만 존재하는 skill(local-only, 아직 수집되지 않음) 찾기
- symlink로 대체할 수 있는 local 사본 식별
- `--stat`으로 file 단위 변경 사항을, `--patch`로 전체 text diff를 확인

## 출력 예시

```
claude
  Local override  local-copy
  Local only      my-local-skill
  New             another-skill, missing-skill

✓ cursor    in sync

! 2 targets: 1 to sync, 1 in sync

Next
  skillshare sync          apply the changes
  skillshare sync --force  also replace local copies
  skillshare collect       copy local-only skills into source
```

### 그룹화된 다중 Target 출력

여러 target의 diff 결과가 동일할 경우, noise를 줄이기 위해 하나의 블록으로 그룹화됩니다.

```
agents, claude
  New       skill-1, skill-2

cursor
  New       skill-1

✓ codex, copilot  in sync
```

결과가 다른 target(예: `include`/`exclude` filter로 인해)은 여전히 별도로 표시됩니다.

## 라벨

| 라벨 | 의미 | 조치 |
|-------|---------|--------|
| New | source에 있지만 target에 없음 | `sync`가 추가함 |
| Restore | target에 있었으나 삭제됨 | `sync`가 복원함 |
| Modified | 콘텐츠 또는 target naming이 변경됨(copy mode) | `sync`가 업데이트함 |
| Renamed | 관리 항목이 이전 `target_naming`이 준 이름을 아직 사용함 | `sync`가 이름을 변경함 |
| Local only, skill kept under old name | 현재 `target_naming`이 skill에 주는 이름을 로컬 폴더가 사용 중이라 skill이 이전 관리 항목에 남음(`name (stays at old-name)`으로 표시) | 폴더 이름을 바꾸거나 삭제한 뒤 `sync` |
| Local override | symlink 대신 local 사본 | `sync --force`로 교체 |
| Orphan | manifest에는 있지만 source에는 없음 | `sync`가 제거함 |
| Local only | target에만 존재, source에는 없음 | `collect`로 가져오기 |

## File 단위 세부 정보

### `--stat`

각 skill 내에서 어떤 file이 다른지 표시합니다.

```bash
skillshare diff --stat
```

```
claude
  Modified  my-skill
            + new-file.md (120 bytes)
            ~ SKILL.md (840 → 912 bytes)
            - old-file.md (64 bytes)
```

### `--patch`

수정된 file에 대한 전체 unified text diff를 표시합니다.

```bash
skillshare diff --patch
```

```
claude
  Modified  my-skill
            ~ SKILL.md (840 → 912 bytes)
            --- SKILL.md
            - old line
            + new line
```

`--stat`과 `--patch` 모두 `--no-tui`(plain text 출력)를 암시합니다.

## Diff가 보여주는 내용

### Merge Mode Target

merge mode(기본값)를 사용하는 target의 경우:
- source에는 있지만 아직 target에 symlink되지 않은 skill을 나열
- symlink 대신 local 사본으로 존재하는 skill을 표시
- target에서 local-only인 skill을 식별(source에는 없음 — sync에 의해 보존됨)

### Copy Mode Target

copy mode를 사용하는 target의 경우:
- source에는 있지만 아직 관리되지 않은 skill을 나열(manifest에 없음)
- checksum 비교를 통해 콘텐츠 변경 사항을 표시
- source에 더 이상 없는 orphan managed 사본을 표시(sync 시 제거됨)
- local-only skill을 식별(source에도 없고 관리되지도 않음)

### Symlink Mode Target

symlink mode를 사용하는 target의 경우:
- symlink가 올바른 source를 가리키는지 단순히 확인
- "in sync"를 표시하거나 잘못된 symlink에 대해 경고

## 사용 사례

### Sync 전에

무엇이 변경될지 확인합니다.

```bash
skillshare diff
# sync가 무엇을 할지 확인한 뒤:
skillshare sync
```

### Local Skill 찾기

target에서 직접 만든 skill을 찾아냅니다.

```bash
skillshare diff claude
# 표시: Local only  my-local-skill

skillshare collect claude  # source로 가져오기
```

### 변경 사항 검사

sync 전에 skill에서 정확히 무엇이 변경되었는지 확인합니다.

```bash
skillshare diff --patch claude   # 전체 text diff
skillshare diff --stat claude    # file 단위 요약
```

### 문제 해결

sync status가 문제를 표시할 때:

```bash
skillshare status          # "needs sync"를 표시
skillshare diff claude     # 정확히 무엇이 다른지 확인
skillshare sync            # 해결
```

## Agent Diff {#agent-diff}

`agents` 키워드를 사용하면 agent target만 diff합니다.

```bash
skillshare diff agents             # agent를 지원하는 모든 target
skillshare diff agents claude      # 특정 target
skillshare diff agents --json      # JSON 출력
```

Agent diff는 누락된 agent(sync 필요), orphan symlink(prune 필요), local-only agent file을 표시합니다. `agents` path가 구성된 target만 포함됩니다. 전체 목록은 [Agents — Supported Targets](/docs/understand/agents#supported-targets)를 참고하세요.

---

## 옵션

| Flag | 설명 |
|------|-------------|
| `--project, -p` | project mode 사용 |
| `--global, -g` | global mode 사용 |
| `--stat` | file 단위 변경 사항 표시(`--no-tui`를 암시) |
| `--patch` | 전체 unified diff 표시(`--no-tui`를 암시) |
| `--no-tui` | plain text 출력(interactive TUI 생략) |
| `--json` | JSON으로 출력(`--no-tui`를 암시) |

## JSON 출력

```bash
skillshare diff --json
```

```json
{
  "targets": [
    {
      "name": "claude",
      "mode": "merge",
      "synced": false,
      "items": [
        {"action": "add", "name": "missing-skill", "kind": "skill", "reason": "source only", "is_sync": true},
        {"action": "modify", "name": "local-copy", "kind": "skill", "reason": "local copy (sync --force to replace)", "is_sync": true},
        {"action": "remove", "name": "my-own-skill", "kind": "skill", "reason": "local only", "is_sync": false},
        {"action": "kept", "name": "prototype", "kind": "skill", "reason": "local folder; the skill stays at _emil-design__skills__prototype", "is_sync": false}
      ],
      "include": [],
      "exclude": []
    }
  ],
  "duration": "0.045s"
}
```

`sync`가 해당 target에서 할 일이 없으면 `synced`는 `true`입니다. `"is_sync": false` 항목(target에만 있는 폴더 등)은 계속 목록에 표시되지만 `false`로 만들지 않습니다. 텍스트 출력에서도 local-only 폴더만 있는 target은 동기화된 것으로 취급합니다.

`action`은 `add`, `modify`, `remove`, `kept` 중 하나입니다. `kept`는 현재 `target_naming`이 skill에 주는 이름을 로컬 폴더가 사용 중이라 skill이 이전 이름으로 남고 sync가 그 폴더를 건드리지 않는다는 뜻입니다.

## 참고

- [sync](/docs/reference/commands/sync) — target으로 sync
- [collect](/docs/reference/commands/collect) — local skill 가져오기
- [status](/docs/reference/commands/status) — 간단한 개요
