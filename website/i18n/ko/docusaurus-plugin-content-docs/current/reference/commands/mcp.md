---
sidebar_position: 3
---

# mcp

이식 가능한 MCP 연결 정의를 관리하고 네이티브 Agent 설정과 동기화합니다.
[Set up MCP once](/docs/how-to/daily-tasks/sharing-mcp)부터 시작하세요.

## Commands

```bash
skillshare mcp
skillshare mcp add
skillshare mcp edit
skillshare mcp edit docs --url https://updated.example/mcp --no-tui
skillshare mcp add docs --url https://example.com/mcp --target claude --sync
skillshare mcp add local --target codex -- company-mcp --workspace /path/to/workspace
skillshare mcp import docs --from claude --target claude --target cursor --sync
skillshare mcp import docs --file ./provider.json --target claude
skillshare mcp list --json
skillshare mcp check
skillshare mcp check docs --json --no-dns
skillshare mcp check --live --timeout 30s
skillshare mcp remove docs --sync
skillshare mcp remove docs --keep-files
skillshare mcp restore BACKUP_ID --dry-run
skillshare sync mcp --dry-run --json
skillshare sync mcp
skillshare sync --all
```

| Option | Meaning |
|---|---|
| `--target CLIENT` | 수신 client; 여러 client를 선택하려면 반복 지정. `--target none`은 서버를 어떤 client에도 쓰지 않고 Skillshare에만 유지합니다. [아래](#keep-a-server-without-syncing-it) 참고 |
| `--url URL` | `add`용 Streamable HTTP 엔드포인트 |
| `-- command args...` | `add`용 로컬 실행 파일과 리터럴 인자 |
| `--disabled` | project mode에서 `add`와 함께 사용: Agent의 global 설정이 정의한 서버를 끕니다. [아래](#turn-off-a-global-server-in-one-project) 참고 |
| `--tools-allow TOOLS` | 이 도구만 허용, 쉼표로 구분; `*`는 임의의 문자와 일치; `""`는 값을 지웁니다. [도구 정책](#tool-policy) 참고 |
| `--tools-deny TOOLS` | 이 도구는 항상 제외, 쉼표로 구분; allow보다 우선; `""`는 값을 지웁니다. [도구 정책](#tool-policy) 참고 |
| `--pi-options JSON` | Pi 내장 MCP의 그 밖의 서버별 필드를 JSON 객체로 지정. [Pi](#pi-options) 참고 |
| `--from CLIENT` | import할 기존 client, 또는 `--file`의 형식 |
| `--file PATH` | 네이티브 JSON/JSONC, TOML 또는 Goose YAML; `.toml`은 기본적으로 Codex로 처리되며, 다른 형식은 MCP 섹션에서 감지됨; 명시적인 방언을 지정하려면 `--from` 사용 |
| `--sync` | 저장 후 동기화; noninteractive add/import/remove는 그렇지 않으면 저장만 함 |
| `--keep-files` | `remove`와 함께 사용: 서버 관리를 멈추고 Agent 항목은 그대로 둡니다. `--sync`와 함께 쓸 수 없습니다. [아래](#stop-managing-a-server) 참고 |
| `--replace` | add/import 중 기존 source 정의를 명시적으로 교체; import 시 가져온 client의 항목이 다르면 이것도 다시 작성 |
| `--dry-run`, `-n` | 저장하거나 네이티브 구성을 작성하지 않고 미리보기 |
| `--json` | 구조화된 출력; sync/preview 보고서에는 이름, 경로, 작업만 포함되며 서버 값은 포함되지 않음 |
| `--no-dns` | `check`와 함께 사용: 원격 서버의 호스트 조회를 건너뜀. [아래](#check-servers-before-an-agent-starts-them) 참고 |
| `--live` | `check`와 함께 사용: 각 로컬 서버를 시작하고 각 원격 서버를 호출하는 검사도 수행. [아래](#probe-servers-live) 참고 |
| `--timeout DURATION` | `check --live`와 함께 사용: 서버별 프로브 제한 시간 (예: `30s`); 기본값 `10s` |
| `--no-tui` | 대화형 메뉴 비활성화; `tui: false`, `--json`, 또는 비터미널 입출력에서도 비활성화됨 |
| `--revision ID` | add/import/remove 또는 `sync mcp`에 일치하는 미리보기 요구 |
| `--global`, `-g` | global Skillshare 구성 사용 |
| `--project`, `-p` | project Skillshare 구성 사용 |

서브커맨드 없이 실행하면 `mcp`는 대화형 터미널에서 검색 가능한 관리자를 열거나,
noninteractive mode에서는 상태를 출력합니다. 이름 없이 noninteractive import를 실행하면
선택할 수 있도록 파싱된 후보 목록을 표시하며 저장하지 않습니다. 후보는 이식 가능한
정의를 포함하며, 인식 가능한 시크릿은 참조로 변환됩니다. Agent 고유 필드는 경고로
표시되고 제외되며, 비활성화된 서버와 지원되지 않는 전송 방식은 후보를 차단합니다.
`restore`는 적용 전에 항상 다시 미리보기를 표시하며, 적용하지 않고 확인하려면
`--dry-run`을 사용하세요.

`--pi-extension`, `--pi-options-prune`, `--direct-tools`는 0.23.0에서 제거되었으며,
이제 대신 무엇을 써야 하는지 알려주는 메시지와 함께 실패합니다.
[0.22에서 Pi 업그레이드하기](#pi-migration)를 참고하세요.

`sync mcp`는 scope flag, `--dry-run`, `--json`, `--no-tui`, `--revision`을 받습니다.
`sync --all`은 skill, agent, extras, MCP + hooks를 포함하며, 일반 `sync`는 기존 리소스 동작을
유지합니다. MCP 충돌은 `--all`이 다른 리소스를 변경하기 전에 확인됩니다. 리소스 유형과
네이티브 파일은 하나의 트랜잭션이 아니라 별개의 작업입니다.

## Interactive management

`skillshare mcp` 또는 `skillshare mcp list`를 실행하면 연결을 추가, import, 편집, 제거, 동기화,
복원할 수 있으며, 선택한 연결의 상세 정보가 목록 옆에 표시됩니다. 연결 목록은 인자, 헤더, 환경 변수
값을 숨기며 URL 쿼리를 생략합니다. 키는 화면 하단에 표시됩니다.

`mcp edit`, `mcp remove`, `mcp restore`는 이름이나 백업 ID가 생략된 경우 선택 메뉴를
제공합니다. 편집기는 command/URL, 인자, 환경 변수, HTTP 헤더, bearer-token 환경 참조,
수신 target, [도구 정책](#tool-policy)(**Tools**)을 다룹니다. 인자는 줄당 하나의 리터럴 인자 또는 JSON 배열로 입력할 수
있습니다. 전송 방식을 전환하면 새 연결 유형에 적용되지 않는 필드는 지워집니다.

Add, edit, remove, import는 **Save and sync** 또는 **Save only** 전에 미리보기를
표시합니다. Remove는 `--keep-files`와 같은 **Stop managing**도 제공합니다. Escape를 누르면 대기 중인 초안이 취소됩니다. Restore는 Agent 항목에 대한
변경 사항을 미리보고 확인하지만, source 정의는 다시 작성하지 않습니다.

서버 이름 없이 import하면 여러 항목을 선택할 수 있습니다. 유효하지 않은 후보는 건너뛰며, `--replace`를 지정하지 않는 한 기존 source 이름은
건너뜁니다. 배치 전체에 대해 호환되는 하나의 수신 client 집합을 선택하세요. 전체 배치는
source가 한 번 저장되기 전에 검증되며, 이후의 네이티브 파일 I/O 실패는 기존 복구 동작을
유지합니다.

스크립트에서는 이름과 flag를 제공하세요. `mcp edit NAME --url URL`,
`mcp edit NAME --target CLIENT`, `mcp edit NAME -- command args...`는 다른 해당 설정을
유지하면서 지정된 필드만 업데이트합니다. `--sync`가 추가되지 않는 한 저장만 합니다.
`--no-tui`를 사용하면 remove는 이름이, restore는 백업 ID가 필요합니다. `--dry-run`은
변경 사항을 저장하거나 동기화하지 않습니다.

## Source fields

인라인 `mcp.servers` 또는 `sources.mcp`로 지정한 외부 파일 중 하나를 선택하세요.
외부 파일에는 최상위 `servers` 매핑이 있습니다. `mcp.targets`와
[`mcp.projects`](#manage-several-projects-from-the-global-config)는 Skillshare 설정에
남아 있습니다. 스키마는 저장소의 `schemas/mcp.schema.json`입니다.

| Server field | Meaning |
|---|---|
| `command` | 로컬 실행 파일; `url`과 함께 사용 불가 |
| `args` | 로컬 실행 파일용 리터럴 인자 목록 |
| `env` | 로컬 환경 값: 문자열 또는 `{fromEnv: VARIABLE}` |
| `url` | HTTP(S) MCP 엔드포인트; 자격 증명이나 fragment 포함 불가 |
| `headers` | HTTP 헤더: 문자열 또는 `{fromEnv: VARIABLE}` |
| `bearerToken` | `{fromEnv: VARIABLE}`; Authorization 헤더와 공존 불가 |
| `transport` | 선택적으로 `stdio` 또는 `streamable-http`; 생략 시 추론됨 |
| `targets` | 선택적 수신 client; `mcp.targets`를 재정의. 빈 목록이면 서버를 Skillshare에만 유지합니다. [아래](#keep-a-server-without-syncing-it) 참고 |
| `tools` | 어떤 도구가 모델에 전달되는지: `allow`, `deny`. 한 번 작성하면 Agent별로 변환됩니다. [도구 정책](#tool-policy) 참고 |
| `piOptions` | Pi 내장 MCP의 그 밖의 서버별 필드. [Pi](#pi-options) 참고 |
| `disabled` | `true`만 가능, 다른 연결 필드 불가, 그리고 project가 scope 안에 있어야 합니다: project mode이거나 `mcp.projects` 아래의 root. [아래](#turn-off-a-global-server-in-one-project) 참고 |

Client ID는 `claude`, `codex`, `cursor`, `vscode`, `opencode`, `kilocode`,
`grok`, `antigravity`, `amp`, `claude-desktop`, `cline`, `copilot`, `factory`, `gemini`,
`goose`, `junie`, `kiro`, `lmstudio`, `warp`, `windsurf`, `pi`입니다.
`grok`은 공식 xAI Grok CLI를 의미합니다. 서버 이름은 문자, 숫자, 점, 밑줄, 하이픈을
사용합니다. 서버는 동기화 전에 직접 또는 `mcp.targets`를 통해 최소 하나의 client를
선택해야 합니다. 단, 서버 자체의 `targets`가 빈 목록인 경우는 예외입니다.

### Keep a server without syncing it {#keep-a-server-without-syncing-it}

`targets: []`인 서버는 Skillshare source에 남아 있으며 어떤 client에도 작성되지 않습니다.
정의는 나중을 위해 유지하면서 서버를 모든 client에서 빼고 싶을 때 사용하세요.
이전에 동기화된 적이 있다면, 다음 sync에서 해당 client의 항목이 제거됩니다.

```yaml
mcp:
  targets: [claude, codex]
  servers:
    docs:
      url: https://example.com/mcp
      targets: []
```

```bash
skillshare mcp add docs --url https://example.com/mcp --target none
skillshare mcp edit docs --target none
skillshare mcp edit docs --target claude   # 다시 되돌리기
```

- **`targets`를 생략하는 것은 다릅니다.** 그 경우 서버는 `mcp.targets`를 상속하며,
  해당 목록도 비어 있으면 거부됩니다.
- `none`은 client와 함께 지정할 수 없습니다.
- 터미널 선택 메뉴에서는 client를 선택하지 않은 채로 확인하세요. 대시보드에서는 모든
  client의 체크를 해제하세요. 서버에 **아직 Agent 없음** 태그가 붙습니다.
- project의 서버와 `mcp.projects` 아래의 서버에도 동일하게 동작합니다.
- `disabled` 항목은 어딘가에서 서버를 꺼야 하므로 여전히 최소 하나의 client가
  필요합니다.
- `mcp list`는 이러한 서버를 `kept no targets`로 표시합니다.

Grok의 경우, 이름은 문자나 밑줄로 시작해야 하고, 문자·숫자·하이픈·단일 밑줄만 포함할
수 있으며, 밑줄로 끝날 수 없습니다. `company-docs`와 같은 이름은 지원되는 모든
client에서 동작합니다.

## Native destinations {#native-destinations}

| Client | Global | Project | Section |
|---|---|---|---|
| Claude Code | `~/.claude.json` | `.mcp.json` | `mcpServers` |
| Codex | `~/.codex/config.toml` | `.codex/config.toml` | `mcp_servers` |
| Cursor | `~/.cursor/mcp.json` | `.cursor/mcp.json` | `mcpServers` |
| VS Code | User `mcp.json` (below) | `.vscode/mcp.json` | `servers` |
| OpenCode | `~/.config/opencode/opencode.json` | `opencode.json` | `mcp` |
| Kilo Code | `~/.config/kilo/kilo.jsonc` | `kilo.jsonc` | `mcp` |
| Grok CLI | `~/.grok/config.toml` | `.grok/config.toml` | `mcp_servers` |
| Antigravity (AGY) | `~/.gemini/config/mcp_config.json` | `.agents/mcp_config.json` | `mcpServers` |
| [Amp](https://ampcode.com/docs/customize/mcp) | `~/.config/amp/settings.json` | `.amp/settings.json` | `amp.mcpServers` (literal key) |
| [Claude Desktop](https://modelcontextprotocol.io/docs/develop/connect-local-servers) | Claude application data directory, `claude_desktop_config.json` | Global only | `mcpServers` |
| [Cline](https://github.com/cline/cline/tree/main/apps/vscode/src/services/mcp) | `~/.cline/data/settings/cline_mcp_settings.json` | Global only | `mcpServers` |
| [Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers) | `~/.copilot/mcp-config.json` | `.github/mcp.json` | `mcpServers` |
| [Factory Droid](https://docs.factory.ai/harness/mcp) | `~/.factory/mcp.json` | `.factory/mcp.json` | `mcpServers` |
| [Gemini CLI](https://geminicli.com/docs/tools/mcp-server/) | `~/.gemini/settings.json` | `.gemini/settings.json` | `mcpServers` |
| [Goose](https://block.github.io/goose/docs/guides/config-files/) | `~/.config/goose/config.yaml` | Global only | `extensions` (YAML) |
| [Junie](https://junie.jetbrains.com/docs/junie-cli-mcp-configuration.html) | `~/.junie/mcp/mcp.json` | `.junie/mcp/mcp.json` | `mcpServers` |
| [Kiro](https://kiro.dev/docs/mcp/configuration/) | `~/.kiro/settings/mcp.json` | `.kiro/settings/mcp.json` | `mcpServers` |
| [LM Studio](https://lmstudio.ai/docs/app/mcp) | `~/.lmstudio/mcp.json` | Global only | `mcpServers` |
| [Warp](https://docs.warp.dev/agents/capabilities/mcp/) | `~/.warp/.mcp.json` | `.warp/.mcp.json` | `mcpServers` |
| [Windsurf (Cascade)](https://docs.devin.ai/desktop/cascade/mcp) | `~/.codeium/windsurf/mcp_config.json` | Global only | `mcpServers` |

대시보드의 서버 폼은 `fromEnv` 참조를 포함해 HTTP 헤더를 환경 변수와 동일한 방식으로
편집합니다. 서버 메뉴와 폼의 파일 개수 옆에 있는 **View what each Agent gets**는,
선택한 client에 대해 Sync가 작성할 네이티브 텍스트를 읽기 전용으로 보여줍니다. 폼에서는
아직 저장되지 않은 편집 내용이 반영됩니다. 시크릿은 참조 형태로 유지됩니다.

JSON 항목은 파일 자체의 들여쓰기에 맞춰 필드마다 한 줄씩 작성됩니다. Skillshare가
소유하지만 여전히 한 줄에 있는 항목은 `update`로 보고되며 다시 레이아웃되어 작성됩니다.
소유하지 않는 항목과 사람이 직접 서식을 지정한 항목은 레이아웃이 유지됩니다.

대시보드는 현재 scope와 호스트 플랫폼에서 사용 가능한 대상만 제공합니다. 각 서버는
한 행이며, 이름 아래에 전달될 client가 chip으로 표시됩니다. 오른쪽의 카운트 버튼은 해당 서버의 전체 client 목록을 엽니다. Global 전용
client는 project mode에서 선택할 수 없습니다. 오른쪽의 **Sync** 박스는 아직 작성되지
않은 변경 사항을 나열합니다: client를 체크하면 source만 편집됩니다. **Sync MCP**는 그
변경 사항을 나열하고, 확인하면 MCP 설정 파일만 작성하며 각 파일의 백업을 남깁니다. 같은 박스의 구분선 아래에서는 서버가 있으면
**검사**로 [서버를 검사](#check-servers-before-an-agent-starts-them)하고, **백업 및 복원**로 그 백업을 살펴볼 수 있습니다.
그 아래의 **Agents**는 이 머신에서 감지된 client를
나열합니다. client의 MCP 파일이 존재하거나, 해당 client가 설정을 보관하는 폴더가
존재하면 감지된 것으로 간주하므로, MCP 파일이 아직 없는 새 설치도 표시됩니다. project
mode에서는 프로젝트에 MCP 파일이 있거나 client가 전역적으로 감지된 경우 나열됩니다.

추가 client 세부 정보:

- `codex` 대상은 Codex CLI, Codex IDE 확장, ChatGPT 데스크톱 앱이 공유하는 하나의
  `config.toml`이므로, `codex`로 동기화된 서버는 셋 모두에 나타납니다. ChatGPT
  데스크톱 앱은 **Settings → MCP servers**에 이를 나열합니다. Codex는 신뢰하는
  프로젝트에서만 `.codex/config.toml`을 읽습니다. 신뢰하지 않는 프로젝트에서는
  동기화된 서버가 오류 없이 로드되지 않습니다. `cwd`, `http_headers_helper`, 승인
  모드, 타임아웃, `oauth` 테이블은 이식 가능한 형태가 없습니다: import는
  이를 경고와 함께 제외하고, sync는 기존 항목에 그대로 유지합니다. `enabled_tools`와
  `disabled_tools`는 [도구 정책](#tool-policy)에서 만들어지며, import 시 도구 정책으로
  가져옵니다. Codex 플러그인이
  번들하는 MCP 서버는 `plugins.<plugin>.mcp_servers` 아래에 구성되며 여기서 관리되지
  않습니다.
- Claude Desktop 파일 동기화는 macOS와 Windows에서 **stdio만** 지원합니다.
  디렉터리는 macOS에서 `~/Library/Application Support/Claude`, Windows에서
  `%APPDATA%/Claude`입니다. 원격 커넥터는 애플리케이션에서 구성하세요.
- Cline은 기본 VS Code Stable 프로필을 대상으로 하며, Cline CLI나 다른 IDE는
  대상이 아닙니다.
- Copilot CLI 항목에는 `tools`가 작성됩니다: [도구 정책](#tool-policy)이 허용하는
  정확한 도구 이름이며, 그렇지 않으면 `["*"]`입니다. import는 `tools`를 다시 도구
  정책으로 읽어옵니다. 프로젝트 `.mcp.json`이 존재하면, Copilot이 `.github/mcp.json`보다 그 파일을 먼저
  읽기 때문에 sync가 중단됩니다. 먼저 파일을 통합하세요. project mode에서 Claude
  Code와 Copilot CLI를 함께 선택하는 것도 파일을 작성하기 전에 차단됩니다. 이 중 하나의
  client에는 global mode를 사용하세요.
- Gemini는 Streamable HTTP에 `httpUrl`을 사용합니다. `url` 필드는 레거시 SSE를
  의미하며 import 시 거부됩니다. Cline은 `type: streamableHttp`를, Goose는
  `type: streamable_http`와 `uri`를 사용합니다. Skillshare가 이를 자동으로 변환합니다.
- Windows용 Goose는 `%APPDATA%/Block/goose/config/config.yaml`을 사용합니다. YAML
  편집은 관련 없는 설정, 주석, 내장 확장을 유지하지만 서식이 바뀔 수 있습니다.
  Alias, merge, 중복 키, 다중 문서는 편집을 차단합니다. 내장 확장과 keychain
  `env_keys`는 이식 가능한 MCP 연결로 import할 수 없습니다.
- Claude Code는 내장 서버용으로 예약된 이름인 `workspace`, `claude-in-chrome`,
  `computer-use`라는 이름의 서버를 건너뜁니다. 또한 자체 자격 증명을 원격 서버로
  전송하지 않습니다: `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`,
  `AWS_BEARER_TOKEN_BEDROCK`, `HTTPS_PROXY`, `NPM_TOKEN`은 `url`과 `headers`에서
  빈 값으로 읽힙니다. Skillshare는 Claude에 대해 둘 다 거부합니다. 자격 증명은
  자체적으로 이름을 지은 변수에 복사하세요.
- Claude Code는 로컬 scope도 가지고 있습니다: `--scope` 없이 `claude mcp add`로
  추가된 서버는 프로젝트별로 `~/.claude.json`에 저장됩니다. 로컬 서버는 `.mcp.json`
  이나 user scope에 있는 동일한 이름의 서버보다 항상 우선합니다. project mode에서
  Skillshare는 이러한 서버를 그것이 가리는 항목 옆에 표시하며, sync를 막지는
  않습니다. 프로젝트 폴더에서 `claude mcp remove NAME -s local`로 제거하세요.
- Cline의 VS Code 확장, CLI, SDK는 `~/.cline/data/settings/`를 공유합니다. 확장은
  이전 VS Code `globalStorage` 파일을 한 번 그곳으로 옮긴 후 더 이상 읽지 않으므로,
  Skillshare는 `~/.cline/data`가 아직 존재하지 않을 때만 이전 파일에 씁니다.
  `CLINE_MCP_SETTINGS_PATH`, `CLINE_DATA_DIR`, `CLINE_DIR`은 이 순서로 존중됩니다.
- Windsurf 지원은 문서화된 Cascade 구성을 대상으로 합니다. Windsurf의 최신 Devin
  Local agent는 자체 `~/.config/devin/mcp_config.json`을 읽으며, Skillshare는 이를
  관리하지 않습니다. Warp 프로젝트 연결은 세션마다 Warp 내부에서 승인이 필요합니다.
- Amp는 `amp mcp approve <name>` 이후에만 프로젝트의 `.amp/settings.json`에서
  서버를 실행합니다. Global 서버는 승인이 필요 없습니다.
- Kiro는 "Mcp Approved Env Vars" 설정에 나열된 이름에 대해서만 `${VARIABLE}`을
  확장하며, localhost에 한해 `http://` URL을 허용합니다.
- VS Code는 `User/profiles/` 아래의 기본이 아닌 각 프로필마다 별도의 `mcp.json`을
  유지합니다. Skillshare는 기본 프로필의 파일을 관리합니다.

환경 참조는 Amp, Copilot CLI, Factory, Gemini CLI, Kiro에 대해 `${VARIABLE}`로,
Cline과 Windsurf에 대해 `${env:VARIABLE}`로 내보내집니다. Claude Desktop, Goose,
Junie, LM Studio, Warp는 네이티브 보간이 검증되지 않았기 때문에 현재 `fromEnv`와
`bearerToken` export를 거부합니다. 사용자 정의 자격 증명이 없는 연결을 사용하거나,
지원되는 경우 수신 client에서 인증하세요. Skillshare는 참조를 평문으로 해석하지
않습니다.

Antigravity는 원격 연결용 `serverUrl`을 포함한 현재 [공식 MCP 구성](https://antigravity.google/docs/mcp)을
사용합니다. Skillshare는 이식 가능한 `url`을 자동으로 변환합니다. 이전
`.gemini/antigravity/`와 `.gemini/antigravity-cli/` 구성 위치는 관리되지 않습니다.
Antigravity의 `fromEnv`와 `bearerToken` export는 문서화된 구성에 환경 보간이
명시되어 있지 않기 때문에 차단됩니다. 사용자 정의 시크릿 헤더가 필요 없는 연결을
사용하고, 지원되는 OAuth 로그인을 Antigravity 내부에서 완료하세요. Skillshare는
참조를 평문 자격 증명으로 확장하지 않습니다.

OpenCode는 global 디렉터리에 대해 `XDG_CONFIG_HOME`을 존중합니다. 기존
`opencode.jsonc`는 `opencode.json`을 생성하는 대신 사용됩니다. 프로젝트에서는
OpenCode가 `.opencode/`에서도 두 이름을 모두 읽으므로, 그곳에 둔 파일이 있으면
Skillshare는 그 파일에 쓰며, 새 파일은 프로젝트 루트에 생성됩니다. 둘 이상
존재하면 동기화 전에 통합하세요. 사용자 지정 OpenCode 구성 경로, 디렉터리
오버라이드, 인라인 구성, 상속된 상위 파일은 관리되지 않습니다. 이들은 OpenCode에서
선택한 대상을 재정의할 수 있습니다.

Kilo Code는 OpenCode와 동일한 형식을 사용합니다. 프로젝트 루트와 `.kilo/`에서
`kilo.jsonc`와 `kilo.json`을 읽고 병합하므로, Skillshare는 이미 존재하는 파일에
쓰며 아무것도 없을 때만 `kilo.jsonc`를 생성합니다. 둘 이상 존재하면 동기화 전에
통합하세요. `KILO_CONFIG`, `KILO_CONFIG_DIR`, 이전 VS Code 확장의
`mcp_settings.json`은 관리되지 않습니다.

Kilo Code는 프로젝트 구성을 신뢰할 수 없는 것으로 취급합니다. 그곳에서는
`{env:VARIABLE}` 참조를 허용하지 않으며, 그런 참조를 발견하면 프로젝트 파일 전체를
무시합니다. 따라서 project mode에서 Skillshare는 `fromEnv`나 `bearerToken`을
사용하는 Kilo Code 서버를 거부합니다. 참조가 허용되는 global mode에서 해당 서버를
정의하세요.

OpenCode와 Kilo Code는 `local`/`remote` 유형과 `{env:VARIABLE}` 참조를 사용하며,
Grok은 `${VARIABLE}` 참조를 사용합니다. Skillshare가 이를 자동으로 변환합니다.
Claude의 `"type": "streamable-http"`는 HTTP로 import됩니다. 비활성화된 연결은
import를 차단합니다. Codex의 `startup_timeout_sec`나 `envFile`처럼 이식 가능한
대응 항목이 없는 다른 네이티브 옵션은 경고와 함께 import에서 제외되며, sync는
Agent의 기존 항목에 이를 유지합니다. Pi는 내장 MCP를 사용합니다.
[아래](#pi)를 참고하세요.

VS Code Stable의 기본 사용자 파일은 다음과 같습니다.

- macOS: `~/Library/Application Support/Code/User/mcp.json`
- Linux: `${XDG_CONFIG_HOME:-~/.config}/Code/User/mcp.json`
- Windows: `%APPDATA%/Code/User/mcp.json`

Global Claude, Codex, Grok, Copilot 경로는 `CLAUDE_CONFIG_DIR`, `CODEX_HOME`,
`GROK_HOME`, `COPILOT_HOME`을 존중합니다. `OPENCODE_CONFIG`와
`OPENCODE_CONFIG_DIR`은 관리되지 않습니다. Amp와 Goose는 `.config` 경로를 사용하는
플랫폼에서 `XDG_CONFIG_HOME`을 존중합니다.
Project 대상은 선택한 프로젝트 루트를 기준으로 합니다. 프로젝트 신뢰, 서버 승인,
인증은 여전히 수신 Agent의 책임입니다.

### Another account of an Agent {#accounts}

[Agent의 다른 계정](/docs/reference/targets/configuration#agent-config-dir)으로 선언된 target은 `claude`(`CLAUDE_CONFIG_DIR`), `codex`(`CODEX_HOME`), `pi`(`PI_CODING_AGENT_DIR`)에 대해 MCP target이기도 합니다. 그 서버는 해당 Agent의 형식으로, 계정 자체의 파일에 작성됩니다. Claude는 `<config_dir>/.claude.json`, Codex는 `<config_dir>/config.toml`, Pi는 `<config_dir>/mcp.json`입니다.

```yaml
targets:
  claude-work:
    agent: claude
    config_dir: ~/.claude-work

mcp:
  targets: [claude, claude-work]      # 두 계정 모두 모든 서버를 받음
  servers:
    docs:
      url: https://example.com/mcp
    jira:
      command: jira-mcp
      targets: [claude-work]          # 업무용 계정만
```

여기서 `docs`는 `~/.claude.json`과 `~/.claude-work/.claude.json`에 작성되고, `jira`는 두 번째 파일에만 작성됩니다. `--target claude-work`는 `mcp add`와 `mcp edit`에서 동작하며, 대시보드는 이 계정을 Agent 옆에 나열합니다.

모든 계정은 동일한 프로젝트 파일을 읽으므로, `mcp.projects` 안과 project mode에서는 Agent 자체의 이름을 사용하세요. Claude Code는 프로젝트의 off 목록을 각 계정의 파일에 유지합니다. [프로젝트에서 서버를 끄면](#turn-off-a-global-server-in-one-project) 해당 서버를 가진 모든 계정에 스위치가 작성됩니다. `mcp import --from claude-work`와 대시보드의 Import from target은 계정 자체의 파일을 읽습니다. `mcp import --file <path> --from claude-work`는 직접 내보낸 파일을 해당 계정의 Agent 형식으로 읽습니다.

## Turn off a global server in one project {#turn-off-a-global-server-in-one-project}

Agent는 자체 global MCP 파일과 프로젝트 파일을 함께 읽습니다. 따라서 global 파일에
정의된 서버는 모든 프로젝트에서 로드됩니다. 특정 프로젝트에서 로드되지 않도록
막으려면, **Agent의 global 파일이 사용하는 것과 동일한 이름**으로 항목을 추가하고
`disabled`로 표시하세요.

이는 다음 네 client에서만 동작합니다.

| Client | Supported | What Skillshare writes |
|---|---|---|
| Claude Code | 예 | `~/.claude.json`: 이름을 이 프로젝트의 `disabledMcpServers` 목록에 추가 |
| OpenCode | 예 | `opencode.json`: `"NAME": {"enabled": false}` |
| Kilo Code | 예 | `kilo.jsonc`: `"NAME": {"enabled": false}` |
| Pi | 예, `mcp.projects`에서 | `.pi/mcp.json`: `"NAME": {"command": "...", "enabled": false}`, 아래 참조 |
| Codex | 아니요 | 아래 참고 |
| Every other client | 아니요 | 하나를 선택하면 오류가 발생하며 아무것도 작성되지 않음 |

스위치만 작성됩니다. Agent는 global 항목의 command나 URL을 그대로 유지합니다. 다른
client는 전체 global 항목을 프로젝트 항목으로 대체하거나 프로젝트 파일이 없기
때문에 거부됩니다. 그런 경우 단독 스위치만으로는 서버를 끄는 대신 오히려 서버를
망가뜨리게 됩니다.

Codex는 다른 이유로 거부됩니다. Codex는 `.codex/config.toml`을 필드 단위로 global
파일 위에 병합하므로, global 구성이 해당 서버를 정의하는 머신에서는
`enabled = false`만으로도 동작합니다. 정의하지 않는 머신에서는 병합된 항목에
`command`나 `url`이 없어 Codex는 `invalid transport`로 전체 구성을 로드하지
못합니다. `.codex/config.toml`은 보통 커밋되므로, 한 팀원의 스위치가 다른 팀원의
Codex 시작을 막을 수 있습니다. 대신 `~/.codex/config.toml`에서 `enabled = false`로
머신별로 서버를 끄세요.

Pi는 같은 이름의 프로젝트 항목으로 global 항목을 통째로 대체합니다. `command`나 `url`이 없는 항목은
Pi 1.0.1 이전에는 건너뛰고, 1.0.1부터는 global 서버를 끄지만 global 구성에 그 서버가 없는 머신에서는
Pi가 시작할 때마다 경고합니다. 그래서 Pi에는 global 서버의 `command`, 또는 query를 뺀 `url`을 `enabled: false`와 함께
씁니다. 꺼진 서버는 시작되지 않으므로 args, env, headers는 프로젝트 파일에 쓰지 않으며, 다른
프로젝트는 그 서버를 그대로 사용합니다. sync할 때마다 이 항목은 global 서버를 기준으로 다시
작성됩니다. global 서버가 필요하므로 global 구성의 `mcp.projects` 아래 프로젝트에서만 동작합니다.
프로젝트 자체 구성에서는 global 서버를 볼 수 없으므로, 그곳의 `disabled` 항목에 `pi`를 쓰면
오류가 납니다.

### OpenCode and Kilo Code

```bash
cd my-project
skillshare mcp add company-docs --disabled --target opencode --target kilocode
skillshare sync mcp
```

```yaml
# .skillshare/config.yaml
mcp:
  servers:
    company-docs:
      disabled: true
      targets: [opencode, kilocode]
```

### Claude Code

Claude Code는 한 scope에서 전체 서버 항목을 가져오며 필드를 병합하지 않으므로,
`.mcp.json`의 스위치는 서버를 끄는 대신 대체해 버립니다. Claude Code는 `/mcp` 패널이
편집하는, `~/.claude.json`에 있는 자체 프로젝트별 off 목록을 유지합니다. Skillshare는
이 프로젝트의 절대 경로 아래에 이름을 추가하며, `.mcp.json`에는 아무것도 쓰지
않습니다.

```bash
skillshare mcp add company-docs --disabled --target claude
skillshare sync mcp
```

- 이 목록은 저장소가 아니라 여러분의 머신에 있습니다. 각 팀원은 자신의 체크아웃에서
  `skillshare sync mcp`를 한 번 실행합니다.
- `/mcp`에서 직접 끈 이름은 절대 가져오거나 제거되지 않습니다.
- `/mcp`에서 서버를 다시 켜면, 다음 sync에서 충돌이 보고됩니다.
  `.skillshare/config.yaml`에서 항목을 제거하거나, replace하여 다시 끄세요.
- 목록은 프로젝트의 경로로 키가 지정되므로, 프로젝트를 이동하면 새 sync가
  필요합니다.

### Rules

- **project가 scope 안에 있어야 합니다.** `skillshare init -p`로 생성된
  `.skillshare/config.yaml`이 있는 프로젝트 안에서 실행하거나, `-p`를 전달하거나,
  항목을 [`mcp.projects`](#manage-several-projects-from-the-global-config)의 프로젝트
  root 아래에 두세요. project가 scope에 없는 global `mcp.servers`에서는 거부됩니다.
- **`disabled`는 단독으로 사용됩니다.** 항목은 `targets`만 받습니다. `command`, `url`,
  `env`, `headers`, `piOptions`, `tools`를 추가하면 오류입니다.
- **`targets`는 생략할 수 있습니다.** 그러면 항목은 프로젝트의 target을
  따릅니다: sync할 때마다 프로젝트가 사용하는 client 중 프로젝트별 스위치가 있는
  client에 작성됩니다. Skillshare가 같은 이름의 global 서버도 알고 있는
  `mcp.projects` 아래에서는 그 서버가 작성되는 client로 더 좁혀집니다. 나중에 프로젝트의 target을 변경해도 항목을
  수정할 필요가 없습니다. 직접 정하려면 `targets`를 명시하세요. 그 목록에 지원되지
  않는 client가 있으면 오류입니다.
- **이름이 일치해야 합니다.** Skillshare는 Agent의 global 파일을 읽지 않으므로,
  이 이름의 서버가 그곳에 존재하는지 확인할 수 없습니다. 아무것도 일치하지 않는
  이름은 문제가 되지 않습니다: Agent가 이를 무시할 뿐입니다.
- **다시 켜려면**, 항목을 제거하고(`skillshare mcp remove company-docs`) sync하세요.
  스위치가 작성된 파일에서 제거됩니다: 프로젝트 자체 파일, 또는 Claude Code의 경우
  `~/.claude.json`.
- **Skillshare가 직접 정의하는 서버는 이것이 필요 없습니다.** 대신 해당 서버에서
  Agent 선택을 해제하면, 다음 sync에서 그 항목이 제거됩니다.

대시보드에서는 **전역 서버 끄기** 버튼이 이에 해당합니다. project mode에서는 **서버** 제목 옆에,
프로젝트의 MCP 탭에서는 **서버 추가** 옆에 있습니다.

## Manage several projects from the global config {#manage-several-projects-from-the-global-config}

Project mode는 각 프로젝트의 MCP 설정을 해당 프로젝트의 `.skillshare/config.yaml`에
보관하며, 그 폴더 안에서 sync합니다. 모든 프로젝트를 한곳에서 관리하고 싶다면,
**global** 설정의 `mcp.projects` 아래에 프로젝트 폴더를 나열하세요. 그러면 어디서든
`skillshare sync mcp`를 한 번 실행하는 것만으로 global 파일과 모든 프로젝트의 파일이
하나의 plan으로 작성됩니다.

```yaml
# ~/.config/skillshare/config.yaml
mcp:
  servers:
    context7:
      command: npx
      args: ["-y", "@upstash/context7-mcp"]
      targets: [claude, opencode]
  projects:
    ~/work/project01:
      targets: [claude, opencode]
      servers:
        context7:                  # 이 프로젝트에서만 꺼짐
          disabled: true
    ~/work/project02:
      servers:
        internal-docs:             # 이 프로젝트에만 존재
          url: https://example.com/mcp
          targets: [opencode]
```

프로젝트에는 global 설정과 다른 부분만 나열합니다. `context7` 같은 global 서버는
여기에 항목이 필요 없습니다. Agent는 자체 global 파일과 프로젝트 파일을 함께 읽으므로,
이미 모든 프로젝트에서 로드됩니다. `disabled` 항목은 그곳에 나열된 client에 대해
[그 폴더에서 서버를 끕니다](#turn-off-a-global-server-in-one-project).

각 키는 프로젝트 폴더입니다: 절대 경로이거나 `~`로 시작하는 경로입니다. 그 아래에는
해당 프로젝트의 자체 `config.yaml`이 `mcp` 아래에 담았을 것과 동일한 `targets`와
`servers`가 들어가며, 동일한 [프로젝트 파일](#native-destinations)에 작성됩니다.
`targets`가 없는 프로젝트는 global `mcp.targets`를 상속합니다.

하나의 서버가 둘 이상의 위치에 나타나면 미리보기에 파일 이름이 표시됩니다:

```text
context7     add          opencode (~/.config/opencode/opencode.json)
context7     add          opencode (~/work/project01/opencode.json)
```

목록에서 프로젝트를 제거하면, 서버를 제거할 때와 마찬가지로 Skillshare가 그곳에
작성한 항목이 다음 sync에서 제거됩니다.

여러 프로젝트에 같은 서버를 제공하려면, YAML anchor로 한 번 정의하고 재사용하세요:

```yaml
mcp:
  projects:
    ~/work/project01:
      servers:
        internal-docs: &internal-docs
          url: https://example.com/mcp
          targets: [opencode]
    ~/work/project02:
      servers:
        internal-docs: *internal-docs
```

anchor는 `mcp.projects` 안에 두세요. `mcp.servers`에 있는 anchor를 가리키는 alias도
동작하지만, `skillshare mcp add`와 대시보드는 `mcp.servers`를 다시 씁니다. 저장할 때
파일이 유효하게 유지되도록 그런 alias를 전체 내용으로 풀어서 작성하므로, 이후 global
서버를 수정해도 더 이상 따라가지 않습니다.

### Projects in the dashboard {#projects-in-the-dashboard}

Global mode에서는 대시보드에 **프로젝트** 페이지가 있습니다. 이 페이지는
[`projects`](/docs/reference/targets/configuration#projects)와 `mcp.projects` 아래의 모든 폴더를 나열하며, 각
프로젝트에는 **MCP** 탭이 있습니다.

![프로젝트 MCP 탭: 프로젝트별로 켜고 끄는 전역 server와 프로젝트 전용 server](/img/projects-mcp-tab.png)

- **프로젝트 추가**는 폴더와 그 target을 받습니다. **MCP**를 체크하면 해당 폴더가
  `mcp.projects` 아래에도 나열됩니다.
- **MCP** 탭은 모든 global 서버를 스위치와 함께 나열합니다. 하나를 끄면
  `targets` 없이 `disabled` 항목이 저장되므로,
  [위](#turn-off-a-global-server-in-one-project)에서 설명한 대로 프로젝트의
  target을 따릅니다. 다시 켜면 그 항목이 제거됩니다. 그 아래에는 해당
  프로젝트에만 존재하는 서버가 있습니다.
- 꺼져 있는 서버는 꺼져 있는 Agent의 로고를 표시합니다. 프로젝트의 Agent 중 하나에
  프로젝트별 스위치가 없으면, 해당 행은 그 Agent에서는 서버가 여전히 로드된다고
  알려줍니다. 프로젝트의 target과 다른 자체 `targets`를 나열한 항목에는
  **프로젝트에 맞추기**가 표시되며, 이는 `targets` 없이 항목을 다시 저장합니다.
- 탭의 Sync 박스에 있는 **Sync MCP**는 전체 MCP 계획을 작성하며, 그 변경 사항 중
  몇 개가 이 프로젝트 밖에 있는지 알려줍니다. 프로젝트 페이지 상단의 **Sync project**는
  이 프로젝트의 skill, agent, MCP만 작성합니다.
- MCP 페이지 하단의 **기본값**은 `mcp.targets`를 편집합니다.
- 프로젝트 자체의 Agent 파일에 Skillshare가 관리하지 않는 서버가 있으면, 탭은 목록 위에
  **Import**와 함께 이를 알려줍니다. [아래](#unmanaged-servers) 참고.

저장하면 변경한 프로젝트만 다시 씁니다. 다른 프로젝트는 anchor와 alias를 포함해
YAML이 작성된 그대로 유지되며, `~/work/app`으로 작성된 폴더는 `~`를 그대로
유지합니다. 이 페이지의 다른 곳과 마찬가지로 저장은 `config.yaml`만 변경하며, 파일은
Sync가 작성합니다.

제한 사항:

- `mcp.projects`는 global 설정에서만 읽습니다. 이를 포함한 프로젝트 설정은
  거부됩니다.
- 이를 편집하는 명령은 없습니다. `skillshare mcp add`는 `mcp.servers`를 관리하며
  `mcp.projects`는 작성된 그대로 둡니다. `config.yaml`에서 직접 편집하거나
  [대시보드](#projects-in-the-dashboard)에서 편집하세요.
- Claude Code에 대한 `disabled` 항목은 global 서버가 작성되는 것과 같은 파일인
  `~/.claude.json`에 작성됩니다. Claude Code가 프로젝트별 off 목록을 그곳에 보관하기
  때문입니다. 서버 자체는 그대로 둡니다.
- 폴더에 같은 항목을 관리하는 자체 `.skillshare/config.yaml`도 있으면, plan은 이를
  덮어쓰지 않고 충돌로 보고합니다.

## Agent가 시작하기 전에 서버 검사하기 {#check-servers-before-an-agent-starts-them}

```bash
skillshare mcp check
skillshare mcp check docs github --json
skillshare mcp check --no-dns
```

`mcp check`는 source의 모든 서버 또는 지정한 서버에 대해 "sync된 그대로 동작할까?"에
답합니다. global 설정에서는 [`mcp.projects`](#manage-several-projects-from-the-global-config)
아래 각 root의 서버도 검사하며, 각 Agent의 규칙과 sync 상태는 해당 root 기준으로 읽습니다.
읽기 전용이므로 [`--live`](#probe-servers-live)를 추가하지 않는 한 서버를 시작하거나, HTTP 요청을 보내거나,
명령을 실행하거나, 파일을 쓰지 않습니다.

| 검사 | 수준 |
|---|---|
| `env`, `headers`, `bearerToken`의 `fromEnv` 변수가 설정되지 않았거나 비어 있음 | error |
| 로컬 서버의 `command`를 `PATH`에서 찾을 수 없음 (앞의 `~/`는 확장됨) | error |
| 원격 서버의 호스트가 DNS로 조회되지 않음 (제한 3초, `--no-dns`로 건너뜀) | warning |
| Agent의 규칙이 서버를 거부함 (예: Claude Code가 예약한 이름) | error |
| Agent의 항목이 source와 충돌함 (`sync mcp --dry-run`과 동일) | error |
| Agent의 항목이 아직 작성되지 않았거나 업데이트되지 않음 | warning |
| 서버가 `targets: []`를 가지며 Skillshare에만 보관됨 | info |
| 선택한 Agent가 서버의 [도구 정책](#tool-policy) 일부를 담을 수 없음 | warning |

변수 값은 출력되지 않습니다. error가 하나라도 있으면 1로, 그렇지 않으면 0으로 종료하며,
warning으로는 실패하지 않습니다. 알 수 없는 서버 이름은 error이며 알려진 이름 목록을 보여줍니다.
이름은 global과 각 프로젝트에서 그 이름을 가진 모든 서버를 선택하며, 알려진 이름에는 프로젝트
서버도 포함됩니다.

터미널에서는 프로젝트 서버의 제목에 해당 프로젝트가 표시됩니다:

```text
✓ docs
  · claude: in sync
✗ docs  (project ~/work/app)
  ✗ command no-such-mcp-binary was not found on PATH
  ! claude: not synced yet; run skillshare sync mcp
```

`--json`을 사용하면 보고서는 다음 형태입니다:

```json
{
  "servers": [
    {
      "name": "docs",
      "ok": false,
      "findings": [
        { "level": "error", "check": "env", "target": "", "message": "bearerToken reads DOCS_TOKEN, which is not set", "subject": "DOCS_TOKEN" },
        { "level": "warning", "check": "sync", "target": "claude", "message": "not synced yet; run skillshare sync mcp" }
      ]
    },
    {
      "name": "docs",
      "project": "/home/me/work/app",
      "ok": true,
      "findings": [
        { "level": "info", "check": "sync", "target": "claude", "message": "in sync" }
      ]
    }
  ],
  "summary": { "errors": 1, "warnings": 1 }
}
```

`check`는 `env`, `command`, `url`, `dns`, `client-rule`, `sync`, `targets`,
`tools`, `live` 중 하나입니다.
`target`은 Agent 또는 계정을 나타내며, 결과가 서버 자체에 관한 것이면 비어 있습니다.
`subject`는 `env`, `command`, `dns` 결과에서 변수, 명령, 호스트를 나타내고, 성공한 `live` 프로브에서는
서버가 보고한 이름을, `live` 로그인 warning에서는 resource metadata URL을 나타내며, 그 외에는 생략됩니다.
`project`는 서버의 `mcp.projects` root를 절대 경로로 나타내며 (앞의 `~`는 확장됨), global 서버에서는
생략됩니다. `summary`는 프로젝트 서버를 포함해 보고서의 모든 서버를 집계합니다.

대시보드에서는 MCP 페이지 Sync 박스의 **검사** 버튼이 같은 검사를 실행합니다. 검사할 서버가 있을 때
나타나고, 클릭할 때만 실행되며, 서버 목록 위에 요약을, 각 서버 아래에 해당 error나 warning을 표시하고,
페이지를 새로 고치면 아무것도 남기지 않습니다. MCP 페이지는 global 서버만 나열하므로, 요약과 각 행에는
global 서버와 이름이 같은 것이라도 프로젝트 서버가 포함되지 않습니다. 프로젝트의 MCP 탭에는 Sync 박스에
자체 **검사**가 있어 해당 프로젝트 자체 서버를 보고합니다. 변수는 `skillshare ui`를 시작한
터미널에서 읽습니다.

### 서버를 실제로 프로브하기 {#probe-servers-live}

```bash
skillshare mcp check --live
skillshare mcp check docs --live --timeout 30s --json
```

`--live`는 먼저 정적 검사를 실행한 뒤, 선택한 서버 중 error가 없는 각 서버에 접속합니다.
error가 있는 서버나 비활성화된 항목에는 접속하지 않으며, `info` 결과가 그 이유를 알려줍니다.

- **로컬 (stdio) 서버.** Skillshare는 현재 환경에 서버의 `env`를 더하고, 각 `fromEnv` 값은
  셸에서 읽어 `command`를 `args`와 함께 시작합니다. 프로젝트 서버는 해당 프로젝트 폴더에서,
  global 서버는 현재 디렉터리에서 시작합니다. 이는 Agent가 하는 것처럼 서버의 코드를 사용자
  컴퓨터에서 실행하므로, 신뢰하는 서버에만 `--live`를 사용하세요. Skillshare는
  `server/discover`를 보냅니다. MCP 프로토콜 error가 아닌 error로 응답하거나 제한 시간의
  3분의 1 안에 응답하지 않는 서버는 MCP 2026-07-28보다 오래된 것으로 간주되어 대신
  `initialize` 핸드셰이크를 받습니다. 그런 다음 Skillshare는 `tools/list`를 호출해 도구 수를
  세고 서버를 중지합니다: stdin을 닫은 뒤, 서버의 프로세스 그룹에 SIGTERM, 이어서 SIGKILL을
  보냅니다. Windows에서는 프로세스를 종료합니다.
- **원격 (Streamable HTTP) 서버.** Skillshare는 서버의 `headers`와 `bearerToken`을 사용해
  `server/discover`를 POST하며, JSON 또는 SSE 응답을 읽습니다. MCP error가 없는 `400`,
  `404`, `405`는 `initialize`로 대체됩니다. `401`은 warning ("sign-in required")이며,
  `WWW-Authenticate` 헤더의 resource metadata URL을 함께 표시합니다. Skillshare는 로그인하거나
  OAuth를 시작하지 않습니다.

각 서버에는 프로브 전체에 대한 하나의 제한 시간이 있습니다: 10초, 또는 `--timeout` (예: `30s`나
`1m`). 최대 네 개의 서버를 동시에 프로브합니다. `--live` 없이 `--timeout`을 쓰면 error입니다.

| 결과 | 수준 |
|---|---|
| 서버가 응답함: 이름과 버전, 프로토콜 버전, 도구 수 | info |
| 원격 서버가 로그인을 요구함 (HTTP 401) | warning |
| 명령을 시작할 수 없거나, 일찍 종료되었거나, 제한 시간 안에 응답하지 않음 | error |
| 프로토콜 error, 지원되지 않는 프로토콜 버전, 또는 그 밖의 HTTP 상태 | error |

로컬 서버가 실패하면 메시지 끝에 해당 서버 stderr의 최대 다섯 줄이 붙습니다. `env`, `headers`,
`bearerToken`의 값은 모든 메시지에서 제거되며, 네 글자보다 짧은 값은 그대로 남습니다. 값은 작성된
그대로 전달됩니다: Skillshare는 Pi `!command` 값을 실행하지 않으며 `piOptions`를 읽지 않습니다.
종료 코드는 같은 규칙을 따르며, error가 하나라도 있으면 1입니다. `--live`는 파일도, 작업 로그
항목도 쓰지 않습니다.

`--json`을 사용하면 응답한 서버에는 `live` 객체도 추가됩니다:

```json
{
  "name": "docs",
  "ok": true,
  "findings": [
    { "level": "info", "check": "live", "target": "", "message": "responds: docs-server 1.4.0, protocol 2026-07-28, 12 tool(s)", "subject": "docs-server" }
  ],
  "live": { "protocolVersion": "2026-07-28", "serverInfo": { "name": "docs-server", "version": "1.4.0" }, "tools": 12 }
}
```

`serverInfo`는 서버가 스스로에 대해 밝힌 내용이며, 아무것도 이를 검증하지 않습니다. 서버를 프로브하지
않았거나 프로브가 실패하면 `live`는 생략됩니다.

대시보드의 **검사** 버튼은 정적 검사만 실행합니다. 대시보드가 서버를 프로브하는 곳은
한 군데뿐입니다: 서버 대화상자의 [도구 섹션](#tool-policy-dashboard)에 있는 **도구 불러오기**로,
대화상자의 현재 입력값으로 서버를 한 번 시작해 도구 목록을 가져옵니다. `--json`을 사용하면 `live`에는
`tools/list`가 반환한 이름인 `toolNames`도 들어갑니다.

## Stop managing a server {#stop-managing-a-server}

```bash
skillshare mcp remove docs --keep-files
```

source에서 `docs`를 제거하고, Skillshare가 이 서버를 위해 작성한 Agent 항목의 기록을
지웁니다. Agent 파일은 바뀌지 않습니다. 이후 그 항목은 사용자의 것이며, sync는 이를
제거하지도 업데이트하지도 않습니다. `--keep-files`는 `--sync`와 함께 쓸 수 없습니다.
터미널 remove wizard는 이를 **Stop managing**으로 제공하며, MCP 페이지와 프로젝트의
**MCP** 탭에 있는 대시보드 제거 대화상자도 마찬가지입니다.

제거한 범위만 바뀝니다. global 서버의 관리를 멈춰도 같은 이름의 프로젝트 서버는 계속
관리되며, 반대도 마찬가지입니다. 항목을 다시 관리하려면 import하세요.

## Servers Skillshare does not manage {#unmanaged-servers}

대시보드는 현재 범위의 Agent 설정 파일과 `mcp.projects` 아래 모든 폴더의 Agent 설정
파일에서, 이 source가 정의하지 않고 어떤 Skillshare 설정도 관리하지 않는 서버를 찾습니다.
찾으면 서버 목록 위의 안내가 몇 개인지, 어느 Agent에 있는지 알려줍니다. **Import**는 그중
첫 번째 Agent가 선택된 상태로 import를 엽니다. 프로젝트의 **MCP** 탭은 그 프로젝트 자체
파일에 대해 같은 안내를 표시하며, 그 import는 프로젝트의 파일을 읽어 서버를 그 프로젝트에
저장합니다. Goose의 내장 확장처럼 연결할 대상이 없는 항목은 세지 않습니다.

### Take over an entry an Agent already has

Agent 파일이 이미 사용하는 이름으로 서버를 추가하면, sync는 그 항목을 덮어쓰지 않습니다.
plan은 `existing entry is not managed` 충돌을 보고하고, 그 항목에 대해 선택할 때까지
파일을 쓰지 않습니다:

- 그 Agent에서 import: `skillshare mcp import NAME --from CLIENT`, 또는 대시보드에서
  충돌의 **Import from** 버튼(예: **Import from Cursor**). source와 일치하는 항목은
  그대로 채택됩니다. `mcp.projects` 아래 폴더의 충돌이라면 버튼은 그 폴더의 파일을 읽고
  해당 project로 가져옵니다.
- source 정의로 교체: 대시보드의 **Replace with source**, 또는 import의 `--replace`.

## Safety and limitations

- JSONC 주석과 관련 없는 설정은 보존됩니다. 변경된 소유 항목은 하나의 단위로
  교체되므로, 해당 항목 안의 주석은 바뀔 수 있습니다. Skillshare가 작성하는 필드만
  비교되고 교체되며, 타임아웃 같은 Agent 고유 필드는 유지됩니다.
  `"type": "stdio"`, 빈 `env`, 헤더 이름 대소문자처럼 Agent가 채우는 기본값은
  변경으로 간주되지 않습니다. `enabled: false`나 `disabled: true`로 관리되는 서버를
  끄는 것은 충돌로 보고됩니다.
  Pi는 예외입니다. `enabled`만 변경하면 소유권 충돌이 발생하지 않습니다. 동기화 시 source의 `piOptions.enabled`가 우선합니다.
- Claude Code가 `~/.claude.json`에서 하는 것처럼 Agent가 같은 파일의 관련 없는
  설정을 다시 쓰는 동안에도 미리보기는 유효하게 유지됩니다. 해당 파일의 MCP 항목이
  변경된 경우에만 새 미리보기가 필요합니다.
- Codex와 Grok 편집은 일반적인 `[mcp_servers.NAME]` 테이블과 그 하위 테이블을
  지원합니다. 업데이트된 항목은 제자리에 유지되며, CRLF 줄바꿈도 유지됩니다.
  인라인/점 표기 MCP 정의는 작성하기 전에 테이블로 변환해야 하며, 그렇지 않으면
  파일을 수정하지 않고 거부됩니다.
- 네이티브 파일 symlink, 손상된 파일, 중복 JSON 속성은 쓰기를 차단합니다.
  symlink된 Skillshare `config.yaml`은 그 대상으로 전달되어 작성됩니다. 파일
  권한은 보존되며, 새 네이티브 파일, 소유권 기록, 백업은 private 권한을 사용합니다.
- source와 이미 일치하는 항목은, 예를 들어 팀원의 변경 사항을 pull한 후처럼, 쓰기
  없이 unchanged로 보고됩니다. 어떤 Agent에서 import한 뒤 그 Agent를 선택한 경우처럼
  이 구성이 이전에 이를 관리하지 않았다면 plan에 `adopt`가 표시됩니다: sync는 파일을
  바꾸지 않고 항목을 관리 대상으로 기록하며, 이후 서버를 제거하거나 그 Agent 선택을
  해제하면 항목도 제거됩니다. Agent에서 직접 끈 서버는 계속 사용자의 것입니다.
  다른 unmanaged 항목은 import나 명시적인 항목별 replace가
  필요합니다. 소유하는 구성 파일이 여전히 존재하는 동안에는 다른 Skillshare
  구성의 소유권을 재정의할 수 없습니다. 그 파일이 사라진 경우에는 그 항목을 영원히
  해제할 수 없으므로, 충돌은 해당 항목이 남겨진 항목임을 알리고 그 파일의 이름을
  표시하며, 터미널에서든 대시보드의 충돌에서든 명시적인 import나 replace로 이를
  가져옵니다. 마운트되지 않은 드라이브에 있는 파일처럼 읽기만 불가능한 경우는
  여전히 소유자가 존재하는 것으로 간주됩니다.
- 대시보드의 MCP 설정은 브라우저가 `localhost`나 IP 주소로 대시보드를 열 때만
  동작합니다. 리버스 프록시를 포함한 도메인 이름을 통하면 MCP 요청은 403을
  반환합니다. DNS rebinding 공격은 항상 도메인 이름을 사용하기 때문입니다.
- 자격 증명은 환경 참조를 사용합니다. 시크릿 저장소, OAuth 세션 동기화, 지속적인
  상태 모니터링, 패키지 설치, 게이트웨이, 레지스트리, 플러그인 동기화는 없습니다.
  서버를 시작하거나 호출하는 명령은 `mcp check --live`뿐입니다.
- VS Code Insiders, 사용자 지정 프로필, 원격 워크스페이스, 레거시 SSE는 이 버전에서
  지원되지 않습니다.
- VS Code는 현재 `headers` 안에서 `${env:VARIABLE}`을 치환하지 않으므로
  ([microsoft/vscode#336232](https://github.com/microsoft/vscode/issues/336232)),
  VS Code로 동기화된 헤더와 `bearerToken` 참조는 해당 문제가 수정될 때까지 서버에
  해석되지 않은 채로 도달합니다.
- 로컬 작업 기록은 Skillshare state 디렉터리의 `mcp/` 아래에 존재합니다:
  `state.json`, 쓰기 중의 `pending.json`, 그리고 `backups/`(Agent 파일별 최신 20개).
  이 디렉터리를 이식 가능한 매니페스트로 공유하지 마세요.


## 도구 정책 {#tool-policy}

`tools`는 서버의 도구 중 어떤 것이 모델에 전달되는지 정합니다. 서버에 한 번만
작성하면, Skillshare가 sync할 때 각 Agent 고유의 필드로 변환합니다.

```yaml
mcp:
  servers:
    github:
      command: github-mcp
      targets: [pi, codex, copilot, opencode]
      tools:
        allow: [get_*, search_code, list_issues]
        deny: [get_secret]
```

```bash
skillshare mcp add github --target pi --target codex --tools-allow 'get_*,search_code' --tools-deny get_secret -- github-mcp
skillshare mcp edit github --tools-allow ''          # clear the allow list
skillshare mcp import github --from claude --target pi --tools-deny get_secret
```

| Field | Meaning |
|---|---|
| `allow` | 설정하면 일치하는 도구만 남습니다 |
| `deny` | 일치하는 도구를 제거하며, `allow`와 일치하더라도 제거됩니다 |

`allow`와 `deny`의 항목은 도구 이름이며, `*`는 임의의 문자와 일치합니다. 그 밖의
와일드카드(`? [ ] { }`), 공백, 쉼표는 거부되며, 같은 이름을 두 번 적어도 거부됩니다.
`allow`가 남긴 도구를 모두 제거하는 `deny` 목록은 오류입니다. `disabled` 항목에는
`tools`를 설정할 수 없습니다. 두 flag는 `mcp add`, `mcp edit`, `mcp import`에서
동작합니다. 목록은 쉼표로 구분하며, 빈 값은 해당 부분을 지웁니다. Pi가 도구를 어떻게
제공하는지는 정책에 포함되지 않습니다. 이는 Pi의 `exposure`이며 [`piOptions`](#pi-options)에서
설정합니다.

### 각 Agent가 받는 내용 {#tool-policy-agents}

모든 Agent가 정책의 모든 부분을 담을 수 있는 것은 아닙니다. Skillshare는 Agent의
문서화된 형식이 지원하는 부분을 작성하고 나머지는 알려줍니다. 어떤 부분도 조용히
버리지 않습니다.

| Agent | 작성되는 내용 | 적용되지 않는 부분 |
|---|---|---|
| [Pi](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/mcp.md) | `toolExposure`에는 거부된 도구를 `hidden`으로, 이어서 허용된 도구를, `allow`가 설정되어 있으면 마지막에 `"*": "hidden"`을 작성 | 없음 |
| [Codex](https://developers.openai.com/codex/config-reference) | `enabled_tools`와 `disabled_tools`, 정확한 이름만. Codex는 `enabled_tools` 다음에 `disabled_tools`를 적용합니다 | `allow`의 `*` 패턴; 정확한 `allow` 목록으로 합칠 수 없는 `deny`의 `*` 패턴 |
| [Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers) | `tools`: 허용된 정확한 이름에서 거부된 이름을 뺀 것, 그렇지 않으면 `["*"]` | `allow`의 `*` 패턴; `allow`가 정확한 이름을 나열하지 않을 때의 `deny`(Copilot에는 거부 목록이 없음) |
| [OpenCode](https://opencode.ai/docs/permissions/), [Kilo Code](https://kilo.ai/docs/code-with-ai/platforms/cli#permissions) | 없음 | 전부. 둘 다 서버 항목 밖의 최상위 `permission` 맵에서 `<server>_<tool>` 키로만 도구를 거릅니다 |
| 그 밖의 모든 Agent | 없음 | 전부 |

Pi에서는 정확한 도구 이름이 어떤 패턴보다 우선하므로, 거부된 패턴과 일치하는 허용된
정확한 이름은 `toolExposure`에서 빠집니다. 허용된 도구는 서버의 `piOptions.exposure`를 받으며,
그것이 설정되지 않았거나 `hidden`이면 Pi 기본값인 `codemode`를 받습니다. 따라서
`allow`와 함께 쓴 `hidden`은 허용된 도구만 보인다는 뜻입니다.

적용되지 않는 부분은 세 곳에 표시됩니다:

- sync plan에서, Agent마다 서버를 나열하는 warning 줄로:

  ```text
  ! tool policy not applied for opencode: allow, deny (github)
  ```

  `--json`을 사용하면 같은 텍스트가 plan의 `notices`에 들어갑니다.
- [`mcp check`](#check-servers-before-an-agent-starts-them)에서, 각 Agent에 대한
  `tools` warning으로.
- 대시보드에서, 서버 대화상자의 도구 섹션과 **각 Agent에 기록될 설정 보기**에서.
  대시보드는 이것들이나 아래의 폐지된 Pi 설정에 대해 페이지 수준 알림을 표시하지
  않습니다.

Codex의 `enabled_tools`와 `disabled_tools`는 관리되는 필드입니다: 정책을 지우면 함께
제거되며, Skillshare가 소유한 항목에서 직접 편집하면 충돌로 표시됩니다. import는
Codex의 `enabled_tools`/`disabled_tools`와 Copilot의 `tools`를 다시 `tools`로 읽어옵니다.
Pi의 `toolExposure`는 서버의 `exposure`와 함께 그 정책을 작성했을 때 정확히 같은
`toolExposure`가 나오는 경우에만 `tools`가 되며, 그렇지 않으면 warning과 함께 `piOptions`에
남습니다. `exposure`는 항상 `piOptions`에 남습니다.

### 대시보드의 도구 {#tool-policy-dashboard}

서버 대화상자에는 target 다음에 **도구** 섹션이 있으며, `disabled` 항목을 제외한 모든
서버에 표시됩니다. 이 섹션은 항상 표시됩니다. 제목 옆의 정보 아이콘이 섹션을
설명하고, 요약에는 `모든 도구`, 정책 내용(예: `1개만 허용, 2개 제외`), 또는 도구를 불러온 뒤의
`9 / 14 선택됨`이 표시됩니다.

- 제목 아래 상자에 도구 목록이 들어갑니다. 불러오기 전에는 **도구 불러오기**가 있으며, 저장 여부와
  관계없이 대화상자의 현재 설정으로 서버를 [`mcp check --live`](#probe-servers-live)와 같은 프로브로
  한 번 시작합니다. 클릭할 때만 실행되고 아무것도 저장하지 않으므로 새 서버에서도 쓸 수 있습니다.
  실패하면 이유를 쉬운 말로 보여 주고, 원래 오류는 옆의 정보 아이콘 툴팁에 있으며, 버튼은 **다시 시도**로
  바뀝니다. 이후 명령, URL 또는 관련 설정을 바꾸면 불러온 목록은 지워집니다.
- 불러오면 도구마다 체크박스가 생기고, 선택한 도구만 모델에 전달됩니다. 선택을 해제하면 그
  도구의 정확한 이름이 `deny`에 추가됩니다. 다시 선택하면 그 이름이 `deny`에서 빠지고, 비어
  있지 않은 `allow`가 여전히 제외하면 `allow`에 이름이 추가됩니다. `deny` 패턴이 제거하는 도구는
  선택할 수 없으며, 툴팁이 해당 규칙을 알려 줍니다. 검색 상자로 목록을 거를 수 있고,
  **모두 선택**과 **모두 해제**는 보이는 행에만 적용되며, 새로고침 버튼은 목록을 다시 불러옵니다.
- 상자 아래쪽의 **제외 규칙** 행에는 `*` 패턴과 서버가 나열하지 않는 이름을 입력합니다. 하나를
  입력하고 Enter를 누르세요. `allow`에 항목이 있으면 그 위에 같은 방식의 **허용만** 행이 있습니다.
  도구를 불러오기 전에는 저장된 항목이 모두 여기에 표시됩니다. 잘못된 이름이나 허용된 도구를 모두
  제거하는 거부 목록은 대화상자에 표시되며 **저장**을 막습니다.
- 그 아래에서 대화상자는 선택된 각 Agent가 실제로 무엇을 받는지 알려 줍니다: 이 목록대로 제공하는
  Agent, 일부만 적용하는 Agent의 동작(예: Copilot CLI는 거부 목록이 없어 선택 해제한 도구도 제공),
  그리고 필터링을 지원하지 않는 Agent입니다.

서버 행에는 정책을 쉬운 말로 나타내는 태그(예: `도구: 도구 2개 제외`)가 표시되며, **각 Agent에 기록될 설정 보기**는 Agent마다
적용하지 않는 부분을 경고합니다.

## Pi {#pi}

Pi ≥ 0.99.0에는 [MCP가 내장](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/mcp.md)되어 있으며,
Skillshare가 Pi에 MCP 서버를 작성하는 방법은 이것뿐입니다. 서드파티
`pi-mcp-adapter`와 `pi-mcp-extension`은 더 이상 sync 대상으로 지원되지 않습니다.

| Scope | File |
|---|---|
| Global | `~/.pi/agent/mcp.json` (`PI_CODING_AGENT_DIR`을 존중) |
| Project | `.pi/mcp.json` |

개인 서버와 자격 증명이 있는 서버는 `~/.pi/agent/mcp.json`에 두세요. `.pi/mcp.json`은
신뢰하는 프로젝트에서 그 프로젝트에 필요한 서버에만 사용하세요. 같은 이름의 project
entry는 global entry 전체를 대체합니다. Skillshare는 미리 보기와 백업을 제공하며 파일을
직접 편집합니다. 프로젝트 신뢰, 서버 실행, 확장 설치, OAuth 인증은 처리하지 않습니다.

```bash
skillshare mcp add docs --url https://example.com/mcp --target pi --tools-deny 'delete_*' --pi-options '{"exposure":"deferred","timeout":120}' --no-tui
skillshare sync mcp --dry-run
skillshare sync mcp
```

```yaml
mcp:
  servers:
    docs:
      url: https://example.com/mcp
      targets: [pi]
      tools:
        deny: [delete_*]
      piOptions:
        exposure: deferred
        timeout: 120
```

네이티브 출력은 `command`/`args` 또는 `url`을 사용하며, 환경 참조는 `${NAME}`입니다.
sync 후에는 `/reload`를 실행하거나 새 Pi 세션을 시작하고, `/mcp`로 연결을 확인하고
OAuth를 승인하세요. 간단한 Pi 전용 설정에는 `pi mcp add`가 global 파일을 편집하며,
project에는 `-l`을 추가하세요. `pi mcp list`는 활성 서버를 모두 시작해 연결을 확인하며,
`pi mcp login NAME`은 사용자 승인이 필요합니다.

Pi 서버 이름에는 영문자, 숫자, `_`, `-`만 쓸 수 있습니다. `-`와 `_`만 다른 이름은 Pi가 같은 서버로
읽으므로 sync는 두 번째 이름을 거부합니다. Pi에서는 프로젝트 항목이 같은 이름의
global 항목을 통째로 대체합니다. 한 프로젝트에서 global 서버를 끄려면
[Turn off a global server in one project](#turn-off-a-global-server-in-one-project)를 참고하세요.

Pi 1.0.1부터 Pi의 `/mcp`는 `enabled`, `exposure`, `toolExposure`만 있는 프로젝트 항목을 추가해
같은 이름의 global 서버를 덮어쓸 수 있습니다. 이 항목은 서버가 아니므로 가져오기에서 건너뜁니다.
프로젝트가 같은 이름의 서버를 정의하면, 그 항목을 대체하거나 Pi에서 덮어쓰기를 제거할 때까지
sync가 충돌을 보고합니다.

### 기타 Pi 설정 {#pi-options}

`piOptions`는 Pi 내장 MCP의 그 밖의 서버별 필드를 담습니다. Pi만 이를 받습니다.

- `exposure`는 `codemode`(Pi 기본값), `codemode-deferred`(`codemode`의 이전 이름), `deferred`, `direct`,
  `hidden`을 받습니다. `toolExposure`는 도구 이름이나 와일드카드 패턴을 이 값 중 하나에
  대응시킵니다: 정확한 이름이 우선하고, 그다음 처음 일치하는 패턴이 적용됩니다.
  Skillshare는 import와 JSON/YAML 변환에서 패턴 순서를 유지합니다. `exposure`는
  [`tools`](#tool-policy) 허용 목록이 남긴 도구를 어떻게 제공할지도 정합니다. `toolExposure`보다
  다른 Agent에도 전달되는 `tools`를 우선 사용하세요. 한 서버에 `tools`와 `toolExposure`를
  함께 설정할 수는 없습니다.
- `timeout`(양수 초), `cwd`, `enabled`, `oauth`, `auth`는 검증됩니다. `description` 같은
  알 수 없는 필드는 그대로 전달됩니다.
- `auth: {provider: NAME}`은 해당 provider의 `/login` 토큰을 bearer 토큰으로 보냅니다.
  https `url`(localhost는 http도 가능)이 필요하며, Pi가 global 파일에서만 읽으므로 global
  모드에서만 쓸 수 있습니다.
- `oauth.authServerMetadataUrl`(Pi 1.0 이상)은 https(localhost는 http도 가능)여야 합니다. Pi가
  자동 탐색 대신 이 문서를 그대로 신뢰하기 때문입니다. Pi 1.0은 OAuth 로그인을 server 이름과
  URL별로 저장하므로, server 이름이나 `url`을 바꾼 뒤에는 Pi에서 다시 로그인해야 합니다.
- 연결 필드는 메인 폼에 둡니다. `directTools`, `includeTools`, `excludeTools`와 그 밖의
  `pi-mcp-adapter` 설정은 Pi 내장 MCP가 읽지 않으므로 거부됩니다. 대신 `tools`를
  사용하세요.
- 최상위 `settings`와 `autoEnableCodemode`는 서버 옵션이 아닙니다: Pi에서 직접
  편집하세요. sync는 이를 유지합니다.
- 자격 증명은 환경 참조로 두세요. 이식 가능한 env/headers의 `!command` 리터럴 값은
  거부되며, `oauth.clientId` 같은 비밀이 아닌 필드를 포함해 `piOptions` 어디에서든
  명령 값은 거부됩니다.

JSON을 지우거나 JSON에서 필드를 제거하면, Skillshare가 작성했고 변경되지 않은 필드는
다음 sync에서 Pi 파일에서도 제거됩니다. Pi에서 직접 추가한 필드는 유지됩니다.
Skillshare가 작성한 뒤 Pi에서 변경된 필드는 import할 때까지 sync를 막습니다.

```bash
skillshare mcp edit docs --pi-options '{"timeout":60}' --no-tui
skillshare mcp edit docs --pi-options '{}' --no-tui
```

대시보드에서는 서버 대화상자의 Pi 블록에 **도구 노출 모드**와 **기타 Pi 설정**이
있습니다. **Pi 설정**과 **도구 노출 모드** 옆의 정보 아이콘이 이를 설명하며, **Pi 설정**
옆의 링크는 Pi의 MCP 문서를 엽니다. 대화상자는 저장하기 전에 **기타 Pi 설정**의
`pi-mcp-adapter` 필드를 표시해 줍니다. 도구 섹션에 설정이 있어도 **도구 노출 모드**는
편집할 수 있습니다. 이때는 `tools`가 작성하는 **기타 Pi 설정**의 `toolExposure`만 거부됩니다.
서버 행의 Pi 칩은 노출 모드를 짧은 말로 보여 줍니다(예: `codemode`는 `코드로 호출`).

### 0.22에서 Pi 업그레이드하기 {#pi-migration}

업그레이드 후 첫 sync 전에 Pi에서 두 가지를 확인하세요.

- **Pi 0.99.0 이상.** Skillshare는 이제 Pi의 서버를 `mcp.json`에만 작성하며, Pi는 0.99.0에서
  추가된 내장 MCP로 이 파일을 읽습니다. 이전 Pi는 이 파일을 읽지 않으므로 Pi를 업데이트할
  때까지 이 서버들은 로드되지 않습니다. Skillshare는 Pi 버전을 확인하지 않습니다.
- **`pi-mcp-adapter` 또는 `pi-mcp-extension`이 아직 설치되어 있다면 Pi에서 제거하세요.** Pi의
  [MCP 문서](https://github.com/earendil-works/pi/blob/v0.99.0/packages/coding-agent/docs/mcp.md)에
  따르면 `/mcp`를 등록하는 extension이 설치되어 있으면 내장 MCP를 대체합니다.
  `pi-mcp-extension`은 `mcp.json`도 직접 읽고, `pi-mcp-adapter`는 3.0.0부터 이 파일을 읽지 않으므로
  Skillshare가 옮긴 서버는 adapter를 통해 로드되지 않습니다.

sync가 서버를 이 extension들에서 옮길 때 `sync mcp --dry-run`, `sync mcp`, `--json`은
한 번 다음과 같이 알립니다.

```text
! Pi's built-in MCP needs Pi 0.99.0 or later; on older Pi these servers stop loading until Pi is updated. If pi-mcp-adapter or pi-mcp-extension is still installed in Pi, remove it, because it can take the place of Pi's built-in MCP
```

이 알림은 sync가 Skillshare가 `mcp-adapter.json`에 작성한 항목을 제거하거나,
`pi-mcp-extension`용으로 작성한 항목을 다시 쓰거나, 이 extension들만 읽는 설정(`piExtension:
pi-mcp-adapter` 또는 `pi-mcp-extension`, `directTools`, 아래에 나열된 `piOptions` 필드)을 찾을 때
나타납니다. 그 sync 이후에는 나타나지 않습니다.

0.23.0에서는 Pi 모드 선택(`piExtension`: `builtin`, `pi-mcp-adapter`,
`pi-mcp-extension`), `piOptionsPrune` 스위치, `directTools`가 제거되었습니다. 이전
config도 그대로 로드됩니다. `sync mcp --dry-run`과 `sync mcp`는 발견한 폐지된 설정의
종류마다 서버 이름을 담은 warning을 출력합니다. 예:

```text
! Pi now uses its built-in MCP; the next sync updates the config: context7, local (shop)
```

`mcp.projects` 아래의 프로젝트에서만 발견된 서버는 괄호 안에 프로젝트 폴더가
표시됩니다.

다음 sync가 하는 일:

| 0.23.0 이전 | sync 이후 |
|---|---|
| `piExtension: builtin` | 키가 제거됩니다. 그 밖에는 바뀌지 않습니다 |
| `piExtension: pi-mcp-extension` | 키가 제거됩니다. 항목이 이미 `mcp.json`에 있었으므로 그 자리에서 내장 형식으로 다시 작성됩니다 |
| `piExtension: pi-mcp-adapter` | 키가 제거됩니다. 서버는 `mcp.json`에 작성되고, Skillshare가 `mcp-adapter.json`에 작성한 항목은 제거됩니다. `mcp-adapter.json`에 직접 추가한 항목은 그대로 둡니다 |
| `piOptionsPrune` | 키가 제거됩니다. sync는 Skillshare가 작성했고 변경되지 않은 지워진 필드를 항상 제거합니다([위](#pi-options)) |
| 서버의 `directTools` | `true` → `piOptions.exposure: direct`; `"search"` → `deferred`; 이름 목록 → 해당 도구를 `direct`로 둔 `piOptions.toolExposure` |
| `mcp.directTools`, 또는 `mcp.projects` 아래 프로젝트의 `directTools` | Pi에 전달되고 자체 값이 없는 각 서버에 기본값이 위와 같이 작성됩니다. 프로젝트의 `false`는 global 값을 재정의합니다 |
| `piOptions.includeTools` / `excludeTools` | `tools.allow` / `tools.deny`가 되며, 함께 설정한 `directTools`는 여전히 `piOptions.exposure`가 됩니다 |
| `piOptions`의 그 밖의 `pi-mcp-adapter` 필드: `approveTools`, 문자열 `auth`(Pi 자체의 `auth` 객체는 유지), `bearerToken`, `bearerTokenEnv`, `bearerTokenStore`, `caFile`, `debug`, `exposeResources`, `idleTimeout`, `inheritEnv`, `lifecycle`, `protocolVersion`, `requestHeadersCommand`, `requestTimeoutMs`, `searchKeywords`, `socket`, `tasks`, `toolPrefix`, `trace` | Pi 내장 MCP가 읽지 않으므로 제거됩니다 |

서버가 이미 설정한 exposure를 덮어쓰게 되거나 도구 이름 목록이 아닌 `directTools`,
`includeTools`, `excludeTools`는 별도의 warning과 함께 버려집니다.

이 변경을 적용하는 첫 sync는 폐지된 설정을 뺀 Skillshare config도 저장합니다:
`config.yaml`, 또는 `sources.mcp`가 가리키는 파일입니다. 쓰기 전에 이전 파일을
reason이 `migrate`인 [파일 이력](/docs/reference/commands/backup#file-history)에 보관하고,
파일마다 한 줄을 출력합니다:

```text
→ Updated config.yaml for 0.23.0 (backup: <path of the saved version>)
```

이는 `skillshare sync mcp`, `skillshare sync --all`(Agent 파일이 바뀌지 않을 때도),
대시보드의 sync에서 일어납니다. `--dry-run`과 미리 보기는 아무것도 쓰지 않습니다.
config 저장에 실패하면 Agent 파일은 이미 작성되었고 config는 그대로 남습니다. 오류
메시지가 이를 알려주며, 다음 sync가 다시 시도합니다. 저장에 성공하면 warning은
사라집니다.

제거된 flag는 이제 메시지와 함께 실패합니다:

| Flag | 대신 사용할 것 |
|---|---|
| `--pi-extension` | 빼세요. Pi는 항상 내장 MCP를 사용합니다 |
| `--pi-options-prune` | 빼세요. sync는 Skillshare가 이전에 작성한 변경되지 않은 필드를 항상 제거합니다 |
| `--direct-tools` | 모든 도구에는 `--pi-options '{"exposure":"direct"}'`, Pi에서 개별 도구에는 `--pi-options '{"toolExposure":{"TOOL":"direct"}}'` |

`skillshare mcp import --from pi`는 Pi의 `mcp.json`과 함께 `pi-mcp-adapter`의
`mcp-adapter.json`도 계속 읽으므로, 서버를 옮겨 올 수 있습니다. 두 파일이 같은 서버를
정의하면 `mcp.json`이 우선합니다. sync는 서버를 Pi의 `mcp.json`에 작성하며,
`mcp-adapter.json`에서는 0.23.0 이전에 자신이 작성한 항목만 제거합니다. 그 `directTools`,
`includeTools`, `excludeTools`는 위와 같이 변환되며, 그 밖의 adapter 전용 필드는
warning과 함께 제외됩니다. 대시보드에서는 **target에서 가져오기**가 두 Pi 파일을 별도의
원본으로 나열합니다.
