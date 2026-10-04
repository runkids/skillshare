---
sidebar_position: 1
---

# ui

시각적인 Skill 관리를 위한 웹 대시보드를 실행합니다.

```bash
skillshare ui                  # 포그라운드에서 실행
skillshare ui start            # 백그라운드 서버 시작(또는 기존 서버 재사용)
skillshare ui stop             # 백그라운드 서버 중지
```

기본 브라우저에서 `http://127.0.0.1:19420`을 엽니다.

## 모드

| Mode | 동작 |
|------|----------|
| `skillshare ui` (기본값) | UI 서버를 포그라운드에서 실행합니다. `Ctrl+C`로 중지합니다 |
| `skillshare ui start` | UI 서버를 백그라운드 프로세스로 시작하고 셸 제어권을 반환합니다. 기존 프로세스가 정상 상태이면 `start`를 다시 실행해도 해당 프로세스를 재사용합니다 |
| `skillshare ui stop` | `skillshare ui start`로 시작한 백그라운드 UI 서버를 중지합니다 |

## 사용 시점

- 시각적인 웹 인터페이스로 skill, target, sync를 관리
- CLI 플래그를 외우지 않고도 skill을 탐색하고 설치
- 시각적인 findings 리포트로 보안 감사를 실행
- CLI에 익숙하지 않은 팀원과 대시보드 화면을 공유

## 플래그

| Flag | 기본값 | 설명 |
|------|---------|-------------|
| `-p`, `--project` | | project mode로 실행합니다(`.skillshare/` 사용) |
| `-g`, `--global` | | global mode로 실행합니다(`~/.config/skillshare/` 사용) |
| `--port <port>` | `19420` | HTTP 서버 포트 |
| `--host <host>` | `127.0.0.1` | 바인딩 주소(Docker에서는 `0.0.0.0` 사용) |
| `-b`, `--base-path <path>` | | reverse proxy용 하위 경로(예: `/skillshare`) |
| `--no-open` | `false` | 브라우저를 자동으로 열지 않음 |
| `--app` | `false` | 가능한 경우 대시보드를 데스크톱 스타일의 Chromium 앱 창으로 엽니다(`start` mode 전용) |
| `--clear-cache` | | 포그라운드 형태에서는 캐시된 UI 자산을 지우고 종료합니다. `start`와 함께 사용하면 캐시를 지운 후 백그라운드에서 시작합니다 |

:::tip 자동 감지
현재 디렉터리에 `.skillshare/config.yaml`이 존재하면 대시보드가 자동으로 project mode로 시작됩니다. global mode를 강제하려면 `-g`를 사용하세요.
:::

## 예제

```bash
# 기본값: localhost:19420에서 브라우저를 엽니다(포그라운드)
skillshare ui

# Project mode(.skillshare/ skill 관리)
skillshare ui -p

# 사용자 지정 포트
skillshare ui --port 8080

# Docker / 원격 접속
skillshare ui --host 0.0.0.0 --no-open

# 백그라운드에서 시작하고 셸로 복귀
skillshare ui start

# 크롬 없는 데스크톱 스타일 앱 창으로 시작
skillshare ui start --app

# 백그라운드 서버 중지(기억된 host/port 사용)
skillshare ui stop

# 캐시된 UI 자산을 지운 후 백그라운드에서 새로 시작
skillshare ui start --clear-cache
```

## 대시보드 페이지

사이드바는 페이지를 작업별로 그룹화합니다: 동기화, 관리 대상, 배포 위치, 유지 관리. 이름 아래 줄에는 `Global · ~/.config/skillshare`처럼 mode와 해당 폴더가 표시됩니다.

일부 페이지는 주의가 필요할 때 사이드바에 개수를 표시합니다. 대시보드 탭이 열려 있는 동안 이 개수는 15초마다 새로고침됩니다:

- **Sync**: sync가 적용할 변경 사항
- **Git Sync**: 커밋되지 않은 파일, 또는 트리가 깨끗할 때 아직 push되지 않은 커밋
- **Audit**: 마지막 스캔에서 차단된 skill과 agent(스캔을 실행한 후 표시됨)

