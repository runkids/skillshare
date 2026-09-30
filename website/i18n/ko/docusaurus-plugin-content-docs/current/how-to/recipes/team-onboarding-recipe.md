---
sidebar_position: 6
---

# Recipe: Team Onboarding

> 5분 이내에 새 팀원의 AI Skill 환경을 설정합니다.

## 시나리오

새로운 개발자가 팀에 합류합니다. 다음이 필요합니다:
- 조직 전체 Skill (코딩 표준, 리뷰 가이드라인)
- 프로젝트별 Skill (도메인 지식, 아키텍처 규칙)
- 사용 중인 모든 AI 도구(Claude Code, Cursor 등)에서 동작하는 환경

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
skillshare init

echo "Installing organization skills..."
skillshare install your-org/org-skills

echo "Running security audit..."
skillshare audit

echo "Syncing to all AI tools..."
skillshare sync

echo "Done! Run 'skillshare list' to see installed skills."
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

신규 입사자가 프로젝트를 클론할 때:

```bash
cd your-project
skillshare sync -p
```

이 명령은 Project 범위의 Skill을 자동으로 가져옵니다.

### 4단계: 모든 것이 동작하는지 확인

```bash
# Global Skill 확인
skillshare list

# Project Skill 확인
skillshare list -p

# Sync 상태 확인
skillshare status
```

## 확인

- `skillshare list`가 조직 Skill을 표시합니다
- `skillshare status`가 모든 Target이 동기화되었음을 보여줍니다
- Claude Code / Cursor를 열면 Skill이 로드되어 있는 것이 보입니다

## 변형

- **Dev container 온보딩**: 팀에서 dev container를 사용한다면 `.devcontainer/Dockerfile`과 `postCreateCommand`에 skillshare를 추가하세요 — 컨테이너가 시작될 때 Skill이 준비됩니다
- **Homebrew 기반 설치**: macOS/Linux 팀에서는 `curl | sh` 대신 `brew install skillshare`를 사용하세요
- **Hub 검색**: 신규 입사자에게 Hub를 알려주세요: `skillshare search --hub https://your-org.github.io/skillshare-hub.json`

## 관련 문서

- [시작하기 가이드](/docs/getting-started)
- [조직 공유](/docs/how-to/sharing/organization-sharing)
- [Project 설정](/docs/how-to/sharing/project-setup)
- [Dev container 가이드](/docs/learn/with-devcontainer)
