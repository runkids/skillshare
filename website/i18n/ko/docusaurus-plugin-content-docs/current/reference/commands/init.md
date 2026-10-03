---
sidebar_position: 1
---

# init

최초 설정입니다. 설치된 AI CLI를 감지하고, 이미 있는 skill을 가져온 뒤 동기화합니다.

```bash
skillshare init              # Interactive setup
skillshare init --dry-run    # Preview without changes
```

## 사용 시점

- 컴퓨터에 skillshare를 처음 설정할 때
- 새 컴퓨터로 마이그레이션할 때 (기존 저장소에 연결하려면 `--remote` 사용)
- 프로젝트에 skillshare 추가할 때 (`--project` 사용)
- 새로 설치된 AI CLI를 발견할 때 (`--discover` 사용)

## 동작 과정

`init`은 먼저 묻고 마지막에 씁니다. 요약을 확인하기 전에는 아무것도 만들지 않으며, 어느 질문에서든 <kbd>Esc</kbd>를 누르면 아무것도 쓰지 않고 취소합니다.

```mermaid
flowchart TD
    TITLE["skillshare init"]
    START{"How do you want to start?"}
    NEW["New setup: tools → import → git → remote (optional)"]
    CONNECT["Connect my existing repo: URL → keep local skills → tools"]
    SUMMARY["Summary: Yes / Change settings / Cancel"]
    APPLY["Write config, copy skills, install built-in skill, commit"]
    SYNC["Sync now?"]
    TITLE --> START
    START --> NEW --> SUMMARY
    START --> CONNECT --> SUMMARY
    SUMMARY --> APPLY --> SYNC
```

모든 질문에는 기본값이 있어서 <kbd>Enter</kbd>만 눌러도 동작하는 설정이 됩니다:

| 질문 | 기본값 |
|------|--------|
| Targets | 감지된 모든 AI CLI |
| Import | 그 도구들에 이미 있는 모든 skill |
| Git | 켜짐. skill만 버전 관리하며, remote를 연결하면 skills, agents, extras를 버전 관리합니다. Plugins, MCP servers, hooks는 각 머신의 `config.yaml`에 남습니다 |
| Built-in skill | 설치 |
| Sync | 예 |

요약의 **Change settings**에서 source 경로, sync mode, git이 버전 관리할 범위를 바꿀 수 있습니다.

**Connect my existing repo**는 두 번째 머신용입니다. 아무것도 쓰지 않고 먼저 저장소를 확인해 구조를 판단합니다. `--git-root root`로 push한 저장소는 skillshare 폴더 전체가 되고, skill이 `skills/` 폴더에 있는 저장소는 그 폴더를 source로 사용합니다. 이 머신과 저장소 양쪽에 있는 같은 이름의 skill은 저장소 버전을 사용합니다. 이 머신에만 있는 skill은 유지할지 묻고, 다음 `skillshare push` 때 저장소에 추가됩니다.

첫 sync에서 source와 바이트 단위로 같은 도구 쪽 skill 폴더는 링크로 바뀝니다. 내용이 다른 폴더는 유지하고 목록으로 보여 주며, `skillshare sync --force`로 바꿀 수 있습니다.

### 터미널이 없을 때

stdin 또는 stdout이 터미널이 아니면(CI, 스크립트, AI agent) `init`은 아무것도 묻지 않고 위의 기본값을 사용합니다. 결정마다 한 줄씩, 바꿀 수 있는 flag와 함께 출력합니다:

```text
✓ Source   ~/.config/skillshare/skills (--source, --subdir)
✓ Targets  claude, cursor, universal (--targets, --no-targets)
✓ Import   all 2 (--copy-from, --no-copy)
✓ Git      skills only (--no-git, --git-root)
✓ Remote   none (--remote <url>)
✓ Skill    install skillshare (--skill, --no-skill)
✓ Sync     merge (--mode)
```

`--remote`를 지정하면 skill이 있는 저장소를 pull하고, 같은 이름의 skill은 저장소 버전을 사용하며 그 이름을 보여 줍니다. 터미널이 없을 때의 출력에는 색이나 escape 코드가 없습니다.

`init`은 skill source 디렉터리 **와** `agents/` 형제 디렉터리를 한 단계에서 생성하므로, 두 종류의 리소스 모두 즉시 사용할 준비가 됩니다. agents 디렉터리는 별도의 프롬프트나 플래그 없이 조용히 생성됩니다. agent 파일 형식은 [Agents](/docs/understand/agents)를 참고하세요.

