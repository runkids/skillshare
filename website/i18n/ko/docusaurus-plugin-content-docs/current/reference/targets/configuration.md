---
sidebar_position: 4
---

# Configuration

skillshare의 config 파일 참조 문서입니다.

## Overview

```text
~/.config/skillshare/
├── config.yaml          ← Configuration 파일
├── skills/              ← Source 디렉터리 (여러분의 skill)
│   ├── .metadata.json   ← Skill 메타데이터 (자동 관리)
│   ├── my-skill/
│   ├── another/
│   └── _team-repo/      ← Tracked repository
├── extras/              ← Extras source root
│   └── rules/           ← Extra 리소스 (예: rules)

~/.local/share/skillshare/
└── backups/             ← 자동 백업
    └── 2026-01-20.../
```

---

## IDE Support (JSON Schema) {#ide-support}

Config 파일에는 지원되는 에디터에서 **자동완성**, **유효성 검사**, **호버 문서**를 제공하는 YAML Language Server 지시문이 포함되어 있습니다.

`skillshare init`으로 생성된 새 config에는 이 지시문이 자동으로 포함됩니다.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
source: ~/.config/skillshare/skills
targets:
  claude:
    path: ~/.claude/skills
```

### Adding to an existing config

이 기능이 추가되기 전에 생성된 config라면, 첫 줄에 다음 주석을 추가하세요.

**Global config** (`~/.config/skillshare/config.yaml`):
```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
```

**Project config** (`.skillshare/config.yaml`):
```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/project-config.schema.json
```

또는 단순히 `skillshare init --force` (global) 또는 `skillshare init -p --force` (project)를 다시 실행하여 schema 주석이 포함된 config를 재생성할 수 있습니다.

### Supported editors

| Editor | Extension required |
|--------|-------------------|
| VS Code | Red Hat의 [YAML](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) |
| JetBrains IDEs | 내장 YAML 지원 |
| Neovim | LSP를 통한 [yaml-language-server](https://github.com/redhat-developer/yaml-language-server) |

---

## Config File

**Location:** `~/.config/skillshare/config.yaml`

### Full Example

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
# Source 디렉터리 (skill을 편집하는 곳)
source: ~/.config/skillshare/skills

# 새 Target의 기본 Sync 모드
mode: merge

# 기본 Target 네이밍 (flat 또는 standard)
# target_naming: flat

# Targets (AI CLI skill 디렉터리)
targets:
  claude:
    path: ~/.claude/skills
    # mode: merge (기본값을 상속함)

  codex:
    path: ~/.codex/skills
    mode: symlink  # 기본 모드를 재정의
    include: [codex-*] # merge/copy 모드에서만 사용

  cursor:
    path: ~/.cursor/skills
    mode: copy  # Cursor를 위한 실제 파일
    exclude: [experimental-*] # merge/copy 모드에서만 사용

  # Custom target
  myapp:
    path: ~/apps/myapp/skills

# Remote skill — install/uninstall에 의해 자동 관리됨
skills:
  - name: pdf
    source: anthropics/skills/skills/pdf
  - name: _team-skills
    source: github.com/team/skills
    tracked: true

# 저장 시 $HOME → ~ 로 접기 (dotfiles 친화적)
# preserve_tilde_on_save: true

# commit/push/pull 대상 디렉터리 (skills 기본값, agents, extras, root)
# git_root: skills

# Custom agents source (옵션, 기본 위치를 재정의)
agents_source: ~/my-agents

# Custom extras source (옵션, 기본 위치를 재정의)
extras_source: ~/my-extras

# 동기화할 non-skill 리소스
extras:
  - name: rules
    source: ~/company-shared/rules   # extra별 선택적 재정의
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy

# Sync 중 무시할 파일
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
  - "**/*.log"
```

---

## Fields

### `source`

여러분의 skill 디렉터리 경로 (단일 진실 공급원).

```yaml
source: ~/.config/skillshare/skills
```

**Default:** `~/.config/skillshare/skills`

### `mode`

모든 Target에 대한 기본 Sync 모드.

```yaml
mode: merge
```

| Value | Behavior |
|-------|----------|
| `merge` | 각 skill이 개별적으로 symlink됨. 로컬 skill이 보존됨. **(기본값)** |
| `copy` | 각 skill이 실제 파일로 복사됨. symlink를 따라갈 수 없는 AI CLI에 적합. |
| `symlink` | Target 디렉터리 전체가 하나의 symlink가 됨. |

### `target_naming`

merge/copy Sync를 위한 기본 Target 네이밍 전략.

```yaml
target_naming: flat
```

