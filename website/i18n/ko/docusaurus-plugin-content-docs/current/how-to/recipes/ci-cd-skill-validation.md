---
sidebar_position: 2
---

# Recipe: CI/CD Skill Validation

> CI 파이프라인에서 Skill을 자동으로 감사하고 동기화하세요.

## 시나리오

팀 Skill 저장소가 있고 모든 PR이 다음을 만족하기를 원합니다:
- 보안 감사 통과(프롬프트 인젝션, 자격 증명 탈취 등 없음)
- SKILL.md 형식 검증
- 오류 없이 동기화됨

## 해결 방법

### GitHub Actions(setup-skillshare 사용)

[`setup-skillshare`](https://github.com/marketplace/actions/setup-skillshare) 액션은
설치, 초기화, 선택적인 보안 감사를 한 단계에서 처리합니다.

```yaml
name: Skill Validation
on:
  pull_request:
    paths:
      - 'skills/**'

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: runkids/setup-skillshare@v1
        with:
          source: ./skills
          audit: true
          audit-threshold: high
      - run: skillshare sync --dry-run
```

### SARIF 업로드를 사용하는 GitHub Actions

[GitHub Code Scanning](https://docs.github.com/en/code-security/code-scanning)을 통해
인라인 PR 주석을 받으려면 SARIF 출력을 사용하세요:

```yaml
name: Skill Security Scan
on:
  pull_request:
    paths: ['skills/**']
  push:
    branches: [main]

jobs:
  validate:
    runs-on: ubuntu-latest
    permissions:
      security-events: write
    steps:
      - uses: actions/checkout@v4
      - uses: runkids/setup-skillshare@v1
        with:
          source: ./skills
          audit: true
          audit-threshold: high
          audit-format: sarif
          audit-output: results.sarif

      - name: Upload SARIF to Code Scanning
        if: always()
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: results.sarif
          category: skillshare-audit

      - run: skillshare sync --dry-run
```

### 액션 없이(수동 설정)

액션을 사용하지 않으려면 skillshare를 직접 설치할 수 있습니다:

```yaml
jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: |
          curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
          echo "$HOME/.local/bin" >> "$GITHUB_PATH"
      - run: skillshare init --no-copy --all-targets --no-git --no-skill --source ./skills
      - run: skillshare audit --threshold high --format json
      - run: skillshare sync --dry-run
```

### GitLab CI

`.gitlab-ci.yml`을 만드세요:

```yaml
skill-validation:
  image: ghcr.io/runkids/skillshare-ci:latest
  stage: test
  script:
    - skillshare init
    - skillshare install . --into ci-check
    - skillshare audit --threshold high --format json
    - skillshare sync --dry-run
  rules:
    - changes:
        - skills/**/*
```

### CI Docker 이미지 사용하기

더 빠른 파이프라인 시작을 위해 미리 빌드된 CI 이미지를 사용하세요:

```yaml
# GitHub Actions
jobs:
  validate:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/runkids/skillshare-ci:latest
    steps:
      - uses: actions/checkout@v4
      - run: skillshare init && skillshare audit --format json
```

## 출력 형식

`audit` 명령어는 다양한 CI/CD 통합 요구 사항을 위해 여러 출력 형식을 지원합니다.

### 종료 코드

```bash
# 임계값 이상의 발견 사항이 있으면 배포 차단
skillshare audit --threshold high
echo $?  # 0 = 정상, 1 = 발견 사항 있음
```

### SARIF 출력

[SARIF(Static Analysis Results Interchange Format)](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html)는
GitHub Code Scanning, VS Code SARIF Viewer, Azure DevOps, SonarQube 등의 정적 분석
도구가 사용하는 OASIS 표준입니다.

```bash
skillshare audit --format sarif              # stdout으로 출력
skillshare audit --format sarif > results.sarif  # 파일로 저장
```

SARIF 출력에는 다음이 포함됩니다:
- **도구 메타데이터** — 도구 이름(`skillshare`), 버전, 정보 URI
- **규칙** — `security-severity` 점수가 포함된 중복 제거된 규칙 서술자
- **결과** — 파일 위치와 심각도 수준이 매핑된 각 발견 사항

SARIF 수준으로의 심각도 매핑:

| skillshare 심각도 | SARIF 수준 | security-severity |
|---------------------|-------------|-------------------|
| CRITICAL | `error` | 9.0 |
| HIGH | `error` | 7.0 |
| MEDIUM | `warning` | 4.0 |
| LOW | `note` | 2.0 |
| INFO | `note` | 0.5 |

### Markdown 리포트

GitHub Issues, Pull Requests, 또는 문서에 붙여넣기 적합한 독립형 Markdown 리포트를
생성하세요:

```bash
skillshare audit --format markdown               # stdout에 출력
skillshare audit --format markdown > report.md   # 파일로 저장
skillshare audit -p --format markdown > report.md  # 프로젝트 모드
```

리포트에는 다음이 포함됩니다:
- **헤더** — 스캔된 개수, 모드, 임계값
- **요약 표** — 통과/경고/실패 개수, 심각도 분류, 리스크 점수, 분석 가능 여부
- **발견 사항** — 심각도, 패턴, 메시지, 위치가 포함된 Skill별 표; 접을 수 있는 스니펫
- **정상 Skill** — 발견 사항이 없는 Skill의 콤마로 구분된 목록

### jq를 이용한 JSON 출력

```bash
# CRITICAL 발견 사항이 있는 모든 Skill 나열
skillshare audit --json | jq '[.skills[] | select(.findings[] | .severity == "CRITICAL")]'

# 모든 Skill의 리스크 점수 추출
skillshare audit --json | jq '.skills[] | {name: .skillName, score: .riskScore, label: .riskLabel}'

# 심각도별 발견 사항 개수
skillshare audit --json | jq '[.skills[].findings[].severity] | group_by(.) | map({(.[0]): length}) | add'
```

## 검증

- PR 체크 통과: audit이 0으로 종료(임계값 이상의 발견 사항 없음)
- Audit JSON 출력을 다운스트림 도구로 파싱 가능
- SARIF 업로드가 PR diff에 인라인 주석으로 발견 사항 표시
- Sync dry-run이 예상되는 심볼릭 링크 작업 표시

## 변형

- **HIGH 심각도에서 차단**: `audit`에 `--threshold HIGH`(또는 `-T HIGH`) 추가 — HIGH 이상의
  발견 사항이 있으면 종료 코드가 0이 아님
- **Code Scanning을 위한 SARIF**: 인라인 PR 주석을 위해 `github/codeql-action/upload-sarif@v3`와
  함께 `--format sarif` 사용
- **병렬 검증**: 더 빠른 피드백을 위해 별도 CI 작업에서 audit과 sync 실행
- **예약된 감사**: 기존 Skill에서 새로 감지된 패턴을 잡기 위해 매일 밤 실행

## 관련 문서

- [Security audit guide](/docs/how-to/advanced/security)
- [`audit` command reference](/docs/reference/commands/audit)
- [`audit rules` reference](/docs/reference/commands/audit-rules)
- [Audit Engine](/docs/understand/audit-engine) — 엔진 작동 방식
- [Docker sandbox guide](/docs/how-to/advanced/docker-sandbox)