:::info Universal target
AI CLI가 감지되면, `init`은 자동으로 **universal** target(`~/.agents/skills`)을 추천합니다. 이는 [vercel-labs/skills](https://github.com/vercel-labs/skills)(`npx skills list`)가 사용하는 공유 디렉터리로, 호환되는 모든 agent에 한 번에 skill을 제공합니다.
:::

:::tip Agents source path
agents source의 기본값은 `<source parent>/agents`입니다 (기본 설치의 경우 `~/.config/skillshare/agents/`). 위치를 변경하려면 `config.yaml`에서 `agents_source:`를 설정하세요. 프로젝트 모드는 항상 프로젝트 디렉터리 안의 `agents/`를 사용하며 `agents_source`를 따르지 않습니다. Agent를 지원하는 target(Claude, Cursor, Augment, OpenCode)은 `skillshare sync`를 실행하면 agent를 자동으로 인식합니다.
:::

## Project Mode

`-p`로 프로젝트 레벨 skill을 초기화합니다:

```bash
skillshare init -p                              # Interactive (no terminal: every detected tool)
skillshare init -p --targets claude,cursor  # Choose the tools
skillshare init -p --visible                    # Use a visible skillshare/ directory
```

### 동작 과정

```mermaid
flowchart TD
    TITLE["skillshare init -p"]
    S1["1. Create .skillshare/skills + .skillshare/agents"]
    S2["2. Detect AI CLI directories"]
    S3["3. Create target skill directories"]
    S4["4. Write config.yaml"]
    TITLE --> S1 --> S2 --> S3 --> S4
```

init 이후, 프로젝트 디렉터리를 git에 commit하세요 (`skills/`와 `agents/` 모두). `.skillshare/` 대신 `skillshare/`를 생성하려면 `--visible`을 사용하세요. 전체 가이드는 [Project Setup](/docs/how-to/sharing/project-setup)을 참고하세요.

## Discover Mode

기존 설정에서 init을 다시 실행하여 새 AI CLI target을 감지하고 추가합니다:

### Global

```bash
skillshare init --discover              # Interactive selection
skillshare init --discover --select codex,opencode  # Non-interactive
```

config에 아직 없는 새로 설치된 AI CLI를 스캔하고 어떤 것을 추가할지 묻습니다(모두 선택된 상태). 터미널이 없으면 새 도구를 모두 추가합니다. `universal` target(`~/.agents/skills`)은 CLI가 감지될 때마다 자동으로 추천됩니다.

### Project

```bash
skillshare init -p --discover           # Interactive selection
skillshare init -p --discover --select antigravity  # Non-interactive
```

프로젝트 디렉터리에서 새 AI CLI 디렉터리(예: `.agents/`)를 스캔하여 target으로 추가합니다. 터미널이 없으면 찾은 새 도구를 모두 추가합니다.

### Discover + Mode 동작

`--discover`와 `--mode`를 함께 사용하면, mode는 이번 discover 실행에서 추가된 target에만 **적용**됩니다.
config에 있는 기존 target은 변경되지 않습니다.

```bash
# Adds cursor with mode=copy, does not change existing targets
skillshare init --discover --select cursor --mode copy

# Project mode variant (same rule)
skillshare init -p --discover --select cursor --mode copy
```

:::tip
이미 초기화된 설정에서 `--discover` 없이 `skillshare init`을 실행하면, 오류 메시지가 이를 사용하라고 안내합니다.
:::

## Options

| Flag | 설명 |
|------|-------------|
| `--source, -s <path>` | 사용자 지정 source 디렉터리 (요약의 **Change settings**에서도 변경 가능) |
| `--remote <url>` | git remote 설정 (`--git`을 암시). skill이 있는 저장소는 pull하며, 같은 이름의 skill은 저장소 버전 사용 |
| `--project, -p` | 현재 디렉터리에 프로젝트 레벨 skill 초기화 |
| `--copy-from, -c <name\|path>` | 특정 CLI 또는 경로에서 skill 복사 |
| `--no-copy` | 빈 source로 시작 (copy 프롬프트 생략) |
| `--targets, -t <list>` | 쉼표로 구분된 target 이름 |
| `--all-targets` | 감지된 모든 target 추가 |
| `--no-targets` | target 선택 생략 |
| `--mode, -m <mode>` | 새로 구성되는 target의 기본 mode 설정 (`merge`, `copy`, `symlink`). `--discover`와 함께 사용하면 새로 추가된 target에만 영향을 줌 |
| `--git` | 프롬프트 없이 git 초기화 (기본값) |
| `--no-git` | git 초기화 생략 |
| `--skill` | built-in skillshare skill 설치 (기본값; AI CLI에 `/skillshare` 추가) |
| `--no-skill` | built-in skill 설치 생략 |
| `--discover, -d` | 기존 config에 새 AI CLI target 감지 및 추가 |
| `--select <list>` | 추가할 target의 쉼표 구분 목록 (`--discover` 필요) |
| `--config local` | 각 개발자가 자신의 target을 관리하도록 `config.yaml`을 gitignore 처리 (프로젝트 모드 전용). [Centralized Skills Repo](/docs/how-to/recipes/centralized-skills-repo) 레시피 참고 |
| `--visible` | `.skillshare/` 대신 눈에 보이는 `skillshare/` 프로젝트 디렉터리 생성 (프로젝트 모드 전용). [Project Skills](/docs/understand/project-skills#visible-project-directory) 참고 |
| `--git-root <scope>` | `commit`/`push`/`pull` 작업을 위한 디렉터리 (기본값 `skills`, `agents`, `extras`, `root`). `root`는 skill + agent + extras를 하나의 저장소로 함께 버전 관리하며 `config.yaml`이 자동으로 무시됨. remote를 연결하면 기본값은 `root`, 아니면 `skills`. 요약의 **Change settings**에서도 변경 가능. `skillshare init --git-root <scope>`를 나중에 다시 실행하면 headless로 scope를 전환할 수 있음 — 새 scope에서 저장소를 초기화하고 설정을 유지하지만, 기존 히스토리는 이동하지 않음 |
| `--subdir <name>` | source 경로로 하위 디렉터리 사용 (예: `skills`); 저장소 연결 시 자동 감지 |
| `--dry-run, -n` | 변경 없이 미리보기 |

`init`은 시작 mode 정책을 설정합니다. 나중에 언제든 target별로 세부 조정할 수 있습니다:

```bash
skillshare target cursor --mode copy
skillshare sync
```

## Source Subdirectory

기본적으로 `init --remote`는 전체 git 저장소 루트를 skill source로 취급합니다. 저장소에 skill이 아닌 파일(README, CI 설정, dotfile 등)도 포함되어 있다면, skill을 하위 디렉터리에 저장할 수 있습니다:

```
# Without --subdir: repo root = source (all files are skills)
~/.config/skillshare/skills/          ← git repo root = source
  ├── my-skill/
  └── another-skill/

# With --subdir skills: source points to a subdirectory
~/.config/skillshare/skills/          ← git repo root
  ├── README.md
  ├── .github/
  └── skills/                         ← source points here
      ├── my-skill/
      └── another-skill/
```

일반적인 사용 사례: 전용 skill 전용 저장소 대신 기존 dotfiles나 monorepo 안에 skill을 포함시키는 경우입니다.

연결한 저장소의 최상위에는 skill이 없고 `skills/` 폴더에 있으면 `init`이 그 폴더를 자동으로 사용합니다. 다른 이름을 쓰려면 `--subdir`를 지정하세요:

```bash
skillshare init --remote git@github.com:you/dotfiles.git --subdir skills
```

## Common Scenarios

### Remote setup (하나 선택)

Interactive: `skillshare init`을 실행하고 **Connect my existing skillshare repo**를 선택하세요.

Non-interactive (프롬프트 없음, 설치된 target 자동 감지):

```bash
skillshare init --remote git@github.com:you/my-skills.git --no-copy --all-targets --no-skill
```

Non-interactive (프롬프트 없음, 기존 Claude skill을 즉시 가져오기):

```bash
skillshare init --remote git@github.com:you/my-skills.git --copy-from claude --all-targets --no-skill
```

### Centralized skills repo

```bash
# Creator: set up shared repo with local config
skillshare init -p --config local --targets claude

# Teammate: clone and auto-detect shared repo
git clone <repo> && cd <repo>
skillshare init -p
skillshare target add myproject ~/DEV/myproject/.claude/skills -p
```

### Other scenarios

```bash
# Standard setup (auto-detect everything)
skillshare init

# Use existing skills directory
skillshare init --source ~/.config/skillshare/skills

# Project-level setup
skillshare init -p
skillshare init -p --targets claude,cursor

# Defaults without prompts (also what runs without a terminal)
skillshare init --no-copy --all-targets --git --skill

# Start with copy mode defaults for newly added targets
skillshare init --mode copy

# Add newly installed CLIs to existing config
skillshare init --discover
skillshare init -p --discover

# Add a newly discovered target and force copy mode only for that new target
skillshare init --discover --select cursor --mode copy
```
