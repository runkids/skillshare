---
sidebar_position: 3
---

# 파일 구조

skillshare의 디렉터리 레이아웃과 파일 위치입니다.

## 개요

```
~/.config/skillshare/        # XDG_CONFIG_HOME
├── config.yaml              # 설정 파일
├── audit-rules.yaml         # 커스텀 Audit 규칙 (선택)
├── mcp.yaml                 # sources.mcp가 이 위치를 가리키는 경우의 MCP 서버 (선택)
├── skills/                  # Skill Source (Skill + 메타데이터)
│   ├── .metadata.json       # 설치된 Skill 메타데이터 (자동 관리)
│   ├── .skillignore         # 선택: Sync에서 Skill 제외
│   ├── my-skill/            # 일반 Skill
│   │   ├── SKILL.md         # Skill 정의 (필수)
│   ├── code-review/         # 다른 Skill
│   │   └── SKILL.md
│   └── _team-skills/        # Tracked 저장소
│       ├── .git/            # Git 히스토리 보존
│       ├── frontend/
│       │   └── ui/
│       │       └── SKILL.md
│       └── backend/
│           └── api/
│               └── SKILL.md
├── agents/                  # Agent Source (단일 .md 파일)
│   ├── .agentignore         # 선택: Sync에서 Agent 제외
│   ├── reviewer.md          # Agent 파일
│   └── auditor.md           # 다른 Agent
├── rules/                   # Extras Source (설정된 경우)
│   ├── coding.md
│   └── testing.md
└── commands/                # Extras Source (설정된 경우)
    └── deploy.md

~/.local/share/skillshare/   # XDG_DATA_HOME
├── backups/                 # 백업 디렉터리
│   ├── 2026-01-20_15-30-00/
│   │   ├── claude/          # claude용 Skill 백업
│   │   ├── claude-agents/   # claude용 Agent 백업
│   │   └── cursor/
│   └── 2026-01-19_10-00-00/
│       └── claude/
└── trash/                   # 삭제된 Skill/Agent (7일간 보관)
    ├── my-skill_2026-01-20_15-30-00/
    │   └── SKILL.md
    └── old-skill_2026-01-19_10-00-00/
        └── SKILL.md

~/.local/state/skillshare/   # XDG_STATE_HOME
├── logs/                    # 작업 로그 (JSONL)
│   ├── operations.log       # install, sync, update 등
│   └── audit.log            # 보안 Audit 스캔
├── mcp/                     # MCP Sync 상태 (자동 관리)
│   ├── state.json           # Skillshare가 소유한 네이티브 항목
│   └── backups/             # 매 작성 전 Agent 파일 백업 (파일당 최신 20개)
└── plugins/                 # 검토된 플러그인 Source 로컬 사본

~/.cache/skillshare/         # XDG_CACHE_HOME      
├── version-check.json       # 버전 확인 캐시 (TTL 24시간)
└── ui/                      # 웹 UI dist 캐시
    └── 0.13.0/              # 버전별 캐시된 에셋
        ├── index.html
        └── assets/
```

---

## 설정 파일

### 위치

```
~/.config/skillshare/config.yaml
```

**XDG로 재정의:**
```
XDG_CONFIG_HOME=/custom/path → /custom/path/skillshare/config.yaml
```

**Windows 기본값:**
```
%AppData%\skillshare\config.yaml
```

