---
sidebar_position: 6
---

# Recipe: Team Onboarding

> 새 팀원에게 팀이 공유하는 Skill과 프로젝트 맥락을 제공합니다.

## 시나리오

새로운 개발자가 팀에 합류합니다. 다음이 필요합니다:
- 조직 전체 Skill (코딩 표준, 리뷰 가이드라인)
- 프로젝트별 Skill (도메인 지식, 아키텍처 규칙)
- 사용 중인 모든 AI 도구(Claude Code, Pi 등)에서 동작하는 환경

한 팀원은 Claude Code, 다른 팀원은 Codex, 새 팀원은 Pi를 사용합니다. 세 사람 모두 피해야 할 레거시 API를 설명하는 리뷰 체크리스트가 필요합니다. 각 도구에 체크리스트를 복사하면 별개의 버전이 생기고, 프로젝트가 바뀌면서 내용이 달라집니다.

프로젝트의 로컬 Skill, `.skillshare/config.yaml`, `.skillshare/skills.lock.json`을 프로젝트 저장소에 보관하세요. Config는 원격 Skill과 Target을 선언하고, lockfile은 원격 Skill의 커밋을 기록합니다. 각 팀원은 이 파일들을 로컬에 적용합니다. 공유 지시문은 공통 맥락을 제공하지만, 각 도구의 권한과 동작은 그대로 유지됩니다.

## 해결 방법

### 1단계: 온보딩 스크립트 작성

팀 위키나 저장소에 `scripts/setup-skills.sh`로 저장하세요:

```bash
#!/bin/bash
set -e

echo "Installing skillshare..."
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"

echo "Initializing..."
skillshare init -g

echo "Installing organization skills..."
skillshare install github.com/your-org/org-skills --track -g

echo "Running security audit..."
skillshare audit -g --threshold high

echo "Syncing to all AI tools..."
skillshare sync -g

echo "Done! Run 'skillshare list -g' to see installed skills."
```

### 2단계: 신규 입사자가 스크립트 실행

```bash
curl -fsSL https://your-org.github.io/setup-skills.sh | sh
```

또는 스크립트가 팀 저장소에 있는 경우:

```bash
git clone your-org/team-tools
./team-tools/scripts/setup-skills.sh
```

### 3단계: 프로젝트별 설정

관리자는 먼저 [프로젝트 설정](/docs/how-to/sharing/project-setup)에 따라 설정하고 프로젝트 config, 로컬 Skill, lockfile을 커밋합니다. 새 팀원이 해당 프로젝트를 클론하면:

```bash
cd your-project
skillshare install -p
skillshare audit -p --threshold high
skillshare sync -p
```

`install -p`는 `.skillshare/config.yaml`에 선언된 원격 Skill을 설치하며, 고정된 커밋이 있으면 그 버전을 사용합니다. `audit -p`는 커밋된 로컬 Skill을 포함한 프로젝트 Skill을 검사합니다. `sync -p`는 설정된 Target에 배포합니다. 순서대로 실행하고, 어느 단계든 실패하면 중단하세요. Audit으로 차단된 내용은 동기화 전에 검토해야 합니다.

프로젝트 업데이트를 pull한 뒤에도 이 절차를 반복하세요. Git은 config와 lockfile을 가져오지만, 누락된 원격 Skill을 설치하거나 Target의 복사본을 갱신하지는 않습니다. 의도적인 Skill 업데이트는 PR에서 검토하고, 생성된 lockfile 변경 사항도 커밋하세요.

### 4단계: 모든 것이 동작하는지 확인

```bash
# Global Skill 확인
skillshare list -g

# Project Skill 확인
skillshare list -p

# Sync 상태 확인
skillshare status -p
```

## 확인

- `skillshare list -g`에 조직 Skill이 표시됩니다
- `skillshare list -p`에 프로젝트의 로컬 Skill과 설치된 원격 Skill이 표시됩니다
- `skillshare status -p`에서 설정된 프로젝트 Target이 동기화되었음을 확인합니다
- 설정된 AI 도구를 열어 예상한 Skill이 있는지 확인하고, 알려진 레거시 API 예제로 리뷰 체크리스트를 시험합니다

## 변형

- **Dev container 온보딩**: 팀에서 dev container를 사용한다면 `.devcontainer/Dockerfile`과 `postCreateCommand`에 skillshare를 추가하세요 — 컨테이너가 시작될 때 Skill이 준비됩니다
- **Homebrew 기반 설치**: macOS/Linux 팀에서는 `curl | sh` 대신 `brew install skillshare`를 사용하세요
- **Hub 검색**: 신규 입사자에게 Hub를 알려주세요: `skillshare search --hub https://your-org.github.io/skillshare-hub.json`

## 관련 문서

- [시작하기 가이드](/docs/getting-started)
- [조직 공유](/docs/how-to/sharing/organization-sharing)
- [Project 설정](/docs/how-to/sharing/project-setup)
- [Dev container 가이드](/docs/learn/with-devcontainer)
