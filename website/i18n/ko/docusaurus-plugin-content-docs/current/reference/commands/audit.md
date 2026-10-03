---
sidebar_position: 3
---

# audit

설치된 skill을 스캔하여 보안 위협과 악성 패턴을 탐지합니다.

```bash
skillshare audit                        # 설치된 모든 skill 스캔
skillshare audit <name>                 # 특정 설치된 skill 스캔
skillshare audit a b c                  # 여러 skill 스캔
skillshare audit --group frontend       # 그룹에 속한 모든 skill 스캔
skillshare audit <path>                 # 파일/디렉터리 경로 스캔
skillshare audit --threshold high       # HIGH 이상에서 차단
skillshare audit -T h                   # --threshold high 와 동일
skillshare audit --format json           # JSON 출력
skillshare audit --format sarif         # SARIF 2.1.0 출력 (GitHub Code Scanning)
skillshare audit --format markdown      # Markdown 보고서 (GitHub Issues/PR용)
skillshare audit --json                 # --format json 과 동일 (사용 중단 예정)
skillshare audit -p                     # project skill 스캔
skillshare audit --quiet                # findings가 있는 skill만 표시
skillshare audit --yes                  # 대규모 스캔 확인 프롬프트 건너뛰기
skillshare audit --no-tui               # 일반 텍스트 출력 (대화형 TUI 없음)
skillshare audit --profile strict       # strict 프로파일 사용 (HIGH 이상 차단)
skillshare audit --dedupe global        # 전체 composite-key 중복 제거
skillshare audit --analyzer static      # static 분석기만 실행
skillshare audit --analyzer static --analyzer dataflow  # 여러 분석기
```

## 사용 시점

- 새 skill 설치 후 보안 findings 검토
- 프롬프트 인젝션, 데이터 유출, 자격 증명 접근 패턴을 위해 모든 skill 스캔
- 조직의 보안 정책에 맞게 audit 규칙 커스터마이징
- 컴플라이언스용(`--format json`), 정적 분석 도구용(`--format sarif`), 문서화용(`--format markdown`) audit 보고서 생성
- CI/CD 파이프라인에 통합하여 skill 배포를 게이팅
- SARIF 결과를 GitHub Code Scanning에 업로드하여 PR 수준 주석 생성

## 탐지 항목

audit 엔진은 skill 디렉터리 내 모든 텍스트 기반 파일을 100개 이상의 내장 규칙(정규식 패턴, 테이블 기반 자격 증명 탐지, 구조적 검사, 콘텐츠 무결성 검증, 공급망 신뢰 분석)에 대해 스캔하며, **CRITICAL**, **HIGH**, **MEDIUM**, **LOW**, **INFO** 5단계 심각도로 구성됩니다.

전체 탐지 카탈로그, 위협 카테고리 심층 분석, 위험 점수 알고리즘, 명령 안전성 등급, cross-skill 상호작용 분석은 [Audit Engine](/docs/understand/audit-engine)을 참조하십시오.

## 출력 예시

```
skillshare audit
Audit  ~/.config/skillshare/skills
  global · blocks at CRITICAL · policy DEFAULT / dedupe:GLOBAL / analyzers:ALL

! ci-release-helper  HIGH · risk 25/100
✗ suspicious-skill   CRITICAL · risk 35/100

Summary
  Scanned      12 skills
✓ Passed       9
! Warning      2
✗ Failed       1
  Severity     1 critical, 2 high, 1 medium
  Threats      credential:1 exfiltration:1 injection:1 privilege:1
  Risk         HIGH 35/100 · 100% auditable

✗ Blocked 1 of 12 skills: findings at CRITICAL or above · 2.1s
  The risk score is informational; only the severity blocks

Next
  skillshare audit suspicious-skill  see its findings
```

`Failed`는 활성 threshold(`--threshold` 또는 config의 `audit.block_threshold`; 기본값 `CRITICAL`) 이상의 findings가 있는 skill 수를 나타냅니다.

`Threats`는 모든 findings를 카테고리별로 짧은 이름으로 분류하여 보여줍니다: `inj`(injection), `exfil`(exfiltration), `cred`(credential), `obfusc`(obfuscation), `priv`(privilege), `integ`(integrity), `struct`(structure), `risk`(risk). findings가 없으면 이 줄은 생략됩니다. 터미널 출력에서는 각 카테고리가 위협 유형별로 색상 구분됩니다.

