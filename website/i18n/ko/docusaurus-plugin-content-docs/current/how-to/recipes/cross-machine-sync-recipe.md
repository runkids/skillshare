---
sidebar_position: 5
---

# Recipe: Cross-Machine Sync

> git push/pull을 사용해 여러 기기 간 Skill을 동기화 상태로 유지하세요.

## 시나리오

데스크톱과 노트북(또는 집과 사무실의 기기)에서 작업합니다. 각 기기에서 설치
명령을 다시 실행하지 않고도 동일한 Skill 라이브러리를 어디서든 사용하고
싶습니다.

## 해결 방법

### 초기 설정(머신 A)

```bash
# skillshare 초기화
skillshare init

# Skill 설치
skillshare install your-org/team-skills
skillshare install another/repo --into tools

# Source를 git remote에 push
skillshare push
```

`skillshare push`는 Source 디렉터리를 git으로 추적되는 브랜치에 커밋하고 설정된
remote로 push합니다.

### 새 기기에서 설정(머신 B)

```bash
# skillshare 설치
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

설치 프로그램이 PATH 설정 안내를 표시할 때만 안내에 따라 설정한 후 아래 명령을 실행하세요. PATH 경고가 없으면 추가 설정은 필요하지 않습니다.

```bash

# 초기화
skillshare init

# remote에서 pull
skillshare pull

# 로컬 Target에 동기화
skillshare sync
```

### 일상적인 동기화 워크플로우

어떤 기기에서든:

```bash
# 다른 기기의 최신 변경 사항 pull
skillshare pull

# 로컬 AI 도구에 동기화
skillshare sync

# 로컬에서 변경한 후
skillshare push
```

## 검증

- `skillshare push`가 0으로 종료되고 커밋된 변경 사항을 보고함
- 다른 기기에서 `skillshare pull`을 실행하면 받은 변경 사항이 표시됨
- `skillshare list`가 양쪽 기기에서 동일한 Skill을 표시함
- `skillshare sync`가 대상 기기에서 심볼릭 링크를 생성함

## 변형

- **로그인 시 자동 동기화**: 셸 프로필(`.bashrc` / `.zshrc`)에 `skillshare pull && skillshare sync` 추가
- **충돌 해결**: `pull`은 두 기기의 커밋을 병합하고 `.metadata.json` 충돌은 스스로 해결합니다. 두 기기가 같은 Skill 파일을 수정한 경우 `pull`은 중단되고, 병합을 되돌리며, 해당 파일을 표시합니다 — Source 디렉터리에서 git으로 해결하세요
- **선택적 동기화**: 각 Target에 동기화될 Skill을 제어하려면 `config.yaml`에서 Target별 `include` / `exclude` 필터 사용

## 관련 문서

- [Cross-machine sync guide](/docs/how-to/sharing/cross-machine-sync)
- [`push` command reference](/docs/reference/commands/push)
- [`pull` command reference](/docs/reference/commands/pull)
