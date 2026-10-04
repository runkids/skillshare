---
sidebar_position: 1
---

# Commands

모든 skillshare 명령어에 대한 전체 레퍼런스입니다.

## 무엇을 하고 싶으신가요?

| 하고 싶은 작업... | 명령어 |
|--------------|---------|
| skillshare를 처음 설정하기 | [`init`](./init.md) |
| GitHub에서 skill 설치하기 | [`install`](./install.md) |
| 나만의 skill 만들기 | [`new`](./new.md) |
| 모든 AI CLI에 skill 동기화하기 | [`sync`](./sync.md) |
| 동기화되지 않은 항목 확인하기 | [`status`](./status.md) / [`diff`](./diff.md) |
| 커뮤니티 skill 검색하기 | [`search`](./search.md) |
| 설치된 skill 업데이트하기 | [`check`](./check.md) 후 [`update`](./update.md) |
| skill을 제거하지 않고 임시로 숨기기 | [`enable` / `disable`](./enable.md) |
| 다른 곳에서 편집하는 링크된 repository의 skill 찾기 | [`follow`](./follow.md) / [`unfollow`](./unfollow.md) |
| git으로 변경사항 저장 또는 동기화하기 | [`commit`](./commit.md) / [`push`](./push.md) / [`pull`](./pull.md) |
| 모든 도구에 대해 MCP 서버를 한 번에 설정하기 | [`mcp`](./mcp.md) |
| 지원되는 도구 전반에서 완전한 plugin 관리하기 | [`plugin`](./plugin.md) |
| skill이 아닌 리소스(rules, commands) 관리하기 | [`extras`](./extras.md) |
| 단일 파일 `.md` agent 관리하기 | 대부분의 명령어가 `agents` 또는 `--kind agent`를 지원합니다 — [Agents](/docs/understand/agents) 참고 |
| 어떤 skill이 가장 많은 context 토큰을 사용하는지 확인하기 | [`analyze`](./analyze.md) |
| 문제 해결하기 | [`doctor`](./doctor.md) |
| 셸에서 탭 완성 활성화하기 | [`completion`](./completion.md) |
| 웹 대시보드 열기 | [`ui`](./ui.md) |

---

## 개요

| 카테고리 | 명령어 |
|----------|----------|
| **Core** | `init`, `install`, `uninstall`, `list`, `search`, `sync`, `status` |
| **Skill Management** | `new`, `check`, `update`, `upgrade`, `enable`, `disable`, `follow`, `unfollow` |
| **MCP Connections** | `mcp` (`add`, `edit`, `import`, `list`, `remove`, `restore`), `sync mcp` |
| **Plugin Management** | `plugin` (`list`, `discover`, `add`, `import`, `inspect`, `sync`, `check`, `update`, `enable`, `disable`, `remove`) |
| **Target Management** | `target`, `diff` |
| **Extras Management** | `extras` (`init`, `list`, `remove`, `collect`) |
| **Sync Operations** | `collect`, `backup`, `restore`, `trash`, `commit`, `push`, `pull` |
| **Security & Utilities** | `analyze`, `audit`, `hub`, `log`, `doctor`, `tui`, `ui`, `completion`, `version` |

---

## Core Commands

| 명령어 | 설명 |
|---------|-------------|
| [init](./init.md) | 최초 설정 |
| [install](./install.md) | 저장소 또는 경로에서 skill 추가 |
| [uninstall](./uninstall.md) | skill 제거 |
| [list](./list.md) | 모든 skill 목록 표시 |
| [search](./search.md) | skill 검색 |
| [sync](./sync.md) | 모든 target으로 skill 전송 |
| [status](./status.md) | 동기화 상태 표시 |

## Skill Management

