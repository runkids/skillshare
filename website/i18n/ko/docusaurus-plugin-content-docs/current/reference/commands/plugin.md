---
sidebar_position: 4
---

# plugin

지원되는 도구 전반에서 완전한 native plugin을 관리합니다. Capability 검사는 설치 지원과 형식 검색(discovery)을 구분합니다. [Manage plugins across tools](/docs/how-to/daily-tasks/sharing-plugins)부터 시작하세요.

```bash
skillshare plugin                         # 대화형 관리자
skillshare plugin add                     # Source → plugin → target → 검토
skillshare plugin discover ./my-plugin --json
skillshare plugin add ./my-plugin --target claude --target codex --no-tui
skillshare plugin add ./my-plugin --no-tui   # Skillshare에서 관리만 하고 target은 나중에 선택
skillshare plugin import review@team --from claude --no-tui
skillshare plugin list --json
skillshare plugin inspect review --json
skillshare plugin disable review --target codex --no-tui
skillshare sync plugins --dry-run --json
skillshare sync plugins --no-tui
skillshare plugin enable review --target codex --no-tui
skillshare plugin check review --json
skillshare plugin update review --target claude --no-tui
skillshare plugin update --all --no-tui
skillshare plugin remove review --no-tui
```

`enable`과 `disable`은 Agent의 네이티브 활성화 상태가 아니라 **Skillshare의 sync 선택**을 변경합니다. target 선택을 해제하면 그 선택이 저장됩니다. 다음 `sync plugins`는 정의는 유지한 채 관리되는 설치를 제거합니다. 다시 선택하면 다음 sync에서 재설치할 수 있습니다. 관리되지 않는 plugin은 영향을 받지 않습니다.

`--target` 없이 `add`하면(또는 대화형 선택에서 아무것도 고르지 않으면) plugin을 어디에도 설치하지 않고 Skillshare에서 관리합니다. 나중에 같은 source와 `--name`으로 target을 추가하거나 dashboard에서 해당 plugin의 행에서 선택할 수 있으며, 그때의 source가 설치됩니다. 이런 plugin은 마지막 target을 제거해도 관리 대상으로 남습니다. `--target` 없는 `remove NAME`은 Skillshare에서 제거합니다.

## 명령

| Command | 동작 |
|---|---|
| `list` | 구성된 바인딩과 네이티브 설치 상태. 터미널에서는 대화형 관리자 |
| `discover SOURCE` | 로컬 디렉터리, `owner/repo`, 또는 HTTPS Git 저장소를 검사 |
| `add [SOURCE]` | target adapter와 함께 전체 plugin을 선택하여 설치하거나, Pi를 통해 `npm:` 패키지를 설치 |
| `import [NATIVE-ID]` | 재설치하거나 활성화하지 않고 기존 설치를 채택 |
| `unimport NAME` | 명시한 대상에서 이 기기의 가져온 설치를 제거하고 공유 소스 선언은 유지 |
| `inspect NAME` | 관리되는 패키지 하나를 검사 |
| `sync [NAME]` | 선택된 target을 재조정하고 미완료된 네이티브 작업을 재시도 |
| `check [NAME]` | source 내용을 기록된 digest와, 또는 Pi npm 패키지 버전을 npm 최신 버전과 비교. 업데이트는 절대 하지 않음 |
| `update [NAME \| --all]` | source 변경 사항을 검토하고 지원되는 네이티브 업데이트 작업을 사용 |
| `enable / disable [NAME]` | 다음 sync에 target을 포함/제외 |
| `remove [NAME]` | 관리되는 바인딩을 제거하고 정의를 삭제 |

인수 없는 명령은 터미널에서 누락된 입력을 프롬프트로 요청합니다. 비대화형 변경 명령은 명시적인 입력이 필요합니다. `sync`와 `check`는 모든 패키지에 대해 동작할 수 있으며, `update`는 `--all`(`-a`)을 지정하면 관리되는 모든 패키지를 업데이트합니다. `sync --all`은 plugin을 포함하지 **않습니다**. `sync plugins`를 명시적으로 사용하세요.

## 옵션

