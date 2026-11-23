---
sidebar_position: 5
---

# enable / disable

skill을 제거하지 않고 일시적으로 활성화하거나 비활성화합니다.

```bash
skillshare disable draft-*          # Disable by pattern
skillshare enable draft-*           # Re-enable
skillshare disable "frontend/**"    # Disable every skill in a folder
skillshare disable my-skill -p      # Project mode
```

## 언제 사용하나요

- skill을 제거하지 않고 sync에서 일시적으로 숨길 때
- 초안이나 실험적인 skill을 모든 target에서 음소거할 때
- list TUI에서 `t` 키로 skill을 켜고 끌 때

## 동작 방식

`disable`은 `.skillignore`에 패턴을 추가하고, `enable`은 이를 제거합니다. 비활성화된 skill은 source 디렉터리에 그대로 남지만 `sync`와 `collect`에서는 제외됩니다.

skill 이름은 그 이름의 폴더에 있는 모든 skill을 가리키므로, `enable inner`는 `group/inner`와 `other/inner` 모두에 적용됩니다. `group/inner` 같은 정확한 경로나 `group__inner` 같은 flat name은 그 skill 하나만 가리킵니다. `enable`은 이름이 가리키는 각 skill에 대해 그 경로의 줄(list TUI와 대시보드가 쓰는 줄)도 제거하므로, `enable inner`는 `group/inner`도 제거합니다. 각 `✓` 행에는 쓰거나 제거한 줄이 표시됩니다. `disable`은 그 줄이 skill과 일치하면 입력한 이름을 그대로 쓰고, 어떤 줄로도 일치시킬 수 없는 flat name이면 대신 skill의 경로를 씁니다.

```mermaid
flowchart LR
    DIS["skillshare disable my-skill"]
    IGN[".skillignore += my-skill"]
    SYNC["sync skips my-skill"]
    DIS --> IGN --> SYNC
```

```mermaid
flowchart LR
    EN["skillshare enable my-skill"]
    IGN[".skillignore -= my-skill"]
    SYNC["sync includes my-skill"]
    EN --> IGN --> SYNC
```

:::tip
활성화하거나 비활성화한 후에는 `skillshare sync`를 실행해 변경 사항을 target에 적용하세요.
:::

## 옵션

| Flag | Description |
|------|-------------|
| `<name\|pattern>` | 하나 이상의 skill 이름 또는 glob 패턴(예: `draft-*`, `frontend/**`) |
| `--project, -p` | project `.skillignore` 사용(`.skillshare/skills/.skillignore`) |
| `--global, -g` | global `.skillignore` 사용(`~/.config/skillshare/.skillignore`) |
| `--dry-run, -n` | 작성하지 않고 미리보기 |
| `--help, -h` | 도움말 표시 |

`-p`와 `-g` 둘 다 지정하지 않으면 다른 명령과 마찬가지로 mode가 자동 감지됩니다.

## 예시

```bash
# Disable a single skill
$ skillshare disable my-draft
✓ my-draft  added to .skillignore

Next
  skillshare sync  apply the change

# Disable by glob pattern
$ skillshare disable "experimental-*"
✓ experimental-*  added to .skillignore

Next
  skillshare sync  apply the change

# Re-enable
$ skillshare enable my-draft
✓ my-draft  removed from .skillignore

Next
  skillshare sync  apply the change

# Preview without writing
$ skillshare disable my-skill --dry-run
  my-skill  would be added to ~/.config/skillshare/skills/.skillignore

Dry run — nothing was written

# 미리보기는 enable이 제거하는 경로 줄을 포함해 모든 줄을 보여 줍니다
$ skillshare enable inner --dry-run
  inner        would be removed from ~/.config/skillshare/skills/.skillignore
  group/inner  would be removed from ~/.config/skillshare/skills/.skillignore

Dry run — nothing was written

# Already disabled
$ skillshare disable my-draft
! my-draft  already disabled
```

## 다른 규칙이 상태를 결정하는 경우

`disable`과 `enable`은 한 줄만 쓰거나 지우지만, 다른 규칙이 skill의 상태를 결정할 수 있습니다. `draft-*` 같은 glob, `frontend/` 같은 폴더 규칙, `.skillignore.local`의 `!` 규칙, tracked repo 자체의 `.skillignore` 등입니다. 쓰고 나면 skillshare는 지정한 각 skill이나 agent를 `skillshare list`가 보여 주는 상태로 확인합니다. 여전히 이전 상태라면 수정할 파일을 알려 주는 `✗` 행을 출력하고 상태 코드 1로 종료합니다.

