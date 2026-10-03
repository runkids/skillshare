---
sidebar_position: 4
---

# 빠른 참조

skillshare 명령 치트시트입니다.

## 핵심 명령

| 명령 | 설명 |
|---------|-------------|
| `init` | 최초 설정 |
| `install <source>` | Skill 추가 |
| `uninstall <name>...` | 하나 이상의 Skill 제거 |
| `list` | 모든 Skill 목록 표시 |
| `search <query>` | Skill 검색 |
| `sync` | 모든 Target에 반영 |
| `status` | Sync 상태 표시 |

## Skill 관리

| 명령 | 설명 |
|---------|-------------|
| `new <name>` | 새 Skill 생성 |
| `update <name>` | Skill 업데이트 (git pull) |
| `update --all` | 모든 Tracked repo 업데이트 |
| `check` | Skill 업데이트 확인 |
| `check --json` | 업데이트 확인 (JSON 출력) |
| `upgrade` | CLI와 내장 Skill 업그레이드 |
| `hub list` | 설정된 Skill Hub 목록 표시 |
| `hub add <url>` | Skill Hub 추가 |

## Target 관리

| 명령 | 설명 |
|---------|-------------|
| `target list` | 모든 Target 목록 표시 |
| `target <name>` | Target 상세 정보 표시 |
| `target <name> --mode <mode>` | Sync 모드 변경 |
| `target add <name> <path>` | 커스텀 Target 추가 |
| `target remove <name>` | Target을 안전하게 제거 |
| `diff [target]` | 차이점 표시 |

## Extras 관리

| 명령 | 설명 |
|---------|-------------|
| `extras init <name> --target <path>` | 설정에 extras 항목 추가 |
| `extras init <name> --file <file> --target <path>` | 파일 하나를 동기화하는 extra 추가 (`--as`로 target에서의 이름 변경) |
| `extras list` | 설정된 extras를 Sync 상태와 함께 목록 표시 |
| `extras remove <name>` | 설정에서 extras 항목 제거 |
| `extras <name> --add-target <path>` | 기존 extras 항목에 Target 추가 |
| `extras <name> --remove-target <path>` | Target 제거 (동기화된 파일까지 삭제하려면 `--prune` 추가) |
| `extras collect <name>` | extras Target의 로컬 파일을 Source로 수집 |

## Agent 관리

| 명령 | 설명 |
|---------|-------------|
| `list agents` | 설치된 Agent 목록 표시 |
| `install <source> --kind agent` | 리포지터리에서 Agent만 설치 |
| `install <source> -a <name>` | 이름으로 특정 Agent 설치 |
| `uninstall --kind agent <name>` | Agent 제거 |
| `sync agents` | Agent만 Target에 Sync |
| `check agents` | Agent 업데이트 확인 |
| `audit agents` | Agent 보안 스캔 |
| `enable --kind agent <name>` | 비활성화된 Agent 다시 활성화 |
| `disable --kind agent <name>` | `.agentignore`로 Agent 비활성화 |

## 플러그인 관리

| 명령 | 설명 |
|---------|-------------|
| `plugin` | 대화형 플러그인 관리자 열기 |
| `plugin list` | 관리 중인 플러그인과 네이티브 설치 상태 표시 |
| `plugin discover <source>` | 디렉터리 또는 Git 리포지터리 검사 |
| `plugin add [source]` | 완전한 네이티브 플러그인 설치 |
| `plugin import [plugin@market] --from claude` | 기존 네이티브 설치를 인계받기 |
| `plugin inspect <name>` | 관리 중인 패키지 검사 |
| `plugin check [name]` | 적용하지 않고 Source 변경 사항 확인 |
| `plugin update [name] --target claude` | 검토한 Source로 지원 Target 업데이트 |
| `plugin enable [name] --target codex` | 다음 Sync에 포함할 Target 선택 |
| `plugin disable [name] --target codex` | 다음 Sync에서 Target 선택 해제 |
| `plugin remove [name]` | 관리 중인 바인딩을 제거하고 정의 삭제 |
| `sync plugins [name]` | 플러그인 Sync 선택 적용. `plugin sync`의 별칭 |

Target: Claude Code, Codex, Cursor, Antigravity (`agy`), Pi, OpenCode.
Project mode는 Claude, Antigravity, Pi, OpenCode를 지원합니다.

enable/disable은 선택 상태만 저장합니다. 다음 플러그인 Sync에서 선택된 바인딩이
설치되고, 선택 해제된 바인딩은 정의는 유지한 채 제거됩니다.
플러그인은 `sync --all`에서 제외됩니다. 변경 사항을 미리 보려면 `--dry-run --json`을,
자동화에는 명시적인 입력과 함께 `--no-tui`를 사용하세요. 네이티브 클라이언트 요구 사항과
지원 Target은 [plugin](/docs/reference/commands/plugin)을 참고하세요.

## Sync 작업

