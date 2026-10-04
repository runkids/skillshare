---
sidebar_position: 2
---

# extras

skill과 함께 동기화되는 skill이 아닌 리소스(rules, commands, prompts 등)를 관리합니다.

## Overview

Extras는 skillshare가 관리하는 추가 리소스 유형입니다 — "skill이 아닌 콘텐츠를 위한 skill"이라고 생각하면 됩니다. 일반적인 사용 사례로는 AI rules, 에디터 commands, prompt 템플릿을 여러 도구에 걸쳐 동기화하는 것이 있습니다.

각 extra는 다음을 가집니다:
- **name** (예: `rules`, `prompts`, `commands`)
- **source directory** — `extras_source` 또는 extra별 `source`로 설정 가능하며, 기본값은 `~/.config/skillshare/extras/<name>/` (전역) 또는 `.skillshare/extras/<name>/` (프로젝트)
- 파일이 동기화되는 하나 이상의 **target**

대시보드의 **Extras → Folders & files**에는 각 extra와 그 Target, 모드가 표시됩니다.

![Extras › Folders & files: 각 Target에 동기화된 rules와 commands](/img/extras-folders.png)

## Commands

### `extras memory` {#extras-memory}

`memory` extra의 공유 Markdown 노트를 관리합니다. 원하는 텍스트 편집기를 사용할 수 있습니다.
[스크린샷 가이드](../../how-to/daily-tasks/sharing-memory)에서 생성부터 Agent 연결까지 확인하세요.

| 하위 명령 | 동작 |
|---|---|
| `init` | Target이 없는 memory extra를 등록하고 누락된 `INDEX.md`, `LEARNED.md`를 생성. 기존 파일과 설정 유지 |
| `list` | 노트 목록. `--search <text>`로 하위 폴더를 포함한 경로와 내용을 대소문자 구분 없이 검색 |
| `show <note.md>` | 노트 읽기. `--json`에는 `version` hash 포함 |
| `write <note.md> --from <file\|->` | 파일 또는 stdin에서 내용 읽기. 새 노트는 `--version`을 생략하고 업데이트는 마지막으로 읽은 version 필요 |
| `delete <note.md> --version <hash>` | 백업 후 지정 version 삭제. 오래되거나 누락된 version은 거부 |
| `instructions` | 실제 source 폴더를 가리키는 읽기 지침 출력. 두 모드 모두 각 작업 시작 시 `INDEX.md`를 읽습니다. `--update-mode passive`(기본값)는 남길 만한 사실을 알려 주되 요청할 때만 노트를 업데이트하게 하고, `--update-mode active`는 그런 사실을 도구 자체 메모리 대신 여기에 저장하고 확실하지 않으면 제안하게 합니다 |

각 하위 명령은 `--json`, `-g` / `--global`, `-p` / `--project`, `--help`를 지원합니다.
Scope는 생략하면 자동 감지합니다. 기본 global 경로는 `~/.config/skillshare/extras/memory/`,
Project는 `.skillshare/extras/memory/`입니다. 기존 extras source 설정이 적용됩니다.

노트는 상대 `.md` 경로의 UTF-8 파일이며 최대 1 MiB입니다. 숨김 파일, 숨김 폴더, 내부 심볼릭 링크는 제외합니다. 너무 크거나 UTF-8이 아닌 파일은 지원되지 않음으로 목록에 남고 다른 유효한 노트는 사용할 수 있습니다. `wiki/architecture.md`는 필요한 폴더를 생성합니다. Dashboard는 트리, **Preview** / **Source**, **Copy path**, **Edit**, **Delete note**, **History**를 제공합니다. **Move or rename**에서 새 상대 `.md` 경로를 지정합니다. 없는 폴더를 만들고 내용과 권한을 보존하며 기존 대상이나 오래된 version은 거부합니다. 이동 전 원래 경로를 백업합니다. Markdown 링크는 직접 수정하세요. Agent 지침이 가리키는 source 루트의 `INDEX.md`는 그 위치에 유지하세요.

저장은 마지막으로 읽은 version을 확인합니다. 충돌하면 초안을 보존하고 최신 저장 내용을 비교용으로 표시합니다. **Save my draft**는 확인 후 갱신된 version을 사용하며 저장 내용을 백업한 다음 대체합니다. 삭제도 확인, version 검사, 백업을 수행합니다. **History**와 삭제 후 복원 링크는 노트의 절대 경로로 필터링된 **Backup Files**를 엽니다. CLI에서는 `backup files show <absolute-path>`와 `backup files restore <absolute-path> <id>`를 사용합니다.

**New note**의 **Link from INDEX.md**는 인덱스를 읽을 수 있을 때 표시되며 기본으로 선택됩니다. 파일 끝에 링크를 추가하고 version을 확인하며 백업합니다. 실패해도 새 노트는 유지됩니다. **Add to INDEX**로 인덱스에 없는 노트를 추가할 수 있습니다. 깨진 링크는 경고하지만 자동 삭제하지 않습니다. CLI 쓰기는 링크를 추가하지 않습니다.

