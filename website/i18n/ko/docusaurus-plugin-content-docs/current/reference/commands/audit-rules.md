---
sidebar_position: 4
---

# audit rules

audit 규칙을 탐색, 활성화, 비활성화, 커스터마이징합니다.

```bash
skillshare audit rules                          # 대화형 TUI 규칙 브라우저
skillshare audit rules --no-tui                 # 일반 텍스트 표
skillshare audit rules --pattern credential-access  # 패턴으로 필터링
skillshare audit rules --severity high          # 심각도로 필터링
skillshare audit rules --disabled               # 비활성화된 규칙만 표시
skillshare audit rules --format json            # JSON 출력

skillshare audit rules disable prompt-injection-0           # 단일 규칙 비활성화
skillshare audit rules disable --pattern credential-access  # 그룹 전체 비활성화
skillshare audit rules enable prompt-injection-0            # 규칙 재활성화
skillshare audit rules enable --pattern credential-access   # 그룹 재활성화

skillshare audit rules severity destructive-commands-2 medium      # 단일 규칙 하향 조정
skillshare audit rules severity --pattern destructive-commands low  # 그룹 전체 하향 조정
skillshare audit rules reset                    # 모든 커스텀 규칙 제거, 기본값 복원

skillshare audit rules init                     # 시작용 audit-rules.yaml 생성
skillshare audit rules init -p                  # project 수준 규칙 파일 생성
```

## 패턴 수준 규칙

`audit-rules.yaml`에서 전체 패턴 그룹을 비활성화하거나 재정의할 수 있습니다:

```yaml
rules:
  # 모든 credential-access 규칙 비활성화
  - pattern: credential-access
    enabled: false

  # 하지만 .env 탐지는 유지
  - id: credential-access-env-file
    enabled: true

  # 모든 destructive-commands를 MEDIUM으로 하향 조정
  - pattern: destructive-commands
    severity: MEDIUM
```

패턴 수준 항목은 `id` 없이 `pattern`을 사용합니다. 병합 순서: 패턴 수준 규칙이 먼저 적용되고, 그 다음 id 수준 규칙이 비활성화된 그룹 내 개별 항목을 재정의할 수 있습니다.

## 커스텀 규칙 {#custom-rules}

YAML 파일을 사용하여 audit 규칙을 추가, 재정의, 비활성화할 수 있습니다. 규칙은 **내장 규칙 → global 사용자 → project 사용자** 순서로 병합됩니다.

주석이 달린 예시가 포함된 시작용 파일을 만들려면 `--init-rules`(또는 `audit rules init`)를 사용하십시오:

```bash
skillshare audit --init-rules         # global 규칙 파일 생성
skillshare audit -p --init-rules      # project 규칙 파일 생성
```

### 파일 위치

| Scope | Path |
|-------|------|
| Global | `~/.config/skillshare/audit-rules.yaml` |
| Project | `.skillshare/audit-rules.yaml` |

### 형식

```yaml
rules:
  # 새 규칙 추가
  - id: my-custom-rule
    severity: HIGH
    pattern: custom-check
    message: "Custom pattern detected"
    regex: 'DANGEROUS_PATTERN'

  # exclude가 있는 규칙 추가 (특정 줄에서 매칭 억제)
  - id: url-check
    severity: MEDIUM
    pattern: url-usage
    message: "External URL detected"
    regex: 'https?://\S+'
    exclude: 'https?://(localhost|127\.0\.0\.1)'

  # 기존 내장 규칙 재정의 (id로 매칭)
  - id: destructive-commands-2
    severity: MEDIUM
    pattern: destructive-commands
    message: "Sudo usage (downgraded to MEDIUM)"
    regex: '(?i)\bsudo\s+'

  # 내장 규칙 비활성화
  - id: insecure-http-0
    enabled: false

  # dangling-link 구조적 검사 비활성화
  - id: dangling-link
    enabled: false
```

### 필드