### 내용

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/runkids/skillshare/main/schemas/config.schema.json
source: ~/.config/skillshare/skills
agents_source: ~/.config/skillshare/agents  # 선택; 기본값은 <source parent>/agents
mode: merge
targets:
  claude:
    path: ~/.claude/skills
    agents:                                 # 선택; 이 Target에 대한 Agent Sync를 활성화
      path: ~/.claude/agents
  cursor:
    path: ~/.cursor/skills
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
```

전체 레퍼런스는 [Configuration](/docs/reference/targets/configuration)을 참고하세요.

---

## 메타데이터 파일

### 위치

```
~/.config/skillshare/skills/.metadata.json
```

설치되고 Tracked된 Skill에 대한 메타데이터를 저장합니다. Source 디렉터리 내부에 위치하여 여러 머신 설정을 위해 git으로 Sync할 수 있습니다. `install`, `uninstall`, `update`에 의해 **자동 관리**되므로 수동으로 편집하지 마세요.

### 내용

```json
{
  "skills": [
    {
      "name": "pdf",
      "source": "anthropics/skills/skills/pdf"
    },
    {
      "name": "_team-skills",
      "source": "github.com/team/skills",
      "tracked": true
    }
  ]
}
```

각 항목은 Skill 이름과 설치 Source를 기록합니다. Tracked 저장소(`_` 접두사)에는 `update` 및 `check` 작업을 위한 전체 저장소 URL이 포함됩니다.

---

## Source 디렉터리

### 위치

```
~/.config/skillshare/skills/
```

**Windows:**
```
%AppData%\skillshare\skills\
```

### 구조

```
skills/
├── .metadata.json                # 중앙화된 Skill 메타데이터 (자동 관리)
├── skill-name/                   # Skill 디렉터리
│   ├── SKILL.md                  # 필수: Skill 정의
│   ├── examples/                 # 선택: 예제 파일
│   └── templates/                # 선택: 코드 템플릿
├── frontend/                     # 카테고리 폴더 (--into 또는 수동으로 생성)
│   └── react-skill/              # 하위 디렉터리의 Skill
│       └── SKILL.md              # frontend__react-skill로 Sync됨
└── _tracked-repo/                # Tracked 저장소
    ├── .git/                     # Git 히스토리
    └── ...                       # Skill 하위 디렉터리
```

---

## Skill 파일

### SKILL.md (필수)

Skill 정의 파일입니다:

```markdown
---
name: skill-name
description: Brief description
---

# Skill Name

Instructions for the AI...
```

자세한 내용은 [Skill Format](/docs/understand/skill-format)을 참고하세요.

### .skillfollow / .skillfollow.local {#skillfollow-optional}

**설정된 skills source 루트**에 직접 자식 링크 이름（예 `_team-skills`）을 선언합니다. Local은 머신별 이름을 추가하며 ignore pattern이나 중첩 repo 설정이 아닙니다. Source `.gitignore`에 `/_team-skills`, `/.skillfollow.local`（끝 `/` 없음）을 추가하고 머신마다 링크를 만드세요. `_` 접두사와 `.git`은 tracked repo이며 루트 `SKILL.md`는 미지원입니다. [설정/엄격한 이름/안전성](../skillfollow.md) 참조.

### .skillignore (선택) {#skillignore-optional}

Discovery 과정에서 Skill을 제외합니다. 두 가지 위치를 지원합니다:

**저장소 레벨** — Tracked Skill 저장소의 루트에 위치. install Discovery와 이후의 모든 명령어(`doctor`, `status`, `list`, `sync` 등)에 영향을 줍니다:

```text title="_team-skills/.skillignore"
# 벤더 패키지를 Discovery에서 숨김
.venv
node_modules

# 내부 도구 제외
validation-scripts
prompt-eval-*
```

**Source 루트** — Source 디렉터리 루트(`~/.config/skillshare/skills/.skillignore`)에 위치. Tracked 여부와 관계없이 모든 Skill에 전역적으로 적용됩니다:

```text title="~/.config/skillshare/skills/.skillignore"
# 실험적인 Skill을 임시로 숨김
my-experimental-skill

# 모든 draft 제외
draft-*
```

[gitignore 문법](https://git-scm.com/docs/gitignore)을 사용합니다 — 한 줄에 하나의 패턴입니다. `*`(단일 세그먼트), `**`(임의 깊이), `?`, `[abc]`(문자 클래스), `!pattern`(부정), `/pattern`(경로 고정), `pattern/`(디렉터리 전용), `\#`/`\!`(이스케이프된 리터럴)을 지원합니다. `#`로 시작하는 줄은 주석입니다. `internal-tools`와 같은 그룹 이름은 해당 디렉터리 아래의 모든 Skill을 제외하며, `internal-tools/helper`는 특정 Skill만 제외합니다. 두 레이어 모두 적용되며 — 어느 한쪽이라도 일치하면 해당 Skill은 제외됩니다.