**Connect to agents**에서 도구와 각각의 업데이트 모드(`passive` 또는 `active`)를 선택하고 **Review changes** → **Apply changes**를 실행하세요. 같은 파일을 읽는 도구는 블록 하나를 공유하므로 모드도 함께 바뀝니다. 설정된 도구의 모드도 같은 검토로 변경할 수 있습니다. 기존 지침 파일이나 공유 소스에 scope/hash 마커 블록을 추가하거나 업데이트하고 다른 내용과 할당은 보존합니다. 변경 내용, 다른 읽기 도구, 알려진 글자 수 제한을 검토할 수 있습니다. 기존 파일을 백업하며 오래된 계획은 거부합니다. 수정되지 않은 오래된 블록은 검토 후 업데이트할 수 있으며 수동 수정되었거나 잘못된 블록은 보존합니다. 동기화되지 않았거나 읽을 수 없는 지침 파일은 건너뜁니다.

**Configured**는 읽기 경로에 현재 안내가 있음을 표시하며 읽었다는 뜻은 아닙니다. **Copy verification prompt**를 새 세션에서 사용하여 `INDEX.md`와 관련 노트를 읽고 전체 경로와 사용자가 추가한 임시 검증 값을 보고하도록 요청하세요. 실제 읽기 이벤트를 수동으로 확인하세요. 읽기 telemetry는 보장하지 않습니다.

**Copy guidance**는 수동 붙여 넣기 대안으로 모드를 골라 복사하며 **Open AGENTS.md**에서 편집할 수 있습니다. 프로젝트 내부 소스는 지침 파일의 위치와 관계없이 **project root** 기준 상대 경로이고 외부 또는 global 소스는 절대 경로입니다. 이동 후 안내를 다시 생성하세요. CLI `instructions`도 같은 scope/hash 블록을 출력합니다. Native automatic memory, 자동 학습, Obsidian 통합을 활성화하지 않습니다.

### `extras init`

새로운 extra 리소스 유형을 생성합니다.

```bash
# Interactive wizard
skillshare extras init

# CLI flags
skillshare extras init <name> --target <path> [--target <path2>] [--mode <mode>]

# Single-file extra
skillshare extras init <name> --file <filename> [--as <filename>] --target <path> [--source <dir>] [--mode <mode>]
```

wizard는 이름 다음에 **What do you want to sync?**를 묻습니다: **Folder** 또는 **Single file**.

**Options:**

