---
sidebar_position: 4
---

# log

디버깅과 컴플라이언스를 위한 영구 operations 및 audit 로그를 확인합니다.

```bash
skillshare log                    # 대화형 TUI (TTY에서 기본값)
skillshare log --audit            # audit 로그만 표시
skillshare log --tail 50          # 마지막 50개 항목 표시
skillshare log --cmd sync         # sync 항목만 표시
skillshare log --status error     # 오류만 표시
skillshare log --since 2d         # 최근 2일간 항목
skillshare log --stats            # 요약 통계 표시
skillshare log --json             # JSONL로 출력
skillshare log --no-tui           # 일반 텍스트 출력
skillshare log --clear            # operations 로그 지우기
skillshare log -p                 # project operations + audit 로그
```

## 사용 시점

- 로그 항목을 대화형으로 탐색하고 필터링
- 실패한 작업 중 발생한 일을 디버깅
- 컴플라이언스나 문제 해결을 위해 audit trail 검토
- 조사를 위해 명령, 상태, 시간 범위로 로그 필터링

## 대화형 TUI

TTY에서 `skillshare log`는 다음 기능을 갖춘 대화형 터미널 UI를 실행합니다:

- **퍼지 필터링** — 타임스탬프, 명령, 상태, source, 세부 내용으로 필터링하기 위해 입력
- **키보드 탐색** — 화살표 키로 탐색, `q`로 종료
- **상세 패널** — 선택된 항목의 전체 타임스탬프, 명령, 상태, 소요 시간, source, 구조화된 args 표시
- **Stats 푸터** — 항상 표시되는 간략한 요약: 작업 수, 성공률, 마지막 작업
- **Stats 패널** — `s`를 눌러 명령별 성공/실패 개수를 포함한 전체 분석 토글
- **병합된 뷰** — 둘 다 볼 때 operations와 audit 항목이 병합되어 시간순 정렬됨

TUI를 건너뛰고 일반 텍스트로 출력하려면 `--no-tui`를 사용하십시오:

```bash
skillshare log --no-tui           # 일반 텍스트 출력
skillshare log --no-tui | less    # 수동으로 pager에 파이프
```

## 로그에 기록되는 내용

변경을 일으키는 모든 CLI 및 Web UI 작업은 타임스탬프, 명령, 상태, 소요 시간, 관련 args와 함께 JSONL 항목으로 기록됩니다.

| Command | Log File |
|---------|----------|
| `install`, `uninstall`, `sync`, `push`, `pull`, `collect`, `backup`, `restore`, `update`, `target`, `trash`, `config`, `check`, `diff`, `init`, `upgrade` | `operations.log` |
| `audit` | `audit.log` |

이 API들을 호출하는 Web UI 작업도 CLI 작업과 동일하게 기록됩니다.

## 로그 종류

### 기본 뷰
하나의 출력에 **두 섹션 모두** 표시:
- Operations 로그
- Audit 로그

```bash
skillshare log
```

### Audit 전용 뷰

보안 audit 스캔을 일반 작업과 별도로 기록합니다.

```bash
skillshare log --audit
```

### 필터링

명령, 상태, 시간 범위로 결과를 좁힙니다. `--cmd`가 특정 로그를 대상으로 할 때(예: `--cmd audit`는 audit.log에만 나타남), 관련 없는 섹션은 자동으로 생략됩니다.

```bash
skillshare log --cmd install              # install 항목만
skillshare log --status error             # 오류만
skillshare log --since 1h                 # 최근 1시간 (30m, 2d, 1w도 가능)
skillshare log --since 2026-01-15         # 특정 날짜 이후
skillshare log --cmd sync --status error  # 필터 조합
```

### JSON 출력

스크립팅과 자동화를 위해 원시 JSONL을 출력합니다:

```bash
skillshare log --json                     # 모든 항목을 JSONL로
skillshare log --json --cmd sync          # 필터링된 JSONL
```

## 출력 예시 (일반 텍스트)

`--no-tui` 사용 시 또는 non-TTY 환경에서:

```
Operations · last 2
✓ sync      2026-02-10 14:35 · 0.3s
            targets: 3
            scope: global

✗ sync      2026-02-10 14:31 · 0.8s · error
            targets: 3
            failed: 1
            scope: global
  ~/.local/state/skillshare/logs/operations.log

Audit · last 1
✗ audit     2026-02-10 14:36 · 1.1s · blocked
            scope: all-skills
            scanned: 12
            passed: 11
            failed: 1
            failed skills:
              - prompt-injection-skill
              - data-exfil-skill
  ~/.local/state/skillshare/logs/audit.log
```

## 로그 형식

항목은 JSONL 형식(줄당 하나의 JSON 객체)으로 저장됩니다:

```json
{"ts":"2026-02-10T14:30:00Z","cmd":"install","args":{"source":"anthropics/skills/pdf"},"status":"ok","ms":1200}
```