```bash
$ skillshare enable draft-one

✗ could not enable draft-one: still disabled: another rule in .skillignore or .skillignore.local matches it — remove that rule to enable it
```

여러 개가 막히면 각각 `✗` 행이 출력됩니다. 적용된 변경은 그대로 유지되어 `✓`로 표시되고, 작업 로그에는 `partial`로 기록됩니다. `disable`이 쓴 줄이 지정한 모든 skill에서 `!` 규칙에 덮어써지면 그 줄은 다시 제거되어 ignore 파일이 원래대로 유지됩니다.

`"draft-*"` 같은 glob 인자는 특정 skill 하나를 가리키지 않으므로 이 확인 없이 쓰거나 지웁니다.

## 폴더 전체 비활성화

`disable`/`enable`은 `.skillignore`와 동일한 glob 문법을 사용하므로 별도의 "group" flag가 없습니다 — 패턴을 폴더로 지정하면 그 안의 모든 skill이 한 번에 토글됩니다.

```bash
# Disable every skill under frontend/ (any depth)
$ skillshare disable "frontend/**"
✓ frontend/**  added to .skillignore

Next
  skillshare sync  apply the change

# Re-enable the whole folder
$ skillshare enable "frontend/**"
✓ frontend/**  removed from .skillignore

Next
  skillshare sync  apply the change
```

:::tip Quote the pattern
쉘이 skillshare에 전달되기 전에 `*`를 확장하지 않도록, 폴더 패턴은 항상 따옴표로 감싸세요(`"frontend/**"`).
:::

`frontend/**`는 `.skillignore`에 한 줄을 작성하며, 이후 폴더에 추가하는 항목도 계속 커버합니다. **동일한** 패턴으로 `enable`을 실행하면 그 줄이 제거됩니다. 개별 skill을 비활성화하려면 이름으로 나열하세요(`skillshare disable a b c`). 전체 glob 참고 자료(`*`, `**`, `?`, `[abc]`, `!negation`, anchored `/`, directory-only `pattern/`)는 [.skillignore pattern syntax](/docs/reference/filtering#skillignore)를 참고하세요.

## TUI 토글

`skillshare list` TUI에서도 skill을 켜고 끌 수 있습니다. 변경 사항은 즉시 `.skillignore`에 기록됩니다. 다른 규칙 때문에 skill이 이전 상태로 남으면 행은 바뀌지 않고, 키 줄에 CLI와 같은 `✗` 메시지가 표시됩니다. TUI나 대시보드에서 skill을 켜면 `skillshare disable <name>`이 쓰는 폴더 이름 줄도 함께 제거됩니다. 단, 그 줄이 켜지 않는 다른 skill에도 일치하면 남겨 둡니다.

## .skillignore는 어디에 있나요?

| Mode | Path |
|------|------|
| Global | `~/.config/skillshare/skills/.skillignore` |
| Project | `.skillshare/skills/.skillignore` |

이 파일은 첫 `disable` 실행 시 자동으로 생성됩니다.

## Agent 지원

agent를 활성화하거나 비활성화하려면 `--kind agent`를 사용하세요. 이 경우 `.skillignore` 대신 `.agentignore`에 기록됩니다.

```bash
skillshare disable --kind agent draft-reviewer     # Disable an agent
skillshare enable --kind agent draft-reviewer      # Re-enable an agent
skillshare disable --kind agent "experimental-*"   # Disable by pattern
```

`.agentignore`의 각 줄은 `.md`를 포함한 파일 이름과 매칭됩니다. `.md` 없는 이름이 같은 파일 이름의 agent를 가리키면 `.md`를 붙여 쓰므로, `draft-reviewer`는 `draft-reviewer.md`로 기록됩니다. agent의 frontmatter `name`을 쓰면 대신 그 경로를 쓰므로, `rev.md`에 대한 `disable --kind agent critic`은 `rev.md`를 기록합니다.

| Mode | `.agentignore` path |
|------|---------------------|
| Global | `~/.config/skillshare/agents/.agentignore` |
| Project | `.skillshare/agents/.agentignore` |

agent 관리에 대한 배경 지식은 [Agents](/docs/understand/agents)를 참고하세요.

## 참고 항목

- [list](./list.md) — 비활성화된 skill 확인 및 `t` 키로 토글
- [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills) — 모든 필터링 레이어
- [.skillignore](/docs/reference/filtering#skillignore) — 패턴 문법
- [sync](./sync.md) — enable/disable 후 변경 사항 적용
- [Agents](/docs/understand/agents) — agent 개념