`audit.block_threshold`는 차단 threshold만 제어합니다. 스캔 자체를 비활성화하지는 **않습니다**.

### 대화형 TUI 모드

대화형 터미널에서 여러 skill을 스캔하면 `audit`는 결과를 한 줄씩 출력하는 대신 전체 화면을 엽니다. 왼쪽에는 skill(발견 사항이 있는 것이 먼저), 오른쪽에는 선택한 skill의 위험 요약과 발견 사항이 표시됩니다. skill을 열면 발견 사항마다 해당 파일의 표시된 줄이 줄 번호와 함께 강조됩니다. 키는 화면 아래쪽에 표시됩니다.

이 화면은 터미널이 대화형이고, 출력이 JSON이 아니며, 결과가 둘 이상일 때만 열립니다. 일반 텍스트를 강제하려면 `--no-tui`를 사용하세요.

### 대규모 스캔 확인

대화형 터미널에서 1,000개 이상의 skill을 스캔할 때, 명령은 진행 전에 확인을 요청합니다. TTY 환경(예: 로컬 자동화 스크립트)에서 이 프롬프트를 건너뛰려면 `--yes`를 사용하십시오. CI/CD 파이프라인(non-TTY)에서는 프롬프트가 자동으로 생략됩니다.

## 정책 및 프로파일

audit 명령은 프로파일, 중복 제거 모드, 분석기 선택을 통한 **정책 기반** 구성을 지원합니다. 이는 CLI 플래그, project config, global config를 통해 설정할 수 있습니다.

### 프로파일

프로파일은 threshold와 중복 제거에 대한 합리적인 기본값을 설정하는 사전 구성입니다:

| Profile | Threshold | Dedupe | 사용 사례 |
|---------|-----------|--------|----------|
| `default` | `CRITICAL` | `global` | 표준 동작 — critical 위협만 차단 |
| `strict` | `HIGH` | `global` | 보안을 중시하는 팀 — high 이상 위협 차단 |
| `permissive` | `CRITICAL` | `legacy` | 권고용 전용 — 최소한의 차단, global dedup 없음 |

```bash
skillshare audit --profile strict       # HIGH 이상 차단, global dedup
skillshare audit --profile permissive   # 권고 모드
```

명시적 플래그는 항상 프로파일 기본값보다 우선합니다:

```bash
skillshare audit --profile strict --threshold medium  # strict 프로파일이지만 MEDIUM 이상에서 차단
```

### 중복 제거

동일한 finding이 여러 분석기(예: static과 dataflow 모두)에 의해 탐지되면, 중복 제거로 불필요한 항목을 제거합니다:

| Mode | 동작 |
|------|------|
| `global` | 모든 findings에 대한 전체 composite-key 중복 제거 (기본값) |
| `legacy` | 분석기별 중복 제거만 수행 (v0.16.9 이전 동작) |

### 분석기 선택

기본적으로 모든 분석기가 실행됩니다. 특정 분석기만 실행하려면 `--analyzer`를 사용하십시오:

```bash
skillshare audit --analyzer static                    # static 패턴 매칭만
skillshare audit --analyzer static --analyzer dataflow # 여러 분석기
```

| Analyzer | 범위 | 설명 |
|----------|-------|-------------|
| `static` | 파일별 | audit 규칙에 대한 정규식 기반 패턴 매칭 |
| `dataflow` | 파일별 | shell 스크립트 및 markdown 코드 블록에 대한 taint tracking |
| `tier` | skill별 | 기능 등급(capability tier) 조합 위험 분석 |
| `integrity` | skill별 | 콘텐츠 해시 검증 (SKILL.md의 `file_hashes`) |
| `metadata` | skill별 | 공급망 신뢰 검증 (publisher 불일치, authority 주장) |
| `structure` | skill별 | markdown 댕글링 링크 탐지 |
| `cross-skill` | 번들 | Cross-skill 유출 및 권한 상승 분석 |

config에서도 설정할 수 있습니다:

```yaml
audit:
  enabled_analyzers: [static, dataflow]
```

### 우선순위 {#precedence}

설정은 다음 순서로 해석됩니다(비어 있지 않은 첫 번째 값이 적용):

