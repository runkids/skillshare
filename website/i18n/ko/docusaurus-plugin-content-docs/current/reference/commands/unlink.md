---
sidebar_position: 3
---

# unlink

[`skillshare link`](./link.md)로 만든 링크나 skills source의 첫 번째 수준 링크를 제거합니다. 휴지통으로 옮겨지는 것은 링크뿐이며, 링크가 가리키는 폴더는 그대로 남습니다.

```bash
skillshare unlink _dev-skills              # 링크 제거
skillshare unlink _team -p                 # project source의 링크 제거
skillshare unlink -- -local                # -로 시작하는 이름 앞에서 옵션 해석 종료
```

## 언제 사용하나요

- 링크한 checkout의 파일은 건드리지 않고 사용만 중단할 때
- 다른 이름으로 다시 링크하기 전에 링크를 제거할 때

```text
$ skillshare unlink _dev-skills
✓ Unlinked _dev-skills → trash, kept 7 days; its target is untouched

Next
  skillshare sync                       remove its skills from your targets
  skillshare trash restore _dev-skills  undo
```

## 동작 방식

`unlink`는 링크 항목만 휴지통으로 옮깁니다. 링크된 폴더 바로 아래에 `SKILL.md`가 있는 경우도 같습니다. 대시보드의 **Unlink**도 같은 동작을 합니다. 휴지통에는 `link → <target>`으로 나열되고, `skillshare trash restore <name>`은 링크(또는 junction)를 다시 만들 뿐 대상의 복사본을 만들지 않습니다. 이후 `skillshare sync`를 실행하면 해당 skill이 target에서 제거됩니다. Global mode에서 `unlink`는 해당 링크 이름의 tracked repository 기록과 감사 승인 기록도 제거합니다. 링크 대상은 그대로 유지됩니다.

첫 번째 수준 링크가 아닌 이름은 거부하므로 일반 skill이나 폴더를 제거하는 일은 없습니다:

```text
$ skillshare unlink my-skill
✗ cannot unlink my-skill: my-skill is not a link
```

| 이유 | 원인 |
|--------|-------|
| `<name> not found in source` | source 바로 아래에 그 이름의 항목이 없습니다 |
| `<name> is not a link` | 항목이 일반 디렉터리나 파일입니다. [`uninstall`](./uninstall.md)을 사용하세요 |
| `"<name>" is not a first-level name` | 이름에 경로 구분자가 있거나 `.` / `..`입니다 |

## 옵션

| Flag | Description |
|------|-------------|
| `--project, -p` | 현재 디렉터리의 project 레벨 config 사용 |
| `--global, -g` | global config 사용(`~/.config/skillshare`) |
| `--` | 옵션 끝. `-`로 시작하는 이름 허용 |
| `--help, -h` | 도움말 표시 |

## 참고

- [link](./link.md) — 링크 만들기
- [trash](./trash.md) — 제거한 링크 복원
- [Configuration — `follow_source_links`](../targets/configuration.md#follow_source_links) — discovery 동작과 링크를 통한 쓰기