| 명령어 | 설명 |
|---------|-------------|
| [new](./new.md) | 새 skill 생성 |
| [check](./check.md) | 사용 가능한 업데이트 확인 |
| [update](./update.md) | skill 또는 tracked 저장소 업데이트 |
| [upgrade](./upgrade.md) | CLI 또는 built-in skill 업그레이드 |
| [enable / disable](./enable.md) | skill을 임시로 활성화/비활성화 |
| [follow](./follow.md) | skills source의 첫 번째 단계 링크 따라가기 |
| [unfollow](./unfollow.md) | 첫 번째 단계 링크 따라가기 중지 |

## Target Management

| 명령어 | 설명 |
|---------|-------------|
| [target](./target.md) | target 관리 |
| [diff](./diff.md) | Source와 target 간 차이 표시 |

## Extras Management

| 명령어 | 설명 |
|---------|-------------|
| [extras](./extras.md) | skill이 아닌 리소스(rules, commands, prompts) 관리 |

## MCP and Plugins

| 명령어 | 설명 |
|---------|-------------|
| [mcp](./mcp.md) | MCP 서버를 한 번 정의하고 각 도구의 네이티브 설정에 동기화 |
| [hooks](./hooks.md) | 네이티브 hooks 관리 |
| [plugin](./plugin.md) | 완전한 plugin을 설치하고 어떤 도구가 받을지 선택 |

## Sync Operations

| 명령어 | 설명 |
|---------|-------------|
| [collect](./collect.md) | target에서 source로 skill 수집 |
| [backup](./backup.md) | target 백업 생성 |
| [restore](./restore.md) | 백업에서 target 복원 |
| [trash](./trash.md) | trash에 있는 제거된 skill 관리 |
| [commit](./commit.md) | push 없이 로컬 git commit 생성 |
| [push](./push.md) | git remote에 commit 및 push |
| [pull](./pull.md) | git remote에서 pull 후 동기화 |

## Security & Utilities

| 명령어 | 설명 |
|---------|-------------|
| [analyze](./analyze.md) | context window 사용량 분석 |
| [audit](./audit.md) | 보안 위협에 대해 skill 스캔 |
| [log](./log.md) | 작업 및 audit 로그 확인 |
| [doctor](./doctor.md) | 문제 진단 |
| [tui](./tui.md) | 대화형 TUI 모드 전환 |
| [ui](./ui.md) | 웹 대시보드 실행 |
| [hub](./hub.md) | skill hub 소스 관리 |
| [completion](./completion.md) | 셸 completion 스크립트 생성 |
| [version](./version.md) | CLI 버전 표시 |

---

## Common Flags

대부분의 명령어가 다음을 지원합니다:

| Flag | 설명 |
|------|-------------|
| `--dry-run`, `-n` | 변경 없이 미리보기 |
| `--help`, `-h` | 도움말 표시 |

---

## Quick Reference

```bash
# Setup
skillshare init
skillshare init --remote git@github.com:you/skills.git

# Install skills
skillshare install anthropics/skills/skills/pdf
skillshare install github.com/team/skills --track

# Create skill
skillshare new my-skill

# Sync
skillshare sync
skillshare sync --dry-run

# Git checkpoints / cross-machine
skillshare commit -m "Update skill"
skillshare push -m "Add skill"
skillshare pull

# Status
skillshare status
skillshare list
skillshare diff

# Enable/disable skills
skillshare disable draft-*
skillshare enable draft-*

# Maintenance
skillshare update --all
skillshare analyze
skillshare audit
skillshare log
skillshare doctor
skillshare backup

# TUI preferences
skillshare tui            # Show current status
skillshare tui off        # Disable interactive TUI
skillshare tui on         # Re-enable TUI

# Web UI
skillshare ui
skillshare ui -p          # Project mode

# Hub
skillshare hub list
skillshare hub add https://hub.example.com/index.json

# Check for updates
skillshare check

# Trash management
skillshare trash list
skillshare trash restore my-skill

# Shell completion
skillshare completion bash --install
skillshare completion zsh --install

# Version
skillshare version
```

---

## Related

- [Quick Reference](/docs/getting-started/quick-reference) — 명령어 치트 시트
- [Workflows](/docs/how-to/daily-tasks) — 일반적인 사용 패턴
