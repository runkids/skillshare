---
sidebar_position: 1
---

# Claude Code와 skillshare 사용하기

> 설치부터 첫 Sync까지 — 5분.

## 사전 요구 사항

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code/overview) 설치 및 정상 동작
- macOS, Linux, 또는 Windows (WSL)

## 1단계: skillshare 설치

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

설치 프로그램이 PATH 설정 안내를 표시할 때만 안내에 따라 설정한 후 아래 명령을 실행하세요. PATH 경고가 없으면 추가 설정은 필요하지 않습니다.

## 2단계: 초기화

```bash
skillshare init
```

이 명령은 Claude Code의 Skill 디렉터리(`~/.claude/skills/`)를 감지하고 자동으로 Target으로 추가합니다.

## 3단계: 첫 번째 Skill 설치

```bash
skillshare install anthropics/courses/prompt-eng
```

Skill이 다운로드되고, 보안 검사를 거친 후, Source 디렉터리에 추가됩니다.

## 4단계: Sync

```bash
skillshare sync
```

이 명령은 Source에서 `~/.claude/skills/`로 심볼릭 링크를 생성합니다. Claude Code는 재시작 없이 즉시 Skill을 인식합니다.

## 5단계: 확인

```bash
ls ~/.claude/skills/
```

설치된 Skill이 심볼릭 링크로 표시되는 것을 확인할 수 있습니다.

## Claude Code 통합 세부 사항

- **Skill 경로**: `~/.claude/skills/` (Global) 또는 `.claude/skills/` (Project)
- **CLAUDE.md**: skillshare Skill은 `SKILL.md` 형식을 사용하며, Claude Code가 이를 기본적으로 읽습니다
- **Project mode**: 저장소 내에서 `skillshare init -p`를 실행하면 프로젝트별로 `.claude/skills/`를 관리할 수 있습니다

## 다음 단계는?

- [여러 Skill 관리하기 →](/docs/how-to/daily-tasks/organizing-skills)
- [팀과 공유하기 →](/docs/how-to/sharing/organization-sharing)
- [더 많은 Skill 탐색하기 →](/docs/reference/commands/search)