| Option | 의미 |
|---|---|
| `--target TARGET` | 반복 가능한 선택: `claude`, `codex`, `cursor`, `antigravity`(`agy` alias), `antigravity-cli`, `copilot`, `grok`, `pi`, `omp`, `opencode`, 또는 [Agent의 다른 계정](#accounts) 이름. 아래 capability 표 참고 |
| `--plugin NAME` | source marketplace에서 plugin 하나를 선택 |
| `--name NAME` | 추가하거나 import할 때의 논리적 패키지 이름 |
| `--from TARGET` | Claude, Codex, Antigravity CLI, Copilot, Grok, Pi, OMP marketplace, OpenCode, 또는 [Agent의 다른 계정](#accounts)에서 import |
| `--all`, `-a` | 관리되는 모든 plugin 업데이트(`update` 전용, 패키지 이름이나 `--source-ref`와 함께 사용할 수 없음) |
| `--dry-run`, `-n` | Skillshare나 Agent 설정을 변경하지 않고 미리보기 |
| `--source-ref REF` | `discover`, `add`, `update`를 위한 Git 브랜치, 태그, 또는 커밋. 원격 source에만 해당 |
| `--entry PATH` | 패키지 루트 기준의 명시적으로 빌드된 OpenCode JS/TS entry(`discover`와 `add`) |
| `--revision ID` | 미리보기 이후 source, 설정, 또는 네이티브 inventory가 변경되었으면 적용을 거부 |
| `--json` | 기계가 읽을 수 있는 출력. TUI를 비활성화 |
| `--no-tui` | 대화형 메뉴를 비활성화. `tui: false`도 준수 |
| `--global`, `-g` | global Skillshare config와 네이티브 사용자 scope |
| `--project`, `-p` | project config. Claude, Antigravity, Pi, OMP, 또는 OpenCode(global로 폴백하지 않음) |

JSON 출력에는 source 경로와 네이티브 식별자가 포함됩니다. source URL에 자격 증명을 넣지 마세요. 실패한 변경 작업도 target별로는 성공한 결과를 반환할 수 있습니다. 어떤 target이든 실패하면 CLI는 0이 아닌 코드로 종료됩니다. 재시도하기 전에 결과를 확인하세요.

## Target 지원

| Target | Format | Global | Project | Update |
|---|---|:---:|:---:|---|
| Claude Code | `.claude-plugin/plugin.json` | 예 | 예 | 네이티브 업데이트 |
| Codex | `.codex-plugin/plugin.json` 또는 Agent Plugins 루트 매니페스트 | 예 | 아니요 | 검토된 Source를 갱신한 뒤 다시 add, Codex에서 활성화된 경우에만 |
| Cursor | `.cursor-plugin/plugin.json` 또는 Agent Plugins 루트 매니페스트 | 예 | 아니요 | 검토된 로컬 사본 교체 |
| Antigravity Desktop | 명시적 이름이 있는 루트 `plugin.json` | 예 | 예 | 검토된 로컬 사본 교체 |
| Pi | `pi` 리소스가 있는 `package.json`, 또는 `pi-package` 키워드와 관례적인 리소스 폴더 | 예 | 예, 네이티브 project trust 사용 시 | 관리되는 source 스냅샷 새로고침 |
| Oh My Pi | 네이티브 호환 메타데이터를 가진 marketplace 기반 로컬/Git plugin | 새 cache 설치/재설치, import, scope 단위 제거 | OMP의 project anchor에서 | 업데이트 차단, 제거 시 공유 cache 유지 |
| OpenCode | SDK dependency가 있는 `package.json`, `.opencode/plugins/` entry, 또는 명시적 `--entry` | 예 | 예 | 관리되는 source 스냅샷 새로고침 |

| Antigravity CLI | `agy`가 허용하는 네이티브 루트 매니페스트 또는 Claude 매니페스트 | 예 | 아니요 | 네이티브 활성화 상태가 확인되고 활성화된 경우에만 검토된 source를 새로고침 |
| GitHub Copilot CLI | `.plugin/plugin.json`, `.github/plugin/plugin.json`, Claude 매니페스트, 또는 Agent Plugins 루트 매니페스트 | 예 | 아니요 | 네이티브 활성화 상태가 확인되고 활성화된 경우에만 검토된 source를 새로고침 |
| Grok Build | `.grok-plugin/plugin.json` 또는 Claude 매니페스트 | import/remove만 가능. 설치에는 네이티브 trust 필요 | 아니요 | 네이티브로 업데이트 |
| Kimi Code | `kimi.plugin.json` 또는 `.kimi-plugin/plugin.json` | discovery만 가능 | 아니요 | 자동화되지 않음 |
| Hermes | `.hermes-plugin/plugin.yaml` | discovery만 가능 | 아니요 | 자동화되지 않음 |
| Devin | `.devin-plugin/plugin.json` | discovery만 가능 | 아니요 | 자동화되지 않음 |

Kimi의 비대화형 lifecycle, Hermes의 profile inventory/consent, Devin의 로컬 inventory/trust/cloud 구분은 아직 이러한 adapter들에 의해 검증되지 않았습니다. 이들의 형식은 discovery 중에 표시되지만, 설치는 사유와 함께 비활성화됩니다. source가 target을 선언한다고 해서 Skillshare가 이를 관리할 수 있다는 의미는 아닙니다. `list --json`과 `discover --json`은 허용된 작업과 함께 `targetDefinitions`를 포함합니다. discovery는 또한 각 형식의 버전, 구성 요소, entry, 검증 문제에 대한 `targetInfo`를 노출합니다. 손상된 매니페스트는 해당 target에만 격리되며, 잘못된 카탈로그는 유효한 형식을 숨기지 않고 경고로 보고됩니다.

### Another account of an Agent {#accounts}

[Agent의 다른 계정](/docs/reference/targets/configuration#agent-config-dir)으로 선언된 target은 `claude`, `codex`, `opencode`, `pi`에 대해 plugin target이기도 합니다. Skillshare는 `CLAUDE_CONFIG_DIR`, `CODEX_HOME`, `OPENCODE_CONFIG_DIR`, `PI_CODING_AGENT_DIR`을 통해 해당 Agent 자체의 CLI를 그 계정의 config 디렉터리에 대해 실행합니다:

```yaml
targets:
  claude-work:
    agent: claude
    config_dir: ~/.claude-work
```

```bash
skillshare plugin add owner/repo --target claude-work
skillshare plugin import demo@market --from claude-work
```

[`cli`](/docs/reference/targets/configuration#agent-config-dir)가 설정된 계정은 대신 그 호환 CLI(예: Pi 계정의 `omo`)를 같은 config 디렉터리에 대해 실행합니다. Pi 계정은 `SENPI_CODING_AGENT_DIR`와 `OMO_CODING_AGENT_DIR`도 설정합니다. Pi의 fork는 `PI_CODING_AGENT_DIR`보다 이 두 변수를 먼저 읽기 때문입니다. CLI를 찾을 수 없으면 작업이 실패하며, Skillshare가 Agent 자체 CLI로 대신 실행하지는 않습니다.

계정은 자신이 속한 Agent의 작업을 그대로 수행하면서 바인딩은 자기 이름으로 따로 관리하므로, 한 계정에는 plugin을 설치하고 다른 계정에는 설치하지 않을 수 있습니다. 계정은 global scope에만 존재합니다. 프로젝트의 plugin은 특정 계정이 아니라 프로젝트에 속합니다. `--target`과 `--from`은 계정 이름을 받으며, 터미널 선택기와 대시보드의 Plugins 페이지는 이를 Agent 옆에 나열합니다.

### Cursor와 Antigravity

이 adapter들은 전체 plugin을 문서화된 로컬 discovery 디렉터리로 복사합니다. CLI 실행 파일이 필요하지 않으며 marketplace 레지스트리를 수정하지도 않습니다:

- Cursor: `~/.cursor/plugins/local/<name>`. local import가 허용되어 있어야 합니다. Cursor를 다시 로드하고 Customize를 확인하세요. 동일한 이름의 설치된 marketplace plugin은 local 사본보다 우선합니다.
- Antigravity desktop: global로는 `~/.gemini/config/plugins/<name>`, workspace에서는 `.agents/plugins/<name>`(또는 기존의 `_agents/plugins/` 디렉터리). 두 workspace 디렉터리가 모두 존재하면 먼저 통합하세요.
- 독립형 **agy CLI**는 별도의 plugin store를 가지고 있습니다. `antigravity` target은 해당 CLI store가 아니라 desktop/workspace discovery 경로를 관리합니다. `agy`는 단지 Skillshare target alias일 뿐입니다. 독립형 CLI에는 `--target antigravity-cli`를 사용하세요. `--target agy`는 기존의 desktop 의미를 그대로 유지합니다.

이러한 local 패키지에는 `plugin add`를 사용하세요. 기존의 local 폴더나 marketplace 설치를 import하는 것은 지원되지 않습니다. Skillshare는 소유하지 않은 폴더, symlink, 또는 로컬에서 편집된 관리 콘텐츠를 덮어쓰기를 거부합니다. 명시적인 Antigravity 매니페스트 이름은 Git checkout과 스냅샷 전반에서 identity를 안정적으로 유지합니다.

### Oh My Pi (OMP) {#omp}

OMP는 Pi의 패키지 명령이 아니라 자체 marketplace lifecycle을 사용합니다. Skillshare는 검토한 로컬/Git plugin source를 관리하고, `name@marketplace`로 식별되는 네이티브 marketplace 항목을 import합니다:

```bash
skillshare plugin add ./my-omp-plugin --target omp --dry-run --json -g
skillshare plugin add ./my-omp-plugin --target omp --no-tui -g
skillshare plugin import demo@my-market --from omp --dry-run --json -g
```

설치하려면 OMP **18.6.1+**, 검토한 로컬/Git source, 검증되었고 이전에 사용되지 않은 cache 대상 경로, 그리고 아직 존재하지 않는 scope-local 런타임 대상 경로가 필요합니다. 런타임 패키지 이름은 marketplace 이름만이 아니라 검토한 source에서 가져옵니다. 기존 모듈 디렉터리나 링크가 있으면 교체하지 않고 설치를 차단합니다. 대상 경로는 미리보기에 묶이며 설치 전에 다시 확인됩니다. adapter는 plugin 트리를 한데 유지하고, 네이티브 marketplace 설치를 사용하며, 결과 등록을 검증합니다. 카탈로그는 `.omp-plugin/marketplace.json`을 우선하며, OMP 카탈로그가 없을 때만 `.claude-plugin/marketplace.json`으로 돌아갑니다. OMP entry는 `package.json`에 `omp.extensions` 또는 `pi.extensions`를 선언한 package를 가리킬 수 있으며, npm 이름이 다르거나 없어도 marketplace의 plugin 이름을 사용합니다. 각 카탈로그가 같은 이름의 plugin을 서로 다른 폴더에 두면 OMP는 자체 경로, 버전, components를 유지합니다. 텍스트 discovery는 build별로 행을 나누고, JSON은 하나의 논리 candidate를 유지하며 `targetInfo`에 target별 경로와 metadata를 제공합니다. target 버전이 다르거나 하나라도 알 수 없으면 공통 `version`은 비어 있으며, Agent 없이 추가해도 임의의 build 버전을 기록하지 않습니다. OMP 카탈로그가 있으면 OMP entry가 없거나 잘못되어도 Claude build로 돌아가지 않습니다. 미리보기는 파일을 쓸 수 있는 OMP의 네이티브 `--dry-run`을 호출하지 않습니다. 적용하기 전에 source를 검토하세요. OMP는 다음 시작 시 project trust 확인 없이 plugin extension과 tool을 프로세스 안에서 실행할 수 있습니다. 설치되었다고 해서 해당 리소스가 성공적으로 로드되었다는 증거는 아닙니다.

**OMP 18.6.1+의 공유 cache 제한:** 네이티브 uninstall과 upgrade는 다른 프로젝트가 아직 참조하는 plugin cache 파일을 삭제하거나 교체할 수 있습니다. 네이티브 inventory는 사용자 레지스트리와 현재 프로젝트의 레지스트리만 나열하므로, 다른 프로젝트에 대해 이 작업이 안전한지 확인할 수 없습니다. Skillshare는 이런 파괴적인 명령을 호출하지 않습니다. **Remove**와 disable 후 sync는 대신 scope 단위 adapter를 사용해 해당 설치의 레지스트리 항목, 런타임 선택, 검증된 `node_modules` 링크를 삭제합니다. plugin은 단순히 비활성화되는 것이 아니라 그 scope에서 제거됩니다. 공유 cache, marketplace, plugin 설정은 유지되며 cache garbage collection은 시도하지 않습니다. 이전 cache가 아직 남아 있으면 재설치 시 다른 프로젝트가 사용할 수 있는 파일을 교체하지 않고 새로운 marketplace/cache identity를 자동으로 선택합니다. 읽을 수 없거나, 링크이거나, 디렉터리가 아닌 cache 대상 경로는 계속 차단됩니다.

제거하려면 검증된 18.6.1+ 메타데이터 계약, 일치하는 설치 버전, 모호하지 않은 JSON, 증명된 런타임 링크 소유권이 필요합니다. 실제 모듈 폴더, 외부 링크, npm dependency 충돌, 모호한 런타임 소유자는 거부됩니다. cache manifest가 없으면 adapter는 scope-local 링크가 해당 설치의 cache를 가리키는 고유한 runtime-lock 키를 요구합니다. marketplace 이름을 추측하지 않으며, 실제 런타임 링크가 여전히 설치된 상태에서 성공을 보고하지도 않습니다. 검증할 수 없는 활성 설치는 계속 차단됩니다. 네이티브 uninstall이 이미 cache, 등록, 런타임 링크/선택을 제거했다면 Forget은 오래된 Skillshare binding만 삭제합니다. 다른 plugin의 `node_modules` 디렉터리는 이를 막지 않습니다. 네이티브 파일이 바뀌면 미리보기는 무효가 됩니다. 일부만 완료된 제거는 전체 네이티브 설정 백업을 저장하지 않고 유지된 cache와 binding identity에서 다시 시도할 수 있습니다. Windows에서 쓰려면 추가로 검증 가능한 비공개 ACL이 필요합니다. Skillshare는 통과시키기 위해 권한을 변경하지 않습니다. extension 코드는 실행되지 않습니다.

adapter는 Skillshare의 제거 작업을 직렬화하고 네이티브 파일 revision을 확인하지만, OMP의 plugin 명령은 그 lock에 참여하지 않습니다. 네이티브 plugin 변경을 동시에 실행하지 마세요. 이것은 여러 파일에 걸친 atomic 네이티브 트랜잭션이 아닙니다. 파괴적이지 않은 upgrade 계약이 마련될 때까지 **업데이트는 계속 차단됩니다**. import된 코드는 검토한 source 없이 업그레이드되지 않습니다.

project 작업은 선택한 루트가 OMP의 네이티브 project anchor와 일치해야 합니다. 조용히 global이나 상위 프로젝트에 설치하지 않습니다. 네이티브 profile/config-root override(`PI_CONFIG_DIR`, `PI_CODING_AGENT_DIR`, `OMP_PROFILE` 또는 `PI_PROFILE`)가 있으면 plugin 관리가 차단됩니다. 네이티브 cache 위치가 바뀌면 기존 미리보기는 무효가 됩니다. 임의의 계정 `config_dir`은 OMP의 plugin store를 바꾸지 않으므로 OMP 계정 plugin target은 지원되지 않습니다.

네이티브 npm/Git/link 패키지는 inventory에 표시되지만 marketplace plugin으로 import되지 않습니다. 이런 패키지는 OMP에서 관리하세요. `plugin add npm:...`은 여전히 Pi 전용 workflow입니다. plugin의 extension, hooks, skills, MCP 항목을 Skillshare의 다른 리소스로 따로 복사하지 마세요.

#### 독립형 OMP extension 선택

OMP target의 **Extensions** 탭을 열어 파일 기반 선택을 검토하세요. 카드는 파일을 source와 scope별로 묶으며, plugin은 검사한 패키지 루트별로, 나머지 파일은 디렉터리별로 묶습니다. 파일 목록은 계속 표시됩니다. plugin 카드는 extension 폴더 이름이 아니라 네이티브 패키지 identity에서 가져온 패키지 아이콘과 `Plugins · <name>` 제목을 사용합니다. 알려진 manifest 버전과 scope는 계속 표시되며, 반복되는 출처 태그 대신 **Open Plugins** 탐색 아이콘이 있습니다. 공유 폴더는 plugin 기준 상대 경로이고, 전체 파일 경로는 tooltip에 남습니다. **Details**는 Pi와 같은 레이블이 붙은 2열 레이아웃으로 전체 source, 네이티브 이름, 선택 식별자, 메모를 보여 줍니다. identity 메타데이터가 없으면 검사한 폴더로 대체하며, 패키지 identity나 버전을 추측하지 않습니다. 읽기 전용 사유와 Hooks 또는 Plugins로 가는 링크는 해당 항목과 함께 계속 표시됩니다. 지원되는 독립형 모듈에는 스위치가 있습니다. **Preview**는 변경 사항을 보여 주고, **Apply**는 선택한 네이티브 YAML 설정 파일의 `disabledExtensions`만 씁니다. global, project, 명시적으로 구성한 계정 디렉터리는 서로 분리됩니다. 편집기는 관련 없는 YAML과 주석을 유지하고, OMP의 네이티브 lock 아래에서 검토한 revision을 다시 확인하며, 오래되었거나 사용 중인 쓰기는 거부합니다.

선택 편집에는 선택한 launcher 뒤에 검증 가능한 OMP **18.6.1+** 패키지가 필요합니다. 식별되지 않은 버전, 독립형 바이너리, wrapper launcher는 읽기 전용으로 남습니다. 이 확인은 OMP를 실행하지 않고 패키지 메타데이터를 읽습니다. 같은 이름의 모듈 그룹, disabled-name 필터를 우회하는 명시적 파일, 주변의 `hooks/pre|post` factory, 불확실한 선택, 링크된 경로는 읽기 전용입니다. Hooks 소유 항목과 plugin 소유 항목은 각각의 관리자에게 맡겨집니다.

백업은 변경 전후의 선택과 설정 해시를 Skillshare state 디렉터리의 `omp-extensions/backups` 아래에 기록하며, 관련 없는 YAML 값이나 자격 증명은 기록하지 않습니다. 이것은 선택 복구 기록이며 설정 전체를 복원하는 것이 아닙니다. 적용에 성공했다는 것은 디스크의 설정을 의미할 뿐 실행 중인 extension을 뜻하지 않습니다. 재시작/다시 로드는 OMP에서 수행해야 합니다.

### Pi와 OpenCode

Pi는 같은 패키지의 첫 번째 전역 등록과 마지막 프로젝트 등록을 우선합니다. 앞선 전역 소스나 뒤의 프로젝트 소스의 identity를 확인할 수 없으면 Skillshare는 어느 등록이 우선하는지 증명할 수 없으므로, 덮어써질 수 있는 항목(상속하는 프로젝트 delta 포함)을 Unknown／읽기 전용으로 유지합니다. URL의 query를 제거해 identity를 추측하지 않습니다. 이 불확실성의 영향을 받지 않는, 확인된 항목은 계속 편집할 수 있습니다.

Pi는 `pi install` / `pi remove`를 사용합니다. inventory는 extension 코드를 로드하지 않고 문서화된 패키지 설정을 읽습니다. `PI_CODING_AGENT_DIR`을 준수합니다. Pi project trust는 Pi에서 직접 설정해야 합니다. Skillshare는 대신 `--approve`를 전달하지 않습니다.

#### pi.dev의 npm 패키지

`plugin add npm:<package>`는 [pi.dev](https://pi.dev/packages)에 나열된 것처럼 npm에 게시된 패키지를 Pi를 통해 설치합니다.

```bash
skillshare plugin add npm:@scope/package --target pi --dry-run --json -g
skillshare plugin add npm:@scope/package@1.2.0 --target pi --no-tui -g
```

Pi가 패키지를 내려받고 install script를 실행하므로 Skillshare는 내용을 미리 검토할 수 없습니다. 추가하기 전에 pi.dev나 npm에서 패키지를 확인하세요. `discover`는 npm 소스를 받지 않으며, npm 소스에는 `--source-ref`, `--entry`, `--plugin`을 쓸 수 없습니다. npm 소스는 Pi target만 받으며, `pi`를 실행하는 Pi 계정도 포함됩니다. 다른 실행 파일을 쓰는 계정은 그 실행 파일로 설치한 뒤 가져오세요. `--project`를 쓰면 Pi가 패키지를 프로젝트 설정에 설치합니다. 프로젝트에 `.pi` 폴더가 있으면 Pi에서 프로젝트를 신뢰해야 Pi가 패키지를 변경합니다.

Pi는 패키지 이름마다 항목을 하나만 유지합니다. Pi에 같은 소스가 이미 있으면 `add`는 그것을 가져옵니다. 같은 패키지의 다른 버전은 설치되며, Pi가 그 항목의 소스를 바꿉니다. `update`는 `pi update`를 실행하지만, 정확한 버전에 고정된 패키지는 Pi가 그대로 유지하므로 새 버전으로 다시 추가하세요. 버전 없이 추가한 패키지라면 `check`는 설치된 패키지의 `package.json` 버전을 공개 registry에서 npm의 `latest` 태그가 가리키는 버전과 비교하고, `update`는 이미 그 버전인 패키지는 그대로 둡니다. 버전 범위나 태그를 붙여 추가한 패키지, 환경 변수나 `.npmrc`가 다른 registry로 보내는 패키지, 버전이 단순한 `X.Y.Z`가 아닌 패키지는 Pi에서 확인할 항목으로 표시됩니다. dashboard는 Plugins 페이지와 Pi Target의 Extensions 탭에서 모든 Pi 패키지의 설치된 버전을 보여 줍니다. 패키지의 일부 extension을 꺼 두었다면 Pi가 그 규칙을 새 버전에도 유지하고, Skillshare도 다시 기록하므로 나중에 다시 설치해도 복원됩니다. 다른 Skillshare 패키지가 이미 관리하는 Pi 패키지는 거부됩니다. 그 패키지를 업데이트하거나 제거하세요.

대시보드의 추가 대화 상자에는 `pi install npm:<package>` 명령이나 패키지의 pi.dev 주소를 그대로 붙여 넣을 수 있습니다. 둘 다 해당하는 `npm:` 소스로 바뀝니다.

#### 패키지의 extension 선택

대시보드에서 `pi`와 Pi 계정의 target 페이지에는 **Extensions** 탭이 있습니다. 해당 target의 `settings.json`에 있는 각 패키지 항목과 그 필터가 선택하는 extension을 보여 줍니다. Skillshare가 관리하는 패키지는 OMP plugin 카드와 같이 패키지 아이콘과 `Plugins · <name>` 제목을 사용합니다. 설치된 버전, scope, 관련 project override 형태는 그대로 유지되며, 탐색 아이콘은 Plugins를 엽니다. 관리되지 않는 Pi 패키지는 네이티브 identity와 Pi 아이콘을 유지합니다. 이 표시 방식은 소유권이나 extension 선택 규칙을 바꾸지 않습니다. extension 행은 계속 표시되며, **Details**는 source와 filter 규칙을 별도로 보여 줍니다. 스위치는 해당 항목의 `extensions` 목록에 정확한 `+path` 또는 `-path` 규칙 하나를 씁니다. 다만 그 파일 자체의 정확한 규칙을 삭제하는 것만으로 스위치가 원하는 상태가 되면 그 규칙을 삭제합니다. 따라서 파일을 남은 규칙이 선택하는 상태로 되돌려도 규칙이 남지 않습니다. **Remove rule**은 스위치가 규칙을 삭제하지 않는 경우에 표시되며, 해당 파일의 정확한 규칙(상대 경로든 절대 경로든)을 삭제하며, 이후 그 파일은 남은 규칙에 따라 결정됩니다. 결과는 미리보기에 표시됩니다. 적용 시에는 항상 먼저 미리보기를 보여 주고 이 목록만 수정합니다. 항목의 다른 키, `skills`, `prompts`, `themes` 필터, glob과 `!` 규칙, 파일의 나머지 부분은 그대로 유지됩니다. 문자열 항목은 규칙을 담을 수 있도록 `{"source": ...}`로 바뀝니다. 문자열 항목에서 Pi는 패키지의 skills, prompts, themes를 `pi` manifest에서만 불러오지만, 객체 항목은 manifest에 없는 것을 패키지의 `skills`, `prompts`, `themes` 폴더에서도 불러옵니다. 그런 폴더가 있는 패키지의 문자열 항목은 변환 후에도 해당 리소스가 그대로인지 Skillshare가 확인할 수 없으므로 읽기 전용입니다. 파일 하나를 가리키는 소스도 Pi가 그대로 불러오고 필터를 무시하므로 읽기 전용입니다. 미리보기 이후 파일이 변경되었거나 Pi가 설정 잠금을 보유하고 있으면 아무것도 쓰지 않습니다. 쓰는 동안 Skillshare는 Pi와 같은 방식으로 그 잠금을 보유하며, 잠금을 잃으면 쓰지 않습니다. 적용 전에 변경되는 extension 목록과 변경 전후 파일 해시의 기록을 저장합니다. 성공적으로 적용된 기록은 자동으로 삭제하지 않고 보존하며, 적용에 실패하면 해당 시도에서 새로 만든 기록만 삭제합니다. 이 기록은 `settings.json`의 사본이 아니며 설정 파일 전체를 복원할 수 없습니다. 이 탭에는 Pi로 직접 설치한 패키지([pi.dev](https://pi.dev/packages)의 `npm:` 패키지 등)를 포함해 설정에 있는 모든 패키지가 표시됩니다. Skillshare는 `plugin`을 통해서만 패키지를 설치하고 제거합니다. `plugin add`는 로컬 디렉터리, Git 소스 또는 [npm 패키지](#pidev의-npm-패키지)를 받고, `plugin import --from pi`는 Pi로 설치한 패키지를 관리 대상으로 가져옵니다.

Skillshare는 패키지를 실행하지 않고 읽기 때문에, 이 탭은 설정이 어떤 파일을 선택하는지(스위치와 선택 메모)를 보여 줄 뿐 Pi가 실제로 로드했는지는 보여 주지 않습니다. 적용 후에는 Pi를 다시 로드하세요. 설정에서 지정했지만 패키지에 없는 파일은 없음으로 표시됩니다. Skillshare가 판단할 수 없는 선택은 **판단할 수 없음**으로 표시되고 이유와 변경 방법이 함께 나오며, 켜짐이나 꺼짐을 추측하지 않습니다. 편집하려면 해당 target의 Pi가 0.99.2 이상(Pi 자체로 확인한 가장 오래된 버전)이어야 하고 설정이 엄격한 JSON이어야 합니다. 그보다 오래된 버전은 읽기 전용이며 탭에 감지한 버전이 표시됩니다. 다른 실행 파일을 쓰는 Pi 계정은 읽기 전용이며 Skillshare는 그것을 실행하지 않습니다. 목록이 `[]`(아무것도 로드하지 않음)인 항목은 읽기 전용이며, Skillshare가 평가할 수 없는 패턴(예: emoji에 대한 `?`)으로 결정되는 extension도 읽기 전용입니다. 소스가 비어 있거나, 소스 또는 규칙에 짝이 없는 UTF-16 서로게이트 이스케이프나 잘못된 UTF-8이 있는 항목은 Skillshare가 Pi와 똑같이 읽을 수 없으므로 작성된 그대로 읽기 전용입니다. Pi는 패키지의 첫 번째 전역 항목만 사용하므로, Skillshare가 그 항목을 읽을 수 없으면 같은 패키지의 이후 항목도 읽기 전용입니다.

Pi로 동기화하는 프로젝트도 프로젝트 페이지에 같은 탭이 있습니다. 프로젝트 설정을 전역 설정 위에 적용했을 때 각 패키지가 무엇을 선택하는지 보여 주며, `pi (global)`에서 상속한 것인지 프로젝트 재정의인지 표시합니다. 스위치는 규칙을 프로젝트의 `.pi/settings.json`에만 저장하며, `pi config`와 같은 방식으로 씁니다. 전역 패키지에는 지정한 파일만 바꾸는 프로젝트 항목 `{"source": ..., "autoload": false, "extensions": [...]}`이 추가되고 전역 항목은 그대로 유지됩니다. 로컬 소스는 `.pi` 기준 상대 경로로, npm 또는 git 소스는 전역 설정에 적힌 그대로 씁니다. 이런 항목의 마지막 프로젝트 규칙을 제거하면 이전 등록의 filters가 활성화되지 않을 때만 항목을 제거하고, 그렇지 않으면 빈 winning override를 유지합니다. 명시적인 JSON `false`만 delta이며 `autoload: null`은 `false`가 아닙니다. 대응하는 전역 항목이 없고 `autoload: false`인 프로젝트 항목은 `+`로 지정한 파일만 로드합니다. 파일과 그 `.pi` 폴더는 적용할 때만 만들어집니다. 전역 설정과 Pi의 `trust.json`은 절대 쓰지 않으며, Skillshare가 대신 프로젝트를 신뢰하지도 않습니다. Pi는 프로젝트를 신뢰할 때만 프로젝트 설정을 사용합니다. 자격 증명이나 쿼리가 있는 전역 소스는 프로젝트에 복사하지 않으므로 그 패키지는 프로젝트에서 읽기 전용이며, 프로젝트 설정에 Skillshare가 읽을 수 없는 항목이 있으면 모든 패키지가 읽기 전용입니다. 적용 중에는 프로젝트 파일에 대한 Pi의 잠금을 유지하고, 쓰기 직전에 두 설정 파일과 패키지를 다시 확인합니다. Pi 자체의 `extensions` 폴더에 있는 extension([extras](./extras.md)가 그곳에 연결한 파일 포함)은 변경할 위치와 함께 읽기 전용으로 표시됩니다. 프로젝트는 자체 폴더(Pi는 프로젝트를 신뢰할 때만 읽음)와 전역 폴더를 표시합니다.

OpenCode는 관리되는 entry를 `opencode.json` 또는 기존의 `opencode.jsonc`에 file URL로 등록하며, 주석과 관련 없는 항목은 유지합니다. Version 1은 `plugin`을 사용하고, version 2는 `plugins`를 사용합니다. `XDG_CONFIG_HOME`과 절대 경로의 global `OPENCODE_CONFIG`를 준수합니다. 모호하거나 지원되지 않는 override는 거부됩니다. Skillshare가 버전의 스키마를 선택할 수 있도록 OpenCode는 PATH에 있어야 합니다.

로컬 OpenCode source는 빌드된 entry(`main`, 문자열 root export, 또는 `index.js`)와 필요한 런타임 의존성을 이미 포함하고 있어야 합니다. Skillshare는 빌드 스크립트를 실행하거나 source에 의존성을 설치하지 않습니다. 등록되었다고 해서 모듈이 성공적으로 로드되었다는 증거는 아닙니다. 다시 로드한 후 OpenCode를 확인하세요.

사용자 범위 npm 등록에 managed cache가 없으면 Unknown／읽기 전용으로 표시합니다. 미설치라는 뜻은 아닙니다. Pi가 Skillshare에서 조회하지 않는 legacy global npm/pnpm 경로를 사용할 수 있습니다.

Import는 일반 Pi source와 Pi 0.99.2 이상에서 지원되는 source 및 옵션 형식의 filtered object를 받아들입니다. 미리보기에는 보존할 키 이름만 표시하며 opaque 값은 표시하지 않습니다. 네이티브 설정과 설치 파일은 변경하지 않고, 원본 entry는 Skillshare의 비공개 state에 저장하며 로컬 receipt에 digest만 기록합니다. sync/update는 현재 entry를 유지합니다. 제거 전에 최신 옵션을 저장하고 재설치 시 네이티브 install 전에 object를 복원하여 다른 리소스가 일시적으로 기본 활성화되는 것을 방지합니다. 같은 Apply에서 여러 항목을 복원할 때는 해당 작업이 직접 쓴 정확한 내용만 허용합니다. 다른 설정 변경이 있으면 이후 복원을 중단합니다. 비공개 state를 보관하세요. 기록이 없거나 변경되었거나 다른 target 소유이면 복원을 거부합니다. Windows의 새 등록 디렉터리는 owner/SYSTEM용 보호 ACL로 생성합니다. 기존 디렉터리와 기록이 현재 사용자와 특권 SYSTEM/Administrators 이외에 접근을 허용하거나 ACL을 검증할 수 없으면 가져오기·복원을 거부합니다. 기존 ACL은 변경하지 않습니다. 기록을 보관하고 소유자가 접근 보호를 복구한 뒤 다시 시도하세요. 불확실한 source·우선순위, 미지원 인코딩, Pi가 정규화할 로컬 참조는 읽기 전용입니다. 일반 OpenCode entry는 가져올 수 있지만 filtered OpenCode는 계속 거부됩니다. 10초 stale 기준을 넘긴 빈 lock 디렉터리는 inode와 mtime이 바뀌지 않은 경우에만 회수합니다. 새 lock, 갱신·교체된 lock, 비어 있지 않은 디렉터리, 파일, symlink는 유지합니다. 오래되었다고 소유자가 종료된 것은 아니며 마지막 확인과 제거는 atomic CAS가 아닙니다. import된 Pi 패키지는 global 모드에서 `pi update SOURCE`로 업데이트되며 설정 entry는 유지됩니다. project의 패키지는 Pi에서 업데이트하세요. `pi update`는 global 패키지에도 영향을 주기 때문입니다. import된 OpenCode v1 패키지는 해당 네이티브 도구에서 업데이트됩니다. OpenCode v2의 global import는 자체 네이티브 업데이트 명령을 사용할 수 있지만, v2 업데이트 명령이 global이기 때문에 project import는 네이티브로 업데이트해야 합니다.

```bash
skillshare plugin add ./cursor-plugin --target cursor --no-tui
skillshare plugin add ./agy-plugin --target agy --no-tui -p
skillshare plugin add ./pi-package --target pi --no-tui
skillshare plugin add ./opencode-package --target opencode --no-tui
skillshare plugin import npm:my-pi-package --from pi --name my-package --no-tui
```

## Ref와 명시적 Entry

**Add plugin**의 고급 옵션은 선택적인 Git ref와 OpenCode entry를 받습니다. 일반적인 가이드 흐름을 사용하려면 비워 두세요. 터미널 마법사도 동일한 플래그를 받아들이며, 자동화에는 다음을 사용할 수 있습니다:

```bash
skillshare plugin discover obra/superpowers --source-ref v6.3.0 --json
skillshare plugin add owner/repo --source-ref v1.0.0 --target copilot --no-tui
skillshare plugin add ./package --entry dist/plugin.js --target opencode --no-tui
skillshare plugin update review --source-ref v1.1.0 --target claude --dry-run --json
```

공유 바인딩은 `source_ref`를 기록하고 해석된 `commit`은 로컬 기록에 저장합니다. 설치는 검토된 commit을 사용합니다. `check`와 `update`는 구성된 ref를 다시 해석하므로, commit이 고정된 상태에서도 브랜치는 진행될 수 있습니다. `--revision`은 Git ref가 아니라 미리보기 토큰입니다. `--entry`는 각 후보의 패키지 루트를 기준으로 하며 이미 존재해야 합니다. 이는 빌드나 패키지 매니저 설치를 트리거하지 않습니다.

Copilot과 Antigravity CLI 설치는 검토된 로컬 스냅샷을 사용합니다. import된 항목은 재설치를 위한 검토된 source가 없습니다: 제거 후에는 네이티브 클라이언트에서 설치하고 다시 sync하세요. Grok도 설치/재설치 전에 네이티브 trust가 필요합니다. Skillshare는 네이티브 trust 승인 플래그를 절대 제공하지 않습니다.

## 호환성과 경계

- Claude는 자체 네이티브 `.claude-plugin/plugin.json` 패키지를 요구합니다.
- Codex는 `.codex-plugin/plugin.json`과 인식된 portable root `plugin.json` 패키지를 받아들입니다. 둘 다 있는 패키지는 Codex가 portable root `plugin.json`에서 설치하므로, Skillshare는 그 manifest의 버전을, 없으면 `1.0.0`을 예상합니다. Claude 전용 패키지는 자동으로 변환되지 않습니다.
- Source에는 local plugin entry가 있는 marketplace가 포함될 수 있습니다. 네이티브 카탈로그는 plugin 이름으로 병합되며 target별 package 경로를 유지합니다. 외부 entry는 저장소를 직접 추가하거나 네이티브로 설치한 후 import하라는 안내와 함께 보고됩니다. command 기반 source는 자동으로 승인되지 않습니다.
- 완전한 source 스냅샷은 plugin 스크립트, asset, 그리고 안전한 상대 symlink(`AGENTS.md → CLAUDE.md` 포함)를 유지합니다. 절대 경로, 범위를 벗어나는(escaping), 끊어진(dangling), 순환(cyclic), `.git`을 참조하는 링크와 특수 파일은 거부됩니다. source는 20,000개 파일과 100MiB로 제한됩니다.
- 네이티브 설치가 런타임 활성화의 증거는 아닙니다. Agent를 재시작/다시 로드하고 해당 Agent에서 인증 또는 hook trust를 완료하세요.
- Codex는 `PATH`의 `codex`를 실행합니다. `PATH`에 없으면 Skillshare는 Homebrew의
  `/opt/homebrew/bin`과 `/usr/local/bin`, 그다음 Codex 데스크톱 app에 포함된 CLI
  (macOS의 `ChatGPT.app` 또는 `Codex.app`, Windows의 `%LOCALAPPDATA%\OpenAI\Codex\bin`)를 차례로 시도합니다. Codex가 다른 위치에 있는 머신에서는
  [`SKILLSHARE_CODEX_CLI`](/docs/reference/appendix/environment-variables#skillshare_codex_cli)를 설정하세요. 자체 `cli`를 가진 account는 검색하지 않습니다.
- Codex의 네이티브 project 설치는 이 adapter에서 제공되지 않습니다. global Codex 설치에 대해서는 sync 선택이 여전히 동작합니다.
- Codex에는 update 명령이 없으므로, 업데이트는 갱신된 스냅샷에서 플러그인을 다시 add합니다. add는 항상 플러그인을 활성화하므로 Codex에서 비활성화된 플러그인은 건너뜁니다. Import한 Codex 플러그인은 `codex plugin marketplace upgrade NAME`으로 업데이트되며, 이는 Codex가 해당 마켓플레이스에서 설치한 모든 플러그인을 다시 설치합니다. Codex도 시작할 때 같은 작업을 합니다.
- 업데이트는 처리할 수 없는 Target을 이유와 함께 건너뛰며, 해당 플러그인의 다른 Agent는 그대로 업데이트됩니다. 건너뛴 업데이트는 대기 상태로 남아 이후 sync에서 처리됩니다.
- import된 plugin은 원래의 marketplace identity를 유지합니다. `check`는 source가 없는 import된 plugin에 대해 release 가능 여부를 추론할 수 없습니다. 단, Pi의 npm 패키지는 npm에서 확인합니다. import한 Claude 또는 Codex plugin의 네이티브 marketplace가 사라지면 sync와 update는 해당 Target을 건너뛰고 이유를 알려 줍니다. 새 머신은 같은 이름의 marketplace가 있어도 명시적인 로컬 가져오기가 필요합니다. Agent에서 marketplace를 복원하거나, 해당 Target을 제거한 뒤 source에서 plugin을 다시 추가하세요. Skillshare는 import된 plugin을 임의로 다른 source로 옮기지 않습니다.
- Skillshare는 관리하는 Claude/Codex plugin마다 marketplace를 하나 등록하고 `skillshare-<plugin>-<hash>`로 이름을 붙입니다(이전 설치는 `skillshare-<hash>`를 유지). plugin을 제거하거나 제외하면 plugin이 이미 없더라도 그 marketplace도 제거하며, 실패한 정리는 다음 sync에서 다시 시도합니다. marketplace가 사라졌다면 update가 다시 등록합니다. 다른 경로에 있는 같은 이름의 등록과 import된 plugin의 marketplace는 건드리지 않습니다. 스냅샷과 네이티브 캐시는 유지됩니다.
- 이 등록은 user 설정이든 project 설정이든 이 머신의 Skillshare 상태 디렉터리를 가리킵니다. Git이나 dotfile 관리 도구로 Agent 설정을 공유하면 다른 머신에는 없는 경로가 함께 옮겨집니다. 각 머신에서 source로부터 plugin을 추가하세요.
- Claude는 skills 디렉터리에서 plugin manifest가 있는 skill 폴더도 `<name>@skills-dir`라는 plugin으로 읽으며, 같은 이름의 plugin은 하나만 불러옵니다. 같은 이름의 Claude plugin을 추가하면 미리보기에서 이를 알려 줍니다. 둘 중 하나의 이름을 바꾸거나 제거할 때까지 Claude는 plugin을 불러오고 skill 폴더는 건너뜁니다.

네이티브 lifecycle은 Claude Code `2.1.276`, Codex CLI `0.154.0`, Pi `0.85.1`, Copilot CLI `1.0.86`으로 검증되었습니다. Antigravity CLI `1.2.6`은 격리된 네이티브 install/list/remove 작업으로 확인되었습니다. OpenCode `1.18.31`은 버전을 인식하는 등록을 검증하는 데 사용되며, v2 스키마는 fixture 테스트로 커버됩니다. Cursor와 Antigravity의 파일시스템 lifecycle은 격리된 디렉터리에서 테스트되며, GUI 런타임 활성화까지 보장하지는 않습니다. 설치된 command capability와 inventory 스키마는 런타임에 확인되며, 지원되지 않는 작업은 설명과 함께 차단됩니다.

## 공식 형식 참고 자료

- [Cursor local plugins](https://prod.cursor.com/docs/plugins)
- [Antigravity desktop plugins](https://www.antigravity.google/docs/plugins)
- [Antigravity standalone CLI plugins](https://www.antigravity.google/docs/cli/plugins)
- [Pi packages](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/packages.md)
- [OpenCode v1 plugins](https://opencode.ai/docs/plugins/)
- [OpenCode v2 plugins](https://opencode.ai/v2/docs/plugins)

## 공유 선언과 머신별 설치 {#host-state}

`plugins.version: 2`는 전역/프로젝트 설정에 source, ref, plugin, entry, 대상별 소스와 `sync` 선택 또는 `imported: true`를 저장합니다. 대상 소스가 명시적으로 기록되므로 패키지 기본값을 바꿔도 다른 배포본으로 바뀌지 않습니다.

```yaml
plugins:
  version: 2
  packages:
    router:
      source: https://github.com/acme/router.git
      plugin: router
      bindings:
        pi: {source: https://github.com/acme/router.git, plugin: router}
        omp: {imported: true}
```

네이티브 ID, 검토한 commit/digest/version, Pi 등록 참조와 대기 작업은 `<StateDir>/plugins/<config-path-hash>/state.json`(로컬 형식 version 1)에 저장합니다. StateDir는 기기마다 따로 유지하세요. 상태는 설정 파일의 **절대 경로**로 구분됩니다. 프로젝트나 설정을 이동하거나 이름을 바꾸면 새 상태 영역이 생기며 이전 설치는 추적하거나 정리하지 않습니다. 이동 전에 이전 설정으로 제거하거나 새 위치에서 명시적으로 관리·가져오세요.

sync, 일반 update, 검증, 실패와 재시도는 공유 선언을 바꾸지 않습니다. `update --source-ref`는 ref가 달라질 때만 선언을 변경합니다. 읽기는 로컬 작업 잠금 아래 레거시 증거를 저장하며 공유 YAML은 바꾸지 않습니다. 다른 작업이 잠금을 보유하면 해당 작업이나 다음 읽기에 저장을 맡깁니다. add, import, 선택, remove는 필요한 선언 변경을 저장하고 외부 변경을 감지합니다. 동시 Syncthing 편집을 병합하지는 않습니다.

소스 검토와 commit 고정은 **기기별**입니다. 브랜치 ref는 해당 기기의 설치·업데이트 시점에 해석되므로 비대화형 sync가 다른 기기에서 검토한 것보다 새로운 commit을 설치할 수 있습니다. ref 변경도 기기별로 해석합니다. 모든 기기에서 같은 버전을 쓰려면 `source_ref`에 commit SHA를 지정하세요. 로컬 소스 경로도 각 기기에 있어야 합니다.

native-only 대상은 이 기기에서 가져오기가 필요하다고 표시되며 sync가 건너뜁니다. 네이티브 클라이언트에서 설치한 뒤 `plugin import LOCAL-ID --from TARGET --name NAME`을 실행하세요. 기존 소스 선언으로 가져오면 로컬 기록만 만들고 다른 기기의 소스나 설치를 바꾸지 않습니다. 검토한 소스의 동일 ID를 다시 가져와도 소스를 유지합니다. 명시적인 Pi `add npm:PACKAGE`는 설치 방법을 공유하고 Pi가 스크립트를 실행합니다. npm import는 native-only 선언을 만듭니다.

가져오기는 원격 소스를 다운로드하지 않습니다. 직접 작성한 원격 소스 설정에 `plugin:`이 없다면 가져오기 전에 네이티브 패키지 이름을 `plugin:`에 지정하세요. 그러면 오프라인으로 패키지의 일치 여부를 확인할 수 있습니다. `plugin add`로 만든 설정에는 이 이름이 이미 기록되어 있습니다. 로컬 디렉터리 설정은 로컬에서 탐색할 수 있습니다.

레거시 읽기와 sync는 YAML을 바꾸지 않으며 다음 선언 편집에서 version 2로 전환합니다. **첫 v2 편집 전에 모든 writer를 업그레이드하고, 공유 YAML이 레거시 형식일 때 각 기기에서 `skillshare plugin list`를 실행하세요.** 네이티브 클라이언트가 일시적으로 없어도 이전 증거를 저장합니다. 채택에는 원래 네이티브 환경의 inventory와, 소스 설치의 경우 소유한 snapshot 증거가 필요합니다. 다른 호스트의 대기 작업은 실행하지 않습니다. 미확인 힌트와 제거 증거는 소유권 확인과 정리가 끝날 때까지 유지합니다. 레거시 npm은 add/import를 구별할 수 없으므로 공유 설치 방법이 필요하면 명시적으로 다시 add하세요.

계정이 바뀌면 이전 네이티브 환경을 먼저 해결한 뒤 정리하세요. 인증, 네이티브 설정, trust는 각 도구에 남습니다.

가져오기를 반복해도 공유 의도를 다시 쓰지 않습니다. 정리가 실패하면 다음 sync에서 재시도할 로컬 기록을 유지합니다. 이전 바이너리는 버전이 지정된 선언 필드를 알지 못하므로 v2를 쓸 수 없습니다.

공유 소스 설정이 바뀌어도 로컬 가져오기는 유지합니다. 이 기기를 공유 소스로 전환하려면 다음을 실행하세요.

```bash
skillshare plugin unimport NAME --target TARGET --no-tui
skillshare plugin sync NAME --no-tui
```

`unimport`에는 하나 이상의 명시적인 `--target`이 필요하며, 선택한 각 대상에 기존 로컬 가져오기 기록과 공유 소스 설정이 있어야 합니다. 대상의 제거 어댑터를 사용해 이 기기에서 가져온 설치를 제거한 다음 로컬 기록을 삭제합니다. 공유 선언과 다른 기기의 설치는 그대로 유지됩니다. 다음 sync는 공유 소스에서 설치합니다.

제거하면 해당 설치와 연결된 네이티브 클라이언트 설정이나 로그인 정보도 삭제됩니다.