| 명령 | 설명 |
|---------|-------------|
| `sync extras` | Skill이 아닌 리소스(rules, commands 등) Sync |
| `sync mcp` | MCP 연결 설정 Sync |
| `sync --all` | Skill + Agent + extras + MCP + hooks Sync (플러그인 제외) |
| `collect <target>` | Target의 Skill을 Source로 수집 |
| `collect --all` | 모든 Target에서 수집 |
| `backup [target]` | 백업 생성 |
| `backup --list` | 백업 목록 표시 |
| `restore <target>` | 백업에서 복원 |
| `commit [-m "msg"]` | push 없이 로컬 git 커밋 생성 |
| `push [-m "msg"] [--pull]` | 커밋 후 git 원격 저장소로 push. `--pull`은 먼저 remote 변경 사항을 병합한 뒤 Sync |
| `pull` | git에서 pull한 뒤 Sync |
| `trash list` | 소프트 삭제된 Skill 목록 표시 |
| `trash restore <name>` | 소프트 삭제된 Skill 복원 |

## 유틸리티

| 명령 | 설명 |
|---------|-------------|
| `analyze` | 컨텍스트 윈도우 사용량 분석 (대화형 TUI) |
| `analyze --filter <text>` | 이름/경로 부분 문자열로 Skill 필터링 |
| `analyze --json` | 컨텍스트 사용량을 JSON으로 출력 |
| `doctor` | 문제 진단 |
| `doctor --json` | 문제 진단 (CI용 JSON 출력) |
| `log` | 작업 및 감사 로그 보기 |
| `ui` | `localhost:19420`에서 웹 대시보드 실행 |
| `ui -p` | Project mode로 웹 대시보드 실행 |
| `completion <shell> --install` | 셸 탭 자동완성 설치 (bash/zsh/fish/powershell/nushell) |
| `version` | CLI 버전 표시 |
| `make test-docker` | 오프라인 Docker sandbox 테스트 실행 |
| `make playground` | playground 시작 + 셸 진입 (한 번에) |
| `make playground-down` | playground 중지 및 제거 |
| `./scripts/sandbox.sh <cmd>` | 고급 sandbox 관리 (up/down/shell/reset/status/logs/bare) |
| `make ui-build` | 프런트엔드 빌드 |
| `make build-all` | 프런트엔드를 포함한 전체 바이너리 빌드 |

---

## 자주 쓰는 워크플로

### Skill 설치하고 Sync하기
```bash
skillshare install anthropics/skills/skills/pdf
skillshare sync
```

### Skill 만들고 배포하기
```bash
skillshare new my-skill
# ~/.config/skillshare/skills/my-skill/SKILL.md 편집
skillshare sync
```

### 머신 간 Sync
```bash
# 설정 (둘 중 하나 선택)
# 대화형 (안내 프롬프트 사용)
skillshare init --remote git@github.com:you/my-skills.git

# 비대화형 (프롬프트 없음, 설치된 Target 자동 감지)
skillshare init --remote git@github.com:you/my-skills.git --no-copy --all-targets --no-skill

# push 없이 로컬 체크포인트만 남기기 (선택 사항)
skillshare commit -m "Save local skill edits"

# 머신 A: 변경 사항 push
skillshare push -m "Add new skill"

# 머신 B: pull 후 Sync
skillshare pull

# 여러 머신에서 편집하는 경우: 한 번의 명령으로 양방향 동기화
skillshare push --pull -m "Update skills"
```

나중에 선택적으로 실행 (설정 이후 AI CLI를 추가로 설치한 경우에만):

```bash
skillshare init --discover
```

discover 중에 모드를 재정의하면, 새로 추가된 Target만 영향을 받습니다.

```bash
skillshare init --discover --select cursor --mode copy
```

### 팀 Skill 공유
```bash
# 팀 리포지터리 설치
skillshare install github.com/team/skills --track

# 특정 브랜치에서 설치 (--track 유무와 관계없이 동작)
skillshare install github.com/team/skills --branch develop --all
skillshare install github.com/team/skills --track --branch develop

# tag 또는 commit SHA로 고정 (일반 설치만)
skillshare install github.com/team/skills --branch v1.2.0 --all

# 팀 리포지터리에서 업데이트
skillshare update --all
skillshare sync
```

### Sandbox playground 세션
```bash
make playground          # 시작 + 셸 진입
skillshare --help
ss status
exit                     # 셸 나가기
make playground-down     # 컨테이너 중지
```

---

## 주요 경로

| 경로 | 설명 |
|------|-------------|
| `~/.config/skillshare/config.yaml` | 설정 파일 |
| `~/.config/skillshare/skills/.metadata.json` | 설치된 Skill 메타데이터 (자동 관리) |
| `~/.config/skillshare/skills/` | Skill Source 디렉터리 |
| `~/.config/skillshare/agents/` | Agent Source 디렉터리 |
| `~/.config/skillshare/extras/<name>/` | Extras Source 디렉터리 |
| `~/.local/state/skillshare/logs/` | 작업 및 감사 로그 |
| `~/.local/share/skillshare/backups/` | 백업 디렉터리 |

---

## 대부분의 명령에서 쓸 수 있는 플래그

| 플래그 | 설명 |
|------|-------------|
| `--dry-run`, `-n` | 변경 없이 미리 보기 |
| `--help`, `-h` | 도움말 표시 |

---

## 함께 보기

- [명령 레퍼런스](/docs/reference/commands) — 전체 명령 문서
- [개념](/docs/understand) — 핵심 개념 설명