| Value | Behavior |
|-------|----------|
| `flat` | 중첩된 skill이 `__` 구분자로 평탄화됨 (예: `frontend__dev`). **(기본값)** |
| `standard` | SKILL.md의 `name` 필드를 그대로 사용 (예: `dev`). [Agent Skills spec](https://agentskills.io/specification)을 따름. |

### `targets`

동기화 대상이 되는 AI CLI skill 디렉터리.

```yaml
targets:
  <name>:
    path: <path>
    mode: <mode>  # 옵션, 기본값을 재정의
    include: [<glob>, ...]  # 옵션, merge/copy 모드에서만 사용
    exclude: [<glob>, ...]  # 옵션, merge/copy 모드에서만 사용
```

**Example:**
```yaml
targets:
  claude:
    path: ~/.claude/skills

  codex:
    path: ~/.codex/skills
    mode: symlink

  custom:
    path: ~/my-app/skills
```

#### Another account of an Agent {#agent-config-dir}

target은 내장 Agent의 두 번째 config 디렉터리일 수 있습니다. `CLAUDE_CONFIG_DIR`로 시작한 Claude Code, `CODEX_HOME`을 쓰는 Codex, `PI_CODING_AGENT_DIR`을 쓰는 Pi가 그렇습니다. Agent와 디렉터리를 지정하면 skills와 agents 경로는 그에 따라 정해집니다.

```yaml
targets:
  claude-work:
    agent: claude
    config_dir: ~/.claude-work   # skills는 ~/.claude-work/skills로, agents는 ~/.claude-work/agents로
  codex-work:
    agent: codex
    config_dir: ~/.codex-work    # skills는 ~/.codex-work/skills로
```

Codex는 공유되는 `~/.agents/skills`도 읽지만, 계정은 자기 디렉터리만 소유하므로 그 skill은 `<config_dir>/skills`로 갑니다. Pi도 같은 방식입니다. agents 디렉터리를 가지는 것은 Claude뿐입니다.

| Field | Description |
|-------|-------------|
| `agent` | 내장 Agent. `claude`(`CLAUDE_CONFIG_DIR`), `codex`(`CODEX_HOME`), `pi`(`PI_CODING_AGENT_DIR`) |
| `config_dir` | 해당 계정의 config 디렉터리. 절대 경로이거나 `~`로 시작해야 하고, Agent의 기본 디렉터리가 아니어야 하며, 하나의 target만 사용 |

`mode`, `include`, `exclude` 및 그 밖의 target 설정은 다른 target과 동일하게 동작합니다. 직접 작성한 `skills.path`나 `agents.path`는 파생된 경로보다 우선합니다. target 이름은 [MCP target](/docs/reference/commands/mcp#accounts)과 [plugin target](/docs/reference/commands/plugin#accounts)으로도 사용할 수 있습니다. [hooks target](/docs/reference/commands/hooks#accounts)으로도 사용할 수 있습니다.

#### 지침 파일 {#target-instructions}

skillshare는 많은 내장 target의 지침 파일(`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, ...)을
알고 있습니다. 그 밖의 도구에는 `instructions`로 도구가 읽는 파일을 알려 주면, 대시보드에서
그 파일을 보고 편집하며 [공유 AGENTS.md](../../how-to/daily-tasks/sharing-instructions.md)를
연결할 수 있습니다. 여기서 설정한 값은 내장 파일을 대신합니다.

```yaml
targets:
  myagent:
    path: ~/.myagent/skills
    instructions:
      path: ~/.myagent/AGENTS.md
      import: true        # the tool follows @path lines
```

| Field | Description |
|-------|-------------|
| `instructions.path` | 도구가 읽는 파일. global config에서는 절대 경로이거나 `~/`로 시작하는 경로. 프로젝트 config에서는 `.myagent/AGENTS.md`처럼 프로젝트 루트 기준 상대 경로. 디렉터리가 아니라 파일이어야 함 |
| `instructions.import` | 도구가 `@path` 줄을 따르면 `true`. 이때 여러 공유 파일을 동시에 쓸 수 있으며, 파일마다 import 줄 하나가 추가됨. 기본값 `false`: 도구는 공유 파일 하나를 쓰며, 자체 파일 자리에 링크됨 |

대시보드는 target을 추가할 때 **사용자 지정 대상** 대화 상자에서, 또는 나중에 target의 지침 탭에서 이 필드를 작성합니다. target이 공유 파일을 사용하는 동안에는
변경하거나 제거하지 않습니다. 제거해도 파일은 삭제되지 않습니다.

#### 그 밖의 파일 {#target-files}

도구는 지침 파일 외에도 Pi의 `APPEND_SYSTEM.md` 같은 일반 파일을 읽을 수 있습니다.
대시보드는 이런 파일을 각각 target 페이지의 탭으로 보여 줍니다. skillshare는 `pi`와
`omp`에 `APPEND_SYSTEM.md`를 추가하며, `files`에는 사용자가 추가한 파일이 나열됩니다.

```yaml
targets:
  pi:
    files:
      - SYSTEM.md
      - prompts/review.md
```

각 항목은 도구 폴더 기준 상대 경로입니다. pi의 경우 global config에서는 `~/.pi/agent`,
프로젝트에서는 `.pi`입니다. 항목에 하위 폴더를 쓸 수 있지만 그 폴더를 벗어날 수는 없습니다.
절대 경로, `..`, 폴더 밖을 가리키는 링크 폴더는 거부됩니다. 이 폴더는 codex의 `~/.codex`나
계정의 [`config_dir`](#agent-config-dir)처럼 도구 자체의 config 폴더이며, skillshare가 알지
못하면 skills 폴더의 상위 폴더입니다. 폴더가 홈 디렉터리나 프로젝트 루트가 되는 target에는
**+** 버튼이 없습니다.

대시보드는 탭을 추가하거나 제거할 때 이 필드를 작성합니다. 탭을 제거해도 파일은 삭제되지
않습니다.

#### Skills 끄기 {#skills-enabled}

`skills.enabled: false`는 target으로의 skill 동기화를 중지하고, skillshare는 agents, MCP 서버, 지침을 계속 관리합니다. 다른 target의 skills 폴더를 이미 읽는 도구에 사용하면 그 도구가 각 skill을 두 번 찾지 않게 됩니다.

```yaml
targets:
  pi:
    skills:
      path: ~/.pi/agent/skills
      enabled: false
```

경로, 모드, 필터는 나중에 skills를 다시 켤 때를 위해 config에 남아 있습니다. `skillshare target <name> --skills=false`로 설정하면 폴더에서 source로 향하는 링크도 함께 제거되며, `--no-skills`로 target을 추가할 수도 있습니다. [Skills 켜기/끄기](/docs/reference/commands/target#skills-off)를 참조하세요.

### `include` / `exclude` (target filters) {#include--exclude-target-filters}

**merge 및 copy 모드**에서 어떤 skill이 동기화될지 제어하려면 Target별 필터를 사용하세요.

```yaml
targets:
  codex:
    path: ~/.codex/skills
    include: [codex-*]
  claude:
    path: ~/.claude/skills
    exclude: [codex-*]
```

Rules:
- 매칭 대상은 Target의 flat 이름입니다 (예: `team__frontend__ui`)
- `include`가 먼저 적용됨
- `include` 이후에 `exclude`가 적용됨
- 패턴 문법은 Go의 `filepath.Match` (`*`, `?`, `[...]`)를 사용
- `symlink` 모드에서는 include/exclude가 무시됨
- 이전에 동기화된 Source의 링크가 제외 대상이 되면, `sync`가 해당 Target 항목을 제거함
- Target에 이미 존재하던 로컬 non-symlink 폴더는 보존됨

#### Pattern cheat sheet

| Pattern | Matches | Typical use |
|---------|---------|-------------|
| `codex-*` | `codex-agent`, `codex-rag` | 접두사 기반 그룹화 |
| `team__*` | `team__frontend__ui` | Repo/그룹 네임스페이스 |
| `*-experimental` | `rag-experimental` | 접미사 기반 정리 |
| `core-?` | `core-a`, `core-1` | 단일 문자 변형 |
| `[ab]-tool` | `a-tool`, `b-tool` | 소규모 명시적 집합 |

#### Scenario A: include only

Target이 한정된 하위 집합만 받아야 할 때 `include`를 사용하세요.

```yaml
targets:
  codex:
    path: ~/.codex/skills
    include: [codex-*, shared-*]
```

Use case:
- Codex를 코딩 워크플로우에만 집중시키기
- 글쓰기/리서치 전용 skill을 이 Target에 보내지 않기

#### Scenario B: exclude only

Target이 알려진 일부를 제외한 거의 모든 것을 받아야 할 때 `exclude`를 사용하세요.

```yaml
targets:
  claude:
    path: ~/.claude/skills
    exclude: [*-experimental, codex-*]
```

Use case:
- 하나의 주요 Target을 폭넓게 유지
- 불안정하거나 Target 전용인 skill을 숨기기

#### Scenario C: include + exclude

폭넓게 include한 뒤 예외를 골라내고 싶을 때 둘 다 사용하세요.

```yaml
targets:
  cursor:
    path: ~/.cursor/skills
    include: [core-*, team__*]
    exclude: [*-deprecated, team__legacy__*]
```

Evaluation order:
1. `include`와 일치하는 이름만 유지
2. `exclude`와 일치하는 항목 제거

Given source skills:
- `core-auth`
- `core-deprecated`
- `team__frontend__ui`
- `team__legacy__docs`
- `misc-tool`

Result for `cursor`:
- Synced: `core-auth`, `team__frontend__ui`
- Not synced: `core-deprecated`, `team__legacy__docs`, `misc-tool`

#### Managing filters via CLI

YAML을 직접 편집하는 대신 `target` 명령어를 사용하세요.

```bash
# Skills
skillshare target claude --add-include "team-*"
skillshare target claude --add-exclude "_legacy*"
skillshare target claude --remove-include "team-*"

# Agents (agents 경로가 있는 Target에서만)
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare target claude --remove-agent-include "team-*"

skillshare sync  # 변경 사항 적용
```

중복 패턴은 조용히 무시됩니다. 잘못된 glob 패턴은 오류를 반환합니다. Agent 필터는 skill 필터와 동일한 glob 문법을 사용하지만, `merge` 및 `copy` 모드에서만 동작합니다. `symlink` 모드에서는 agents 디렉터리 전체가 하나의 단위로 링크되기 때문에 agent 필터가 무시됩니다.

전체 참조 문서는 [target command](/docs/reference/commands/target#target-filters-includeexclude)를 확인하세요.

#### Skill-level targets {#skill-level-targets}

Skill은 SKILL.md의 `metadata.targets`를 사용해 어떤 Target과 호환되는지 선언할 수 있습니다. 최상위 `targets` 필드는 이전 skill과의 하위 호환을 위해 여전히 폴백으로 지원되지만, 둘 다 있을 경우 `metadata.targets`가 우선합니다.

```yaml
---
name: claude-prompts
metadata:
  targets: [claude]
---
```

이는 config 수준 include/exclude와 함께 동작하는 **두 번째 계층**의 필터링입니다.

```
Source Skills
  │
  ├─ Config include/exclude    ← Target별, 소비자가 설정
  │
  └─ Skill targets field       ← skill별, 작성자가 설정
      │
      ▼
  Skills synced to target
```

**Evaluation order:**
1. `include` — 일치하는 이름만 유지
2. `exclude` — 일치하는 이름 제거
3. `targets` 필드 — 이 Target을 포함하지 않는 targets 목록을 가진 skill 제거

두 계층 모두 통과해야 합니다 (AND 관계). Config 필터가 항상 우선합니다 — skill이 `targets: [claude]`를 선언하더라도, config의 `exclude: [claude-*]`가 여전히 그 skill을 제외시킵니다.

**Cross-mode matching:** `targets: [claude]`는 global Target `claude`와 project Target `claude` 모두와 일치합니다. 둘 다 동일한 AI CLI를 가리키기 때문입니다. [supported targets](/docs/reference/targets/supported-targets)를 참고하세요.

:::tip
**소비자**가 무엇을 어디로 보낼지 제어하고 싶을 때는 config 필터(`include`/`exclude`)를 사용하세요. **작성자**가 특정 AI CLI에서만 동작한다는 것을 알고 있을 때는 skill 수준의 `targets`를 사용하세요.
:::

#### Existing target entries when filters change

필터를 추가하거나 변경한 뒤 `skillshare sync`를 실행하면:

| Existing item in target | What happens |
|-------------------------|--------------|
| 필터로 제외된 Source 연결 symlink/junction | 제거됨 (링크 해제) |
| 필터로 제외된 관리형 복사본 (copy 모드) | 제거됨 |
| Target에 생성된 로컬 non-symlink 디렉터리 | 보존됨 |
| 관련 없는 로컬 콘텐츠 | 보존됨 |

### `skills`

원격으로 설치된 skill을 추적합니다. `skillshare install`과 `skillshare uninstall`에 의해 자동 관리됩니다.

```yaml
skills:
  - name: pdf
    source: anthropics/skills/skills/pdf
  - name: _team-skills
    source: github.com/team/skills
    tracked: true
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Skill 디렉터리 이름 |
| `source` | Yes | GitHub URL 또는 로컬 경로 |
| `tracked` | No | `--track`으로 설치된 경우 `true` (기본값: `false`) |

인자 없이 `skillshare install`을 실행하면, 아직 설치되지 않은 목록의 모든 skill이 설치됩니다. 이 덕분에 `config.yaml`은 이식 가능한 skill 매니페스트가 됩니다 — 다른 머신으로 복사한 뒤 `skillshare install && skillshare sync`를 실행하면 됩니다.

`skills:` 목록은 각 `install`과 `uninstall` 작업 후 자동으로 업데이트됩니다. 수동으로 편집할 필요가 없습니다.

:::note Migrated to .metadata.json
v0.16.2부터 설치된 skill 항목이 `config.yaml`에서 별도 파일로 이동했습니다. 현재 버전에서는 모든 설치 메타데이터가 `skills/` 디렉터리 내부의 중앙화된 `.metadata.json`에 저장됩니다. 이전 형식(`registry.yaml`, skill별 `.skillshare-meta.json`)에서의 마이그레이션은 처음 실행 시 자동으로 이루어집니다.
:::

### `agents_source` {#agents-source}

Agent를 위한 Custom source 디렉터리. 기본값인 `~/.config/skillshare/agents/`를 재정의합니다.

```yaml
agents_source: ~/my-agents
```

설정하면, 모든 agent가 기본값 대신 이 디렉터리에서 읽힙니다. `~` 확장을 지원합니다.

Default: `~/.config/skillshare/agents/` (자동 감지되므로, custom 위치를 원하지 않는 한 명시적으로 설정할 필요 없음).

:::note Global mode only
Project mode는 항상 `.skillshare/agents/`를 사용하며 `agents_source`를 지원하지 않습니다.
:::

Agent 파일 형식, Sync 동작, 지원되는 Target에 대한 자세한 내용은 [Agents](/docs/understand/agents)를 참고하세요.

### `projects` {#projects}

이 global 설정에서 skill과 agent를 받는 프로젝트 폴더. 폴더는 자체 `.skillshare/`가 필요 없으며, 어디서든 `skillshare sync`를 한 번 실행하면 모두 기록됩니다.

프로젝트마다 다른 skill을 받아야 할 때 사용하세요. Global target은 이미 모든 프로젝트에 같은 세트로 도달하고, [project mode](/docs/understand/project-skills)는 팀원을 위해 설정을 프로젝트 저장소에 보관합니다. [Many Projects, One Config](/docs/how-to/recipes/many-projects-one-config#scenario)에서 세 가지를 비교합니다.

```yaml
projects:
  <folder>:                  # 절대 경로 또는 ~로 시작
    name: <name>             # 옵션, 기본값은 폴더 이름
    targets: [<target>, ...] # 이 프로젝트에서 사용하는 tool
    skills:                  # 있으면 skill 동기화, 비어 있으면 전체
      mode: <mode>
      target_naming: <flat|standard>
      include: [<glob>, ...]
      exclude: [<glob>, ...]
    agents:                  # 있으면 agent 동기화, 비어 있으면 전체
      mode: <mode>
      include: [<glob>, ...]
      exclude: [<glob>, ...]
```

**Example:**
```yaml
projects:
  ~/work/shop-web:
    targets: [claude, cursor, codex]
    skills:
      mode: copy
      include: ["frontend-*"]
    agents: {}
  ~/work/api-server:
    targets: [claude]
    skills: {}
```

`targets`의 각 항목은 [supported target](./supported-targets.md) 이름입니다. Skillshare는 폴더 내부의 해당 tool의 프로젝트 경로에 기록합니다. 예를 들어 `claude`의 경우 `.claude/skills`와 `.claude/agents`입니다. 설정할 `path`는 없습니다.

- **공유 폴더는 한 번만 기록됩니다.** 여러 tool이 같은 프로젝트 폴더를 읽는 경우(`cursor`와 `codex` 모두 `.agents/skills`를 읽음) 하나의 sync target으로 합쳐집니다.
- **출력에 표시되는 이름.** `sync`, `status`, `diff`, `doctor`, `backup`은 프로젝트의 target을 `<name>@<target>` 형태로 표시합니다. 예: `shop-web@claude`. `name`에는 `@`, `/`, `\`를 포함할 수 없으며, 두 프로젝트가 같은 이름을 공유할 수 없습니다.
- **Agent**는 프로젝트 agent 폴더가 있는 tool에만 기록됩니다. `agents`만 있고 `skills`가 없는 프로젝트는 agent만 동기화합니다.
- **폴더가 없으면 건너뜁니다.** `sync`는 `project <folder>: folder not found, skipped`를 출력하며, 이동하거나 삭제한 프로젝트를 다시 만들지 않습니다.
- **`target`과 `collect`는 프로젝트를 건드리지 않습니다.** `skillshare target`은 `targets` 섹션만 나열하고 편집하며, `collect`는 프로젝트 자체 skill을 source로 가져오지 않습니다. `config.yaml`에서 `projects`를 직접 편집하거나 대시보드의 **프로젝트** 페이지를 사용하세요.
- `targets` frontmatter 필드가 있는 skill은 tool을 기준으로 매칭되므로, `targets: [claude]`는 `shop-web@claude`에 도달합니다.

같은 폴더의 MCP 서버는 같은 폴더를 키로 하는 [`mcp.projects`](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config) 아래에 나열됩니다. 단계별 설정은 [Many Projects, One Config](/docs/how-to/recipes/many-projects-one-config)를 참고하세요.

### `extras` {#extras}

임의의 디렉터리로 동기화할 non-skill 리소스(rules, commands, prompts 등).

```yaml
extras_source: ~/my-extras            # 옵션, 전역 기본 source
extras:
  - name: rules
    source: ~/company-shared/rules    # extra별 선택적 재정의
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
  - name: agents
    targets:
      - path: ~/.claude/agents
        flatten: true                 # 하위 디렉터리 파일을 평탄화하여 동기화
  - name: commands
    targets:
      - path: ~/.claude/commands
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Extra 식별자 |
| `source` | No | 이 extra를 위한 Custom source 디렉터리 (`extras_source`와 기본값을 재정의) |
| `file` | No | source 디렉터리에서 이 파일만 동기화: `system.md`나 `AGENTS.md` 같은 단순한 파일 이름. [single-file extras](../commands/extras.md#single-file-extras) 참고 |
| `targets` | Yes | Target 경로 목록 |
| `targets[].path` | Yes | 대상 디렉터리 |
| `targets[].mode` | No | `merge` (기본값), `copy`, 또는 `symlink`. `import`는 single-file extra 전용 |
| `targets[].as` | No | single-file extra에서 target 쪽 파일 이름 (기본값: `file` 이름) |
| `targets[].flatten` | No | `true`이면 하위 디렉터리 파일을 Target 루트로 직접 동기화 (`symlink` 또는 `file`과 함께 사용 불가) |

`extras_source`는 `skillshare init` 또는 첫 `extras init` 시 기본 경로(`~/.config/skillshare/extras/`)로 자동 채워집니다. 모든 extra에 custom 위치를 사용하려면 이 값을 재정의하세요.

**Source resolution** (3단계 우선순위):
1. extra별 `source` → 정확한 경로 (예: `~/company-shared/rules`)
2. `extras_source` → `<extras_source>/<name>/` (예: `~/my-extras/rules/`)
3. Default → `~/.config/skillshare/extras/<name>/`

**Sync modes:**
- `merge` (기본값) — 파일별 symlink
- `copy` — 파일별 복사
- `symlink` — 디렉터리 전체 symlink

동기화하려면 `skillshare sync extras`를 실행하거나, skill과 extras를 함께 동기화하려면 `skillshare sync --all`을 실행하세요.

:::info Both modes supported
Extras는 global mode와 project mode 모두에서 동작합니다. Project mode에서는 source가 `.skillshare/extras/<name>/`입니다.
:::

사용법에 대한 자세한 내용은 [sync extras](/docs/reference/commands/sync#sync-extras)를 참고하세요.

### `ignore`

Sync 중 건너뛸 파일에 대한 glob 패턴.

```yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
```

**Default patterns:**
- `**/.DS_Store`
- `**/.git/**`

### `gitlab_hosts`

중첩된 subgroup을 사용하는 self-managed GitLab 인스턴스의 호스트명. 이름에 `gitlab` 또는 `jihulab`이 포함된 호스트는 자동으로 감지됩니다 — 이 필드는 다른 custom 도메인에만 필요합니다.

```yaml
gitlab_hosts:
  - git.company.com
  - code.internal.io
```

호스트명이 여기 나열되면, `skillshare install`은 표준 `owner/repo` 두 세그먼트 분할을 가정하는 대신, 전체 URL 경로를 repository로 취급합니다 (최대 20단계까지의 중첩 subgroup 지원).

**Without `gitlab_hosts`:**
```bash
# git.company.com/team/frontend/ui → "team/frontend"를 클론, 하위 디렉터리 "ui"
skillshare install git.company.com/team/frontend/ui
```

**With `gitlab_hosts: [git.company.com]`:**
```bash
# git.company.com/team/frontend/ui → "team/frontend/ui" (전체 경로)를 클론
skillshare install git.company.com/team/frontend/ui
```

**Workaround without config:** repo 경로의 끝을 표시하기 위해 `.git`을 붙이세요.
```bash
skillshare install git.company.com/team/frontend/ui.git
```

항목은 순수 호스트명이어야 합니다 (scheme, 경로, 포트 없음). 소문자로 정규화됩니다.

#### Environment variable

Config 파일이 없는 CI/CD 파이프라인에서는 `SKILLSHARE_GITLAB_HOSTS` (쉼표로 구분)를 사용하세요.

```bash
SKILLSHARE_GITLAB_HOSTS=git.company.com,code.internal.io skillshare install git.company.com/team/frontend/ui
```

Config 파일과 환경 변수가 모두 설정된 경우, 값들은 **병합**됩니다 (중복 제거). 환경 변수의 잘못된 항목은 조용히 건너뜁니다.

### `azure_hosts`

Self-hosted Azure DevOps Server 인스턴스의 호스트명. `dev.azure.com`과 `*.visualstudio.com`에 대한 내장 패턴은 항상 활성화되어 있습니다 — 이 필드는 custom 도메인을 사용하는 온프레미스 Azure DevOps Server에만 필요합니다.

```yaml
azure_hosts:
  - azuredevops.mycompany.com
```

호스트명이 여기 나열되면, `/_git/`을 포함한 URL은 Azure DevOps 파싱 로직을 통해 라우팅되어, clone URL에 `.git`을 붙이지 않고도 org, project, repo를 정확히 추출합니다.

**Without `azure_hosts`:**

```bash
# 일반 HTTPS 파싱으로 넘어감 — clone URL은
# https://azuredevops.mycompany.com/Org/Project.git (WRONG)이 됨
skillshare install https://azuredevops.mycompany.com/Org/Project/_git/Repo
```

**With `azure_hosts: [azuredevops.mycompany.com]`:**

```bash
# 올바르게 파싱됨 — clone URL은
# https://azuredevops.mycompany.com/Org/Project/_git/Repo
skillshare install https://azuredevops.mycompany.com/Org/Project/_git/Repo
```

항목은 순수 호스트명이어야 합니다 (scheme, 경로, 포트 없음). 소문자로 정규화됩니다.

#### Environment variable

CI/CD 파이프라인에서는 `SKILLSHARE_AZURE_HOSTS` (쉼표로 구분)를 사용하세요.

```bash
SKILLSHARE_AZURE_HOSTS=azuredevops.mycompany.com skillshare install \
  https://azuredevops.mycompany.com/Org/Project/_git/Repo
```

### `gitea_hosts`

Self-hosted Gitea 인스턴스의 호스트명. `gitea.com`이나 `gitea.company.com`처럼 이름에 `gitea`가 포함된 호스트는 자동으로 감지됩니다. 이 필드는 다른 custom 도메인에만 필요합니다.

```yaml
gitea_hosts:
  - git.company.com
```

호스트명이 여기 나열되면:

- `install`과 `update`는 해당 호스트에서 HTTPS 인증을 위해 [`GITEA_TOKEN`](/docs/reference/appendix/environment-variables#gitea_token)을 사용합니다
- Sparse checkout을 사용할 수 없거나 실패할 경우, `install`은 전체 repository를 클론하는 대신 Gitea Contents API를 통해 하위 디렉터리를 다운로드합니다. API 호출마저 실패하면 전체 클론으로 폴백합니다.

항목은 순수 호스트명이어야 합니다 (scheme, 경로, 포트 없음). 소문자로 정규화됩니다.

#### Environment variable

CI/CD 파이프라인에서는 `SKILLSHARE_GITEA_HOSTS` (쉼표로 구분)를 사용하세요.

```bash
SKILLSHARE_GITEA_HOSTS=git.company.com skillshare install https://git.company.com/team/skills/review
```

Config 파일과 환경 변수가 모두 설정된 경우, 값들은 **병합**됩니다 (중복 제거).

### `cnb_hosts`

Self-hosted [CNB](https://cnb.cool) 인스턴스의 호스트명. `cnb.cool`은 자동으로 감지됩니다. 이 필드는 다른 도메인에서의 private 배포에만 필요합니다.

```yaml
cnb_hosts:
  - cnb.company.com
```

나열된 호스트는 HTTPS 인증에 [`CNB_TOKEN`](/docs/reference/appendix/environment-variables#cnb_token)을 사용하며, 하위 디렉터리 설치는 동일한 전체 클론 폴백과 함께 CNB contents API를 통해 이루어질 수 있습니다.

항목은 순수 호스트명이어야 합니다 (scheme, 경로, 포트 없음). 소문자로 정규화됩니다.

#### Environment variable

```bash
SKILLSHARE_CNB_HOSTS=cnb.company.com skillshare install https://cnb.company.com/team/skills/review
```

### `audit`

보안 audit 설정.

```yaml
audit:
  block_threshold: CRITICAL
  profile: default
  dedupe_mode: global
  enabled_analyzers: [static, dataflow, tier, integrity]
```

| Field | Values | Default | Description |
|-------|--------|---------|-------------|
| `block_threshold` | `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `INFO` | `CRITICAL` | `skillshare install`을 차단할 최소 심각도 |
| `profile` | `default`, `strict`, `permissive` | `default` | Audit 프로파일 사전 설정 (threshold와 dedupe의 기본값 설정) |
| `dedupe_mode` | `legacy`, `global` | `global` | 발견 항목 중복 제거 모드 |
| `enabled_analyzers` | analyzer ID 배열 | *(전체)* | 실행할 analyzer의 allowlist (생략 시 전체) |

**Profiles**는 명시적 필드 값으로 재정의할 수 있는 합리적인 기본값을 설정합니다.

| Profile | Threshold | Dedupe | Description |
|---------|-----------|--------|-------------|
| `default` | `CRITICAL` | `global` | 현재 동작과 동일 |
| `strict` | `HIGH` | `global` | 보안을 중시하는 팀을 위한 더 엄격한 차단 |
| `permissive` | `CRITICAL` | `legacy` | 권고 전용, 최소한의 차단 |

**Analyzer IDs:** `static`, `dataflow`, `tier`, `integrity`, `structure`, `cross-skill`

**Precedence:** CLI flag → project config → global config → profile 기본값.

- `block_threshold`는 install이 **차단**되는 시점만 제어합니다 — 스캔은 항상 실행됩니다
- 단일 install에 대해 스캔을 우회하려면 `--skip-audit`을 사용하세요
- 차단을 재정의하려면 `--force`를 사용하세요 (발견 항목은 여전히 표시됨)

### `context_budget`

토큰 예산 경고 임계값. `sync`와 `analyze` 이후 토큰 수가 예산을 초과하면 경고가 표시됩니다.

```yaml
context_budget:
  warn_always_loaded_tokens: 10000
  warn_on_demand_tokens: 100000
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `warn_always_loaded_tokens` | integer | `10000` | Always-loaded 토큰이 이 값을 초과하면 경고. `0`은 비활성화 |
| `warn_on_demand_tokens` | integer | `100000` | On-demand 토큰이 이 값을 초과하면 경고. `0`은 비활성화 |

생략하면 기본값(10K / 100K)이 적용됩니다. 경고를 억제하려면 `skillshare sync --quiet`를 사용하세요. 출력 형식은 [sync — Context Cost](/docs/reference/commands/sync#context-cost)를 참고하세요.

### `preserve_tilde_on_save`

`true`이면 `config.yaml`을 쓰기 전에 `$HOME` 접두사를 다시 `~`로 접습니다. Config가 dotfiles(stow, chezmoi, yadm, bare git repo)를 통해 공유될 때 유용하며, 디스크상의 config를 여러 머신에서 이식 가능하게 유지합니다.

```yaml
preserve_tilde_on_save: true
```

**Default:** `false` (기존 동작 그대로 — 경로가 절대 경로로 저장됨).

이 flag가 없으면, 저장할 때마다 `~/...` 경로가 `/home/alice/...` (확장된 형태)로 다시 쓰입니다. Config가 버전 관리되고 여러 머신에서 공유될 때, 이는 지저분한 diff를 만들고 이식성을 깨뜨립니다.

이 flag를 활성화하면, 직렬화된 YAML은 `$HOME` 아래의 모든 경로에 `~`를 사용합니다.

```yaml
# Before (기본값): 절대 경로, 머신별
source: /home/alice/.config/skillshare/skills
targets:
  claude:
    skills:
      path: /home/alice/.claude/skills

# After (preserve_tilde_on_save: true): 이식 가능
source: ~/.config/skillshare/skills
targets:
  claude:
    skills:
      path: ~/.claude/skills
```

메모리 상의 config는 영향받지 않습니다 — `Load()`는 여전히 평소처럼 `~`를 확장합니다. Home이 아닌 절대 경로(예: `/opt/shared/skills`)는 그대로 통과합니다.

:::note Global mode only
이 옵션은 global `config.yaml`에만 적용됩니다. Project config(`.skillshare/config.yaml`)는 일반적으로 상대 경로를 사용하며 tilde 접기가 필요하지 않습니다.
:::

### `git_root` {#git-root}

`skillshare commit`, `push`, `pull`이 어떤 디렉터리에서 동작할지 선택합니다.

```yaml
git_root: skills
```

| Value | Directory versioned |
|-------|---------------------|
| `skills` (기본값) | Skill source (`~/.config/skillshare/skills/`) |
| `agents` | Agent source (`~/.config/skillshare/agents/`) |
| `extras` | Extras source (`~/.config/skillshare/extras/`) |
| `root` | Config root (`~/.config/skillshare/`) — skill + agent + extras를 하나의 repo에; `config.yaml`은 자동으로 무시됨 |

**Default:** `skills`

`skillshare init --git-root <scope>`로 init 중에 설정하거나, init 마법사에서 대화형으로 설정하세요.

#### Changing the scope after init

이미 초기화된 설정에서 헤드리스로 scope를 전환할 수 있습니다.

```bash
skillshare init --git-root <scope>   # global mode; cwd가 project라면 -g 추가
```

이 명령은 새 scope 디렉터리에 git repo를 초기화하고 (이미 있다면 재사용), config에 `git_root`를 저장하며, 프롬프트를 표시하거나 `--remote`를 요구하지 않습니다. 다만 기존 repo를 **이동시키지는 않습니다** — scope 전환은 "다른 디렉터리를 버전 관리하기 시작한다"는 의미이지, "히스토리를 이전한다"는 의미가 아닙니다.

- **Fresh history** — `skillshare init --git-root <scope>`는 새 scope에 빈 repo를 초기화합니다.
- **Keep history** — 먼저 `mv <old-scope>/.git <new-scope>/.git`을 실행한 뒤, `skillshare init --git-root <scope>`로 scope를 기록하세요.

`config.yaml`에서 `git_root`를 직접 편집할 수도 있습니다. `git_root`가 repo가 없는 디렉터리를 가리키는데 다른 scope 디렉터리에는 repo가 있다면, `commit`/`push`/`pull`은 문제를 해결할 정확한 `skillshare init` / `mv` 명령을 포함한 "Git root mismatch" 오류를 출력합니다.

:::note Global mode only
`git_root`는 global mode에만 적용됩니다. Project mode는 `.skillshare/` 디렉터리를 사용하며 이 필드를 지원하지 않습니다.
:::

---

## Project Config

**Location:** `.skillshare/config.yaml` (프로젝트 루트 안)

Project config는 global config와 다른 형식을 사용합니다.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/project-config.schema.json
# Targets — 문자열 또는 객체 형태
targets:
  - claude                    # 문자열: 기본값을 가진 알려진 Target
  - cursor
  - name: custom-ide               # 객체: custom 경로와 모드
    path: ./tools/ide/skills
    mode: symlink
  - name: codex                    # 필터가 있는 객체
    include: [codex-*]
    exclude: [codex-experimental-*]

# Remote skill — install/uninstall에 의해 자동 관리됨
skills:
  - name: pdf
    source: anthropic/skills/pdf
  - name: _team-skills
    source: github.com/team/skills
    tracked: true                  # git 히스토리와 함께 클론됨

# Audit — global과 동일한 필드
audit:
  block_threshold: HIGH
  profile: strict
```

### `targets` (project)

두 가지 YAML 형태를 지원합니다.

| Form | Example | When to use |
|------|---------|-------------|
| **String** | `- claude` | 알려진 Target, 기본 경로와 merge 모드 |
| **Object** | `- name: x, path: ..., mode: ..., include: [...], exclude: [...]` | Custom 경로, 모드 재정의, 또는 Target별 필터 |

Object 항목에는 프로젝트 루트 기준 상대 경로로 [`instructions`](#target-instructions)도 설정할 수 있습니다.

### `skills` (project)

[Global `skills` 필드](#skills)와 동일한 스키마입니다. `skillshare install -p`와 `skillshare uninstall -p`에 의해 자동 관리됩니다.

:::tip Portable Manifest
`config.yaml`은 global mode와 project mode 모두에서 이식 가능한 skill 매니페스트입니다. 새 머신에서 `skillshare install && skillshare sync`(또는 프로젝트에서 `skillshare install -p`)를 실행하면 동일한 설정을 재현할 수 있습니다.
:::

---

## Managing Config

### View current config

```bash
skillshare status
# Source, Targets, 모드를 표시
```

### Edit config directly

```bash
# 에디터에서 열기
$EDITOR ~/.config/skillshare/config.yaml

# 그런 다음 변경 사항을 적용하기 위해 동기화
skillshare sync
```

### Reset config

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## Custom Audit Rules

**Location:**

| Mode | Path |
|------|------|
| Global | `~/.config/skillshare/audit-rules.yaml` |
| Project | `.skillshare/audit-rules.yaml` |

규칙은 다음 순서로 병합됩니다: **내장 → global → project**. 새 규칙을 추가하거나, 내장 규칙을 비활성화하거나, 심각도를 재정의할 수 있습니다.

```yaml
rules:
  # Custom 규칙 추가
  - id: flag-todo
    severity: MEDIUM
    pattern: todo-comment
    message: "TODO comment found"
    regex: '(?i)\bTODO\b'

  # 내장 규칙 비활성화
  - id: insecure-http-0
    enabled: false
```

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | 고유 규칙 식별자 |
| `severity` | Yes | `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `INFO` |
| `pattern` | Yes | 패턴 카테고리 이름 |
| `message` | Yes | 사람이 읽을 수 있는 발견 항목 설명 |
| `regex` | Yes | 매칭할 정규 표현식 |
| `exclude` | No | 라인이 이 정규 표현식에도 일치하면 매칭을 억제 |
| `enabled` | No | 내장 규칙을 비활성화하려면 `false`로 설정 |

시작 파일을 생성하려면:

```bash
skillshare audit --init-rules       # Global
skillshare audit --init-rules -p    # Project
```

전체 내용은 [audit command](/docs/reference/commands/audit)를 참고하세요.

---

## Environment Variables

| Variable | Description |
|----------|-------------|
| `SKILLSHARE_CONFIG` | Config 파일 경로 재정의 |
| `GITHUB_TOKEN` | API rate limit 문제 해결용 |

**Example:**
```bash
SKILLSHARE_CONFIG=~/custom-config.yaml skillshare status
```

---

## Skill Metadata

Skill을 설치하면, skillshare는 중앙화된 `.metadata.json` 파일에 해당 메타데이터를 기록합니다.

```json
{
  "skills": [
    {
      "name": "pdf",
      "source": "anthropics/skills/skills/pdf",
      "type": "github",
      "installed_at": "2026-01-20T15:30:00Z",
      "repo_url": "https://github.com/anthropics/skills.git",
      "subdir": "skills/pdf",
      "version": "abc1234"
    }
  ]
}
```

각 skill 항목은 다음을 포함합니다.

| Field | Description |
|-------|-------------|
| `name` | Skill 디렉터리 이름 |
| `source` | 원래 install source 입력값 |
| `type` | Source 유형 (`github`, `local` 등) |
| `installed_at` | 설치 타임스탬프 |
| `repo_url` | Git clone URL (git source에만 해당) |
| `subdir` | 하위 디렉터리 경로 (모노레포 source에만 해당) |
| `version` | 설치 시점의 Git commit hash |

이는 `skillshare update`와 `skillshare check`가 업데이트를 어디서 가져올지 알기 위해 사용됩니다.

**이 파일을 수동으로 편집하지 마세요.**

---

## Platform Differences

### macOS / Linux

```yaml
source: ~/.config/skillshare/skills
targets:
  claude:
    path: ~/.claude/skills
```

symlink를 사용합니다.

### Windows

```yaml
source: %AppData%\skillshare\skills
targets:
  claude:
    path: %USERPROFILE%\.claude\skills
```

폴더는 NTFS junction으로 링크됩니다 (관리자 권한 불필요). 단일 파일 — `merge` 모드의 agents와 디렉터리 extras, 그리고 single-file extras — 은 파일 symlink가 필요하며, 이는 Developer Mode가 있어야 합니다. 없으면 대신 복사됩니다. [Windows 문제 해결](/docs/troubleshooting/windows#file-links-need-windows-developer-mode-copying-instead)을 참고하세요.

---

## Related

- [Source & Targets](/docs/understand/source-and-targets) — 핵심 개념
- [Sync Modes](/docs/understand/sync-modes) — Merge vs copy vs symlink
- [Environment Variables](/docs/reference/appendix/environment-variables) — 모든 변수
