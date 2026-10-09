---
sidebar_position: 1
---

# target

sync target(AI CLI skill 디렉터리)을 관리합니다.

```bash
skillshare target add <name> <path>    # Add a target
skillshare target remove <name>        # Remove a target
skillshare target list                 # List all targets
skillshare target <name>               # Show target info
skillshare target <name> --mode merge  # Change sync mode
skillshare target <name> --target-naming standard  # Change naming
skillshare target <name> --skills=false    # Stop syncing skills
```

## 언제 사용하나요

- 새 AI CLI 도구를 설치한 후 새 target을 추가할 때
- 더 이상 사용하지 않는 target을 제거할 때
- target의 sync mode(merge, copy, symlink)를 변경할 때
- target의 naming(flat, standard 또는 prefixed)을 변경할 때
- 하나의 global mode를 강제하는 대신 target별로 호환성을 조정할 때
- 선택적 skill 동기화를 위한 include/exclude 필터를 설정할 때
- 다른 target의 폴더를 이미 읽는 도구에 skill 동기화를 중지하면서, agents, MCP 서버, 지침은 계속 관리하고 싶을 때

## 하위 명령어

### target add

skill 동기화를 위한 새 target을 추가합니다.

```bash
skillshare target add windsurf ~/.windsurf/skills
```

이 명령은 다음을 검증합니다.
- 경로가 존재하거나 상위 디렉터리가 존재함
- 경로가 skill 디렉터리처럼 보임
- target 이름이 고유함

skill을 동기화하지 않고 target을 추가하려면 `--no-skills`를 붙이세요. agents, MCP 서버, 지침은 계속 관리되며, skills 폴더가 존재하지 않아도 됩니다.

```bash
skillshare target add gemini ~/.gemini/skills --no-skills
# Added target: gemini -> ~/.gemini/skills (skills off)
```