| Page | 설명 |
|------|-------------|
| **Dashboard** | skill, agent, extras, MCP 서버, hook, plugin, target의 개수와 주의가 필요한 항목 |
| **Sync** | target별로 기록 전에 모든 변경 사항을 미리 봅니다. 포함할 부분(Skills, Agents, Extras, MCP)을 선택합니다. target 내부에서 편집된 파일은 **Force**가 켜져 있지 않은 한 유지됩니다. target에만 존재하는 항목은 여기서 source로 다시 수집할 수 있습니다. 각 sync는 먼저 target 폴더를 백업합니다. target이 실패해도 다른 target은 sync됩니다. 실패한 target은 해당 부분(Skills, Agents, Extras, Config)과 오류와 함께 다른 경고보다 위에 표시되고, 흔한 원인(다른 곳을 가리키는 symlink, 권한 거부, 읽기 전용 파일 시스템, 없는 파일이나 폴더, 잘못된 target 설정)에는 쉬운 설명도 붙으며, 다른 곳을 가리키는 Skills symlink에는 **Force 켜기**가 제공되며, 변경 목록에서도 해당 target이 표시됩니다. 모든 target이 실패해도 같은 방식으로 표시됩니다. **마지막 동기화** 카드에는 가장 최근 sync에서 실패한 target이 표시됩니다 |
| **Git Sync** | source repo를 commit하고 push하며, 아직 remote에 없는 커밋을 push하고, pull합니다. 페이지를 열면 remote에서 fetch하므로 **Pull** 버튼에 remote에 있는 커밋 수가 표시됩니다. Pull은 [`pull`](/docs/reference/commands/pull)과 마찬가지로 repo scope가 담고 있는 것(`skills`, `agents`, `extras`, 또는 `root`)을 동기화합니다. **Sync both ways**는 로컬 변경 사항을 commit하고, pull해 병합하고, target을 동기화한 뒤 push합니다([`push --pull`](/docs/reference/commands/push#push-and-pull-together)과 같음). 충돌이 발생하면 push하기 전에 멈춥니다. remote에 더 새로운 커밋이 있어 push가 거부되면 오류 알림에서 **Pull**을 제공합니다. 첫 pull이 remote와 병합할 수 없을 때는 로컬 파일을 remote 브랜치로 교체하는 force pull을 제공합니다 |
| **Hubs** | Skills 페이지에서 접근합니다. 내장 hub, 저장된 hub, 내 Hub(**내 Hub**)를 나열하며, 하나를 선택해 Skill을 필터링하고 설치합니다. **Hub 추가 또는 만들기**는 기존 hub 추가, 새 Hub 만들기, `skillshare-hub.json` 가져오기를 합니다. 내 Hub는 **편집**으로 변경하고 **공유**로 인덱스를 다운로드하고 `hub add` 명령을 만듭니다. [`hub`](/docs/reference/commands/hub) 참고 |
| **Skills** / **Agents** | 설치된 항목, **Updates** 탭, **Trash** 탭. Skills에는 각 skill이 target의 context에 추가하는 토큰 수를 추정하는 **Analyze** 탭도 있습니다. **Install**은 GitHub를 검색하거나 URL 또는 경로에서 설치합니다. **+ New Skill**은 생성 마법사를 엽니다. 목록과 카드 보기는 **Folder**(tracked repo, `frontend/react` 같은 폴더, 또는 **Root**)로 필터링하고 **Folder**로 그룹화할 수 있으며, Root가 먼저 오고 나머지 폴더는 이름 A→Z 순입니다. **tree** 보기는 왼쪽에 source 폴더를, 오른쪽에 상세 정보를 보여 줍니다. 폴더나 skill을 클릭해 선택하고, Cmd/Ctrl 클릭으로 선택에 추가하고, Shift 클릭으로 범위를 선택하며, skill을 더블클릭하면 열립니다. 오른쪽에서는 선택한 모든 항목을 스위치 하나로 켜거나 끄고, targets를 설정하며(tracked repo와 그 하위 폴더 포함), 각 skill을 개별 스위치와 함께 나열합니다. tracked repo에는 **Update repo**와 **Uninstall repo**도 있습니다. `disable-model-invocation: true`가 설정된 skill은 목록, 타일, 상세 페이지에 **manual only** 태그가 표시되며, 이는 [`list`](/docs/reference/commands/list)에서 `m`으로 전환하는 것과 같은 상태입니다. skill 편집기에서 **Add field**는 각 frontmatter 필드가 하는 역할을 설명합니다. **Sync skills** / **Sync agents**는 미리 본 뒤 해당 종류만 모든 target에 동기화합니다. 업데이트, 제거 또는 collect 후에는 **Sync Now**에서 같은 대화상자가 열립니다 |
| **Extras** | **폴더**: skill과 함께 동기화되는 rules, commands, 기타 폴더. **AGENTS.md**: global 모드에서는 공유 `AGENTS.md`와 그것을 쓰는 target, 프로젝트에서는 프로젝트의 `./AGENTS.md`와 각 target이 그것을 읽는지 여부. [여러 도구에서 하나의 AGENTS.md 공유하기](../../how-to/daily-tasks/sharing-instructions.md)를 참고하세요  **Memory**: [메모리 공유 가이드](../../how-to/daily-tasks/sharing-memory.md)(영어 스크린샷). 노트 검색, 미리 보기, 편집, 삭제, INDEX 링크, 충돌 시 초안 보존, Backup Files 기록과 복원, **Connect to agents** → **Review changes** → **Apply changes**. 설정 상태는 읽기를 증명하지 않습니다 |
| **MCP** | 서버당 한 행이며, 동기화 대상인 Agent가 chip으로 표시되며, 행의 카운트 버튼을 누르면 해당 토글이 열립니다. **서버 추가**는 URL, command, 붙여넣은 snippet 또는 파일을 받습니다. **target에서 가져오기**는 설치된 Agent가 이미 가지고 있는 것을 읽어옵니다. 각 서버의 메뉴에는 **각 Agent에 기록될 설정 보기**가 있어 source와 모든 Agent의 파일을 나열하고, 선택한 파일을 저장되지 않은 편집을 포함해 보여줍니다. 충돌 시 **해당 Agent에서 가져오기** 또는 **원본으로 덮어쓰기**를 제공합니다. 서버 대화상자의 **도구** 섹션은 [도구 정책](./mcp.md#tool-policy)을 설정합니다: 저장 여부와 관계없이 대화상자의 현재 설정으로 서버를 한 번 시작해 도구를 선택할 수 있는 목록으로 보여 주는 **도구 불러오기**, `*` 패턴용 **제외 규칙** 행이 있습니다. 이 섹션은 정책 일부를 적용하지 않는 선택된 Agent를 각각 나열하며, 정책이 설정되면 행에 태그가 표시됩니다. Pi 서버의 경우, 대화상자에는 [Pi 내장 MCP](./mcp.md#pi) 설정도 있습니다: 도구 노출 방식과 기타 Pi 설정이며, 이 설정은 [`piOptions`](./mcp.md#pi-options)를 JSON으로 받고 도움말은 정보 툴팁에 있습니다. **기본값**은 `mcp.targets`를 편집합니다. Sync 박스의 **Sync MCP**는 대기 중인 변경 사항을 나열하고 MCP 설정 파일만 작성하며, 각 파일의 백업을 남깁니다. 그 아래에서 [**검사**](./mcp.md#check-servers-before-an-agent-starts-them)는 서버를 검사하고, **백업 및 복원**는 그 백업을 미리 보고 복원합니다. |
| **Plugins** | plugin당 한 행이며, 해당 Agent가 토글로 표시됩니다. 행을 펼치면 소스가 지원하는 다른 Agent도 나열되며, 하나를 선택하면 설치를 미리 봅니다. 행 메뉴는 sync, update, remove를 수행하거나, Skillshare가 검토한 로컬 사본을 읽기 전용으로 탐색하는 **View files**를 엽니다. [Manage plugins across tools](/docs/how-to/daily-tasks/sharing-plugins) 참고 |
| **Targets** | 상태가 표시된 target 목록. **대상 추가**는 **다른 계정**도 받습니다. 이미 사용 중인 Agent의 두 번째 config 폴더이며, 어디에 기록되는지 미리보기로 보여줍니다. 각 target의 페이지에서 include/exclude 필터를 편집하고 로컬 전용 skill을 source로 다시 수집합니다. 목록에는 각 Agent가 받는 MCP 서버 수도 표시됩니다. MCP 설정 파일이 있는 target에는 **MCP** 탭이 있어 서버마다 한 행으로 고릅니다. 클릭하면 바로 저장되고 **Sync all targets**가 모든 target의 MCP 파일에 기록합니다. 각 target에는 지침 파일 이름을 딴 탭(**CLAUDE.md**, **GEMINI.md**, **AGENTS.md**, ...)도 있어, 읽는 순서를 보여 주고 파일을 편집하며 `AGENTS.md`로 변환합니다 |
| **프로젝트** | Global mode 전용. global 설정이 동기화하는 프로젝트 폴더이며, [`projects`](/docs/reference/targets/configuration#projects)와 [`mcp.projects`](./mcp.md#projects-in-the-dashboard)에서 옵니다. **프로젝트 추가**는 폴더, 그 target, 동기화할 항목을 받습니다. 각 프로젝트에는 필터, 미리보기, 기록될 폴더를 보여주는 **Skills**와 **Agents** 탭, 그리고 해당 폴더에서 global 서버를 끄거나 프로젝트 자체 서버를 부여하며 그 서버를 위한 **검사**가 있는 **MCP** 탭이 있습니다. **Sync project**는 미리 본 뒤 해당 프로젝트의 skill, agent, MCP만 동기화합니다. 이미 프로젝트 폴더를 가리키는 Target은 변환할 수 있습니다 |
| **Audit** | skill과 agent에 대한 보안 스캔이며, 심각도별로 findings를 표시합니다. **Rules** 탭에서는 카테고리별로 모든 rule을 탐색할 수 있습니다: rule을 끄거나, 심각도를 변경하거나, 카테고리 전체에 심각도를 적용하거나, 스캔 profile(`default`, `strict`, `permissive`)을 선택하거나, 사용자 지정 `audit-rules.yaml` 편집기를 엽니다 |
| **Settings** | 탭으로 구성: **General**(source 경로, sync mode, 외관), **Backup**(대상 폴더 스냅샷, `AGENTS.md` 같은 파일의 이전 버전, MCP 설정 백업. [`backup`](./backup.md#dashboard) 참고), **Log**(작업 이력), **Health**([`doctor`](/docs/reference/commands/doctor)와 동일한 검사), **Extensions**(sync 시점의 파일 변환), **Files**(`config.yaml`, `.skillignore`, `.agentignore`를 위한 직접 편집기) |

변경 목록 옆의 **변경 사항 버리기**는 확인 후 선택한 Git 범위 내의 추적 중인 파일과 스테이징 영역을 마지막 커밋으로 복원하고 추적되지 않은 파일과 폴더를 삭제합니다. Git에서 무시한 파일, 중첩된 Git 저장소 및 `root` 범위의 `config.yaml`은 유지됩니다. 커밋 기록을 변경하거나 원격 저장소로 푸시하지 않으며, 이 작업은 되돌릴 수 없습니다. **시험 실행**은 파일을 변경하지 않고 미리 봅니다. 변경 사항을 버리려면 저장소에 첫 번째 커밋이 있어야 합니다.

**Updates** 탭의 진행률 표시줄은 업데이트 진행 상황을 보여 주며 업데이트 중인 행도 표시됩니다. 차단되거나 실패한 업데이트는 별도 섹션에 표시됩니다.

`/collect`, `/install`, `/search`, `/trash`, `/analyze`, `/backup`, `/log`, `/doctor`와 같은 이전 링크는 새 위치로 리디렉션됩니다.

**Files** 탭은 편집기 옆에 패널을 배치합니다. `config.yaml`의 경우 커서 아래 필드가 하는 역할, 파일 구조, 저장되지 않은 변경 사항을 보여줍니다. ignore 파일의 경우 패턴이 현재 무엇을 숨기고 있는지 나열합니다. `Cmd+S` / `Ctrl+S`로 저장합니다. **Audit -> Rules -> Edit YAML** 아래의 rules 편집기에는 동일한 패널과 함께, 붙여넣은 줄에 대해 rule의 regex를 실행하는 **Test** 탭이 있습니다.

### 테마 시스템

대시보드는 두 가지 시각적 스타일과 세 가지 색상 모드를 지원하며, 사이드바의 **Theme** 버튼으로 전환할 수 있습니다:

| Setting | Options | 기본값 |
|---------|---------|---------|
| **Style** | `Clean`(전문적인 느낌), `Playful`(굵은 외곽선, 강한 그림자, 손글씨 스타일 제목) | Playful |
| **Mode** | `Light`, `Dark`, `System`(OS 설정을 따름) | Light |

테마 설정은 세션 간에도 localStorage에 유지됩니다.

### Project Mode 차이점

project mode(`-p`)로 실행할 때 대시보드는 다음과 같이 달라집니다:

- **Sidebar**는 이름 아래에 `Project · <project path>`를 표시합니다
- **Git Sync page**는 숨겨집니다(project skill은 프로젝트 자체의 git을 사용)
- **Sync**는 `skillshare sync -p`와 마찬가지로 agent target 폴더만 백업합니다
- **Backup tab**은 Settings에서 숨겨집니다(대신 버전 관리를 사용)
- **Tracked Repos section**은 Dashboard에서 숨겨집니다(해당 없음)
- **Settings -> Files**는 global 버전 대신 `.skillshare/config.yaml`과 프로젝트 수준의 `.skillignore`를 표시합니다
- **Available targets**는 프로젝트 수준의 target을 나열합니다(예: 프로젝트 루트 기준 `.claude/skills/`)
- **Targets**는 프로젝트의 MCP 서버를 세고 켜고 끄며, 프로젝트 자체 파일(예: `.mcp.json`)에 기록합니다. Claude Code, OpenCode, Kilo Code, Pi의 **MCP** 탭에는 이 프로젝트에서 전역 서버를 끄는 스위치도 표시됩니다
- **Install**은 프로젝트 config의 `skills:` 항목을 자동으로 재조정합니다
- **Extras -> AGENTS.md**는 공유 파일 대신 프로젝트의 `./AGENTS.md`를 편집하며, 자체 파일만 읽는 target에는 간단한 수정을 제안합니다

## UI 미리보기

<div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '1rem'}}>
  <img src="/img/web-install-demo.png" alt="설치 흐름" />
  <img src="/img/web-dashboard-demo.png" alt="대시보드 개요" />
  <img src="/img/web-skills-demo.png" alt="Skills 탐색기" />
  <img src="/img/web-skill-detail-demo.png" alt="Skill 상세 보기" />
  <img src="/img/web-sync-demo.png" alt="Sync 컨트롤" />
  <img src="/img/web-search-skills-demo.png" alt="GitHub 검색 화면" />
  <img src="/img/web-projects-demo.png" alt="프로젝트 폴더 목록을 보여 주는 Projects 페이지" />
</div>

## REST API

웹 대시보드는 `/api/`에서 REST API를 제공합니다. 모든 엔드포인트는 JSON을 반환합니다.

| Method | Path | 설명 |
|--------|------|-------------|
| GET | `/api/overview` | Skill/target 개수, mode, 버전, config 폴더(`configDir`) |
| GET | `/api/skills` | 메타데이터와 함께 모든 skill 목록 조회 |
| GET | `/api/skills/{name}` | Skill 상세 정보 + SKILL.md 내용 |
| GET | `/api/skills/templates` | skill 생성에 사용 가능한 패턴과 카테고리 조회 |
| POST | `/api/skills` | 새 skill 생성(name, pattern, category, scaffoldDirs) |
| DELETE | `/api/skills/{name}` | skill 제거 |
| GET | `/api/targets` | 상태, include/exclude 필터, target별 예상 개수와 함께 target 목록 조회 |
| POST | `/api/targets` | target 추가 |
| DELETE | `/api/targets/{name}` | target 제거 |
| POST | `/api/sync` | sync 실행(`dryRun`, `force`, `kind`, 그리고 sync를 해당 프로젝트의 target으로 제한하는 선언된 프로젝트 루트인 `project` 지원). `dryRun`이 설정되지 않은 한 먼저 target을 백업합니다 |
| POST | `/api/git/commit` | push 없이 source repo에서 로컬 git commit 생성 |
| POST | `/api/git/discard` | 설정된 Git 범위 내의 커밋되지 않은 변경 사항을 버립니다(전역 모드 전용, 저장소에 첫 번째 커밋 필요). `dryRun`을 지원합니다. Git에서 무시한 파일, 중첩된 Git 저장소 및 `root` 범위의 `config.yaml`을 유지합니다 |
| GET | `/api/git/status` | 아직 push되지 않은 커밋(`ahead`)과 마지막 fetch 기준으로 아직 pull하지 않은 upstream 커밋(`behind`)을 포함한 source repo 상태. fetch는 하지 않습니다 |
| POST | `/api/push` | 변경 사항을 commit한 후 push합니다. 첫 push 시 upstream을 설정합니다. remote에 이 repo에 없는 커밋이 있으면 `409`와 오류 코드 `push_rejected`로 실패합니다. pull한 후 다시 push하세요 |
| POST | `/api/pull` | pull한 후 repo scope가 담고 있는 것을 sync합니다. 갈라진 히스토리는 병합되며, `.metadata.json` 충돌은 자동으로 해결되고, 그 외의 충돌은 병합을 되돌린 채 실패합니다. 첫 pull이 병합에 실패하면 오류 코드 `merge_failed`로 실패합니다. 로컬 파일을 remote 브랜치로 교체하려면 `force: true`로 재시도하세요. `alwaysSync: true`를 지정하면 새로 가져온 것이 없어도 target을 sync합니다. 브랜치가 없는 remote에서는 `400 remote_empty`를 반환합니다 |
| GET | `/api/diff` | source와 target 간의 diff |
| GET | `/api/search?q=` | GitHub에서 skill 검색 |
| POST | `/api/install` | source에서 skill 설치 |
| GET | `/api/audit` | 모든 skill에 대해 보안 위협 스캔 |
| GET | `/api/audit/rules` | 사용자 지정 audit rules YAML 조회 |
| PUT | `/api/audit/rules` | 사용자 지정 audit rules 저장(regex 검증) |
| POST | `/api/audit/rules` | 초기 audit-rules.yaml 생성 |
| GET | `/api/audit/rules/compiled` | built-in rule과 사용자 지정 rule을 병합한 모든 rule, 그리고 활성 profile |
| POST | `/api/audit/rules/toggle` | rule 또는 패턴 전체를 활성화, 비활성화, 재평가 |
| POST | `/api/audit/rules/reset` | 사용자 지정 rule을 삭제하고 built-in 기본값으로 복원 |
| PATCH | `/api/audit/policy` | `blockThreshold`, `profile`, 또는 둘 다 설정 |
| GET | `/api/log` | 선택적 필터와 함께 로그 항목 조회 |
| GET | `/api/config` | config를 YAML로 조회 |
| PUT | `/api/config` | config YAML 업데이트 |
| GET | `/api/skillignore` | `.skillignore` 내용 + ignore 통계 조회 |
| PUT | `/api/skillignore` | `.skillignore` 내용 업데이트 |
| GET | `/api/doctor` | 모든 상태 검사 실행(JSON) |
| GET | `/api/health` | liveness probe. 서버가 준비되면 `200`을 반환합니다 |
| GET | `/api/version` | 현재/최신 버전 + 업그레이드 가능 여부 |
| POST | `/api/upgrade` | 제자리에서 `skillshare upgrade` 실행(바이너리가 dev build인 경우 `devMode: true` 반환) |
| POST | `/api/restart` | 로컬 UI 서버 재시작. 선택적으로 `{ "clearCache": true }` body를 전달하면 먼저 캐시된 UI 자산을 지웁니다 |

## 제자리 업그레이드

대시보드가 더 최신 CLI 릴리스가 있음을 감지하면, **Update** 대화상자와 **Doctor** 페이지의 *Version* 카드 모두에 **Update now** 버튼이 표시됩니다:

1. UI가 `POST /api/upgrade`를 호출하여 호스트에서 `skillshare upgrade`를 실행합니다.
2. 새 바이너리가 준비되면 UI가 `POST /api/restart`를 호출하여 로컬 서버를 재시작합니다.
3. 브라우저는 `GET /api/health`를 폴링하며 새 서버가 준비되면 자동으로 새로고침합니다.

실행 중인 바이너리가 개발 빌드(`version == "dev"`)인 경우, upgrade 엔드포인트는 `devMode: true`를 반환하고 UI는 디스크의 어떤 것도 수정하지 않은 채 재시작을 시뮬레이션합니다.

자동 새로고침이 완료되지 않으면, 대화상자는 백그라운드 서버를 다시 시작하기 위해 `skillshare ui start`를 실행하라고 안내합니다.

## 리버스 프록시 {#reverse-proxy}

공유 서버에서 reverse proxy(예: 홈랩, 내부 도구 플랫폼) 뒤에 대시보드를 실행하는 경우, `--base-path`를 사용하여 다른 서비스와 함께 하위 경로에서 서비스할 수 있습니다:

```bash
skillshare ui --base-path /skillshare --host 0.0.0.0 --no-open
```

또는 환경 변수를 통해:

```bash
SKILLSHARE_UI_BASE_PATH=/skillshare skillshare ui --host 0.0.0.0 --no-open
```

### Nginx

```nginx
location /skillshare/ {
    proxy_pass http://127.0.0.1:19420;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

### Caddy

```
handle_path /skillshare/* {
    reverse_proxy 127.0.0.1:19420
}
```

:::tip
`--base-path` 없이는 대시보드가 이전과 동일하게 동작합니다 — `localhost:19420`에서 직접 접속할 때는 별도 설정이 필요 없습니다.
:::

:::note MCP 설정
MCP 페이지는 브라우저가 `localhost` 또는 `http://192.168.1.20:19420`과 같은 IP 주소로 대시보드를 열 때만 동작합니다. reverse proxy를 포함해 도메인 이름을 통하면 MCP 요청은 403을 반환합니다: DNS 리바인딩 공격은 항상 도메인 이름을 사용하기 때문입니다. 원격 머신에서 MCP 설정을 관리하려면 `ssh -L 19420:127.0.0.1:19420 HOST`로 포트를 포워딩한 후 `http://localhost:19420`을 여세요.
:::

## Docker 사용법

Docker 내부에서 웹 UI를 사용하려면(최초 UI 다운로드를 위해 네트워크 접속이 필요합니다):

```bash
make playground

# 컨테이너 내부:
skillshare ui --host 0.0.0.0 --no-open
```

그런 다음 호스트 머신에서 `http://localhost:19420`을 여세요(포트 19420은 자동으로 매핑됩니다).

## Project Mode

웹 대시보드는 프로젝트 수준의 skill을 완전히 지원합니다:

```bash
cd my-project
skillshare ui -p
```

`.skillshare/config.yaml`이 존재하면(자동 감지) 단순히 `skillshare ui`만 실행해도 됩니다.

대시보드는 CLI와 마찬가지로 `.skillshare/config.yaml`을 읽고 쓰며, 프로젝트 로컬 target으로 sync하고, 설치 후 원격 skill 항목을 재조정합니다.

## 런타임 UI 다운로드

`skillshare ui`는 최초 실행 시 일치하는 GitHub Release에서 미리 빌드된 UI 자산을 자동으로 다운로드합니다. 자산은 `~/.cache/skillshare/ui/<version>/`에 캐시되어(`XDG_CACHE_HOME`을 따름) 이후 실행은 즉시, 오프라인으로 이루어집니다.

- **First run**은 UI 자산(~1MB)을 다운로드하기 위해 인터넷 연결이 필요합니다
- **Subsequent runs**은 캐시된 자산을 사용합니다 — 네트워크가 필요 없습니다
- **On upgrade** 시, 이전에 캐시된 버전은 자동으로 정리되며, 새 UI는 `skillshare upgrade` 중에 미리 다운로드됩니다
- **To clear the cache manually**, `skillshare ui --clear-cache`를 실행하세요

## Homebrew 참고 사항

모든 설치 방법(Homebrew, 설치 스크립트, 수동 바이너리)은 런타임 UI 다운로드를 사용합니다. `skillshare ui`를 실행하면 최초 실행 시 GitHub에서 UI 자산을 자동으로 다운로드합니다. 이후에는 캐시된 자산이 오프라인으로 사용됩니다.

다운로드된 UI 캐시를 지우려면:

```bash
skillshare ui --clear-cache
```

## 아키텍처

웹 UI는 일치하는 GitHub Release에서 런타임에 다운로드되고 디스크 캐시(`~/.cache/skillshare/ui/<version>/`)에서 제공되는 단일 페이지 React 애플리케이션입니다.

```
skillshare ui
  ├── Go HTTP server (net/http)
  │   ├── /api/*    → REST API handlers
  │   └── /*        → Cached React SPA (runtime download)
  └── Browser opens http://127.0.0.1:19420
```

## 참고

- [status](/docs/reference/commands/status) — CLI 상태 확인
- [sync](/docs/reference/commands/sync) — CLI sync 명령
- [Project Setup](/docs/how-to/sharing/project-setup) — Project mode 설정 가이드
- [Docker Sandbox](/docs/how-to/advanced/docker-sandbox) — Docker에서 UI 실행
