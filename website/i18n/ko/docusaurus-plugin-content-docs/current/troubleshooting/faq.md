---
sidebar_position: 4
---

# FAQ

skillshare에 대한 자주 묻는 질문입니다.

## General

### Isn't this just `ln -s`?

핵심은 그렇습니다. 하지만 skillshare는 다음을 처리합니다.
- 다중 Target 감지
- 백업/복원
- Merge 모드 (skill별 symlink)
- 여러 기기 간 동기화
- 깨진 symlink 복구

그래서 여러분이 직접 할 필요가 없습니다.

### What happens if I modify a skill in the target directory?

Target은 symlink이므로, 변경 사항은 Source에 직접 적용됩니다. 모든 Target이 즉시 그 변경을 확인합니다.

### How do I keep a CLI-specific skill?

`merge` 모드(기본값)를 사용하세요. Target 안의 로컬 skill은 덮어써지거나 동기화되지 않습니다.

```bash
skillshare target claude --mode merge
skillshare sync
```

그런 다음 `~/.claude/skills/`에 직접 skill을 만드세요 — 건드려지지 않습니다.

### I use a dotfiles manager (stow/chezmoi/yadm) — will skillshare break my symlinks?

Symlink source **root**와 외부 target-directory symlink는 지원합니다. [Dotfiles 호환성](/docs/reference/commands/sync#dotfiles-manager-compatibility) 참조. Source **내부** 링크는 별도로 첫 계층 그룹/repo를 [`.skillfollow`](../reference/skillfollow.md)에 선언해야 검색하며 쓰기 권한은 주지 않아 uninstall/내용 쓰기를 거부합니다. 단일 skill과 중첩 링크는 미지원이고 실제 Windows junction 런타임 검증은 미완료입니다.

dotfiles로 `config.yaml`을 버전 관리한다면, 경로를 절대 경로 대신 `~/...`로 유지하도록 `preserve_tilde_on_save: true`를 활성화하는 것을 고려하세요 — [Configuration](/docs/reference/targets/configuration#preserve_tilde_on_save)를 참고하세요.

---

## Installation

### Can I sync skills to a custom or uncommon tool?

네. 해당 도구의 skill 디렉터리와 함께 `skillshare target add <name> <path>`를 사용하세요.

```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
skillshare sync
```

### Can I use skillshare with a private git repo?

네. SSH URL을 사용하세요.

```bash
skillshare init --remote git@github.com:you/private-skills.git
```

---

## Sync

### Why do I need to run `sync` after every install/update?

Sync는 의도적으로 별도의 단계로 분리되어 있습니다. `install`, `update`, `uninstall` 같은 작업은 **Source** 디렉터리만 수정합니다 — `sync`가 그 변경 사항을 모든 Target에 전파합니다.

이는 다음을 가능하게 합니다.
- **변경 일괄 처리** — 5개 skill을 설치한 뒤, 5번이 아니라 한 번만 동기화
- **먼저 미리보기** — 적용 전에 `sync --dry-run` 실행
- **주도권 유지** — Target을 언제 업데이트할지 직접 결정

**Note:** `pull`은 의도 자체가 "모든 것을 최신 상태로 만든다"이기 때문에, 자동으로 동기화되는 유일한 명령어입니다.

전체 설계 근거는 [Why Sync is a Separate Step](/docs/understand/source-and-targets#why-sync-is-a-separate-step)을 참고하세요.

### How do I sync across multiple machines?

git 기반의 여러 기기 간 동기화를 사용하세요.

```bash
# 머신 A: 변경 사항 push
skillshare push -m "Add new skill"

# 머신 B: pull 후 동기화
skillshare pull
```

전체 설정은 [Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync)를 참고하세요.

### What if I accidentally delete a skill through a symlink?

git이 초기화되어 있다면(권장), 다음으로 복구하세요.

```bash
cd ~/.config/skillshare/skills
git checkout -- deleted-skill/
```

또는 백업에서 복원하세요.
```bash
skillshare restore claude
```

### What if I accidentally uninstall a skill?

제거된 skill은 휴지통으로 이동해 7일간 보관됩니다. 다음으로 복원하세요.

```bash
skillshare trash list                  # 휴지통에 있는 항목 확인
skillshare trash restore my-skill      # Source로 복원
skillshare sync                        # Target으로 다시 동기화
```

skill이 remote source에서 설치된 것이었다면, 재설치할 수도 있습니다.

```bash
skillshare install github.com/user/repo/my-skill
skillshare sync
```

Project mode의 경우, 휴지통은 프로젝트 디렉터리 내의 `.skillshare/trash/`에 있습니다. trash 명령어와 함께 `-p` flag를 사용하세요.

현재 휴지통 상태(항목 수, 크기, 경과 시간)를 보려면 `skillshare doctor`를 실행하세요.

### What's the difference between backup and trash?

| | backup | trash |
|---|---|---|
| **보호 대상** | Target 디렉터리 (sync 스냅샷) | Source skill (uninstall) |
| **위치** | `~/.local/share/skillshare/backups/` | `~/.local/share/skillshare/trash/` |
| **트리거** | `sync`, `target remove` | `uninstall` |
| **복원 방법** | `skillshare restore <target>` | `skillshare trash restore <name>` |
| **자동 정리** | 수동 (`backup --cleanup`) | 7일 |

둘은 상호 보완적입니다 — backup은 sync로 인한 변경으로부터 Target을 보호하고, trash는 실수로 인한 삭제로부터 Source skill을 보호합니다.

### Can I sync specific skills to specific CLIs?

네. 예를 들어, skill A는 Claude에만, skill B는 Antigravity와 Codex에, skill C는 모두에게:

**Option 1: SKILL.md의 `targets` 필드** (skill 작성자가 설정)

```yaml
# skills/skill-a/SKILL.md
---
name: skill-a
targets: [claude]
---
```

```yaml
# skills/skill-b/SKILL.md
---
name: skill-b
targets: [antigravity, codex]
---
```

```yaml
# skills/skill-c/SKILL.md — targets 필드 없음 = 모든 Target에 동기화
---
name: skill-c
---
```

**Option 2: config의 `include`/`exclude` 필터** (소비자가 설정)

```yaml
# ~/.config/skillshare/config.yaml
targets:
  claude:
    path: ~/.claude/skills
    include: [skill-a, skill-c]
  codex:
    path: ~/.codex/skills
    include: [skill-b, skill-c]
```

두 방식은 함께 사용할 수 있습니다 — config 필터가 먼저 적용된 뒤, skill 수준의 `targets` 필드가 적용됩니다. [Skill Format — `targets`](/docs/understand/skill-format#targets)와 [Configuration — filters](/docs/reference/targets/configuration#skill-level-targets)를 참고하세요.

---

## Targets

### Using universal alongside npx skills {#using-universal-alongside-npx-skills}

`universal` Target은 [npx skills CLI](https://github.com/vercel-labs/skills)가 사용하는 것과 동일한 디렉터리인 `~/.agents/skills`를 가리킵니다. 두 도구는 몇 가지 주의사항과 함께 이 디렉터리를 동시에 관리할 수 있습니다.

**What works:**
- merge 모드(기본값)에서, skillshare는 `~/.agents/skills/`에 **symlink**를 만들고, npx skills는 **실제 디렉터리**를 만듭니다. skill 이름이 충돌하지 않는 한 두 도구는 공존합니다.
- skillshare의 정리(prune) 로직은 자신이 관리하는 항목만 제거합니다 — npx skills가 설치한 파일을 삭제하지 않습니다.
- Agent CLI(Claude Code, Pi 등)는 디렉터리를 직접 읽으므로, 두 도구 모두의 skill을 볼 수 있습니다.

**What to watch out for:**
- **이름 충돌** — 두 도구가 같은 이름의 skill을 설치하면, 마지막 sync/install이 우선합니다. 두 도구로 동일한 skill을 설치하지 마세요.
- **Copy 모드가 더 공격적** — copy 모드(`skillshare target universal --mode copy`)에서는, skillshare가 매 sync마다 관리형 디렉터리를 덮어씁니다. npx skills가 sync 사이에 동일한 이름의 skill을 수정하면, skillshare가 그것을 교체합니다. merge 모드(기본값)는 symlink만 생성하므로 공존에 더 안전합니다.
- **`npx skills list`에는 skillshare skill이 표시되지 않음** — npx skills CLI는 디렉터리를 스캔하는 대신 lock 파일(`~/.agents/.skill-lock.json`)로 설치를 추적합니다. skillshare가 동기화한 skill은 `npx skills list -g`에 나타나지 않지만, Agent CLI에는 **보입니다**.
- **다른 agent 전용 Target도 여전히 유용함** — `universal`과 `claude`는 서로 다른 경로(`~/.agents/skills` vs `~/.claude/skills`)를 가리킵니다. 둘 다 선택하는 것은 안전하며 중복도 아닙니다.

**Recommended workflow:**
```bash
# skillshare를 주요 skill 관리자로 사용
skillshare install github.com/user/skills --track
skillshare sync

# 동기화가 필요 없는 일회성 커뮤니티 설치에는 npx skills만 사용
npx skills add someone/skill -g
```

:::tip
npx skills와 가장 안전하게 공존하려면 universal Target을 **merge 모드**(기본값)로 유지하세요. npx skills를 전혀 사용하지 않는 경우가 아니라면 copy 모드로 전환하지 마세요.
:::

### I used `claude-code` (or `gemini-cli`, etc.) as a project target — is that still valid?

네. `claude-code`, `gemini-cli`, `github-copilot` 같은 이전 project target 이름은 alias를 통해 여전히 해석됩니다. 예를 들어, `gemini-cli`는 `gemini`로 해석됩니다. `.skillshare/config.yaml`을 정식 이름으로 업데이트하는 것을 권장합니다.

```yaml
# Before
targets:
  - claude-code

# After
targets:
  - claude
```

### How does `target remove` work? Is it safe?

네, 안전합니다.

1. **Backup** — Target의 백업 생성
2. **Detect mode** — symlink 모드인지 merge 모드인지 확인
3. **Unlink** — skillshare가 관리하는 모든 symlink를 제거하고, Source 내용을 실제 파일로 복사. merge 모드에서는 Source 디렉터리를 가리키는 symlink만 제거되며, 로컬(non-symlink) skill은 보존됨
4. **Update config** — config.yaml에서 Target 제거

이것이 `rm -rf ~/.claude/skills`가 Source 파일을 삭제하는 것과 달리, `skillshare target remove`가 안전한 이유입니다.

### Why is `rm -rf` on a target dangerous?

symlink 모드에서는 Target 디렉터리 전체가 Source에 대한 symlink입니다. 그것을 삭제하면 Source가 삭제됩니다.

merge 모드에서는 각 skill이 symlink입니다. symlink를 통해 skill을 삭제하면 Source 파일이 삭제됩니다.

**항상 다음을 사용하세요:**
```bash
skillshare target remove <name>   # 안전함
skillshare uninstall <skill>      # 안전함
```

---

## Tracked Repos

### How do tracked repos differ from regular skills?

| Aspect | Regular Skill | Tracked Repo |
|--------|---------------|--------------|
| Source | Source로 복사됨 | `.git`과 함께 클론됨 |
| Update | `install --update` | `update <name>` (git pull) |
| Prefix | 없음 | `_` 접두사 |
| Nested skills | 평탄화됨 | `__`로 평탄화됨 |

### Why the underscore prefix?

`_` 접두사는 tracked repository를 식별합니다.
- 일반 skill과 구분하는 데 도움
- 이름 충돌 방지
- 목록에서 명확하게 표시

---

## Skills

### What's the SKILL.md format?

```markdown
---
name: skill-name
description: Brief description
---

# Skill Name

Instructions for the AI...
```

전체 내용은 [Skill Format](/docs/understand/skill-format)을 참고하세요.

### What does "unknown target" warning mean?

`skillshare check` 또는 `skillshare doctor`를 실행하면 다음이 표시될 수 있습니다.

```
! Skill targets: my-skill: unknown target "*"
```

이는 skill의 `SKILL.md` frontmatter에 있는 `targets` 필드가 인식되지 않는 이름 — 흔히 `"*"` (와일드카드) — 을 가지고 있다는 뜻입니다. skillshare는 glob 패턴이 아니라 **정확한 Target 이름**(예: `claude`, `cursor`, `codex`)을 기대합니다.

**skill이 모든 Target에 동기화되길 원한다면**, `targets` 필드를 아예 생략하세요.

```yaml
---
name: my-skill
description: Works everywhere
# targets 필드 없음 = 모든 Target에 동기화
---
```

**이 경고가 서드파티 skill에서 나온 것이라면**, skill 작성자가 지원되지 않는 문법을 사용한 것입니다. 다음 중 하나를 선택할 수 있습니다.
1. **경고 무시** — skill은 여전히 설치되며, 단지 특정 Target으로 자동 필터링되지 않을 뿐입니다
2. **Fork 후 수정** — skill의 `SKILL.md`에서 `targets` 필드를 제거하거나 수정하세요

전체 명세는 [Skill Format — `targets`](/docs/understand/skill-format#targets)를 참고하세요.

### Can a skill have multiple files?

네. skill 디렉터리에는 다음이 포함될 수 있습니다.
- `SKILL.md` (필수)
- 추가 파일 (예시, 템플릿 등)

SKILL.md의 안내에서 이를 참조하세요.

---

## Performance

### Sync seems slow

skill 디렉터리에 큰 파일이 있는지 확인하세요. ignore 패턴을 추가하세요.

```yaml
# ~/.config/skillshare/config.yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
  - "**/*.log"
```

### How many skills can I have?

하드 리밋은 없습니다. 성능은 다음에 따라 달라집니다.
- skill 수
- skill 파일 크기
- Target 수

수천 개의 작은 skill도 문제없이 동작합니다.

---

## Backups

### Where are backups stored?

```
~/.local/share/skillshare/backups/<timestamp>/
```

### How long are backups kept?

기본적으로 무기한입니다. 다음으로 정리하세요.
```bash
skillshare backup --cleanup
```

---

## Agents

### What's the difference between agents and skills?

Skill은 `SKILL.md` 파일(그리고 선택적으로 헬퍼, 예시, 템플릿)을 담은 **디렉터리**입니다. Agent는 중첩 구조가 없는 frontmatter를 가진 **단일 `.md` 파일**입니다. 둘 다 install, sync, audit, check, backup, trash를 지원합니다.

전체 비교와 agent 파일 형식은 [Agents](/docs/understand/agents)를 참고하세요.

### Which targets support agents?

기본 제공: `claude`, `cursor`, `augment`, `opencode` (그리고 `universal` alias). 다른 Target은 agent sync 중 `No agents folder: <targets>` 경고와 함께 조용히 건너뛰어집니다. `config.yaml`을 편집해 agent 경로를 수동으로 추가할 수 있습니다.

```yaml
targets:
  myapp:
    path: ~/myapp/skills
    agents:
      path: ~/myapp/agents
```

### How do I disable a single agent without deleting it?

`disable` 명령어를 사용하세요 (또는 `.agentignore`를 직접 편집).

```bash
skillshare disable my-agent --kind agent     # .agentignore에 항목 추가
skillshare enable my-agent --kind agent      # 항목 제거
```

`.agentignore`는 agents source root(global의 경우 `~/.config/skillshare/agents/.agentignore`, project mode의 경우 `.skillshare/agents/.agentignore`)에 위치하며 [gitignore 문법](https://git-scm.com/docs/gitignore)을 사용합니다. 로컬 전용 재정의를 위한 `.agentignore.local` 오버레이도 지원됩니다.

### Can I backup agents in project mode?

네 — 그리고 **agent만** 가능합니다. `backup`은 skill에 대해서는 project mode에서 허용되지 않지만, agent 흐름은 명시적인 예외입니다.

```bash
skillshare backup -p agents     # Project agent target 백업
skillshare backup -p --all      # 위와 동일; --all은 project mode에서 agent로 좁혀짐
```

`agents` 필터를 잊으면 `backup is not supported in project mode (except for agents)`가 표시됩니다. `restore`에도 동일한 규칙이 적용됩니다. Agent 백업은 일반 skill 백업 옆의 `<target>-agents/` 아래에 저장됩니다.

---

## Security

### Can I trust third-party skills?

skill은 AI agent를 위한 지시문입니다 — 악의적인 skill은 AI에게 비밀 정보를 유출하거나 파괴적인 명령을 실행하도록 지시할 수 있습니다. skillshare는 내장 보안 스캐너로 이를 완화합니다.

- **설치 시 자동 스캔** — `skillshare install` 중 모든 skill이 스캔됨
- **CRITICAL 발견 시 차단** — prompt injection, 데이터 유출, 자격 증명 접근은 기본적으로 차단됨
- **수동 스캔** — 언제든 `skillshare audit`을 실행해 설치된 모든 skill을 스캔

탐지 패턴의 전체 목록은 [audit command](/docs/reference/commands/audit)를 참고하세요.

### What if audit blocks my install?

skill이 CRITICAL 발견 항목을 유발하면, 설치가 차단됩니다. 두 가지 선택지가 있습니다.

1. **발견 항목 검토** — 오탐인지 확인하세요 (예: 문서 예시)
2. **강제 설치** — Source를 신뢰한다면 검사를 우회하기 위해 `--force` 사용

```bash
skillshare install suspicious-skill --force
```

### Does audit catch everything?

완벽한 스캐너는 없습니다. `skillshare audit`은 prompt injection, 비밀 정보가 포함된 `curl`/`wget`, 자격 증명 파일 접근, 난독화된 payload 같은 흔한 패턴을 탐지합니다. 신뢰할 수 없는 source의 skill은 항상 직접 검토하세요.

---

## Getting Help

### Where do I report bugs?

[GitHub Issues](https://github.com/runkids/skillshare/issues)

### Where do I ask questions?

[GitHub Discussions](https://github.com/runkids/skillshare/discussions)

---

## Related

- [Common Errors](./common-errors.md) — 오류 해결 방법
- [Windows](./windows.md) — Windows 관련 FAQ
- [Troubleshooting Workflow](./troubleshooting-workflow.md) — 단계별 디버깅