| Field | 필수 여부 | 설명 |
|-------|----------|------|
| `id` | 예 | 안정적인 식별자. 일치하는 ID는 내장 규칙을 재정의합니다. |
| `severity` | 예* | `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `INFO` 중 하나 |
| `pattern` | 예* | 규칙 카테고리 이름 (예: `prompt-injection`) |
| `message` | 예* | findings에 표시되는 사람이 읽을 수 있는 설명 |
| `regex` | 예* | 각 줄에 대해 매칭할 정규식 |
| `exclude` | 아니오 | 줄이 `regex`와 `exclude` 둘 다에 매칭되면 finding이 억제됨 |
| `enabled` | 아니오 | 규칙을 비활성화하려면 `false`로 설정. 비활성화 시 `id`만 필요 |

*`enabled: false`가 아닌 경우 필수.

### 병합 규칙

각 레이어(global, 그다음 project)는 이전 레이어 위에 적용됩니다:

- **동일한 `id`** + `enabled: false` → 해당 규칙 비활성화
- **동일한 `id`** + 다른 필드 → 규칙 전체를 교체
- **새로운 `id`** → 커스텀 규칙으로 추가
- **`pattern`만** (id 없음) + `enabled: false` → 해당 패턴에 일치하는 모든 규칙 비활성화
- **`pattern`만** + `severity` → 일치하는 모든 규칙의 severity를 재정의
- **패턴 다음 id** → id 수준 항목이 비활성화된 패턴 그룹 내 개별 규칙을 재활성화할 수 있음

### 실전 템플릿

실제 정책 조정을 위한 시작점으로 다음을 사용하십시오:

```yaml
rules:
  # 교육/참고용 skill을 위해 hardcoded-secret을 MEDIUM으로 하향 조정
  - pattern: hardcoded-secret
    severity: MEDIUM

  # 내장 suspicious-fetch를 내부 allowlist로 재정의
  - id: suspicious-fetch-0
    severity: MEDIUM
    pattern: suspicious-fetch
    message: "External URL used in command context"
    regex: '(?i)(curl|wget|invoke-webrequest|iwr)\s+https?://'
    exclude: '(?i)https?://(localhost|127\.0\.0\.1|artifacts\.company\.internal|registry\.company\.internal)'

  # 거버넌스 예외: 노이즈가 많은 insecure-http 신호 비활성화
  - id: insecure-http-0
    enabled: false
```

### `init`으로 시작하기

`audit rules init`(또는 `audit --init-rules`)는 주석이 달린 예시가 포함된 시작용 `audit-rules.yaml`을 생성하며, 이를 주석 해제하여 수정할 수 있습니다:

```bash
skillshare audit rules init          # → ~/.config/skillshare/audit-rules.yaml
skillshare audit rules init -p       # → .skillshare/audit-rules.yaml
```

생성된 파일은 다음과 같습니다:

```yaml
# Custom audit rules for skillshare.
# Rules are merged on top of built-in rules in order:
#   built-in → global (~/.config/skillshare/audit-rules.yaml)
#            → project (.skillshare/audit-rules.yaml)
#
# Each rule needs: id, severity, pattern, message, regex.
# Optional: exclude (suppress match), enabled (false to disable).

rules:
  # Example: flag TODO comments as informational
  # - id: flag-todo
  #   severity: MEDIUM
  #   pattern: todo-comment
  #   message: "TODO comment found"
  #   regex: '(?i)\bTODO\b'

  # Example: disable a built-in rule by id
  # - id: insecure-http-0
  #   enabled: false

  # Example: disable the dangling-link structural check
  # - id: dangling-link
  #   enabled: false

  # Example: override a built-in rule (match by id, change severity)
  # - id: destructive-commands-2
  #   severity: MEDIUM
  #   pattern: destructive-commands
  #   message: "Sudo usage (downgraded)"
  #   regex: '(?i)\bsudo\s+'
```

파일이 이미 존재하면 `init`은 오류와 함께 종료됩니다 — 기존 규칙을 절대 덮어쓰지 않습니다.

## 워크플로우: False Positive 수정하기

규칙을 커스터마이징하는 흔한 이유는 정상적인 skill이 내장 규칙을 트리거하는 경우입니다. 단계별 예시는 다음과 같습니다:

**1. audit를 실행하고 false positive 확인:**

```bash
$ skillshare audit ci-helper
Audit  ~/.config/skillshare/skills
  global · blocks at CRITICAL · policy DEFAULT / dedupe:GLOBAL / analyzers:ALL