1. CLI 플래그 (`--profile`, `--threshold`, `--dedupe`, `--analyzer`)
2. Project config (`.skillshare/config.yaml`)
3. Global config (`~/.config/skillshare/config.yaml`)
4. 프로파일 기본값

## 자동 스캔

### 설치 시점

Skill은 설치 중 자동으로 스캔됩니다. `audit.block_threshold`(기본값: `CRITICAL`) 이상의 findings는 설치를 차단합니다:

```bash
skillshare install /path/to/evil-skill
# Error: security audit failed: critical threats detected in skill

skillshare install /path/to/evil-skill --force
# 경고와 함께 설치됨 (주의해서 사용)

skillshare install /path/to/skill --audit-threshold high
# 명령별 차단 threshold 재정의

skillshare install /path/to/skill -T h
# --audit-threshold high 와 동일

skillshare install /path/to/skill --skip-audit
# 스캔 우회 (주의해서 사용)
```

`--force`는 차단 결정을 무시합니다. `--skip-audit`는 해당 install 명령에 대한 스캔을 비활성화합니다.

설치 시점 audit를 전역적으로 비활성화하는 config 플래그는 없습니다. 의도적으로 스캔을 우회하려는 명령에서만 `--skip-audit`를 사용하십시오.

차이 요약:

| Install 플래그 | Audit 실행 여부 | Findings 확인 가능 여부 |
|--------------|-------------|---------------------|
| `--force` | 예 | 예 (설치는 계속 진행됨) |
| `--skip-audit` | 아니오 | 아니오 (스캔이 우회됨) |

둘 다 지정되면 audit이 실행되지 않으므로 사실상 `--skip-audit`가 우선합니다.

### 업데이트 시점

