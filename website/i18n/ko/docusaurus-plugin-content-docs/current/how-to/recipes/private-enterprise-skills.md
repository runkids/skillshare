---
sidebar_position: 3
---

# Recipe: Private Enterprise Skills

> 토큰 인증을 사용해 비공개 저장소에서 Skill을 설치합니다.

## 시나리오

조직에서 비공개 GitHub/GitLab 저장소에 내부 Skill을 호스팅하고 있습니다. Config 파일에 자격 증명을 노출하지 않고 이 Skill들을 설치하고 업데이트해야 합니다.

## 해결 방법

### 1단계: 인증 설정

skillshare는 환경 변수에서 토큰을 감지하며, 플랫폼별 변수가 일반 대체 변수보다 우선합니다:

| 플랫폼 | 환경 변수 |
|----------|---------------------|
| GitHub / GitHub Enterprise | `GITHUB_TOKEN` |
| GitLab / 자체 호스팅 GitLab | `GITLAB_TOKEN` |
| Bitbucket | `BITBUCKET_TOKEN` (+ 선택적 `BITBUCKET_USERNAME`) |
| Azure DevOps | `AZURE_DEVOPS_TOKEN` |
| Gitea / 자체 호스팅 Gitea | `GITEA_TOKEN` |
| CNB | `CNB_TOKEN` |
| 모든 플랫폼 (대체) | `SKILLSHARE_GIT_TOKEN` |

```bash
# 옵션 A: Git credential helper (GitHub에 권장)
gh auth login   # HTTPS용 git credential helper를 설정합니다

# 옵션 B: 플랫폼별 환경 변수
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxx      # GitHub
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxx    # GitLab
export AZURE_DEVOPS_TOKEN=your-pat-here    # Azure DevOps

# 옵션 C: 일반 대체 (모든 HTTPS 호스트에서 동작)
export SKILLSHARE_GIT_TOKEN=your-token-here
```

### 2단계: 비공개 저장소에서 설치

```bash
skillshare install your-org/internal-skills --track
```

skillshare는 위에 나열된 환경 변수에서 토큰을 자동으로 감지합니다.

### 3단계: 추적 확인

```bash
skillshare list
```

설치된 저장소는 `_` 접두사가 붙은 채로 나타납니다 (tracked repository):

```
_your-org-internal-skills/
├── code-review/
├── testing-standards/
└── deployment-checklist/
```

### 4단계: 업데이트 주기

```bash
skillshare check    # 업스트림 변경 사항 감지
skillshare update   # 최신 내용 가져오기
skillshare sync     # Target에 반영
```

## 확인

- `skillshare list`에 tracked repo가 표시됩니다
- `skillshare check`가 원격에 접속해 해시를 비교할 수 있습니다
- `skillshare sync`가 모든 Target에 심볼릭 링크를 생성합니다

## 변형

- **선택적 설치**: `skillshare install your-org/internal-skills --skill code-review`는 하나의 Skill만 설치합니다
- **CI/CD 토큰**: 파이프라인에서는 CI 시크릿으로부터 플랫폼별 환경 변수(예: `GITHUB_TOKEN`)를 설정하세요
- **자체 호스팅 GitLab**: `GITLAB_TOKEN`을 설정하고 HTTPS URL을 사용하세요: `skillshare install https://gitlab.internal.com/team/skills.git --track`
- **자체 호스팅 Gitea**: `GITEA_TOKEN`을 설정하세요. 호스트명에 `gitea`가 포함되어 있지 않다면 [`gitea_hosts`](/docs/reference/targets/configuration#gitea_hosts)에도 등록하세요
- **Gitee / AtomGit**: `SKILLSHARE_GIT_TOKEN`을 사용한 HTTPS URL로 지원됩니다

## 관련 문서

- [`install` 명령어 레퍼런스](/docs/reference/commands/install)
- [`update` 명령어 레퍼런스](/docs/reference/commands/update)
- [조직 공유 가이드](/docs/how-to/sharing/organization-sharing)
- [URL 형식 레퍼런스](/docs/reference/appendix/url-formats)