HIGH      Sudo escalation  SKILL.md:42
  destructive-commands-2 / static
  "sudo apt-get install -y jq"

  Risk       HIGH 15/100
  Auditable  100%

! ci-helper has findings below CRITICAL · 0.2s
```

**2. [내장 규칙 표](#built-in-rule-ids)에서 규칙 ID 확인:**

`sudo`가 포함된 `destructive-commands` 패턴은 규칙 `destructive-commands-2`에 매칭됩니다.

**3. 커스텀 규칙 파일 생성 (아직 없다면):**

```bash
skillshare audit rules init
```

**4. 억제 또는 하향 조정을 위한 규칙 재정의 추가:**

```yaml
# ~/.config/skillshare/audit-rules.yaml
rules:
  # CI 자동화 skill을 위해 sudo를 MEDIUM으로 하향 조정
  - id: destructive-commands-2
    severity: MEDIUM
    pattern: destructive-commands
    message: "Sudo usage (downgraded for CI automation)"
    regex: '(?i)\bsudo\s+'
```

또는 완전히 비활성화:

```yaml
rules:
  - id: destructive-commands-2
    enabled: false
```

**5. audit를 다시 실행하여 확인:**

```bash
$ skillshare audit ci-helper
Audit  ~/.config/skillshare/skills
  global · blocks at CRITICAL · policy DEFAULT / dedupe:GLOBAL / analyzers:ALL

