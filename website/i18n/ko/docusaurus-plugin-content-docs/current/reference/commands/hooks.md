---
sidebar_position: 4
---

# hooks

이름이 있는 hooks를 관리하고 각 Agent의 네이티브 설정에 동기화합니다. Dashboard의 **Hooks**에서 추가, 편집, 가져오기, 활성화／비활성화, 미리보기와 동기화를 할 수 있습니다. **Targets**, **Projects**, **Sync**, **Settings → Backups**에도 표시됩니다.

hook 행의 메뉴에서 각 대상의 네이티브 설정이나 코드, 스크립트 파일과 출력 경로를 확인할 수 있습니다. 이 읽기 전용 미리보기는 해당 hook의 내용만 표시하며 공유 파일의 다른 설정은 유지합니다. 비활성화된 hook도 확인할 수 있지만 게시하지 않습니다.

## 명령

```bash
skillshare hooks
skillshare hooks list --json
skillshare hooks add check --file ./check.yaml
skillshare hooks edit check --file ./updated-check.yaml --sync
skillshare hooks import --from claude --json
skillshare hooks import imported --from claude --file ./settings.json --dry-run
skillshare hooks disable check --sync
skillshare hooks enable check --sync
skillshare hooks sync --dry-run --json
skillshare hooks sync
skillshare hooks sync check --replace --dry-run
skillshare sync hooks --dry-run --json
skillshare sync hooks
skillshare hooks remove check --sync
skillshare hooks remove check --keep-files
skillshare hooks restore BACKUP_ID --dry-run
skillshare hooks restore BACKUP_ID
```

하위 명령이 없으면 entries를 나열합니다. `add`／`edit`는 `--file`의 Entry JSON/YAML을 읽습니다. 이름 없는 가져오기는 후보(Agent event 또는 파일마다 하나)를 나열하고 이름을 지정하면 저장합니다. 가져오기를 저장하면 읽은 등록을 그 자리에서 인수합니다. 다음 동기화는 `--replace` 없이 adopt하며, 가져오지 않은 event는 관리되지 않은 채 남습니다. 설정과 코드만 읽고 실행하지 않습니다. 추가, 편집, 가져오기, 활성화, 비활성화, 제거는 소스만 저장하며 `--sync`를 붙이면 동기화합니다. 동기화와 복원은 적용 전 다시 미리 봅니다.