| Field | 설명 |
|-------|------|
| `ts` | ISO 8601 타임스탬프 |
| `cmd` | 명령 이름 |
| `args` | 명령별 컨텍스트 (source, name, target 등) |
| `status` | `ok`, `error`, `partial`, `blocked` 중 하나 |
| `msg` | 오류 메시지 (status가 ok가 아닐 때) |
| `ms` | 소요 시간 (밀리초) |

## 로그 위치

```
~/.local/state/skillshare/logs/operations.log    # global operations
~/.local/state/skillshare/logs/audit.log         # global audit
<project>/.skillshare/logs/operations.log   # project operations
<project>/.skillshare/logs/audit.log        # project audit
```

## Git에서 로그 추적하기 (Project Mode)

Project mode는 노이즈가 많은 커밋을 피하기 위해 기본적으로 `.skillshare/logs/`를 무시합니다.

팀이 로그 파일을 버전 관리하려면, 관리되는 블록 뒤에 `.skillshare/.gitignore`에 다음 **사용자 재정의** 규칙을 추가하십시오:

```gitignore
# User override: track logs
!logs/
!logs/*.log
```

저장소 루트의 `.gitignore`도 `.skillshare/`를 무시하는 경우, 그곳에도 일치하는 unignore 규칙을 추가하십시오.

## 옵션

| Flag | 설명 |
|------|------------|
| `-a`, `--audit` | audit 로그만 표시 |
| `-t`, `--tail <N>` | 마지막 N개 항목 표시 (기본값: 20) |
| `--cmd <name>` | 명령 이름으로 필터링 (예: `sync`, `install`, `audit`) |
| `--status <status>` | 상태로 필터링 (`ok`, `error`, `partial`, `blocked`) |
| `--since <dur\|date>` | 시간으로 필터링 (`30m`, `2h`, `2d`, `1w`, 또는 `2006-01-02`) |
| `--stats` | 요약 통계 표시 (전체, 성공률, 명령별 분석) |
| `--json` | 원시 JSONL 출력 (줄당 하나의 JSON 객체) |
| `--no-tui` | 대화형 TUI 비활성화, 일반 텍스트 출력 사용 |
| `-c`, `--clear` | 선택한 로그 파일 지우기 (기본값은 operations, `--audit`와 함께 사용 시 audit) |
| `-p`, `--project` | project 수준 로그 사용 |
| `-g`, `--global` | global 로그 사용 |
| `-h`, `--help` | 도움말 표시 |

## Web UI

로그는 `/log` 경로의 web dashboard에서도 사용할 수 있습니다:

```bash
skillshare ui
# Log 페이지로 이동
```

Log 페이지는 다음을 제공합니다:
- `All`, `Operations`, `Audit`에 대한 **탭**
- 명령, 상태, 시간 범위(1h, 24h, 7d, 30d)에 대한 **필터**
- 시간, 명령, 세부 정보, 상태, 소요 시간을 보여주는 **표 뷰**
- 있을 경우 실패/경고 skill 이름을 보여주는 **Audit 상세 행**
- **Clear** 및 **Refresh** 컨트롤

## Stats 뷰

### CLI

```bash
skillshare log --stats                # 모든 작업의 요약
skillshare log --stats --cmd sync     # sync만의 통계
skillshare log --stats --since 7d     # 최근 7일간의 통계
```

### TUI

TUI에서 `s`를 눌러 stats 패널을 토글하면 다음이 표시됩니다:

- 전체 작업 수와 수평 막대 차트가 포함된 명령별 분석
- 명령별 성공/실패 개수 (색상 구분)
- 시각적 진행률 바가 포함된 전체 성공률
- 마지막 작업 타임스탬프

푸터 바는 항상 간략한 요약을 표시합니다: `20 ops | ✓ 92.3% | last: sync 2h ago`

### 상세 패널 스크롤링

로그 항목에 긴 상세 내용이 있는 경우(예: 많은 skill이 포함된 audit 결과), `j`/`k`를 사용해 상세 패널을 위아래로 스크롤하십시오.

## 로그 보존

로그는 무제한 증가를 방지하기 위해 자동으로 잘립니다. 기본 제한은 **로그 파일당 1000개 항목**입니다(CLI 또는 Web UI 작업 1건 = 항목 1개). `operations.log`와 `audit.log`는 별도로 관리됩니다.

기본값을 재정의하려면 `config.yaml`에 다음을 추가하십시오:

```yaml
log:
  max_entries: 500  # 파일당 항목 수; 0 = 무제한 (기본값: 1000)
```

## 참고

- [audit](/docs/reference/commands/audit) — 보안 스캔 (audit.log에 기록됨)
- [status](/docs/reference/commands/status) — 현재 sync 상태 표시
- [doctor](/docs/reference/commands/doctor) — 설정 문제 진단
