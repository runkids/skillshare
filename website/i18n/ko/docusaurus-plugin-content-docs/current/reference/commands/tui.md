---
sidebar_position: 5
---

# tui

인터랙티브 TUI 모드를 전역으로 켜고 끕니다.

## 언제 사용하나요

- 모든 명령에서 인터랙티브 TUI 대신 일반 텍스트 출력을 선호할 때
- CI/CD 파이프라인이나 비인터랙티브 환경에서 skillshare를 실행할 때
- TUI를 비활성화한 후 다시 활성화하고 싶을 때

## 사용법

```bash
skillshare tui          # 현재 상태 표시
skillshare tui on       # 모든 명령에 대해 TUI 활성화
skillshare tui off      # 모든 명령에 대해 TUI 비활성화 (일반 텍스트 출력)
```

## 동작 방식

TUI가 비활성화되면, 평소 인터랙티브 인터페이스를 실행하는 명령(`list`, `log`, `search`, `audit rules`, `trash`, `restore`, `diff`, `target list`, `mcp`)이 일반 텍스트 출력으로 대체됩니다. 이는 모든 명령에 `--no-tui`를 붙이는 것과 동일합니다.

| 상태 | 의미 |
|-------|------|
| `on (default)` | 설정 파일에 TUI 키가 없음 — TUI 활성화 |
| `on` | 명시적으로 활성화됨 |
| `off` | 명시적으로 비활성화됨 |

이 설정은 `config.yaml`에 `tui: false`로 저장됩니다. 이 키를 제거하면 기본값(활성화)으로 돌아갑니다.

## 우선순위

개별 명령의 `--no-tui` 플래그는 항상 전역 설정보다 우선합니다. 예를 들어 `tui on`이 설정되어 있어도 `skillshare list --no-tui`는 TUI를 비활성화합니다.

## 예시

```
$ skillshare tui
  TUI  on · default

$ skillshare tui off
✓ TUI disabled

$ skillshare tui
  TUI  off

$ skillshare tui on
✓ TUI enabled
```

## 참고 항목

- [list](./list.md) — Skill 목록 표시 (활성화 시 TUI 사용)
- [log](./log.md) — 작업 로그 보기 (활성화 시 TUI 사용)
- [target](./target.md) — 타겟 목록 (활성화 시 TUI 사용)
- [audit rules](./audit-rules.md) — 감사 규칙 브라우저 (활성화 시 TUI 사용)
