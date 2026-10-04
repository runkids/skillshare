---
sidebar_position: 6
---

# follow

skills source의 첫 번째 단계 링크를 선언해 discovery가 그 링크를 따라가게 합니다. 파일을 직접 편집하지 않고 [`.skillfollow`](../skillfollow.md)를 설정하는 한 단계 방법입니다.

```bash
skillshare follow _team-skills --to ~/work/team-skills   # Create the link and declare it
skillshare follow _team-skills                           # Declare a link that already exists
skillshare follow _team-skills --local                   # Declare it for this machine only
skillshare follow _team-skills -p                        # Project mode
```

## 언제 사용하나요

- 작업 중인 repository를 평소 편집하는 위치에 그대로 두고 skillshare가 그 안의 skill을 찾게 할 때
- `doctor`가 `undeclared-link`로 보고한 링크를 따라가는 entry로 바꿀 때
- 팀원과 공유하지 않는 이 머신 전용 링크를 추가할 때(`--local`)

## 동작 방식

1. `--to <dir>`를 지정하면 `<dir>`를 가리키는 링크 `<source>/<name>`를 만듭니다. macOS/Linux에서는 symlink, Windows에서는 junction입니다. `<dir>`는 skills source 밖에 이미 존재하는 디렉터리여야 합니다. `<source>/<name>`가 이미 있으면 같은 디렉터리를 가리키는 링크여야 합니다.
2. `--to`를 지정하지 않으면 `<source>/<name>`가 링크나 실제 디렉터리로 이미 존재해야 합니다. 실제 디렉터리도 받아들이며 `not-link`로 보고합니다. 어차피 discovery에서 찾아집니다.
3. `<name>`을 `.skillfollow`에, `--local`을 지정하면 `.skillfollow.local`에 추가합니다. 주석, 빈 줄, 기존 줄의 순서는 그대로 유지됩니다. 이미 선언된 이름은 건드리지 않습니다.
4. source가 Git 작업 트리 안에 있으면 고정된 ignore 줄(예: `/_team-skills`)을 source의 `.gitignore`에 추가합니다. `doctor`가 요구하는 줄과 같습니다. `--local`을 지정하면 `/.skillfollow.local`도 추가합니다. Git이 이미 무시하는 경로의 줄은 다시 추가하지 않습니다.
5. entry의 최종 [state](../skillfollow.md#states)와 이유를 출력하므로 따라갈 수 없는 선언을 바로 확인할 수 있습니다.

`follow`는 sync를 실행하지 않습니다. 이후에 `skillshare sync`를 실행하세요.

`_`로 시작하고 `.git`을 포함한 이름은 tracked repository로, 나머지 이름은 group으로 따라갑니다. `<name>`은 바로 아래 자식 이름이어야 합니다. `/`나 `\`, glob이나 부정 문자, 절대 경로나 드라이브 이름은 사용할 수 없습니다.

링크가 이미 Git에 추적되고 있으면 `follow`는 추적을 해제하지 않습니다. 직접 실행할 `git rm --cached` 명령을 출력하며, 이는 `commit`과 `doctor`가 출력하는 것과 같습니다.

## 옵션

| Flag | Description |
|------|-------------|
| `<name>` | skills source의 첫 번째 단계 entry |
| `--to <dir>` | 먼저 `<dir>`를 가리키는 링크 생성(Windows에서는 junction) |
| `--local` | `.skillfollow` 대신 `.skillfollow.local`에 쓰고 Git에서 무시 |
| `--project, -p` | project skills source 사용(`.skillshare/skills/`) |
| `--global, -g` | global skills source 사용 |
| `--json` | JSON으로 출력 |
| `--help, -h` | 도움말 표시 |

`-p`와 `-g` 둘 다 지정하지 않으면 다른 명령과 마찬가지로 mode가 자동 감지됩니다.

## 예시

```bash
# Create the link and declare it in a Git source
$ skillshare follow _team-skills --to ~/work/team-skills
✓ _team-skills  linked to /home/me/work/team-skills
✓ _team-skills  added to .skillfollow
✓ .gitignore    added /_team-skills
✓ _team-skills  followed — following directory

Next
  skillshare sync  apply the change

# Already declared
$ skillshare follow _team-skills
! _team-skills  already in .skillfollow
✓ _team-skills  followed — following directory

# A declaration that cannot be followed is reported, not hidden
$ skillshare follow out --to ~/.claude
✓ out         linked to /home/me/.claude
✓ out         added to .skillfollow
✓ .gitignore  added /out
! out         target-overlap — target overlaps active skills target /home/me/.claude/skills

# The link is already tracked by Git
$ skillshare follow _dev
✓ _dev        added to .skillfollow
✓ .gitignore  added /_dev
! _dev        indexed in Git; run git rm --cached -- '_dev'
✓ _dev        followed — following directory
```

거부된 entry는 선언된 채로 남아 있으며, 고치거나 [`unfollow`](./unfollow.md)를 실행할 때까지 target 정리를 일시 중지합니다. [States and recovery](../skillfollow.md#states)를 참고하세요.

## JSON 출력

```bash
skillshare follow _team-skills --json
```

```json
{
  "name": "_team-skills",
  "source": "/home/me/.config/skillshare/skills",
  "file": ".skillfollow",
  "added": true,
  "link": "/home/me/.config/skillshare/skills/_team-skills",
  "link_created": false,
  "ignore_file": "/home/me/.config/skillshare/skills/.gitignore",
  "ignore_lines_added": ["/_team-skills"],
  "state": "followed",
  "reason": "following directory",
  "resolved_target": "/home/me/work/team-skills"
}
```

`--to`를 지정하면 `link_target`이, 링크가 Git에 추적되고 있으면 `untrack_command`가 나타납니다. 실패하면 `{"error": "..."}`를 출력하고 상태 코드 1로 종료합니다.

## 참고 항목

- [unfollow](./unfollow.md) — entry 따라가기 중지
- [.skillfollow](../skillfollow.md) — 파일 형식, state, 안전 규칙
- [doctor](./doctor.md) — 선언된 state와 누락된 ignore 줄 보고
- [sync](./sync.md) — 변경 사항을 target에 적용