`skillshare update`는 tracked repo를 pull한 후 보안 audit를 실행합니다. 활성 threshold(기본적으로 `audit.block_threshold`, 또는 `--audit-threshold` / `--threshold` / `-T` 재정의) 이상의 findings는 rollback을 트리거합니다. 자세한 내용은 [`update --skip-audit`](/docs/reference/commands/update#security-audit-gate)를 참조하십시오.

`--force`로 수락한 findings는 해당 skill에 대해 기억되므로, 이후 업데이트에서 동일한 규칙이 동일한 텍스트에 매칭되어도 차단되지 않습니다. 새로운 finding이나 동일한 규칙이 다른 텍스트에 매칭되는 경우에는 다시 차단됩니다. [Accepted Findings](/docs/reference/commands/update#accepted-findings)를 참조하십시오.

install을 통해 tracked repo를 업데이트할 때(`skillshare install <repo> --track --update`), 게이트는 동일한 threshold 정책(`audit.block_threshold` 또는 `--audit-threshold` / `--threshold` / `-T`)을 사용합니다.

## CI/CD 통합

`audit` 명령은 파이프라인 자동화를 위해 설계되었습니다. non-TTY 환경(CI 러너, 파이프된 출력)에서는 대화형 TUI와 확인 프롬프트가 자동으로 비활성화됩니다 — `--yes`나 `--no-tui`가 필요하지 않습니다.

전체 CI/CD 워크플로우(GitHub Actions, GitLab CI, SARIF 업로드, 출력 형식)에 대해서는 [CI/CD Skill Validation 레시피](/docs/how-to/recipes/ci-cd-skill-validation)를 참조하십시오.

### Pre-commit Hook

[pre-commit](https://pre-commit.com/) 프레임워크를 사용하여 모든 커밋마다 `skillshare audit`를 자동으로 실행합니다. 이 hook은 `.skillshare/` 또는 `skills/` 디렉터리와 일치하는 파일을 스캔하고, findings가 구성된 threshold를 초과하면 커밋을 차단합니다.

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/runkids/skillshare
    rev: v0.16.11  # 최신 릴리스 태그 사용
    hooks:
      - id: skillshare-audit
```

전체 설정 방법은 [Pre-commit Hook 레시피](/docs/how-to/recipes/pre-commit-hook)를 참조하십시오.

## 모범 사례

### 개인 개발자용

- **신뢰하기 전에 audit** — 신뢰할 수 없는 출처에서 skill을 설치한 후에는 항상 `skillshare audit`를 실행
- **통과 여부뿐 아니라 findings를 검토** — "통과"한 skill도 조사할 가치가 있는 LOW/MEDIUM findings가 있을 수 있음
- **skill 파일을 직접 읽기** — 자동 스캔은 알려진 패턴을 탐지하지만, 새로운 공격은 사람의 검토가 필요함

### 팀 및 조직용

- **`audit.block_threshold: HIGH` 설정** — 기본값 `CRITICAL`보다 엄격하며, obfuscation 및 destructive 명령을 잡아냄
- **조직 전체 커스텀 규칙 생성** — 내부 secret 형식(예: `corp-api-key-*`)에 대한 패턴 추가
- **project 모드 규칙을 재정의에 사용** — 전역이 아닌 project 단위로 예상되는 패턴을 하향 조정

### 권장 Audit 워크플로우

1. **설치**: skill이 자동으로 스캔됨 — threshold 초과 시 차단
2. **주기적 스캔**: 설치 이후 업데이트된 규칙을 잡아내기 위해 `skillshare audit`를 정기적으로 실행
3. **Pre-commit hook**: [pre-commit 프레임워크](/docs/how-to/recipes/pre-commit-hook)로 커밋 전에 문제를 포착
4. **CI 게이트**: 공유 skill 저장소를 위해 CI 파이프라인에 audit 추가
5. **커스텀 규칙**: 조직의 위협 모델에 맞게 탐지를 조정
6. **보고서 검토**: 컴플라이언스에는 `--format json`, GitHub Code Scanning에는 `--format sarif`, GitHub Issues/PR에는 `--format markdown` 사용

### Threshold 구성

config 파일에서 차단 threshold를 설정합니다:

```yaml
# ~/.config/skillshare/config.yaml
audit:
  block_threshold: HIGH  # HIGH 이상에서 차단 (기본값 CRITICAL보다 엄격)
```

또는 명령별로:

```bash
skillshare audit --threshold medium  # MEDIUM 이상에서 차단
```

### 전체 Audit 구성

모든 audit 설정은 `config.yaml`에 저장할 수 있습니다:

```yaml
# ~/.config/skillshare/config.yaml (또는 project의 경우 .skillshare/config.yaml)
audit:
  block_threshold: HIGH                         # 차단 심각도 게이트
  profile: strict                               # 프로파일 프리셋 (default/strict/permissive)
  dedupe_mode: global                           # Dedup 모드 (global/legacy)
  enabled_analyzers: [static, dataflow, tier]   # 특정 분석기로 제한
```

CLI 플래그가 config 값보다 우선합니다. 전체 해석 순서는 [우선순위](#precedence)를 참조하십시오.

`skillshare status` 명령은 모든 우선순위 계층을 적용한 후의 유효한 프로파일, threshold, dedupe 모드, 분석기 목록을 포함하여 해석된 audit 정책을 표시합니다.

## Web UI

audit 기능은 `/audit` 경로의 web dashboard에서도 사용할 수 있습니다:

```bash
skillshare ui
# Audit 페이지로 이동 → "Run Audit" 클릭
```

![Security Audit page in web dashboard](/img/web-audit-demo.png)

Dashboard 페이지에는 빠른 스캔 요약이 포함된 Security Audit 섹션이 있습니다.

### 커스텀 규칙 에디터

web dashboard에는 브라우저에서 직접 커스텀 규칙을 생성하고 편집할 수 있는 전용 **Audit Rules** 페이지가 `/audit/rules`에 있습니다:

- **생성**: `audit-rules.yaml`이 없으면 "Create Rules File"을 클릭하여 새로 생성
- **편집**: 구문 강조와 검증 기능이 있는 YAML 에디터
- **저장**: 저장하기 전에 YAML 형식과 정규식 패턴을 검증

Audit 페이지의 "Custom Rules" 버튼에서 접근할 수 있습니다.

## Exit Codes

| Code | 의미 |
|------|---------|
| `0` | 활성 threshold 이상의 findings 없음 |
| `1` | 활성 threshold 이상의 findings가 하나 이상 있음 |

## 스캔 대상 파일

audit는 skill 디렉터리 내 텍스트 기반 파일을 스캔합니다:

- `.md`, `.txt`, `.yaml`, `.yml`, `.json`, `.toml`
- `.sh`, `.bash`, `.zsh`, `.fish`
- `.py`, `.js`, `.ts`, `.rb`, `.go`, `.rs`
- 확장자가 없는 파일 (예: `Makefile`, `Dockerfile`)

스캔은 각 skill 디렉터리 내에서 재귀적으로 수행되므로, `SKILL.md`, 중첩된 `references/*.md`, `scripts/*.sh` 모두 지원되는 텍스트 파일 형식에 해당하면 검사됩니다.

바이너리 파일(이미지, `.wasm` 등)과 숨김 디렉터리(`.git`)는 건너뜁니다.

## 옵션

| Flag | 설명 |
|------|------------|
| `-G`, `--group` `<name>` | 그룹에 속한 모든 skill 스캔 (반복 가능) |
| `-p`, `--project` | project 수준 skill 스캔 |
| `-g`, `--global` | global skill 스캔 |
| `--threshold` `<t>`, `-T` `<t>` | 차단 threshold: `critical`\|`high`\|`medium`\|`low`\|`info` (약어: `c`\|`h`\|`m`\|`l`\|`i`, 그리고 `crit`, `med`) |
| `--profile` `<p>` | Audit 프로파일 프리셋: `default`, `strict`, `permissive` |
| `--dedupe` `<mode>` | Dedup 모드: `legacy`, `global` (기본값) |
| `--analyzer` `<id>` | 지정한 분석기만 실행 (반복 가능). ID: `static`, `dataflow`, `tier`, `integrity`, `metadata`, `structure`, `cross-skill` |
| `--format` `<f>` | 출력 형식: `text` (기본값), `json`, `sarif`, `markdown` |
| `--json` | JSON 출력 (**사용 중단 예정**: `--format json` 사용) |
| `--yes`, `-y` | 대규모 스캔 확인 프롬프트 건너뛰기 (자동 확인) |
| `--quiet`, `-q` | findings가 있는 skill과 요약만 표시 (정상 ✓ 줄 숨김) |
| `--no-tui` | 대화형 TUI 비활성화, 일반 텍스트 출력 |
| `--init-rules` | 시작용 `audit-rules.yaml` 생성 (`-p`/`-g` 반영) |
| `-h`, `--help` | 도움말 표시 |

### 하위 명령

| 하위 명령 | 설명 |
|-----------|-------------|
| `rules` | audit 규칙 탐색, 활성화, 비활성화 (참조: [`audit rules`](/docs/reference/commands/audit-rules)) |

## Agent 지원

`skillshare audit agents`는 보안 스캔 범위를 agent로만 한정하여, agent 소스 디렉터리 내 `.md` 파일을 스캔합니다:

```bash
skillshare audit agents                    # 모든 agent 스캔
skillshare audit agents --threshold high   # agent에 대해 HIGH 이상에서 차단
skillshare audit agents --format sarif     # agent용 SARIF 출력
skillshare audit agents -p                 # project agent 스캔
```

Agent는 skill과 동일한 audit 규칙, 심각도 수준, threshold 게이팅의 적용을 받습니다. `agents` 인수가 없으면 `audit`는 skill만 스캔합니다(기본 동작). 배경 지식은 [Agents](/docs/understand/agents)를 참조하십시오.

## 참고

- [Audit Engine](/docs/understand/audit-engine) — 엔진의 동작 방식 (위협 모델, 위험 점수, 명령 등급)
- [`audit rules`](/docs/reference/commands/audit-rules) — 규칙 관리 및 커스터마이징
- [install](/docs/reference/commands/install) — skill 설치 (자동 스캔 포함)
- [check](/docs/reference/commands/check) — skill 무결성 및 sync 상태 확인
- [doctor](/docs/reference/commands/doctor) — 설정 문제 진단
- [list](/docs/reference/commands/list) — 설치된 skill 목록 표시
- [Securing Your Skills](/docs/how-to/advanced/security) — 팀과 조직을 위한 보안 가이드
- [CI/CD Skill Validation](/docs/how-to/recipes/ci-cd-skill-validation) — 파이프라인 자동화 레시피
- [Pre-commit Hook](/docs/how-to/recipes/pre-commit-hook) — 모든 커밋마다 자동 audit
- [Agents](/docs/understand/agents) — Agent 개념