| Option | Meaning |
|---|---|
| `--file PATH` | 추가／편집의 Entry JSON/YAML 또는 가져올 네이티브 설정／코드 |
| `--from AGENT` | 가져올 Agent 또는 네이티브 형식 |
| `--sync` | 저장 후 동기화 |
| `--keep-files` | `remove`와 함께 사용: hook 관리를 멈추고 네이티브 항목은 그대로 둡니다. `--sync`와 함께 쓸 수 없습니다. [아래](#stop-managing-a-hook) 참고 |
| `--replace` | 기존 소스 entry 또는 해당 entry의 충돌 출력을 명시적으로 교체 |
| `--dry-run, -n` | 저장하거나 쓰지 않고 미리보기 |
| `--json` | 구조화된 출력 |
| `--revision ID` | 지정한 미리보기 revision과 일치 요구 |
| `--global, -g` | global 설정 |
| `--project, -p` | project 설정 |

`hooks list --json`으로 백업 ID와 전체 대상 경로를 확인합니다. 설정이 바뀌면 이전 미리보기가 무효가 되므로 저장／동기화 전 새로 확인하세요.

## hook 관리 중단 {#stop-managing-a-hook}

```bash
skillshare hooks remove check --keep-files
```

소스에서 `check`를 제거하고, Skillshare가 이 hook을 위해 쓴 네이티브 등록과 파일을 더 이상 기억하지 않습니다. Agent 파일은 바뀌지 않습니다. 이후 그 항목은 사용자의 것이 되어, 동기화가 제거하거나 업데이트하지 않으며 `hooks import`에서 다시 후보로 나타납니다. `--keep-files`는 `--sync`와 함께 쓸 수 없습니다. 대시보드의 제거 대화상자는 Hooks 페이지와 프로젝트의 **Hooks** 탭에서 **관리 중단**으로 이 선택지를 제공합니다. **소스에서만 제거**는 다릅니다. 다음 동기화 때 Agent 파일에서 해당 hook 항목을 삭제합니다.

제거한 범위만 바뀝니다. global hook의 관리를 멈춰도 같은 이름의 프로젝트 hook은 계속 관리되며, 그 반대도 마찬가지입니다. 다시 관리하려면 가져오기(import)하세요.

## 소스 필드

선택한 Skillshare 설정의 `hooks.entries`에 선언합니다. 이름은 entry를 식별하며 `bindings`는 받을 Agent와 네이티브 정의를 지정합니다. 빈 bindings는 소스만 유지하고 게시하지 않습니다.

```yaml
hooks:
  entries:
    check:
      description: Run the project's check after Claude finishes
      enabled: true
      bindings:
        claude:
          events:
            Stop:
              - hooks:
                  - type: command
                    command: "make check"
                    timeout: 120
```

`hooks add check --file check.yaml` 파일에는 Entry의 `description`, `enabled`, `bindings`만 포함하고 바깥쪽 `hooks.entries`는 넣지 않습니다.

Skillshare는 `hooks` 섹션을 들여쓴 block 형식으로 쓰며, 저장할 때마다 이전에 한 줄로 압축된 entry도 펼칩니다. Hooks 페이지의 **config.yaml** 버튼은 **Settings → Files**를 `hooks:` 섹션 위치에서 엽니다. `hooks` 키를 클릭하면 오른쪽 패널에 설명이 표시되고, **정리**는 중첩된 한 줄 섹션을 펼칩니다. `targets: [claude, codex]` 같은 짧은 목록은 한 줄로 유지됩니다.

| Option | Meaning |
|---|---|
| `description` | 선택 설명 |
| `enabled` | 기본 true. false는 소스를 유지하고 다음 동기화에서 변경되지 않은 소유 출력을 제거 |
| `bindings` | Agent ID별 네이티브 binding |
| `bindings.AGENT.events` | 설정형 Agent의 네이티브 event map |
| `bindings.AGENT.code` | Pi, Amp, OpenCode의 네이티브 extension/plugin 소스 |
| `bindings.AGENT.files` | 상대 파일명을 key로 하는 선택 UTF-8 스크립트 |

Agent ID는 `claude`, `codex`, `gemini`, `copilot`, `cursor`, `droid`, `qwen`, `antigravity`, `pi`, `amp`, `opencode`입니다. `factory`는 `droid`, `antigravity-cli`와 `agy`는 `antigravity`의 별칭입니다. event, matcher, handler type, command, timeout 단위와 payload는 원래 형식을 유지하고 자동 변환하지 않습니다. event 이름은 각 command Agent 문서의 event와 대조합니다. 알 수 없는 이름(예: 철자가 틀린 `Stopp`)은 미리보기와 plan의 `warnings`에 경고로 표시되지만, Agent가 event를 계속 추가하므로 동기화를 막지 않습니다. Pi, Amp, OpenCode 코드는 확인하지 않습니다. Pi, Amp, OpenCode의 코드와 imports는 설치된 버전에 맞춰 제공하며 전용 `skillshare-NAME.ts`에 기록됩니다. 공통 실행 엔진을 생성하지 않습니다. command binding 스크립트는 Agent 설정 디렉터리의 `hooks/skillshare/NAME/`에 저장하며 command의 macro／명시 경로를 변경하지 않습니다. 미리보기에서 전체 경로를 확인하세요.

## 네이티브 저장 위치

| Agent | Global | Project | Format |
|---|---|---|---|
| [Claude Code](https://code.claude.com/docs/en/hooks) | `~/.claude/settings.json` | `.claude/settings.json` | `hooks` event map with matcher groups |
| [Codex](https://learn.chatgpt.com/docs/hooks) | `~/.codex/hooks.json` | `.codex/hooks.json` | Wrapped `hooks` event map |
| [Gemini CLI](https://geminicli.com/docs/hooks/reference/) | `~/.gemini/settings.json` | `.gemini/settings.json` | `hooks` event map |
| [Copilot CLI](https://docs.github.com/en/copilot/reference/hooks-reference) | `~/.copilot/hooks/skillshare-NAME.json` | `.github/hooks/skillshare-NAME.json` | Version 1, `hooks` event map |
| [Cursor](https://cursor.com/docs/hooks) | `~/.cursor/hooks.json` | `.cursor/hooks.json` | Version 1, native lowerCamelCase events |
| [Factory Droid](https://docs.factory.com/harness/hooks) | `~/.factory/hooks.json` | `.factory/hooks.json` | Unwrapped event map |
| [Qwen Code](https://qwenlm.github.io/qwen-code-docs/en/users/features/hooks/) | `~/.qwen/settings.json` | `.qwen/settings.json` | `hooks` event map |
| [Antigravity](https://antigravity.google/docs/hooks) | `~/.gemini/config/hooks.json` | `.agents/hooks.json` | hook마다 하나의 이름 있는 블록 |
| [Pi](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/extensions.md) | `~/.pi/agent/extensions/skillshare-NAME.ts` | `.pi/extensions/skillshare-NAME.ts` | Native extension code |
| [Amp](https://ampcode.com/docs/plugin-api) | `~/.config/amp/plugins/skillshare-NAME.ts` | `.amp/plugins/skillshare-NAME.ts` | Native plugin code |
| [OpenCode](https://opencode.ai/docs/plugins/) | `~/.config/opencode/plugins/skillshare-NAME.ts` | `.opencode/plugins/skillshare-NAME.ts` | Supplied v1/v2 plugin code |

global scope는 네이티브 설정 디렉터리 환경 변수 override를 사용합니다. project scope는 프로젝트에만 쓰며 global 경로로 대체하지 않습니다. Codex inline TOML 같은 다른 소스는 별도로 유지됩니다. Antigravity와 CLI(`agy`)는 같은 `hooks.json`을 읽습니다. 각 hook은 이름이 붙은 블록 하나이며 가져와도 이름이 유지됩니다. CLI의 `~/.gemini/antigravity-cli/settings.json`에 있는 hooks는 별도로 유지됩니다. Droid의 독립 hooks 파일은 로딩 소스를 바꿀 수 있으므로 기존 inline hooks를 먼저 확인하세요. Copilot은 신뢰된 폴더에서만 `.github/hooks`의 프로젝트 hooks를 불러옵니다.


Droid inline hooks가 활성화된 경우 독립 파일 생성을 거부합니다. 가져와서 검토하고 원래 inline hooks를 제거한 뒤 동기화하세요.

## 프로젝트, 충돌과 복원

global 설정의 `hooks.projects`는 절대 프로젝트 경로를 같은 Entry 형식의 `entries`에 연결합니다. **Projects → Hooks**에서 관리합니다. 자체 `.skillshare/config.yaml`이 있는 프로젝트는 project scope로 관리하고, 프로젝트 동기화는 해당 루트만 적용합니다.

관련 없는 설정과 소유하지 않은 hooks는 유지합니다. 내용이 같다고 소유권을 부여하지 않습니다. 소유 출력이 외부에서 편집되면 비활성화, 제거, 복원도 충돌로 보고합니다. 명시적 교체는 선택한 entry에만 적용됩니다. 공유 파일의 plan 각 행에는 entry가 추가(`+`), 업데이트(`~`), 제거(`−`)하는 event가 표시되며 JSON plan의 `events`에도 포함됩니다. `update`는 entry가 그 파일에 등록을 남긴다는 뜻이고, `remove`는 파일에서 완전히 빠진다는 뜻입니다. 편집은 파일의 형식(압축 또는 들여쓰기)을 유지하며, Skillshare가 추가한 `hooks` 키는 마지막 hook이 빠질 때 함께 제거합니다. Skillshare가 만든 파일은 다른 내용이 남지 않으면 삭제하며, 만들었다가 비게 된 폴더도 함께 삭제합니다. 미리보기에는 파일 삭제로 표시됩니다. 원래 있던 파일과 폴더, 그리고 사용자가 추가한 내용은 유지됩니다.

백업은 이후의 관련 없는 변경을 유지하며 네이티브 출력을 복원하고 소스 정의는 바꾸지 않습니다. **Settings → Backups → Hooks** 또는 `hooks restore`로 미리 보고 복원하세요.

**Synced**는 Skillshare가 설정을 기록했다는 뜻입니다. Agent 절차에 따라 다시 시작／로딩하세요. 신뢰, hooks 활성화와 코드 호환성은 Agent가 제어합니다. 관리 작업은 hook command를 실행하거나 네이티브 신뢰를 자동 변경하지 않습니다.