:::tip
`.skillignore`는 세 가지 필터링 레이어 중 하나입니다. Target별 필터와 SKILL.md `targets`를 포함한 모든 시나리오는 [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills)를 참고하세요.
:::

### .skillignore.local (선택) {#skillignorelocal-optional}

`.skillignore`와 함께 동작하는 로컬 전용 오버라이드 파일입니다. `.skillignore`와 같은 디렉터리(Source 루트 또는 Tracked 저장소 루트)에 위치시킵니다. `.skillignore.local`의 패턴은 `.skillignore` 뒤에 추가되므로, 부정 패턴(`!pattern`)으로 기본 파일을 오버라이드할 수 있습니다:

```text title="_team-skills/.skillignore.local"
# 저장소의 .skillignore가 private-*를 차단하지만, 나는 내 것이 필요하다
!private-mine
```

이 파일은 버전 관리에 커밋되어서는 **안 됩니다** — `.gitignore`에 추가하세요. 저장소를 사용하는 사람이 저장소 관리자의 `.skillignore`를 수정하지 않고도 로컬에서 오버라이드할 수 있도록 존재합니다.

활성화되면 `sync -v`, `status`, `doctor`가 `.local active` 표시를 보여줍니다.


---

## Agent 파일

Agent는 Skill과는 별도의 리소스 종류입니다. 형제 관계의 Source 디렉터리에 위치하며, Agent를 지원하는 Target(Claude, Cursor, Augment, OpenCode)에 Sync됩니다.

### Agent Source 디렉터리

```
~/.config/skillshare/agents/      # Global mode
.skillshare/agents/               # Project mode
```

Agent Source는 `skillshare init`에 의해 `skills/`와 함께 자동으로 생성됩니다. `agents_source` 설정 필드로 Global 위치를 재정의할 수 있으며, Project mode는 항상 `.skillshare/agents/`를 사용합니다.

### Agent 파일 형식

각 Agent는 frontmatter가 포함된 단일 Markdown 파일입니다:

```markdown title="~/.config/skillshare/agents/reviewer.md"
---
name: reviewer
description: Reviews pull requests for security and style issues.
---

# Reviewer

Instructions for the AI agent...
```

Agent 파일 이름은 `a-z`, `0-9`, `_`, `-`, `.`만 사용해야 합니다. Skill과 달리 Agent는 **단일 파일**이며 — 하위 디렉터리를 포함하지 않습니다.

전체 파일 형식과 Discovery 규칙은 [Agents](/docs/understand/agents)를 참고하세요.

### .agentignore (선택)

Sync에서 Agent를 제외합니다. Agent Source 루트에 위치합니다:

```text title="~/.config/skillshare/agents/.agentignore"
# draft 숨김
draft-*

# 특정 Agent 비활성화
experimental-reviewer
```