✓ No issues found in ci-helper · 0.1s   # 이제 통과함 (또는 HIGH 대신 MEDIUM 표시)
```

### 변경 사항 검증

규칙을 편집한 후, audit를 다시 실행하여 확인하십시오:

```bash
skillshare audit                     # 모든 skill 확인
skillshare audit <name>              # 특정 skill 확인
skillshare audit --json | jq '.skills[].findings'  # findings를 프로그래밍 방식으로 검사
```

요약 해석:

- `Failed`는 활성 threshold 이상의 findings가 있는 skill 수입니다.
- `Warning`은 threshold 미만이지만 clean보다 높은 findings가 있는 skill 수입니다 (예: threshold가 `CRITICAL`일 때 `HIGH/MEDIUM/LOW/INFO`).

## 내장 규칙 ID {#built-in-rule-ids}

특정 내장 규칙을 재정의하거나 비활성화하려면 `id` 값을 사용하십시오:

정규식 기반 규칙의 소스:
[`internal/audit/rules.yaml`](https://github.com/runkids/skillshare/blob/main/internal/audit/rules.yaml)

:::note 구조적, tier, cross-skill 검사

`dangling-link`, `content-tampered`, `content-oversize`, `content-missing`, `content-unexpected`는 (정규식이 아닌 파일시스템 조회와 해시 비교를 수행하는) **구조적 검사**입니다. `low-analyzability`는 [Analyzability Score](/docs/understand/audit-engine#analyzability-score)에서 생성되는 **analyzability finding**입니다. `tier-stealth`, `tier-destructive-network`, `tier-network-heavy`, `tier-interpreter`, `tier-interpreter-network`는 [Command Safety Tiering](/docs/understand/audit-engine#command-safety-tiering) 프로파일에서 생성되는 **tier 조합 findings**입니다. `cross-skill-*` findings는 [Cross-Skill Interaction Detection](/docs/understand/audit-engine#cross-skill-interaction-detection)에서 생성됩니다. 이들은 모두 아래 표에 나타나지만 `rules.yaml`에는 정의되어 있지 않습니다.

:::

| ID | Pattern | Severity |
|----|---------|----------|
| `prompt-injection-0` | prompt-injection | CRITICAL |
| `prompt-injection-1` | prompt-injection | CRITICAL |
| `prompt-injection-2` | prompt-injection | HIGH |
| `prompt-injection-3` | prompt-injection | CRITICAL |
| `prompt-injection-4` | prompt-injection | CRITICAL |
| `hidden-unicode-1` | invisible-payload | CRITICAL |
| `data-exfiltration-0` | data-exfiltration | CRITICAL |
| `data-exfiltration-1` | data-exfiltration | CRITICAL |
| `data-exfiltration-2` | data-exfiltration | MEDIUM |
| `data-exfiltration-3` | data-exfiltration | HIGH |
| `credential-access-ssh-private-key` | credential-access | CRITICAL |
| `credential-access-env-file` | credential-access | CRITICAL |
| `credential-access-aws-credentials` | credential-access | CRITICAL |
| `credential-access-etc-shadow` | credential-access | CRITICAL |
| `credential-access-git-credentials` | credential-access | CRITICAL |
| `credential-access-netrc` | credential-access | CRITICAL |
| `credential-access-gnupg` | credential-access | CRITICAL |
| `credential-access-kube-config` | credential-access | CRITICAL |
| `credential-access-vault-token` | credential-access | CRITICAL |
| `credential-access-terraform-creds` | credential-access | CRITICAL |
| `credential-access-gnome-keyring` | credential-access | CRITICAL |
| `credential-access-npmrc` | credential-access | CRITICAL |
| `credential-access-pypirc` | credential-access | CRITICAL |
| `credential-access-gem-credentials` | credential-access | CRITICAL |
| `credential-access-ssl-private` | credential-access | CRITICAL |
| `credential-access-ssh-host-key` | credential-access | CRITICAL |
| `credential-access-pgpass` | credential-access | CRITICAL |
| `credential-access-mysql-cnf` | credential-access | CRITICAL |
| `credential-access-etc-passwd` | credential-access | MEDIUM |
| `credential-access-azure-creds` | credential-access | HIGH |
| `credential-access-gcloud-creds` | credential-access | HIGH |
| `credential-access-docker-config` | credential-access | HIGH |
| `credential-access-gh-cli-token` | credential-access | HIGH |
| `credential-access-password-store` | credential-access | HIGH |
| `credential-access-macos-keychain-user` | credential-access | HIGH |
| `credential-access-macos-keychain-sys` | credential-access | HIGH |
| `credential-access-terraformrc` | credential-access | HIGH |
| `credential-access-cargo-credentials` | credential-access | HIGH |
| `credential-access-op-cli` | credential-access | HIGH |
| `credential-access-age-keys` | credential-access | HIGH |
| `credential-access-shell-history` | credential-access | LOW |
| `credential-access-openvpn` | credential-access | LOW |
| `credential-access-auth-log` | credential-access | INFO |
| `credential-access-unknown-dotdir` | credential-access | INFO |

> **참고:** 위의 각 credential 항목은 접근 방법별로 변형 ID도 생성합니다: `-copy`, `-redirect`, `-dd`, `-exfil` (예: `credential-access-ssh-private-key-copy`). 특정 변형을 비활성화하려면 `audit-rules.yaml`에서 전체 ID를 사용하십시오.

| ID | Pattern | Severity |
|----|---------|----------|
| `hidden-unicode-0` | hidden-unicode | HIGH |
| `hidden-unicode-2` | hidden-unicode | HIGH |
| `config-manipulation-0` | config-manipulation | HIGH |
| `hidden-comment-injection-1` | hidden-comment-injection | HIGH |
| `self-propagation-0` | self-propagation | HIGH |
| `destructive-commands-0` | destructive-commands | HIGH |
| `destructive-commands-1` | destructive-commands | HIGH |
| `destructive-commands-2` | destructive-commands | HIGH |
| `destructive-commands-3` | destructive-commands | HIGH |
| `destructive-commands-4` | destructive-commands | HIGH |
| `dynamic-code-exec-0` | dynamic-code-exec | HIGH |
| `dynamic-code-exec-1` | dynamic-code-exec | HIGH |
| `shell-execution-0` | shell-execution | HIGH |
| `hidden-comment-injection-0` | hidden-comment-injection | HIGH |
| `obfuscation-0` | obfuscation | HIGH |
| `fetch-with-pipe-0` | fetch-with-pipe | HIGH |
| `fetch-with-pipe-1` | fetch-with-pipe | HIGH |
| `fetch-with-pipe-2` | fetch-with-pipe | HIGH |
| `hardcoded-secret-0` | hardcoded-secret | HIGH |
| `hardcoded-secret-1` | hardcoded-secret | HIGH |
| `hardcoded-secret-2` | hardcoded-secret | HIGH |
| `hardcoded-secret-3` | hardcoded-secret | HIGH |
| `hardcoded-secret-4` | hardcoded-secret | HIGH |
| `hardcoded-secret-5` | hardcoded-secret | HIGH |
| `hardcoded-secret-6` | hardcoded-secret | HIGH |
| `hardcoded-secret-7` | hardcoded-secret | HIGH |
| `hardcoded-secret-8` | hardcoded-secret | HIGH |
| `hardcoded-secret-9` | hardcoded-secret | HIGH |
| `data-uri-0` | data-uri | MEDIUM |
| `escape-obfuscation-0` | escape-obfuscation | MEDIUM |
| `suspicious-fetch-0` | suspicious-fetch | MEDIUM |
| `ip-address-url-0` | ip-address-url | MEDIUM |
| `hidden-unicode-3` | hidden-unicode | MEDIUM |
| `untrusted-install-0` | untrusted-install | MEDIUM |
| `untrusted-install-1` | untrusted-install | MEDIUM |
| `insecure-http-0` | insecure-http | LOW |
| `external-link-0` | external-link | LOW |
| `dangling-link` | dangling-link | LOW |
| `content-tampered` | content-tampered | MEDIUM |
| `content-oversize` | content-oversize | MEDIUM |
| `content-missing` | content-missing | LOW |
| `content-unexpected` | content-unexpected | LOW |
| `shell-chain-0` | shell-chain | INFO |
| `low-analyzability` | low-analyzability | INFO |
| `tier-stealth` | tier-stealth | CRITICAL |
| `tier-destructive-network` | tier-destructive-network | HIGH |
| `tier-network-heavy` | tier-network-heavy | MEDIUM |
| `tier-interpreter` | tier-interpreter | INFO |
| `tier-interpreter-network` | tier-interpreter-network | MEDIUM |
| `cross-skill-exfiltration` | cross-skill-exfiltration | HIGH |
| `cross-skill-privilege-network` | cross-skill-privilege-network | MEDIUM |
| `cross-skill-stealth` | cross-skill-stealth | HIGH |
| `cross-skill-cred-interpreter` | cross-skill-cred-interpreter | MEDIUM |

## 하위 명령

| 하위 명령 | 설명 |
|-----------|-------------|
| `rules` | audit 규칙 탐색, 활성화, 비활성화 |
| `rules disable <id>` | ID로 단일 규칙 비활성화 |
| `rules disable --pattern <p>` | 패턴에 일치하는 모든 규칙 비활성화 |
| `rules enable <id>` | ID로 단일 규칙 재활성화 |
| `rules enable --pattern <p>` | 패턴에 일치하는 모든 규칙 재활성화 |
| `rules severity <id> <level>` | 단일 규칙의 severity 재정의 |
| `rules severity --pattern <p> <level>` | 패턴 그룹 내 모든 규칙의 severity 재정의 |
| `rules reset` | 모든 커스텀 규칙 제거 (내장 기본값 복원) |
| `rules init` | 시작용 `audit-rules.yaml` 생성 (`audit --init-rules`와 동일) |

## 참고

- [`audit`](/docs/reference/commands/audit) — 메인 audit 명령 레퍼런스
- [Audit Engine](/docs/understand/audit-engine) — 엔진의 동작 방식 (위협 모델, 위험 점수, 등급)
- [Securing Your Skills](/docs/how-to/advanced/security) — 팀을 위한 보안 가이드
- [CI/CD Skill Validation](/docs/how-to/recipes/ci-cd-skill-validation) — 파이프라인 자동화 레시피
