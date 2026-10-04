---
sidebar_position: 7
---

# unfollow

skills source의 첫 번째 단계 링크를 더 이상 따라가지 않습니다. 링크가 가리키는 대상은 절대 건드리지 않습니다.

```bash
skillshare unfollow _team-skills               # Remove the declaration and the link
skillshare unfollow _team-skills --keep-link   # Remove the declaration only
skillshare unfollow _team-skills --local       # Remove it from .skillfollow.local only
skillshare unfollow _team-skills -p            # Project mode
```

## 동작 방식

1. `<name>`을 포함한 모든 선언 파일(`.skillfollow`와 `.skillfollow.local`, 두 파일은 합집합)에서 제거하고 편집한 각 파일을 나열합니다. 주석과 다른 entry는 유지됩니다.
2. 링크 `<source>/<name>` 자체를 제거합니다. 실제 디렉터리(`not-link`)는 절대 제거하지 않습니다.
3. 링크를 제거한 경우에만 source `.gitignore`의 managed 블록에서 그 링크의 ignore 줄을 제거합니다. 그렇지 않으면 줄을 남겨 두고 알려 줍니다.

`--local`을 지정하면 `.skillfollow.local`만 편집합니다. `.skillfollow`가 아직 그 이름을 선언하고 있으면 `unfollow`는 entry가 계속 따라가진다고 알리고 링크를 남겨 둡니다.

쓰기에 실패하면 `unfollow`는 실패를 보고하고 이미 편집한 파일을 알려 줍니다. 일부만 끝난 unfollow를 성공으로 보고하지 않습니다.

unfollow 후에는 `skillshare sync`를 실행하세요. entry의 skill이 더 이상 발견되지 않으므로 sync가 그 managed 링크를 정리합니다. 외부 경로를 직접 가리키는 링크의 처리 방식은 [Cleanup safety](../skillfollow.md#cleanup)를 참고하세요.

## 옵션

| Flag | Description |
|------|-------------|
| `<name>` | 선언된 첫 번째 단계 entry |
| `--local` | `.skillfollow.local`에서만 이름 제거 |
| `--keep-link` | 링크와 그 ignore 줄 유지 |
| `--project, -p` | project skills source 사용(`.skillshare/skills/`) |
| `--global, -g` | global skills source 사용 |
| `--json` | JSON으로 출력 |
| `--help, -h` | 도움말 표시 |

## 예시

```bash
$ skillshare unfollow _team-skills
✓ _team-skills  removed from .skillfollow
✓ _team-skills  link removed; its target was not touched
✓ .gitignore    removed /_team-skills

Next
  skillshare sync  prune the entry's managed links

# Keep the link
$ skillshare unfollow _team-skills --keep-link
✓ _team-skills  removed from .skillfollow
  _team-skills  link kept: --keep-link
  .gitignore    kept /_team-skills

# The committed file still declares it
$ skillshare unfollow _team-skills --local
✓ _team-skills  removed from .skillfollow.local
! _team-skills  still declared in .skillfollow; it remains followed

# A real directory stays
$ skillshare unfollow realdir
✓ realdir  removed from .skillfollow
  realdir  link kept: not a link; a real directory is never removed
```

## JSON 출력

```bash
skillshare unfollow _team-skills --json
```

```json
{
  "name": "_team-skills",
  "source": "/home/me/.config/skillshare/skills",
  "files_edited": [".skillfollow"],
  "still_declared_in": [],
  "not_declared": false,
  "link": "/home/me/.config/skillshare/skills/_team-skills",
  "link_removed": true,
  "ignore_line": "/_team-skills",
  "ignore_line_removed": true,
  "ignore_line_kept": false
}
```

링크를 유지한 경우 `link_kept`에 이유가 들어갑니다. 실패하면 `{"error": "..."}`를 출력하고 상태 코드 1로 종료합니다.

## 참고 항목

- [follow](./follow.md) — entry 선언
- [.skillfollow](../skillfollow.md) — 파일 형식, state, 안전 규칙
- [sync](./sync.md) — entry의 managed 링크 정리