| Flag | 설명 |
|------|-------------|
| `--target <path>` | target 디렉터리 경로 (반복 가능) |
| `--file <filename>` | source 디렉터리에서 이 파일만 동기화하여 [single-file extra](#single-file-extras)를 만듭니다. `/`나 `\` 없는 단순한 파일 이름 |
| `--as <filename>` | 모든 target에 쓸 파일 이름 (기본값: `--file` 이름). `--file` 필요 |
| `--mode <mode>` | 동기화 mode: `merge` (기본값), `copy`, 또는 `symlink`. `import`는 `--file`과 함께만 사용 가능 |
| `--flatten` | 하위 디렉터리의 파일을 target 루트에 바로 동기화 (`symlink` mode 또는 `--file`과 함께 사용 불가) |
| `--source <path>` | 이 extra에 대한 사용자 지정 source 디렉터리 (`extras_source` 및 기본값을 재정의; 프로젝트 모드에서는 프로젝트 루트 기준 상대 경로) |
| `--force` | extra가 이미 존재하면 덮어쓰기 |
| `--no-tui` | interactive wizard 생략, CLI 플래그만 사용 |
| `--project, -p` | 프로젝트 config(`.skillshare/`)에 생성 |
| `--global, -g` | 전역 config에 생성 |

:::note
`--source`는 전역 모드에서만 지원됩니다. 프로젝트 모드는 항상 `.skillshare/extras/<name>/`를 source 디렉터리로 사용합니다.
:::

**Examples:**

```bash
# Sync rules to Claude and Cursor
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules

# Use a custom source directory
skillshare extras init rules --target ~/.claude/rules --source ~/company-shared/rules

# Overwrite an existing extra with new targets
skillshare extras init rules --target ~/.cursor/rules --force

# Project-scoped extra with copy mode
skillshare extras init prompts --target .claude/prompts --mode copy -p

# Sync agents flat (tools like Claude Code only discover flat files)
skillshare extras init agents --target ~/.claude/agents --flatten

# Sync one file, renamed at the target
skillshare extras init pi-prompt --file system.md --as APPEND_SYSTEM.md \
  --source ~/dotfiles/prompts --target ~/.pi/agent
```

`extras init`는 config만 작성합니다. source 파일을 만들지 않으며 동기화하지도 않습니다. single-file extra의 경우 source와 target 파일의 전체 경로를 출력합니다:

```
  Source    ~/dotfiles/prompts/system.md
  Target    ~/.pi/agent/APPEND_SYSTEM.md · merge

✓ Created extra pi-prompt (single file)

Next
  skillshare sync extras  sync it
```

source 파일이 아직 없으면 source 줄 끝에 `(not found)`가 붙고, 마지막 줄은 `Create the source file, then run 'skillshare sync extras'.`가 됩니다.

### `extras list`

구성된 모든 extra와 동기화 상태를 나열합니다. 기본적으로 interactive TUI를 실행합니다.

```bash
skillshare extras list [--json] [--no-tui] [-p|-g]
```

**Options:**

| Flag | 설명 |
|------|-------------|
| `--json` | JSON 출력 (`source_type`: `per-extra` / `extras_source` / `default` 포함, 설정된 경우 target별 `extension` 필드 포함) |
| `--no-tui` | interactive TUI 비활성화, 일반 텍스트 출력 사용 |
| `--project, -p` | 프로젝트 모드 extras 사용 (`.skillshare/`) |
| `--global, -g` | 전역 extras 사용 (`~/.config/skillshare/`) |

#### Interactive TUI

TTY에서 `extras list`는 대화형 화면을 엽니다. 왼쪽에는 extras, 오른쪽에는 선택한 extra의 target과 파일이 표시됩니다. 여기서 extras를 만들고, 제거하고, sync하고, collect할 수 있으며 target의 모드나 flatten 설정도 바꿀 수 있습니다. 키는 화면 아래쪽에 표시됩니다.

`skillshare tui off`로 TUI를 영구적으로 끌 수 있습니다.

#### Plain text output

TUI가 비활성화된 경우 (`--no-tui`, `skillshare tui off`, 또는 파이프된 출력을 통해):

```
$ skillshare extras list --no-tui
rules  ~/.config/skillshare/extras/rules · 2 files
✓ ~/.claude/rules  merge
✓ ~/.cursor/rules  copy

codex-agents  ~/.config/skillshare/agents · 3 files
✓ ~/.codex/agents  extension: codex-agents

2 extras
```

[single-file extra](#single-file-extras)의 경우 source와 각 target은 디렉터리 대신 파일의 전체 경로를 표시합니다.

동기화된 행은 아이콘, 경로, mode만 표시합니다. 동기화되지 않은 행은 상태 단어(`drift`, `modified`, `not synced`, `no source`)를 추가로 표시합니다. transform extension이 있는 target은 동기화 mode 대신 `extension: <name>`으로 표시됩니다 (실제 mode는 항상 `copy`).

### `extras source`

전역 `extras_source` 디렉터리를 표시하거나 설정합니다. 이는 extras source 파일이 저장되는 기본 상위 디렉터리입니다.

```bash
skillshare extras source            # show current value
skillshare extras source <path>     # set new value
```

인수 없이 실행하면 현재 `extras_source` 경로를 표시합니다 (자동 감지된 경우 `(default)` 표시). 경로 인수를 전달하면 전역 config의 `extras_source`를 업데이트합니다.

:::note
이 명령어는 전역 전용입니다. 프로젝트 모드는 항상 `.skillshare/extras/`를 사용하며 `extras_source`를 지원하지 않습니다.
:::

**Examples:**

```bash
# Show current extras_source
skillshare extras source

# Set to a shared directory
skillshare extras source ~/company-shared/extras
```

### Operating on an existing extra

`extras <name>`으로 target의 sync mode와 flatten 설정을 변경하거나 target을 추가·제거합니다. mode, flatten, target 추가 후 `skillshare sync extras`로 적용하세요. `--remove-target --prune`은 관리 파일을 즉시 복원하거나 제거합니다.

```bash
skillshare extras <name> --mode <mode> [--target <path>] [-p|-g]
skillshare extras <name> --flatten | --no-flatten [--target <path>]
skillshare extras <name> --add-target <path> [--as <filename>] [--mode <mode>] [--flatten] [-p|-g]
skillshare extras <name> --remove-target <path> [--prune] [-p|-g]
skillshare extras <name> --help
```

**Options:**

| Flag | 설명 |
|------|-------------|
| `--mode <mode>` | 새 동기화 mode: `merge`, `copy`, 또는 `symlink`. `import`는 [single-file extra](#single-file-extras) 전용 |
| `--flatten` | flatten 활성화 (하위 디렉터리 파일을 target 루트에 동기화) |
| `--no-flatten` | flatten 비활성화 |
| `--add-target <path>` | extra에 새 target 추가 |
| `--as <filename>` | `--add-target`의 target 파일 이름 (single-file extra 전용, 기본값은 `file`) |
| `--remove-target <path>` | extra에서 target 제거 (기본적으로 config 전용) |
| `--prune` | `--remove-target`과 함께: 해당 target 아래의 skillshare 관리 파일도 삭제. single-file extra에서는 대신 target 파일을 복원 |
| `--target <path>` | target 디렉터리 경로 (multi-target extra에서 `--mode`에 필요; 생략 시 `--flatten`/`--no-flatten`은 모든 target에 적용) |
| `--project, -p` | 프로젝트 모드 extras 사용 (`.skillshare/`) |
| `--global, -g` | 전역 extras 사용 (`~/.config/skillshare/`) |

**Examples:**

```bash
# Change rules mode (single target — auto-resolved)
skillshare extras rules --mode copy

# Specify target explicitly (required for multi-target extras)
skillshare extras rules --mode copy --target ~/.claude/rules

# Enable / disable flatten on all targets at once
skillshare extras agents --flatten
skillshare extras agents --no-flatten

# Add a new target to an existing extra (then sync)
skillshare extras rules --add-target ~/.cursor/rules
skillshare extras commands --add-target ~/.config/opencode/commands --mode copy
skillshare extras personal --add-target ~/.claude --as CLAUDE.md --mode import

# Remove a target (leaves synced files in place)
skillshare extras rules --remove-target ~/.cursor/rules

# Remove a target and delete its synced files
skillshare extras rules --remove-target ~/.cursor/rules --prune
```

Web UI(각 target의 mode 드롭다운과 flatten 체크박스)와 TUI(`e` 키)에서도 사용할 수 있습니다.

### `extras remove`

config에서 extra를 제거합니다.

```bash
skillshare extras remove <name> [--force] [-p|-g]
```

source 파일은 유지됩니다. 디렉터리 extra는 동기화된 target을 그대로 둡니다. [single-file extra](#single-file-extras)는 target을 복원한 뒤 config 항목을 제거하며, 복원에 실패하면 재시도할 수 있도록 항목을 유지합니다.

### `extras collect`

target의 로컬 파일을 extras source 디렉터리로 다시 수집합니다. 파일은 source로 복사되고 symlink로 대체됩니다. copy mode target은 파일을 일반 복사본으로 유지합니다. [single-file extra](#single-file-extras)에서는 collect를 지원하지 않습니다.

source에 이미 존재하는 파일은 건너뜁니다. `--force`를 사용하면 target 버전으로 덮어씁니다. 예를 들어 copy mode target에서 직접 수정한 내용을 다시 가져올 때 사용합니다. 내용이 이미 source와 같은 파일은 여전히 건너뜁니다.

```bash
skillshare extras collect <name> [--from <path>] [--force] [--dry-run] [-p|-g]
```

**Options:**

| Flag | 설명 |
|------|-------------|
| `--from <path>` | 수집할 target 디렉터리 (여러 target이 있으면 필수) |
| `--force`, `-f` | source에 이미 존재하는 파일 덮어쓰기 |
| `--dry-run` | 변경 없이 수집될 항목 미리보기 |

**Example:**

```bash
# Collect rules from Claude back to source
skillshare extras collect rules --from ~/.claude/rules

# Preview what would be collected
skillshare extras collect rules --from ~/.claude/rules --dry-run

# Pull target edits back over existing source files
skillshare extras collect rules --force
```

---

## Sync Modes

| Mode | Behavior |
|------|----------|
| `merge` (기본값) | target에서 source로의 파일별 symlink |
| `copy` | 파일별 복사 |
| `symlink` | 디렉터리 전체 symlink |
| `import` | [single-file extra](#single-file-extras) 전용: target 파일 안의 `@<source file>` 한 줄 |

Developer Mode가 없는 Windows에서는 `merge`가 각 파일을 링크하는 대신 복사하며, `sync`는 `file links need Windows Developer Mode; copying instead`를 출력합니다. 이때 `extras list`와 `status`는 target을 `copy`로 표시합니다. 복사본은 추적되므로 이후 sync가 업데이트하고 정리하며, 사용자 파일은 유지하고, 파일 링크를 쓸 수 있게 되면 링크로 교체합니다. [Windows 문제 해결](/docs/troubleshooting/windows#file-links-need-windows-developer-mode-copying-instead)을 참고하세요.

내용이 같은 로컬 파일은 `local preserved`로 표시되며, `sync extras`는 해당 파일에 `--force`를 권하지 않습니다. 관리되는 링크가 아닌 로컬 파일로 유지됩니다.

mode를 전환할 때 (예: `merge`에서 `copy`로), 다음 `sync`는 기존 symlink를 새 mode 형식으로 자동으로 대체합니다. `--force`는 필요하지 않습니다 — symlink는 항상 안전하게 대체됩니다. 로컬에서 생성된 일반 파일을 덮어쓰려면 `--force`가 필요합니다.

---

## Flatten

일부 AI 도구(예: Claude Code의 `/agents`)는 config 디렉터리의 **최상위 레벨**에 있는 파일만 인식합니다 — 하위 디렉터리로 재귀 탐색하지 않습니다. extras source가 구성을 위해 하위 디렉터리를 사용하는 경우, 동기화된 파일이 해당 도구에는 보이지 않게 됩니다.

`flatten` 옵션은 source의 하위 디렉터리 깊이와 상관없이 모든 파일을 target 루트에 바로 동기화하여 이를 해결합니다:

```yaml
extras:
  - name: agents
    targets:
      - path: ~/.claude/agents
        flatten: true
```

**Behavior:**
- `flatten: true`: `source/curriculum/tactician.md` → `target/tactician.md`
- `flatten: false` (기본값): `source/curriculum/tactician.md` → `target/curriculum/tactician.md`

**Filename collisions:** 서로 다른 하위 디렉터리에 있는 두 파일이 같은 이름을 가질 때 (예: `team-a/agent.md`와 `team-b/agent.md`), 첫 번째 파일이 우선합니다 (경로 기준 알파벳순 정렬). 이후 충돌은 경고와 함께 건너뜁니다.

**Constraints:**
- `merge`와 `copy` mode에서만 작동합니다 — `symlink` mode와는 함께 사용할 수 없습니다
- `collect`는 새로 수집된 파일을 source 루트에 배치합니다 (신규 파일에 대한 하위 디렉터리 매핑 없음)

---

## Extension transforms

일부 도구는 markdown을 읽지 않습니다. Gemini CLI는 TOML commands를, Codex CLI는 TOML agents를 요구합니다. target의 `extension` 필드는 동기화 중 각 source 파일을 target의 네이티브 형식으로 변환하는 외부 스크립트를 실행합니다.

```yaml
extras:
  - name: commands
    targets:
      - path: .claude/commands        # no extension — synced as-is
      - path: .gemini/commands
        extension: gemini-commands           # transform during sync
```

**Resolution** — 단순 이름은 extensions 디렉터리(`~/.config/skillshare/extensions/<name>` 전역, `.skillshare/extensions/<name>` 프로젝트) 아래에서 해석됩니다. 경로(`./x.sh`, `/abs/x`)는 그대로 사용됩니다.

**Copy semantics** — `extension`은 `copy` mode를 암시합니다. `extension`이 있는 target에 `mode: merge` 또는 `mode: symlink`를 설정하면 오류입니다.

**One-way** — transform은 source → target 방향으로만 실행됩니다. `extras collect`는 extension target을 건너뜁니다.

**Overwrite safety** — 생성된 출력은 `copy` mode와 동일한 충돌 규칙을 따릅니다. 출력 경로에 남아 있는 symlink는 자동으로 대체됩니다. 사용자가 직접 만든 기존 일반 파일이나 디렉터리는 그대로 유지되며 `--force`를 전달하지 않으면 건너뜁니다 (`--force`를 사용하면 충돌하는 디렉터리가 생성된 파일로 완전히 대체됩니다).

### Extension layout

단일 실행 파일이거나, manifest가 있는 디렉터리입니다:

```
.skillshare/extensions/gemini-commands/
├── extension.yaml
├── convert.js        # mapping rules you edit
└── md-toml.js        # helper for markdown/frontmatter/TOML
```

`extension.yaml`:

```yaml
run: ["node", "convert.js"]      # explicit command (argv), execed directly
output_ext: toml                  # .md → .toml; omit to keep the source extension
description: "Markdown command → Gemini CLI TOML"
```

manifest가 없는 단순 단일 파일 실행 파일은 직접 exec됩니다 (Unix에서는 shebang에 의존) 그리고 source 확장자를 유지합니다. 확장자 이름을 변경하는 transform은 디렉터리 형식을 사용해야 합니다.

### Execution contract

- source 파일 내용은 **stdin**으로 전달되며, 스크립트는 변환된 내용을 **stdout**으로 씁니다.
- 환경 변수: `SS_SRC_PATH`, `SS_REL_PATH` (source 루트 기준 상대 경로 — Gemini의 `/namespace:command` 네이밍에 유용), `SS_TARGET_DIR`, `SS_MODE`.
- 0이 아닌 종료 코드는 해당 파일을 실패로 표시합니다. 다른 파일은 계속 처리됩니다.

### Cross-platform

이 메커니즘은 크로스 플랫폼입니다. extension이 실행되는지 여부는 해당 인터프리터에 따라 달라집니다. `run`이 명시적인 명령어이기 때문에, `node`나 `python3`용으로 작성된 extension은 Windows, macOS, Linux에서 모두 작동합니다. 순수 `bash` 스크립트는 셸을 사용할 수 있는 환경(Unix, 또는 Git Bash가 설치된 Windows)에서만 실행됩니다. 모든 플랫폼에 균일하게 제공되므로 참조용 extension에는 Node가 선호되는 인터프리터입니다.

### Reference extensions

skillshare 저장소는 `extensions/` 아래에 예제 extension(`gemini-commands`, `codex-agents`, `opencode-agents`)을 제공합니다. 하나를 extensions 디렉터리로 복사하여 조정하세요 — 이들은 참조용이며 자동으로 설치되지 않습니다. 각 참조 extension은 `convert.js`를 짧게 유지하여 필드 매핑만 편집하면 되도록 하며, `md-toml.js`가 markdown 읽기, 단순 frontmatter 파싱, TOML 작성을 처리합니다.

### Recipe: Codex agents

Codex CLI는 markdown이 아닌 TOML agents를 요구합니다. `source`가 임의의 디렉터리를 가리킬 수 있으므로, agents source를 extras source로 재사용하고 `codex-agents`로 변환할 수 있습니다:

```yaml
extras:
  - name: codex-agents
    source: ~/.config/skillshare/agents   # reuse the agents source
    targets:
      - path: ~/.codex/agents
        extension: codex-agents
```

`skillshare sync extras`는 각 `<agent>.md`를 `~/.codex/agents/<agent>.toml`로 변환하여, frontmatter의 `name`, `description`, `model`을 매핑하고 markdown 본문을 `developer_instructions`로 접어 넣습니다 (다른 frontmatter 키는 제거됩니다). [Codex custom agent schema](https://developers.openai.com/codex/subagents#custom-agent-file-schema)는 `name`, `description`, `developer_instructions`를 요구하므로, 참조 transform은 해석된 name, description, 또는 Markdown 본문이 비어 있을 경우 명확한 오류를 보고합니다. agents의 별도 사본은 필요하지 않습니다.

Agent target은 extra 없이도 `extension`을 직접 사용할 수 있습니다. [extension으로 agent 변환하기](/docs/understand/agents#extensions)를 참고하세요.

---

## Recipe: shared instructions across agents

:::tip Dashboard
웹 대시보드에서 미리보기, 백업, 복원 버튼과 함께 이 설정을 대신 해 줍니다.
[여러 도구에서 하나의 AGENTS.md 공유하기](../../how-to/daily-tasks/sharing-instructions.md)를
참고하세요. 디렉터리 대신 [single-file extra](#single-file-extras)를 사용합니다.
:::

현재 대부분의 코딩 agent는 표준 지침을 위해 `AGENTS.md`를 읽지만, 각 agent는
사용자 레벨 사본을 서로 다른 디렉터리에 보관합니다. 여러 target을 가진 extra 하나로
단일 source 파일을 모두에게 배포할 수 있습니다:

```bash
skillshare extras init instructions \
  --target ~/.codex \
  --target ~/.config/opencode \
  --target ~/.claude \
  --target ~/.gemini \
  --no-tui
```

`AGENTS.md`를 해석된 source 디렉터리
(기본값 `~/.config/skillshare/extras/instructions/`)에 넣은 다음
`skillshare sync extras`를 실행하세요.

| Agent | Global path | `AGENTS.md` 읽는 방식 |
|-------|-------------|-------------------|
| Codex CLI | `~/.codex/AGENTS.md` | 직접 |
| opencode | `~/.config/opencode/AGENTS.md` | 직접 |
| Claude Code | `~/.claude/AGENTS.md` | `CLAUDE.md` import를 통해 |
| Antigravity | `~/.gemini/AGENTS.md` | `GEMINI.md` import를 통해 |

두 agent는 사용자 레벨에서 자신만의 고정된 파일명을 읽으므로, 각각 동기화된 파일 옆에
한 줄짜리 파일이 필요합니다. 이것을 한 번만 작성하면 skillshare는 다시 건드리지
않습니다:

```markdown title="~/.claude/CLAUDE.md"
@AGENTS.md
```

```markdown title="~/.gemini/GEMINI.md"
@AGENTS.md
```

Claude Code는 `AGENTS.md`가 아니라 `CLAUDE.md`를 읽으며, 이 import 방식은
[memory documentation](https://code.claude.com/docs/en/memory)에서 다른 agent와 하나의
파일을 공유하기 위해 권장하는 방법입니다. Antigravity는 전역 rules를
`~/.gemini/GEMINI.md`에 보관하며 상대 경로 `@filename`을 rules 파일 자체의
디렉터리를 기준으로 해석하므로, 동일한 한 줄이 동기화된 `AGENTS.md`를 인식합니다.
`~/.gemini` target은 동일한 전역 파일을 읽는 Antigravity CLI도 함께 커버합니다.

source 파일 이름은 `AGENTS.md`로 유지하세요. `memory.md`와 같은 중립적인 이름도
동기화는 똑같이 되지만 더 이상 읽히지 않게 됩니다: Codex는 이름으로 `AGENTS.md`
파일들을 연결하며 import 문법이 없으므로, 그 이름으로만 파일을 인식합니다.

target은 디렉터리이므로, 모든 target은 각 파일을 source 이름 그대로 받습니다.
source 디렉터리는 모든 곳에 배포하고 싶은 파일로만 유지하세요 — 여분의 파일이
있으면 네 개의 target 모두에 그대로 전달됩니다.

:::note
이 레시피는 사용자가 작성한 지침을 공유하는 것이지, agent가 스스로 작성하는
메모리를 공유하는 것이 아닙니다. Agent는 자신만의 학습 내용을 비공개 형식으로
저장합니다 — Claude Code는 Markdown 디렉터리, Codex는 데이터베이스, Cursor는
파일이 아닌 저장소를 사용하며, 이러한 것들은 target 간 파일 복사로 이식할 수
없습니다.
:::

---

## Single-file extras {#single-file-extras}

`file`이 있는 extra는 source 디렉터리 전체가 아니라 그 안의 파일 하나만 동기화합니다.
각 target은 `<path>/<as>`를 받으며, `as`의 기본값은 `file` 이름입니다. 고정된 경로의 파일
하나를 읽는 도구라면 어디에나 사용할 수 있습니다. 예를 들어 Pi는 `~/.pi/agent/APPEND_SYSTEM.md`를
system prompt에 덧붙입니다. 그 내용을 dotfiles에 `system.md`로 두고 링크하세요:

```yaml
extras:
  - name: pi-prompt
    source: ~/dotfiles/prompts     # 프로젝트 모드: 프로젝트 루트 기준 상대 경로
    file: system.md                # ~/dotfiles/prompts/system.md
    targets:
      - path: ~/.pi/agent
        as: APPEND_SYSTEM.md       # ~/.pi/agent/APPEND_SYSTEM.md becomes a link
```

같은 extra를 CLI로 만들려면:

```bash
skillshare extras init pi-prompt --file system.md --as APPEND_SYSTEM.md \
  --source ~/dotfiles/prompts --target ~/.pi/agent
skillshare sync extras
```

`extras init`의 `--as`는 모든 target에 적용됩니다. 특정 target에서만 다른 파일 이름을 쓰려면
그 target을 따로 추가하세요:

```bash
skillshare extras pi-prompt --add-target ~/Documents/prompts --as pi-system.md
```

대시보드의 [공유 AGENTS.md](../../how-to/daily-tasks/sharing-instructions.md)도 single-file extra이며,
일반 링크, 이름 변경, import를 함께 사용할 수 있습니다:

```yaml
extras:
  - name: personal
    file: AGENTS.md                # ~/.config/skillshare/extras/personal/AGENTS.md
    targets:
      - path: ~/.codex             # ~/.codex/AGENTS.md becomes a link
      - path: ~/.gemini
        as: GEMINI.md              # ~/.gemini/GEMINI.md becomes a link
      - path: ~/.claude
        as: CLAUDE.md
        mode: import               # ~/.claude/CLAUDE.md keeps its content and imports the file
```

| Mode | Target 파일 |
|------|-------------|
| `merge` (기본값) 또는 `symlink` | source 파일에 대한 symlink (Developer Mode가 없는 Windows에서는 복사본) |
| `copy` | source 파일의 복사본 |
| `import` | 사용자의 파일 그대로, 맨 위 관리 블록에 `@<source file>` 한 줄 추가 |

`import`는 `@` 줄을 `<!-- skillshare:instructions:begin -->`과
`<!-- skillshare:instructions:end -->` 사이에 두며, 파일의 나머지 부분은 절대 바꾸지
않습니다. Claude Code처럼 `@` import를 따르는 도구에만 사용하세요.

규칙:

- 링크 또는 `copy` mode의 target 파일은 공유 파일 하나만 사용할 수 있으며 다른 공유 파일을 동시에 import할 수 없습니다.
- `file`과 `as`는 `/`나 `\` 없는 단순한 파일 이름이어야 합니다.
- `as`와 `import`에는 `file`이 필요합니다. `flatten`과 `extension`은 single-file
  extra와 함께 쓸 수 없습니다.
- target에 이미 다른 일반 파일이나 symlink가 있으면, sync는 `--force` 없이도 그것을
  백업한 뒤 교체합니다. 그 자리에 디렉터리가 있으면 건너뜁니다.
- 링크된 뒤 `modified`가 된 target도 교체됩니다. 편집된 파일은 복원 지점이 아니라
  drift 백업으로 보관됩니다.
- 링크가 내용이 다른 일반 파일로 바뀌거나 관리 중인 복사본이 편집되면 `extras list`에 `modified`로 표시됩니다.
- target을 `merge`, `symlink`, `copy`에서 `import`로 바꾸면 마지막 `import` mode의 자체 내용
  (빈 내용 포함)을 복원합니다. 사용한 적이 없다면 연결 전 내용을 사용합니다. import 블록을 추가하며
  편집된 복사본은 먼저 drift 백업으로 보관합니다.
- `extras remove`와 `--remove-target --prune`은 각 target 파일을 복원합니다. 링크,
  복사본 또는 import 줄이 사라지고, 첫 sync 전에 있던 파일이나 symlink가 돌아옵니다
  (원래 없었다면 파일도 없습니다). `modified` target은 먼저 drift 백업으로 보관됩니다.
  `--prune` 없는 `--remove-target`은 single-file target을 그대로 두고 관리를 중단하며 복원 지점을 잊습니다. 이후 sync는 이를 정리하지 않으며, 다시 연결할 때 새 복원 지점을 기록합니다.
- `extras collect`는 지원하지 않습니다. target에서 한 편집을 유지하려면 source 파일에 다시
  복사하세요. 공유 `AGENTS.md`의 경우 대시보드 **AGENTS.md** 탭의 **공유 파일에 반영**이
  이 작업을 대신합니다.

대시보드에서 `file`이 `AGENTS.md`인 single-file extra는 **AGENTS.md** 탭에 표시되고, 그 밖의
single-file extra는 **Folders & files**에 표시됩니다. 그곳의 **Add extra**에서 **Folder** 또는
**Single file**을 고를 수 있고, 각 target에는 **File name**이 있으며, 단일 파일은 `merge`, `copy`,
`import`를 사용할 수 있습니다. 단일 파일의 **Name**은 직접 입력하기 전까지 확장자를 뺀 파일 이름으로
채워집니다(`APPEND_SYSTEM.md`는 `APPEND_SYSTEM`). 대시보드는 파일 내용을 편집하지 않으므로 source 파일을 직접 편집하세요.

### 폴더 하나, 여러 파일

여러 single-file extra가 하나의 `source` 디렉터리를 공유할 수 있습니다. 파일마다
extra를 하나씩 만드세요. 어떤 extra에도 지정되지 않은 폴더 안의 파일은 동기화되지 않습니다:

```yaml
extras:
  - name: pi-system
    source: ~/dotfiles/pi
    file: system.md
    targets:
      - path: ~/.pi/agent
        as: APPEND_SYSTEM.md
  - name: pi-agents
    source: ~/dotfiles/pi
    file: agents.md
    targets:
      - path: ~/.pi/agent
        as: AGENTS.md
```

```bash
skillshare extras init pi-system --source ~/dotfiles/pi --file system.md \
  --as APPEND_SYSTEM.md --target ~/.pi/agent
skillshare extras init pi-agents --source ~/dotfiles/pi --file agents.md \
  --as AGENTS.md --target ~/.pi/agent
```

프로젝트 모드에서 `source`는 프로젝트 루트 기준 상대 경로이며 프로젝트 안에 있어야 합니다.
절대 경로는 거부됩니다:

```bash
skillshare extras init review -p --source .skillshare/extras/prompts \
  --file review.md --target .claude/commands
skillshare extras init plan -p --source .skillshare/extras/prompts \
  --file plan.md --target .claude/commands
```

대시보드에서는 공유 extras 폴더의 single file에 **Source folder** 필드가 있습니다. 기본값은
extra 이름이며, 다른 extra의 폴더를 입력하면 두 파일을 한 폴더에 둘 수 있습니다.

백업은 skillshare의 state 디렉터리(macOS와 Linux에서는
`~/.local/state/skillshare/extras/backups/`)에 파일당 최근 10개까지 보관됩니다.
drift 백업은 그 안의 `extras/backups/<id>/drift/`에 저장되며, `<id>`는 target 파일
경로에서 만들어집니다. 복원에는 절대 사용되지 않습니다. 저장된 버전을 조회하거나
복원하려면 [`backup files`](./backup.md#file-history)를 사용하세요.

---

## Directory Structure

```
~/.config/skillshare/
├── config.yaml          # extras config lives here
├── skills/              # skill source
└── extras/              # extras source root
    ├── rules/           # extras/rules/ source files
    │   ├── coding.md
    │   └── testing.md
    └── prompts/
        └── review.md
```

---

## Configuration

`config.yaml`에서:

```yaml
# Optional: set a global default extras source directory
extras_source: ~/my-extras

extras:
  - name: rules
    source: ~/company-shared/rules    # optional per-extra override
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
  - name: agents
    targets:
      - path: ~/.claude/agents
        flatten: true                  # sync subdirectory files flat
  - name: prompts
    targets:
      - path: ~/.claude/prompts
```

### Source Resolution Priority

각 extra의 source 디렉터리는 세 단계 우선순위로 해석됩니다:

1. **Per-extra `source`** (최우선) — 정확한 경로, 있는 그대로 사용
2. **`extras_source`** — `<extras_source>/<name>/`
3. **Default** — `~/.config/skillshare/extras/<name>/` (전역) 또는 `.skillshare/extras/<name>/` (프로젝트)

`extras list --json` 출력에는 어느 단계에서 경로가 해석되었는지를 나타내는
`source_type` 필드(`per-extra`, `extras_source`, 또는 `default`)가 포함됩니다.

:::tip Auto-populated
`skillshare init`을 실행하거나 `extras init`으로 첫 extra를 생성하면 `extras_source`는
자동으로 기본 경로(`~/.config/skillshare/extras/`)로 설정됩니다. 나중에 변경하려면
`skillshare extras source <path>`를 사용하세요.
:::

---

## Syncing

Extras는 다음으로 동기화됩니다:

```bash
skillshare sync extras        # sync extras only
skillshare sync --all         # sync skills + extras together
```

`--json`, `--dry-run`, `--force` 옵션을 포함한 전체 동기화 문서는
[sync extras](/docs/reference/commands/sync#sync-extras)를 참고하세요.

---

## Workflow

```bash
# 1. Create a new extra
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules

# 1b. Or with a custom source directory
skillshare extras init rules --target ~/.claude/rules --source ~/my-rules

# 1c. Reconfigure an existing extra (overwrite)
skillshare extras init rules --target ~/.cursor/rules --force

# 2. Add files to the source directory
# (edit the resolved source dir — check with: skillshare extras list --json)

# 3. Sync to targets
skillshare sync extras

# 4. List status (source_type shows where each extra's source is resolved from)
skillshare extras list

# 5. Collect a file edited in a target back to source
skillshare extras collect rules --from ~/.claude/rules

# 6. Change the global extras source directory
skillshare extras source ~/company-shared/extras
```

---

## See Also

- [sync](/docs/reference/commands/sync#sync-extras) — extras를 target에 동기화
- [status](/docs/reference/commands/status) — extras 파일 및 target 수 표시
- [Configuration](/docs/reference/targets/configuration#extras) — Extras config 레퍼런스
