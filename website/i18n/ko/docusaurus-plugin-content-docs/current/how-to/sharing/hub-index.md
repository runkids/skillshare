---
sidebar_position: 4
---

# Hub Index 가이드

GitHub API나 토큰 없이 조직을 위한 중앙 집중식 Skill 카탈로그를 구축합니다.

## Hub Index를 사용하는 이유

Hub index는 Skill의 이름, 설명, Source를 나열하는 JSON 파일(`skillshare-hub.json`)입니다. 내부적으로 호스팅하면 모든 팀원이 여기서 Skill을 검색하고 설치할 수 있습니다.

| 사용 사례 | GitHub 검색 | Hub Index |
|----------|--------------|-----------|
| 조직 전체 Skill 카탈로그 | 아니오 | **예** |
| 비공개/내부 Skill | 아니오 | **예** |
| 에어갭 / VPN 전용 환경 | 아니오 | **예** |
| 큐레이션된, 승인된 Skill 세트 | 아니오 | **예** |
| GitHub 토큰 불필요 | 아니오 | **예** |

실제 사례는 [Public Hub](#public-hub) 섹션을 참고하세요.

## 빠른 시작

### 1. Index 생성

```bash
# Global Skill로부터
skillshare hub index

# Project로부터
skillshare hub index -p

# 출력: <source>/skillshare-hub.json
```

### 2. Index 검색

```bash
# 로컬 파일
skillshare search react --hub ./skillshare-hub.json

# 원격 URL
skillshare search react --hub https://internal.corp/skills/skillshare-hub.json

# 모든 Skill 둘러보기 (쿼리 없음)
skillshare search --hub ./skillshare-hub.json --json
```

### 3. 결과에서 설치

대화형 검색 흐름은 GitHub 검색과 동일하게 동작합니다 — Skill을 선택하면 설치됩니다.

## Audit 정보 추가

팀원이 한눈에 Skill의 안전성을 확인할 수 있도록 index에 보안 위험 점수를 추가하세요:

```bash
# Audit 점수와 함께 index 생성
skillshare hub index --audit

# 전체 메타데이터와 결합
skillshare hub index --full --audit
```

`--audit`을 사용하면 각 Skill이 `skillshare audit` 규칙으로 스캔되며, index에는 `riskScore`(0~100), `riskLabel`(clean/low/medium/high/critical), `auditedAt` 타임스탬프가 포함됩니다. 스캔에 실패한 Skill은 위험 필드 없이 포함됩니다.

Audit된 index에서 나온 검색 결과에는 위험 배지가 표시됩니다:

```
  1. safe-skill               owner/repo/safe-skill         [clean]
  2. risky-skill              owner/repo/risky-skill        [high]
```

## 공유 전략

### 파일 공유 (가장 간단함)

Index 파일을 공유 위치에 복사하세요:

```bash
skillshare hub index -o /shared/team/skillshare-hub.json
```

팀원은 다음으로 검색합니다:
```bash
skillshare search --hub /shared/team/skillshare-hub.json
```

### HTTP 서버

로컬에서 index를 생성한 다음 호스팅에 업로드하세요:

```bash
# 1단계: 생성
skillshare hub index -o ./skillshare-hub.json

# 2단계: 업로드 (원하는 방법 사용)
scp ./skillshare-hub.json server:/var/www/skills/
# 또는: aws s3 cp ./skillshare-hub.json s3://my-bucket/
# 또는: rsync, FTP 등
```

팀원은 다음으로 검색합니다:
```bash
skillshare search --hub https://skills.company.com/skillshare-hub.json
```

### Git 저장소

Index를 공유 저장소에 커밋하여 팀원이 pull할 수 있도록 하세요:

```bash
skillshare hub index -o ./skillshare-hub.json
git add skillshare-hub.json && git commit -m "Update skill index"
git push
```

팀원은 raw URL, SSH, 또는 로컬 clone을 통해 검색할 수 있습니다:
```bash
# raw URL을 통해
skillshare search --hub https://raw.githubusercontent.com/team/skills/main/skillshare-hub.json

# SSH를 통해 — 저장소를 clone하고 index를 읽습니다 (수동 clone 불필요)
skillshare search --hub git@github.com:team/skills.git
skillshare search --hub git@ghe.corp.com:team/skills.git//hubs/team.json

# 또는 clone 후 로컬에서 검색
git pull
skillshare search --hub ./skillshare-hub.json
```

:::tip 비공개 및 GitHub Enterprise 저장소
SSH Hub Source는 여러분의 SSH 에이전트/키로 clone되므로, raw HTTPS URL이 로그인 페이지로 리디렉션되는 비공개 저장소나 GitHub Enterprise(GHE) 호스트에서도 동작합니다. 저장소 내부의 index 경로는 `//path` 접미사에서 가져오며, 기본값은 저장소 루트의 `skillshare-hub.json`입니다. scp 스타일(`git@host:org/repo.git`)과 scheme 스타일(`ssh://git@host/org/repo.git`) URL 모두 동작합니다. [`hub add`](/docs/reference/commands/hub#hub-add)로 한 번 저장해 두면 라벨로 검색할 수 있습니다.

GitHub/GHE Hub가 SSH로 로드되면, 동일한 호스트의 도메인 접두사가 붙은 Skill Source는 Hub의 SSH ID를 상속받습니다. 예를 들어 `acme@acme.ghe.com:Org/skills.git//hubs/team.json`이라는 Hub URL을 사용하면 `acme.ghe.com/Org/skills/skills/reviewer`라는 항목 Source가 SSH로 설치될 수 있습니다. Hub가 HTTP, 로컬 파일, 또는 다른 호스트로 로드된 경우 도메인 접두사가 붙은 Source는 HTTPS Source로 남습니다.
:::

## 웹 대시보드

### JSON을 작성하지 않고 Hub 만들기

대시보드(`skillshare ui`)에서 **Skills → Hub**를 열고 **Hub 추가 또는 만들기 → 새 Hub 만들기**를 선택하세요. 새 Hub는 바로 편집 상태로 열립니다.

1. Hub에 **이름**과 선택적 **설명**을 지정하세요. 이름은 공유하는 `skillshare hub add` 명령의 `--label`이 되며, 둘 다 내보낸 index에는 포함되지 않습니다.
2. **skill 추가**를 선택하세요. **URL 붙여 넣기** 탭에서 **Git URL**을 입력하고 **찾기**를 선택한 뒤 **버전**을 고르고 추가할 Skill을 선택하세요. **설치됨** 탭에서는 이 머신에 설치된 Skill을 선택합니다. 또는 **찾을 수 없나요? 소스를 직접 입력**을 선택해 빈 행을 추가하세요.
3. 각 Skill의 **이름**, **소스**, **버전**을 편집하세요. 예를 들어 `runkids/demo-skills/skills/pdf`는 원격 저장소 내부의 Skill을 식별합니다. 행을 펼치면 **스킬 설명**, **태그 (쉼표 구분)**, **스킬 선택자 (선택)**를 편집할 수 있으며, 스킬 선택자는 여러 Skill을 포함하는 저장소에서 Skill을 선택합니다.
4. **저장**을 선택하세요. 페이지가 모든 항목을 검사합니다. 다른 사람이 설치할 수 없는 Skill이 있으면 편집기가 열린 채로 해당 행을 표시합니다.
5. **공유 → skillshare-hub.json 다운로드**를 선택하세요. 표시된 Skill을 **편집**으로 고치기 전까지 다운로드할 수 없습니다.
6. 다운로드한 파일을 자신의 Git 저장소에 커밋하거나 HTTP 서버에 업로드하세요. **공유** 대화상자에 해당 URL을 붙여 넣으면 수신자를 위한 `skillshare hub add` 명령을 복사할 수 있습니다. URL은 Hub와 함께 저장됩니다.

다운로드는 아무것도 게시하지 **않습니다**. 카탈로그는 Skill을 참조할 뿐 파일을 번들로 포함하지 않습니다. Source 검증은 문법만 확인하며, 저장소가 실제로 존재하는지 또는 수신자가 권한을 가지고 있는지는 확인하지 않습니다. 비공개 저장소는 여전히 접근 권한이 필요합니다.

:::tip 로컬 Skill도 Hub에 유지할 수 있습니다
원격 origin을 알 수 없는 설치된 Skill은 로컬 Source를 유지합니다. 이를 Hub에 저장할 수 있습니다. 원격 설치 Source를 제공하거나 해당 항목을 제거할 때까지 다운로드가 차단됩니다. 빌더는 이를 조용히 누락시키지 않습니다.
:::

### 카탈로그 재개 또는 가져오기

내 Hub는 Hub 목록에 **내 Hub**로 표시됩니다. 이들은 대시보드를 실행하는 머신에서 활성 설정 파일 옆의 `hub-drafts/`에 저장됩니다. Global 및 Project 설정은 별도의 Hub를 가집니다. 다시 로드하기 전에 **저장**을 선택하세요. 편집 중에는 Hub 목록이 잠기며, 저장하지 않은 변경 사항이 있는 채로 나가면 변경 사항을 버릴지 묻는 메시지가 표시됩니다. 오래된 창에서의 저장은 최신 리비전을 덮어쓰지 못하도록 거부됩니다. **취소**는 마지막으로 저장된 버전을 다시 불러옵니다.

기존 v1 `skillshare-hub.json`(최대 4MB)을 가져오려면 **Hub 추가 또는 만들기 → skillshare-hub.json 가져오기**를 사용하세요. 지원되지 않는 버전과 잘못된 필드 유형은 오류를 발생시킵니다. 표시 이름이 같은 항목도 별도로 유지됩니다. 추가 JSON 필드와 `skill` 셀렉터는 보존됩니다. 이전 index에 `sourcePath`가 포함되어 있다면, 기존 index 리더와 마찬가지로 상대 Source가 로컬 경로로 해석됩니다. 내보내기 전에 원격 Source로 변경해야 합니다.

이식 가능한 내보내기는 작성자의 `sourcePath`와 알려진 로컬 메타데이터(`relPath`, `flatName`, `installedAt`, `isInRepo`)를 제거합니다. 여기에는 Hub의 이름, 설명, ID, 리비전, 호스팅 URL이 아니라 index만 포함됩니다. 항목의 Source나 skill 셀렉터를 변경하면 이전의 audit 점수, 라벨, 타임스탬프가 지워집니다. URL 자격 증명, 쿼리 문자열, 프래그먼트는 거부됩니다. 저장소 인증은 별도로 구성하세요.

**더 보기 → Hub 삭제**는 확인을 요청하며 해당 Hub만 삭제합니다. Skill을 제거하거나, 호스팅된 index를 삭제하거나, 구독한 Hub를 제거하지 않습니다.

### 공유된 Hub 검색

1. **Skills → 설치**를 열고 **검색**을 선택하세요.
2. **위치** 선택기에서 Hub를 선택하세요. URL, SSH 저장소, 또는 로컬 index 경로를 추가하려면 선택기 옆의 **허브 관리**를 선택한 뒤 Hub 페이지에서 **Hub 추가 또는 만들기 → 기존 Hub 추가**를 선택하세요.
3. Skill을 검색, 미리보기, 설치하세요.

Hub 페이지에서 Hub를 선택해 해당 Skill을 필터링하고 바로 설치할 수도 있습니다. 구독한 Hub Source는 활성 skillshare 설정에 저장되며 CLI와 공유됩니다. 이는 내 Hub와는 별개입니다.

기존의 `skillshare hub index` 명령과 `/api/hub/index` 엔드포인트는 로컬 Source 지원을 포함해 이전과 동일하게 index를 생성합니다. 위의 이식 가능한 내보내기 규칙은 대시보드 빌더에도 적용됩니다.

## Index 스키마

Index는 Schema v1을 따릅니다:

```json
{
  "schemaVersion": 1,
  "generatedAt": "2026-02-12T10:00:00Z",
  "sourcePath": "/home/user/.config/skillshare/skills",
  "skills": [
    {
      "name": "my-skill",
      "description": "Does something useful",
      "source": "owner/repo/.claude/skills/my-skill",
      "tags": ["workflow", "productivity"]
    }
  ]
}
```

### 필수 필드 (소비자 계약)

| 필드 | 필수 | 설명 |
|-------|----------|-------------|
| `name` | 예 | Skill 표시 이름 |
| `source` | 예 | 설치 Source (GitHub 축약형, URL, 또는 로컬 경로) |
| `description` | 권장 | 검색 매칭을 위한 짧은 설명 |
| `skill` | 아니오 | 여러 Skill을 포함하는 저장소 내의 특정 Skill 이름 (`install -s`와 함께 사용) |
| `tags` | 아니오 | 필터링과 그룹화를 위한 분류 태그 |

### 문서 수준 필드

| 필드 | 설명 |
|-------|-------------|
| `schemaVersion` | 항상 `1` |
| `generatedAt` | RFC 3339 타임스탬프 |
| `sourcePath` | 상대 Source를 해석하기 위한 기준 경로 |

### Source 경로 해석

`sourcePath`가 설정되어 있고 Skill의 `source`가 상대 경로인 경우, 검색 소비자가 이를 결합합니다:

```
sourcePath: /home/user/.config/skillshare/skills
source:     _team/frontend-skill
→ resolved: /home/user/.config/skillshare/skills/_team/frontend-skill
```

이렇게 하면 상대 경로가 GitHub 축약형(`owner/repo`)으로 잘못 해석되는 것을 방지합니다.

### Source를 tag 또는 commit에 고정

항목을 특정 버전에 고정하려면 경로에 ref가 들어 있는 웹 URL을 사용하세요. `tree/` 또는 `blob/`(GitHub), `-/tree/` 또는 `-/blob/`(GitLab), `src/`(Bitbucket) 뒤의 브랜치, tag 또는 commit SHA가 `install --branch`와 동일하게 설치 ref로 사용됩니다:

```json
{
  "name": "reviewer",
  "source": "github.com/owner/repo/tree/v1.2.0/skills/reviewer"
}
```

Hub에서 설치하는 모든 사람이 해당 리비전을 받으며, `skillshare update`도 그 리비전을 유지합니다. 고정을 옮기려면 index에서 ref를 수정하세요. remote에 없는 ref는 기본 브랜치로 대체하지 않고 설치를 실패시킵니다.

절대 경로, URL, 도메인 접두사가 붙은 경로는 절대 결합되지 않습니다:

| Source 패턴 | 결합됨? |
|----------------|---------|
| `_team/my-skill` | 예 |
| `subdir/skill` | 예 |
| `/absolute/path` | 아니오 |
| `github.com/owner/repo/skill` | 아니오 |
| `https://...` | 아니오 |

## 수동 작성 Index

`hub index`를 사용하지 않고도 수동으로 index를 만들 수 있습니다. 이는 GitHub 검색과 공개 도구가 도달할 수 없는 Source인, 비공개 인프라에서 호스팅되는 내부 Skill에 특히 유용합니다:

```json
{
  "schemaVersion": 1,
  "skills": [
    {
      "name": "company-style",
      "description": "Company coding standards and review checklist",
      "source": "ghe.internal.company.com/platform/ai-skills/company-style",
      "tags": ["quality", "workflow"]
    },
    {
      "name": "deploy-helper",
      "description": "Internal deployment automation",
      "source": "gitlab.internal.company.com/ops/skills/deploy-helper",
      "tags": ["devops"]
    },
    {
      "name": "onboarding",
      "description": "New hire onboarding skill for AI assistants",
      "source": "ghe.internal.company.com/hr/ai-skills/onboarding",
      "tags": ["workflow"]
    }
  ]
}
```

:::tip GitHub 검색만 사용하면 안 되나요?
`skillshare search`는 github.com의 공개 저장소만 찾습니다. Hub index는 GitHub Enterprise, 비공개 GitLab, 내부 서버 등 **모든** Source를 가리킬 수 있습니다 — VPN 뒤에 있는 직원만 접근할 수 있는 것들입니다. 이것이 Hub가 조직 전체 Skill 배포의 최선책이 되는 이유입니다.
:::

수동 작성 index를 위한 팁:
- `sourcePath`는 선택 사항입니다 — 모든 Source가 절대 경로라면 생략하세요
- `tags`는 선택 사항입니다 — 웹사이트나 검색에서 필터링에 유용합니다
- `name`이 비어 있는 Skill은 건너뜁니다
- 결과는 이름 알파벳순으로 정렬됩니다
- SSH 전용 GitHub Enterprise 설치의 경우, 명시적인 SSH Source(`user@host:owner/repo.git//path`)를 사용하거나 Hub 자체를 SSH로 로드하여 동일 호스트의 GitHub/GHE 도메인 접두사 항목이 해당 SSH ID를 상속받도록 하세요

## 조직 배포

비공개 Hub는 검토된 Skill을 검색할 수 있는 카탈로그를 제공합니다. 카탈로그와 Skill Source는 조직이 관리하는 인프라에 두세요. 인증과 접근 제어는 Git 호스트나 HTTP 서버가 제공합니다.

### 1. Skill과 Source 큐레이션

Skill과 카탈로그 변경 사항을 PR에서 검토하세요. SSH 전용 Git 호스트에서는 [Index 항목](#수동-작성-index)에 명시적인 SSH Source를 사용하세요. 예:

```json
{
  "schemaVersion": 1,
  "skills": [
    {
      "name": "code-review",
      "description": "Team code-review checklist",
      "source": "git@ghe.example.com:platform/ai-skills.git//skills/code-review"
    }
  ]
}
```

설치된 원격 Skill에서 `skillshare hub index --audit`로 카탈로그를 생성할 수도 있습니다. 게시 전에 각 Source에 팀원이 접근할 수 있는지 확인하세요. 로컬 파일에서 만든 Index에는 머신의 로컬 경로가 포함될 수 있으므로 공유 Source로 바꾸세요. Audit 배지는 특정 시점의 스캔 결과이며 영구적인 승인이 아닙니다.

### 2. 검토된 CLI 버전으로 변경 사항 Audit

Skill 저장소에서 고정된 CLI 버전과 심각도 Threshold로 PR을 검사하세요:

```yaml
name: Validate shared skills
on:
  pull_request:
    paths: ['skills/**', 'skillshare-hub.json']

jobs:
  audit:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      # 가독성을 위해 태그 사용. 각 Action은 검토된 커밋 SHA로 고정하세요
      - uses: actions/checkout@v4
      - uses: runkids/setup-skillshare@v1
        with:
          version: '0.23.5' # 예제: 팀에서 검토한 CLI 버전을 선택하세요
          source: ./skills
          audit: true
          audit-threshold: high
```

이 예제는 체크아웃한 저장소의 `skills/`에 Skill이 있다고 가정합니다. 내부 Git 서버에서는 CI runner의 checkout 및 접근 설정을 사용하세요. 스캔 명령은 같습니다. 다른 CI 시스템은 [CI/CD Skill 검증](/docs/how-to/recipes/ci-cd-skill-validation)을 참고하세요.

Action의 `version` input은 CLI release를 고정하며, Action 자체나 Skill 내용은 고정하지 않습니다. 예제는 가독성을 위해 태그를 사용합니다. 조직 정책에 따라 각 Action을 검토된 전체 커밋 SHA로 고정하세요. [프로젝트 lockfile](/docs/understand/project-skills#lockfile)로 원격 Skill 커밋을 기록하고, 각 종류의 업데이트를 별도로 검토하세요. `hub index --audit`는 카탈로그에 스캔 결과를 추가합니다. 해당 심각도의 탐지 결과를 거부하려면 `skillshare audit --threshold high` 또는 위 파이프라인 게이트를 사용하세요.

### 3. 카탈로그를 비공개로 배포

검토 후 `skillshare-hub.json`을 내부 Skill 저장소의 루트에 커밋하세요. Git 호스트를 통해 팀원에게 읽기 권한을 부여하세요. 팀 환경에서 Index를 가져올 수 있다면 내부 HTTP 호스팅도 가능합니다. 공개 Hub를 fork하거나 공개 raw URL을 제공할 필요는 없습니다.

### 4. 등록, 검색 및 동기화

[skillshare 초기화](/docs/getting-started/first-sync) 후 팀원은 비공개 카탈로그를 한 번만 등록합니다:

```bash
skillshare hub add git@ghe.example.com:platform/ai-skills.git --label company -g
skillshare search code-review --hub company -g
# Select a skill to install, then distribute it to global targets
skillshare sync -g
```

SSH Hub URL은 저장소 루트에서 `skillshare-hub.json`을 읽습니다. 카탈로그가 다른 위치에 있다면 `git@ghe.example.com:platform/ai-skills.git//catalog/skillshare-hub.json`처럼 경로를 붙이세요. SSH 접근은 팀원의 기존 SSH 설정을 사용합니다. Git 호스트는 카탈로그와 각 Skill Source 모두에 대한 접근을 허용해야 합니다. Hub는 Skill을 찾는 수단이며 다른 Source에서의 설치를 막지 않습니다.

### 5. 프로젝트 의존성 기록

특정 프로젝트에 필요한 Skill은 Project mode로 설치하고 생성된 config와 lockfile을 커밋하세요. 팀원은 clone하거나 업데이트를 pull한 후 `skillshare install -p`, Audit, Sync를 실행합니다. 순서는 [팀 온보딩](/docs/how-to/recipes/team-onboarding-recipe)을 참고하세요. 카탈로그 큐레이션, Skill 업데이트, CLI 업그레이드는 각각 명시적으로 검토된 변경 사항으로 관리하세요.

## Public Hub

[skillshare-hub](https://github.com/runkids/skillshare-hub)는 엄선된 양질의 Skill 카탈로그입니다. 이는 **기본 Hub**로, Source를 지정하지 않고 `search --hub`를 실행하면 여기에서 검색합니다:

```bash
skillshare search --hub              # Public Hub의 모든 Skill 둘러보기
skillshare search react --hub        # "react" Skill 검색
```

이는 또한 조직 고유의 Hub를 구축하기 위한 참고 자료 역할도 합니다:

- **Index 구조** — 이름, 설명, Source, 태그로 `skillshare-hub.json`을 구성하는 방법
- **CI 검증** — 모든 PR에서 자동화된 JSON 형식 검사와 `skillshare audit` 보안 스캔
- **기여 워크플로** — Fork → 항목 추가 → PR, CI 게이트 포함

팀을 위한 내부 Hub를 구축하고 싶으신가요? 저장소를 Fork하고, Skill을 조직의 카탈로그로 교체한 다음, 보안 정책에 맞게 CI 파이프라인을 커스터마이즈하세요.

## 팁

- **Index 생성 자동화** — Skill 변경 후 CI 파이프라인에 `skillshare hub index`를 추가하세요
- **Audit에는 `--full`을 사용하세요** — 전체 모드에는 버전, 설치 날짜, 유형 정보가 포함됩니다
- **Project mode와 결합** — `skillshare hub index -p`는 Project 레벨 Skill만 index합니다

---

## 참고 자료

- [search](/docs/reference/commands/search) — Hub에서 Skill 검색
- [hub](/docs/reference/commands/hub) — Hub Source 관리
- [install](/docs/reference/commands/install) — 발견된 Skill 설치
