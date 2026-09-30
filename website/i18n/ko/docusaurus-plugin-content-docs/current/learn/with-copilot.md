---
sidebar_position: 2
---

# GitHub Copilot와 함께 skillshare 사용하기

> 설치부터 첫 Sync까지 — 5분.

## 사전 준비

- VS Code 또는 JetBrains에서 [GitHub Copilot](https://github.com/features/copilot) 코딩 에이전트 활성화
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

Copilot의 Skill 디렉터리(`~/.copilot/skills/`)를 감지하여 자동으로 Target으로 추가합니다.

## 3단계: Copy 모드로 전환 (권장)

Copilot이 간혹 심볼릭 링크를 제대로 따라가지 못한다는 보고를 받았습니다. 문제를 피하려면 Copilot Target을 **copy 모드**로 전환하세요:

```bash
skillshare target copilot --mode copy
```

Copy 모드는 심볼릭 링크를 만드는 대신 Skill 파일을 `~/.copilot/skills/`에 실제로 복사합니다. 트레이드오프는 Source의 수정 사항이 즉시 반영되지 않는다는 점입니다 — 변경 사항을 반영하려면 `skillshare sync`를 실행해야 합니다. 하지만 플랫폼 간 신뢰성은 더 높습니다.

:::tip merge (symlink) 모드를 사용해야 할 때
macOS나 Linux를 사용 중이고 사용자의 기기에서 Copilot이 심볼릭 링크를 제대로 읽는다면, 기본값인 merge 모드로도 문제없이 동작합니다. 언제든 다시 전환할 수 있습니다:

```bash
skillshare target copilot --mode merge
```
:::

## 4단계: 첫 Skill 설치

```bash
skillshare install runkids/my-skills
```

## 5단계: Sync

```bash
skillshare sync
```

Skill이 `~/.copilot/skills/`에 복사됩니다. Copilot은 이를 사용자 지정 명령어(custom instructions)로 인식합니다.

## 6단계: 확인

```bash
ls ~/.copilot/skills/
```

설치한 Skill이 실제 디렉터리(copy 모드) 또는 심볼릭 링크(merge 모드)로 표시되는 것을 확인할 수 있습니다.

## Copilot 관련 참고 사항

- **Skill 경로**: `~/.copilot/skills/` (Global mode) 또는 `.github/skills/` (Project mode)
- **Agent 경로**: `~/.copilot/agents/` (Global mode) 또는 `.github/agents/` (Project mode) — Copilot CLI는 skillshare가 관리하는 것과 동일한 `.agent.md` 형식으로 커스텀 Agent를 읽으므로, `skillshare sync agents`는 변환 없이 그대로 배포합니다. [Agents](/docs/understand/agents)를 참고하세요.
- **Project mode**: `skillshare init -p`를 실행하면 프로젝트 단위 Copilot Skill을 관리할 수 있으며, 코드베이스와 함께 `.github/skills/`에 저장됩니다
- **심볼릭 링크 문제**: Copilot이 Skill을 인식하지 못한다면 해당 Target이 merge 모드인지(`skillshare status`) 확인하고, 위에 설명한 대로 copy 모드로 전환하세요
- **`.github/copilot-instructions.md`**: 기존 명령어 파일이 있다면, skillshare Skill은 이를 대체하지 않고 보완합니다

## 다음 단계는?

- [여러 Skill 관리하기 →](/docs/how-to/daily-tasks/organizing-skills)
- [팀과 공유하기 →](/docs/how-to/sharing/organization-sharing)
- [더 많은 Skill 탐색하기 →](/docs/reference/commands/search)