[Skills 켜기/끄기](#skills-off)를 참조하세요.

#### Another account of an Agent {#another-account}

`CLAUDE_CONFIG_DIR=~/.claude-work`로 실행하는 Claude Code, `CODEX_HOME`을 쓰는 Codex, `PI_CODING_AGENT_DIR`을 쓰는 Pi처럼 Agent의 두 번째 계정을 별도의 config 디렉터리에서 사용한다면, 그 디렉터리를 target으로 추가하세요. Skillshare가 그 디렉터리에서 skills와 agents 경로를 알아냅니다.

```bash
skillshare target add claude-work --agent claude --config-dir ~/.claude-work
# Added target: claude-work -> ~/.claude-work/skills
```

가지고 있는 계정 수만큼, 각각 고유한 이름으로 추가하세요. 이 이름은 [MCP target](./mcp.md#accounts)으로도 사용할 수 있으므로, 한 번의 sync로 모든 계정의 skill, agent, MCP 서버에 반영됩니다.

계정이 호환 CLI(예: Pi의 omo)를 사용한다면, `--cli`를 지정해 그 계정의 [plugin 명령](./plugin.md#accounts)이 해당 CLI를 사용하게 하세요.

```bash
skillshare target add omo --agent pi --config-dir ~/.omo/agent --cli omo
```

`--agent`는 `claude`(`CLAUDE_CONFIG_DIR`), `codex`(`CODEX_HOME`), `pi`, `omp`(둘 다 `PI_CODING_AGENT_DIR` 사용)를 받습니다. Codex, Pi, OMP 계정은 skill을 `<config_dir>/skills`로 동기화하며, agents 디렉터리를 함께 가지는 것은 Claude뿐입니다. OMP 계정은 skills, instructions, files, MCP, 네이티브 코드 hooks를 지원하지만 plugin 명령은 지원하지 않습니다. 디렉터리는 절대 경로이거나 `~`로 시작해야 하고, Agent의 기본 디렉터리여서는 안 되며, 두 target이 함께 사용할 수 없습니다.

이런 target을 제거할 때 MCP 때문에 실패하는 일은 없습니다. `mcp.targets`나 어떤 서버의 `targets`가 여전히 그 이름을 가리키고 있어도, `skillshare target remove`는 target을 제거하고 그쪽에서도 이름을 빼라고 경고합니다.

### target remove

target을 제거하고 해당 skill을 일반 디렉터리로 복원합니다.

```bash
skillshare target remove cursor           # Remove single target
skillshare target remove --all            # Remove all targets
skillshare target remove cursor --dry-run # Preview
```

**진행 과정:**
1. target의 백업을 생성합니다
2. sync mode를 감지합니다.
   - **Symlink mode:** 디렉터리 symlink를 제거하고 source 콘텐츠를 실제 디렉터리로 다시 복사합니다
   - **Merge mode:** source를 가리키는 symlink만(경로 접두사 기준) 제거하고 각 skill을 실제 파일로 다시 복사합니다. 로컬(symlink가 아닌) skill은 보존됩니다.
   - **Copy mode:** `.skillshare-manifest.json`을 제거합니다. 관리되는 복사본과 로컬 skill은 일반 디렉터리로 보존됩니다.
3. config에서 target을 제거합니다

같은 skills 폴더에 쓰는 다른 target이 있으면(예: `codex`와 `universal`은 둘 다 `~/.agents/skills`를 사용) 2단계를 건너뜁니다. skills는 그 target을 위해 링크된 채 남고, 제거한 target만 config에서 빠집니다.

[skills를 끈](#skills-off) target은 동기화된 것이 없으므로 이 경우에도 2단계를 건너뛰고 폴더를 그대로 둡니다.

### target list

구성된 모든 target을 나열합니다.

```bash
skillshare target list                 # Interactive TUI (default on TTY)
skillshare target list --no-tui        # Plain text output
skillshare target list --json          # JSON output for CI/scripts
```

#### Interactive TUI

TTY에서 `target list`는 대화형 화면을 엽니다. 왼쪽에는 target, 오른쪽에는 선택한 target의 경로, 모드, 필터가 표시됩니다. 여기서 target의 sync 모드, 이름 지정 방식, include/exclude 필터를 바꾸거나 target을 제거할 수 있습니다(`target remove`와 같이 백업 후 연결 해제). 키는 화면 아래쪽에 표시됩니다.

변경 사항은 즉시 config에 저장됩니다. 적용하려면 `skillshare sync`를 실행하세요.

TUI를 건너뛰고 일반 텍스트를 출력하려면 `--no-tui`를 사용하세요.

```
claude
  Skills    ~/.claude/skills  merge · flat · merged · 43 shared
  Agents    ~/.claude/agents  merge · 2/2 linked

cursor
  Skills    ~/.cursor/skills  merge · flat · merged · 43 shared, 1 local
  Agents    ~/.cursor/agents  merge · 2/2 linked

codex
  Skills    ~/.openai-codex/skills  symlink · flat · linked

3 targets
```

#### JSON Output

```bash
skillshare target list --json
```

```json
{
  "targets": [
    {
      "name": "claude",
      "path": "~/.claude/skills",
      "mode": "merge",
      "targetNaming": "flat",
      "include": [],
      "exclude": [],
      "skillsEnabled": true
    },
    {
      "name": "cursor",
      "path": "~/.cursor/skills",
      "mode": "merge",
      "targetNaming": "standard",
      "include": [],
      "exclude": [],
      "skillsEnabled": true
    }
  ]
}
```

`warning`은 sync가 해당 target을 거부할 때만 추가됩니다(예: copy 이외의 mode에서 `prefixed` naming). 문구에 해결 방법이 포함됩니다.

### target info / settings

target 세부 정보를 표시하거나 설정을 변경합니다.

```bash
# Show info
skillshare target claude

# Change mode
skillshare target claude --mode symlink
skillshare target claude --mode merge

# Change target naming
skillshare target claude --target-naming standard
skillshare target claude --target-naming flat

skillshare sync  # Apply changes
```

## Sync Modes

| Mode | Behavior |
|------|----------|
| `merge` | 각 skill이 개별적으로 symlink됩니다. 로컬 skill을 보존합니다. **기본값.** |
| `copy` | 각 skill이 실제 파일로 복사됩니다. symlink를 따라갈 수 없는 AI CLI용. |
| `symlink` | 디렉터리 전체가 하나의 symlink입니다. 모든 곳에서 정확히 동일한 복사본. |

`target --mode`는 주된 호환성 제어 수단입니다. global 기본값은 단순하게 유지하고, 필요한 곳에서만 재정의하세요.

## Target Naming

| Naming | Behavior |
|--------|----------|
| `flat` | 중첩된 skill이 `__` 구분자로 평탄화됩니다(예: `frontend__dev`). **기본값.** |
| `standard` | SKILL.md의 `name` 필드를 그대로 사용합니다(예: `dev`). [Agent Skills spec](https://agentskills.io/specification)을 따릅니다. |
| `prefixed` | Copy mode 전용. `standard`와 같지만 tracked repo 안의 skill은 폴더 이름과 복사본의 `name:` 모두 `<repo>-<name>`으로 이름이 지정됩니다(예: `mattpocock-skills-prototype`). |

`target --target-naming`은 target에서 skill 디렉터리의 이름 지정 방식을 제어합니다. `standard` 및 `prefixed` mode에서는 이름이 유효하지 않거나 충돌하는 skill에 대해 경고가 표시되고 건너뜁니다. `flat`과 `standard`는 symlink mode에서 무시됩니다. target이 copy mode로 skills를 sync하지 않으면 `--target-naming prefixed`는 거부되며, target이 `prefixed`를 사용하는 동안 `--mode`로 copy mode를 벗어나는 것도 거부됩니다. 둘을 한 번에 바꾸려면 함께 지정하세요: `skillshare target cursor --mode copy --target-naming prefixed`. `--mode`, `--agent-mode`, `--target-naming`은 이렇게 함께 쓸 수 있으며, 함께 검사되어 한 번에 저장됩니다. target이 이미 가진 값은 변경되지 않음으로 표시됩니다. `--skills`나 include/exclude 플래그와는 함께 쓸 수 없으니 별도 명령으로 실행하세요. [Target Naming](/docs/understand/sync-modes#target-naming) 참고.

```bash
# Set target to copy mode (for Cursor, Copilot CLI, etc.)
skillshare target cursor --mode copy
skillshare sync  # Apply the change
```

### 혼합 전략 예시

```bash
# Keep default merge behavior for most targets
skillshare target claude --mode merge

# Compatibility-first for one target
skillshare target cursor --mode copy

# Exact mirror for another target
skillshare target codex --mode symlink

skillshare sync
```

## Target Filters (include/exclude) {#target-filters-includeexclude}

CLI에서 skill과 agent 모두에 대해 target별 include/exclude 필터를 관리합니다.

```bash
# Skills
skillshare target claude --add-include "team-*"
skillshare target claude --add-exclude "_legacy*"
skillshare target claude --remove-include "team-*"
skillshare target claude --remove-exclude "_legacy*"

# Agents
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare target claude --remove-agent-include "team-*"
skillshare target claude --remove-agent-exclude "draft-*"
```

필터를 변경한 후에는 `skillshare sync`를 실행해 적용하세요.

필터는 **merge 및 copy mode**에서 동작합니다. 패턴은 Go `filepath.Match` 문법(`*`, `?`, `[...]`)을 사용합니다. symlink mode에서는 필터가 무시됩니다.

Agent 필터는 빌트인 target 정의 또는 config의 명시적 `agents.path` 재정의를 통해 agents 경로를 가진 target에서만 사용할 수 있습니다.

패턴 치트 시트와 시나리오는 [Configuration](/docs/reference/targets/configuration#include--exclude-target-filters)을 참조하세요.

:::tip
Target 필터는 세 가지 필터링 계층 중 하나입니다. `.skillignore` 및 SKILL.md `targets`와 어떻게 상호작용하는지는 [Filtering Reference](/docs/reference/filtering)를 참조하세요.
:::

## Skills 켜기/끄기 {#skills-off}

어떤 도구는 자체 폴더뿐 아니라 다른 target의 폴더에서도 skill을 읽습니다. 예를 들어 Pi는 `~/.pi/agent/skills`와 함께 `universal` target의 폴더인 `~/.agents/skills`도 읽습니다. 두 곳 모두에 skill을 동기화하면 Pi가 각 skill을 두 번 찾게 됩니다. Pi는 먼저 찾은 것을 유지하고 다른 하나에 대해 경고하며, 일부 도구는 둘 다 표시합니다. 해당 target의 skills를 끄면, skillshare는 agents, MCP 서버, 지침은 계속 관리하되 skills 폴더는 건드리지 않습니다.

```bash
skillshare target pi --skills=false --dry-run   # Preview
skillshare target pi --skills=false
```

```
✓ Removed   2 links  alpha, beta
  Kept      1 local skill  my-notes

✓ Skills off for pi
  Agents, MCP servers and instructions are still managed
```

skills를 끄면 config에 `skills.enabled: false`가 저장되고, 이어서 폴더를 정리합니다.

- **Merge mode:** source를 가리키는 링크를 제거합니다. 직접 만든 skill은 남습니다.
- **Symlink mode:** 폴더에서 source로 향하는 링크만 제거하며, 링크가 가리키는 대상은 절대 제거하지 않습니다.
- **Copy mode:** 복사본은 직접 편집했을 수도 있는 실제 폴더이므로 유지하고, 따로 나열합니다. 도구는 여전히 이 복사본을 읽으므로, 같은 skill을 다른 폴더에서도 읽는다면 복사본을 직접 삭제하세요.

  ```
  ! Kept      2 copied skills  alpha, beta

  ✓ Skills off for pi
    The tool still loads these copies; delete them if it reads the same skills elsewhere
    Agents, MCP servers and instructions are still managed
  ```

- **공유 폴더:** skills가 켜진 다른 target이 같은 폴더에 쓰고 있으면 아무것도 제거하지 않습니다.

이후 `sync`, `diff`, `status`, `doctor`는 해당 target의 skills를 건너뛰며, `status`와 `sync`는 이를 `skills off`로 표시합니다. `--skills=true`로 skills를 다시 켜면 다음 `skillshare sync`에서 다시 동기화됩니다.

`--skills`는 한 명령에서 include/exclude 플래그와 함께 사용할 수 없으므로 따로 실행하세요. project mode(`-p`)에서도 동일하게 동작합니다.

웹 대시보드에서는 target의 Skills 탭에서 **Skills 동기화 중지**를 사용하세요. 제거하기 전에 무엇이 제거되고 무엇이 남는지 보여 주며, 다른 도구가 같은 폴더를 읽고 있으면 경고합니다.

## 옵션

### target add

| Flag | Description |
|------|-------------|
| `--agent <agent>` | 경로 대신 이 Agent의 [다른 계정](#another-account)을 추가합니다. `--config-dir`과 함께 사용 |
| `--config-dir <dir>` | 해당 계정이 사용하는 config 디렉터리 |
| `--cli <executable>` | 해당 계정의 plugin 명령을 Agent 자체 대신 이 호환 CLI로 실행. `PATH`에 있는 이름이나 절대 경로 |
| `--no-skills` | [skills를 끈](#skills-off) 상태로 target 추가 |

### target remove

| Flag | Description |
|------|-------------|
| `--all, -a` | 모든 target 제거 |
| `--dry-run, -n` | 변경 없이 미리보기 |

### target list

| Flag | Description |
|------|-------------|
| `--json` | JSON으로 출력 |
| `--no-tui` | 대화형 TUI 비활성화, 일반 텍스트 출력 사용 |

### target info / settings

| Flag | Description |
|------|-------------|
| `--mode, -m <mode>` | sync mode 설정(merge, copy, symlink) |
| `--agent-mode <mode>` | agents sync mode 설정(merge, copy, symlink) |
| `--target-naming <naming>` | target naming 설정(flat, standard 또는 prefixed; prefixed는 copy mode 필요) |
| `--skills <true\|false>` | skills 동기화 [켜기 또는 끄기](#skills-off). `--skills=false` 형식도 가능 |
| `--dry-run, -n` | `--skills=false`와 함께 사용 시 제거될 항목 미리보기 |
| `--add-include <pattern>` | include 필터 패턴 추가 |
| `--add-exclude <pattern>` | exclude 필터 패턴 추가 |
| `--remove-include <pattern>` | include 필터 패턴 제거 |
| `--remove-exclude <pattern>` | exclude 필터 패턴 제거 |
| `--add-agent-include <pattern>` | agent include 필터 패턴 추가 |
| `--add-agent-exclude <pattern>` | agent exclude 필터 패턴 추가 |
| `--remove-agent-include <pattern>` | agent include 필터 패턴 제거 |
| `--remove-agent-exclude <pattern>` | agent exclude 필터 패턴 제거 |

## Supported AI CLIs

skillshare는 `init` 중에 다음을 자동 감지합니다.

| CLI | Default Path |
|-----|-------------|
| Claude Code | `~/.claude/skills` |
| Cursor | `~/.cursor/skills` |
| OpenCode | `~/.opencode/skills` |
| Windsurf | `~/.windsurf/skills` |
| Codex | `~/.openai-codex/skills` |
| Antigravity (앱) | `~/.gemini/config/skills` |
| Antigravity CLI | `~/.gemini/antigravity-cli/skills` |
| Gemini CLI | `~/.gemini/skills` |
| Amp | `~/.amp/skills` |
| ... 그 외 45개 이상 | [지원 target](/docs/reference/targets/supported-targets) 참조 |

## 예시

```bash
# Add custom target
skillshare target add my-tool ~/my-tool/skills

# Check target status
skillshare target claude

# Switch to copy mode (for AI CLIs that can't read symlinks)
skillshare target cursor --mode copy
skillshare sync

# Switch to symlink mode
skillshare target claude --mode symlink
skillshare sync

# Configure agent sync mode
skillshare target claude --agent-mode copy
skillshare sync

# Add/remove skill filters
skillshare target claude --add-include "team-*"
skillshare target claude --add-exclude "_legacy*"
skillshare target claude --remove-include "team-*"
skillshare sync

# Add/remove agent filters
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare sync

# Remove target (restores skills)
skillshare target remove cursor
```

## Project Mode

현재 project의 target을 관리합니다.

```bash
skillshare target add windsurf -p                                # Add known target
skillshare target add custom ./tools/ai/skills -p                # Add custom path
skillshare target remove cursor -p                                # Remove target
skillshare target list -p                                         # List project targets
skillshare target claude -p                                  # Show target info
skillshare target claude --add-include "team-*" -p          # Add filter
skillshare target claude --add-agent-include "team-*" -p    # Add agent filter
```

### 차이점

| | Global | Project (`-p`) |
|---|---|---|
| Config | `~/.config/skillshare/config.yaml` | `.skillshare/config.yaml` |
| Paths | 절대 경로(예: `~/.claude/skills`) | 상대 또는 절대 경로(예: `.claude/skills`) |
| Sync mode | Merge, copy, symlink | Merge, copy, symlink(기본값 merge) |
| Mode change | `--mode` flag | `--mode` flag |

### Project Target List 예시

```
claude
  Skills    .claude/skills  merge · flat · merged · 3 shared

cursor
  Skills    .cursor/skills  merge · flat · merged · 3 shared

custom-tool
  Skills    ./tools/ai/skills  merge · flat · merged · 3 shared

3 targets
```

Project mode의 target은 다음을 지원합니다.
- **알려진 target 이름**(예: `claude`, `cursor`) — project 로컬 경로로 해석됨
- **커스텀 경로** — project 루트 기준 상대 경로 또는 `~` 확장을 포함한 절대 경로

## 참고

- [sync](/docs/reference/commands/sync) — target에 skill 동기화
- [status](/docs/reference/commands/status) — target 상태 표시
- [Targets](/docs/reference/targets) — target 관리 가이드
- [Project Skills](/docs/understand/project-skills) — project mode 개념