[gitignore 문법](https://git-scm.com/docs/gitignore)을 사용합니다. `.skillignore`와 동일한 패턴 규칙(`*`, `**`, `!negation`, `#` 주석 등)이 적용됩니다. 비활성화된 Agent는 Source 디렉터리에 남아 있지만 Sync에서 제외됩니다.

`skillshare disable <agent>`와 `skillshare enable <agent>`가 항목을 자동으로 추가/제거합니다.

### .agentignore.local (선택)

로컬 전용 오버라이드 파일입니다(`.skillignore.local`과 동일한 방식). `.agentignore` 옆에 위치시킵니다. 패턴은 `.agentignore` 뒤에 추가되므로, `!negation` 패턴으로 기본 파일이 비활성화한 Agent를 다시 활성화할 수 있습니다. 버전 관리에 커밋해서는 안 됩니다.

---

## 백업 디렉터리

### 위치

```
~/.local/share/skillshare/backups/
```

### 구조

```
backups/
└── <timestamp>/             # YYYY-MM-DD_HH-MM-SS
    ├── claude/              # Target의 백업
    │   ├── skill-a/
    │   └── skill-b/
    └── cursor/
        └── ...
```

백업은 다음 시점에 생성됩니다:
- `sync`와 `target remove` 실행 전 자동으로
- `skillshare backup`을 통해 수동으로

---

## 휴지통 디렉터리

### 위치

```
~/.local/share/skillshare/trash/
```

**Project mode:**
```
<project>/.skillshare/trash/
```

### 구조

```
trash/
└── <skill-name>_<timestamp>/    # skill-name_YYYY-MM-DD_HH-MM-SS
    ├── SKILL.md
    └── ...                      # 원본 파일이 모두 보존됨
```

휴지통으로 이동된 Skill은:
- `skillshare uninstall`에 의해 생성됨
- 7일간 보관된 후 자동으로 정리됨
- 원래 Skill 이름과 타임스탬프로 이름이 지정됨

---

## 로그 디렉터리

### 위치

```
~/.local/state/skillshare/logs/
```

**Project mode:**
```
<project>/.skillshare/logs/
```

---

## Target 디렉터리

Target은 AI CLI Skill 디렉터리입니다. Sync 후에는 Source에 대한 심볼릭 링크(또는 복사본)가 포함됩니다.

### Merge 모드

각 Skill이 개별적으로 심볼릭 링크됩니다. 매니페스트가 관리 대상 Skill을 추적하여 고아 항목(orphan) 정리를 수행합니다:
```
~/.claude/skills/
├── my-skill -> ~/.config/skillshare/skills/my-skill
├── code-review -> ~/.config/skillshare/skills/code-review
├── local-only/              # 심볼릭 링크되지 않음 (사용자가 만든 것, 보존됨)
└── .skillshare-manifest.json  # 관리 대상 Skill 추적
```

### Copy 모드

각 Skill이 실제 파일로 복사됩니다. 매니페스트가 체크섬을 추적하여 증분 Sync를 수행합니다:
```
~/.cursor/skills/
├── my-skill/                  # 실제 파일 (Source에서 복사됨)
├── code-review/               # 실제 파일
├── local-only/                # 사용자가 만든 것, 보존됨
└── .skillshare-manifest.json  # 관리 대상 Skill + 체크섬 추적
```

### Symlink 모드

전체 디렉터리가 심볼릭 링크됩니다:
```
~/.claude/skills -> ~/.config/skillshare/skills/
```

### 파일 링크 대신 복사본 (Windows) {#copies-in-place-of-file-links-windows}

Developer Mode가 없는 Windows에서는 `merge` 모드의 agent target과 디렉터리 extras가 파일 링크 대신 복사본을 받습니다. 이때 target 디렉터리에는 각 복사본의 checksum을 기록하는 `.skillshare-manifest.json`이 생기며, 이후 sync는 이를 이용해 복사본을 업데이트하거나 정리하고 사용자 파일은 그대로 둡니다:
```
~/.claude/agents/
├── reviewer.md                # Copy of the source agent
├── local-agent.md             # User-created, preserved
└── .skillshare-manifest.json  # Tracks copied agents + checksums
```

single-file extra(공유 `AGENTS.md` 등)는 복사본을 target 디렉터리가 아니라 skillshare의 [extras 백업 폴더](../commands/extras.md#single-file-extras)에 기록합니다.

---

## Tracked 저장소

Tracked 저장소(`--track`로 설치됨)는 git 히스토리를 보존합니다:

```
_team-skills/
├── .git/                    # Git 보존됨
├── frontend/
│   └── ui/
│       └── SKILL.md
└── backend/
    └── api/
        └── SKILL.md
```

### 명명 규칙

- `_` 접두사: Tracked 저장소
- 평탄화된 이름의 `__`: 경로 구분자

**Source에서:**
```
_team-skills/frontend/ui/SKILL.md
```

**Target에서 (평탄화됨):**
```
_team-skills__frontend__ui/SKILL.md
```

---

## 플랫폼별 차이

:::tip XDG Base Directory
skillshare는 XDG Base Directory Specification을 따릅니다. `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`으로 기본 디렉터리를 재정의할 수 있습니다.

자세한 내용은 [Environment Variables](./environment-variables.md#xdg_config_home)를 참고하세요.
:::

### macOS / Linux

| 항목 | 경로 |
|------|------|
| 설정 | `~/.config/skillshare/config.yaml` |
| 메타데이터 | `~/.config/skillshare/skills/.metadata.json` |
| Skill Source | `~/.config/skillshare/skills/` |
| Agent Source | `~/.config/skillshare/agents/` |
| 백업 | `~/.local/share/skillshare/backups/` |
| 휴지통 | `~/.local/share/skillshare/trash/` |
| 로그 | `~/.local/state/skillshare/logs/` |
| 버전 캐시 | `~/.cache/skillshare/version-check.json` |
| UI 캐시 | `~/.cache/skillshare/ui/{version}/` |
| 링크 유형 | 심볼릭 링크 |

### Windows

| 항목 | 경로 |
|------|------|
| 설정 | `%AppData%\skillshare\config.yaml` |
| 메타데이터 | `%AppData%\skillshare\skills\.metadata.json` |
| Skill Source | `%AppData%\skillshare\skills\` |
| Agent Source | `%AppData%\skillshare\agents\` |
| 백업 | `%AppData%\skillshare\backups\` |
| 휴지통 | `%AppData%\skillshare\trash\` |
| 로그 | `%AppData%\skillshare\logs\` |
| 버전 캐시 | `%AppData%\skillshare\version-check.json` |
| UI 캐시 | `%AppData%\skillshare\ui\{version}\` |
| 링크 유형 | 폴더는 NTFS Junction, 단일 파일은 symlink (Developer Mode 필요, 없으면 복사) |

## XDG Base Directory 레이아웃

skillshare는 Unix 시스템에서 [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir/latest/)을 따릅니다:

| XDG 변수 | 기본 경로 | skillshare 사용처 |
|-------------|-------------|---------------------|
| `XDG_CONFIG_HOME` | `~/.config` | `skillshare/config.yaml`, `skillshare/skills/` (`.metadata.json` 포함), `skillshare/agents/` |
| `XDG_DATA_HOME` | `~/.local/share` | `skillshare/backups/`, `skillshare/trash/` |
| `XDG_STATE_HOME` | `~/.local/state` | `skillshare/logs/` |
| `XDG_CACHE_HOME` | `~/.cache` | `skillshare/ui/` (다운로드된 웹 대시보드) |

### Windows 경로

| 용도 | 경로 |
|---------|------|
| 설정 + Skill | `%AppData%\skillshare\` |
| 데이터 (백업, 휴지통) | `%AppData%\skillshare\` |
| 상태 (로그) | `%AppData%\skillshare\` |
| 캐시 (UI) | `%AppData%\skillshare\` |

### 마이그레이션 참고 사항

XDG 분리 이전 버전에서 업그레이드하는 경우, skillshare는 첫 실행 시 이전 위치(`~/.config/skillshare/`)에서 올바른 XDG 디렉터리로 데이터를 자동으로 마이그레이션합니다.

---

## 관련 문서

- [Configuration](/docs/reference/targets/configuration) — 설정 파일 상세
- [Skill Format](/docs/understand/skill-format) — SKILL.md 형식
- [Agents](/docs/understand/agents) — Agent 파일 형식 및 Discovery
- [Tracked Repositories](/docs/understand/tracked-repositories) — Tracked 저장소
