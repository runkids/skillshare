---
sidebar_position: 3
---

# Codex와 함께 skillshare 사용하기

> 설치부터 첫 Sync까지 — 5분.

## 사전 준비

- [OpenAI Codex CLI](https://github.com/openai/codex) 설치 및 정상 동작 확인
- macOS, Linux, 또는 Windows

## 1단계: skillshare 설치

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

설치 프로그램이 PATH 설정 안내를 표시할 때만 안내에 따라 설정한 후 아래 명령을 실행하세요. PATH 경고가 없으면 추가 설정은 필요하지 않습니다.

## 2단계: 초기화

```bash
skillshare init
```

Codex는 `~/.agents/skills/`를 읽습니다. 이는 Codex가 사용자 수준 Skill 경로로 문서화한 공유 디렉터리입니다. `init`은 Codex의 설정 디렉터리(`~/.codex/`)를 통해 Codex를 감지하여 공유 `universal` Target을 자동으로 설정합니다.

## 3단계: 첫 Skill 설치

```bash
skillshare install runkids/my-skills
```

## 4단계: Sync

```bash
skillshare sync
```

Skill이 `~/.agents/skills/`에 심볼릭 링크로 연결됩니다.

## 5단계: 확인

```bash
ls ~/.agents/skills/
```

설치한 Skill이 심볼릭 링크로 표시되는 것을 확인할 수 있습니다.

## Codex 관련 참고 사항

- **Skill 경로**: `~/.agents/skills/` (Global mode) 또는 `.agents/skills/` (Project mode)
- **기존 설정**: 설정에서 `codex`가 여전히 `~/.codex/skills`를 가리키고 있다면 Codex는 계속 그 경로를 읽습니다. 다만 `universal`도 함께 활성화되어 있으면 모든 Skill이 두 번씩 표시되므로, `codex` Target을 제거하세요(`skillshare target remove codex --dry-run`으로 미리 확인할 수 있습니다)
- **설명 길이 제한**: Codex는 Skill 설명에 1024자 제한이 있습니다. `SKILL.md` frontmatter의 `description` 필드를 간결하게 유지하세요
- **Project mode**: `skillshare init -p`를 실행하면 프로젝트 단위 Codex Skill을 관리할 수 있습니다

## 다음 단계는?

- [여러 Skill 관리하기 →](/docs/how-to/daily-tasks/organizing-skills)
- [팀과 공유하기 →](/docs/how-to/sharing/organization-sharing)
- [더 많은 Skill 탐색하기 →](/docs/reference/commands/search)
