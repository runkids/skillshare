---
sidebar_position: 2
---

# Security-First 설계

> AI skill은 실행 가능한 지침입니다. skillshare는 이를 신뢰할 수 없는 입력으로 취급합니다.

## 위협 모델

GitHub에서 skill을 설치할 때, 여러분은 코드 생성, 파일 수정, 그리고 잠재적으로 명령 실행에 영향을 미칠 지침을 AI 도구에 제공하는 것입니다. 악성 skill은 다음을 할 수 있습니다:

- **프롬프트 인젝션** — AI의 안전 가이드라인을 무시하도록 함
- **데이터 유출** — 파일 내용을 외부 URL로 보내도록 AI에게 지시
- **파괴적인 명령 실행** — AI의 셸 접근을 통해
- **자격 증명 탈취** — 환경 변수나 config 파일에 접근하여

이는 이론적인 이야기가 아닙니다. 프롬프트 인젝션은 AI 도구 생태계에서 가장 우려되는 보안 문제입니다.

## Audit Engine

skillshare에는 5단계 심각도에 걸쳐 15개 이상의 탐지 패턴에 대해 모든 설치된 skill을 검사하는 내장 보안 스캐너(`skillshare audit`)가 포함되어 있습니다:

| 심각도 | 예시 |
|----------|----------|
| CRITICAL | 프롬프트 인젝션, 시스템 프롬프트 재정의 |
| HIGH | 데이터 유출 URL, 자격 증명 접근 패턴 |
| MEDIUM | 파괴적인 명령 (`rm -rf`, `DROP TABLE`), 파일시스템 쓰기 |
| LOW | 네트워크 요청, 외부 도구 호출 |
| INFO | 큰 파일 크기, 비정상적인 형식 |

### 동작 방식

audit engine은 패턴 매칭과 휴리스틱을 사용해 SKILL.md 콘텐츠를 스캔합니다:

```bash
# Scan all installed skills
skillshare audit

# JSON output for CI integration
skillshare audit --json

# Scan project skills only
skillshare audit -p
```

### 자동 차단

`skillshare install` 중에 audit이 자동으로 실행됩니다. CRITICAL 발견 사항이 감지되면 설치가 차단됩니다:

```
CRITICAL: Prompt injection detected in "malicious-skill"
  → Pattern: "ignore previous instructions"
  → Installation blocked. Use --force to override (not recommended).
```

## 다층 방어

Audit engine은 하나의 계층일 뿐입니다. skillshare의 보안 모델은 다음을 포함합니다:

1. **설치 시점 audit** — AI 도구에 도달하기 전에 위협을 잡아냄
2. **요청 시 audit** — 새로운 패턴이 추가될 때 기존 skill을 다시 스캔
3. **공유 Source** — 링크된 Target은 같은 Skill 파일을 읽고 Source 변경 사항이 모두에게 전달되므로, 변경 사항을 검토하고 Source 쓰기 권한을 제한해야 함
4. **변경 전 백업** — `skillshare backup`이 전체 skill 라이브러리를 스냅샷함
5. **TTL이 있는 Trash** — 삭제된 skill은 영구 삭제 전에 먼저 trash로 이동함
6. **작업 로깅** — 모든 변경 작업이 `operations.log`(JSONL)에 기록됨

Symlink는 공통 Source를 제공하지만 보안 샌드박스는 아닙니다. 마찬가지로 `.skillignore`는 Skill 검색 대상을 필터링하며 AI 도구의 파일이나 명령 접근을 차단하지 않습니다. 런타임 권한과 승인은 AI 도구의 책임입니다. 자세한 역할 구분은 [공유 Source와 실행 경계](/docs/how-to/advanced/security#shared-source-and-execution-boundaries)를 참고하세요.

## 공급망 고려사항

AI skill 생태계는 아직 초기 단계입니다. 검토 절차가 있는 패키지 레지스트리도, 코드 서명도, 의존성 해석도 없습니다. Skill은 git 저장소 안의 Markdown 파일일 뿐입니다.

skillshare의 접근 방식:
- **모든 것을 스캔** — 신뢰할 수 있는 소스의 skill이라도
- **기본적으로 차단** — CRITICAL 발견 사항은 설치를 막음
- **모든 것을 로깅** — audit 결과는 포렌식 검토를 위해 저장됨
- **패턴 업데이트** — 새로운 탐지 패턴이 skillshare 릴리스마다 함께 제공됨

## Audit 동작 설정하기

`config.yaml`에서 차단 임계값을 설정해 어떤 심각도가 설치를 차단할지 제어하세요:

```yaml
# config.yaml
audit:
  block_threshold: HIGH   # Block on HIGH and CRITICAL (default: CRITICAL)
```

규칙별 커스터마이즈를 위해서는 별도의 `audit-rules.yaml` 파일을 사용하세요(`skillshare audit --init-rules`로 초기화):

```yaml
# audit-rules.yaml
rules:
  - id: network-request-0
    enabled: false          # Disable this specific rule
  - id: my-custom-check
    severity: MEDIUM
    pattern: "TODO|FIXME"
    description: Policy violation — unresolved TODOs
```

## 관련 문서

- [`audit` 명령 참조](/docs/reference/commands/audit)
- [보안 가이드](/docs/how-to/advanced/security)
- [CI/CD 검증 레시피](/docs/how-to/recipes/ci-cd-skill-validation)
