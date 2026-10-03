---
sidebar_position: 4
---

# analyze

각 target의 skill에 대한 context window 사용량과 skill 품질을 분석합니다.

```bash
skillshare analyze                    # Interactive TUI (default)
skillshare analyze claude             # Details for a single target
skillshare analyze --verbose          # Top 10 largest descriptions
skillshare analyze --json             # Machine-readable output
skillshare analyze -p                 # Project mode
```

## 사용 시점

### Context 예산 최적화

어떤 skill이 가장 많은 context window 토큰을 소비하는지 확인합니다:

```bash
skillshare analyze           # Browse all targets interactively
```

### Target 간 비교

target 간 context 사용량이 어떻게 다른지 확인합니다 (예: Claude vs Cursor):

```bash
skillshare analyze           # Tab to switch targets in TUI
```

### Skill 품질 확인

필드 누락, 짧은 description, trigger phrase가 없는 skill을 찾습니다:

```bash
skillshare analyze           # Lint icons (✗/⚠) appear in TUI
```

### CI/스크립팅

기계 판독 가능한 context 지표와 lint 결과를 가져옵니다:

```bash
skillshare analyze --json | jq '.targets[].always_loaded.estimated_tokens'
skillshare analyze --json | jq '.targets[].skills[] | select(.lint_issues | length > 0)'
```

## 동작 방식

`analyze`는 각 skill에 대해 두 계층의 context 비용을 계산합니다:

1. **Always loaded** — SKILL.md frontmatter의 `name + description` (skill 매칭을 위해 모든 요청 시 context에 로드됨)
2. **On-demand** — frontmatter 이후의 skill 본문 (skill이 트리거될 때만 로드됨)

토큰 추정치는 근사값으로 `chars / 4`를 사용합니다.

### Skill 품질 Lint

토큰 분석 외에도 `analyze`는 모든 skill에 대해 내장 lint 엔진을 실행합니다. Lint 규칙은 SKILL.md 구조와 description 품질을 검사하며, TUI와 JSON 출력에 문제를 직접 표시합니다.

| Rule | Severity | 검사 내용 |
|------|----------|----------------|
| `missing-name` | error | `name` 필드가 비어 있거나 없음 |
| `missing-description` | error | `description` 필드가 비어 있거나 없음 |
| `empty-body` | error | skill 본문(frontmatter 이후)이 비어 있음 |
| `description-too-short` | warning | description이 50자 미만 |
| `description-too-long` | warning | description이 1024자 목표 제한을 초과 |
| `description-near-limit` | warning | description이 900–1024자 사이 |
| `no-trigger-phrase` | warning | description에 trigger phrase가 없음 (예: "Use when…") |

TUI에서는 lint 문제가 있는 skill의 이름 옆에 ✗ (error) 또는 ⚠ (warning) 아이콘이 표시됩니다. detail 패널에는 모든 결과를 나열하는 **Quality** 섹션이 있습니다.

## Interactive TUI

TTY에서 `analyze`는 한 번에 하나의 target을 보는 대화형 화면을 엽니다. 왼쪽에는 토큰 사용량 순으로 정렬된 skill, 오른쪽에는 선택한 skill의 토큰 내역, lint 문제, 설명이 표시됩니다. skill을 열면 front matter를 포함한 파일 원문이 표시되어 lint 문제와 바로 대조할 수 있습니다. 키는 화면 아래쪽에 표시됩니다. 일반 텍스트로 보려면 `--no-tui`를 사용하세요.

### Color Coding

토큰 소비량 수준은 target별 동적 백분위 임계값을 사용합니다:

| Color | Meaning |
|-------|---------|
| 🔴 Red | P75 이상 (상위 25% 소비자) |
| 🟡 Yellow | P25–P75 (중간 50%) |
| 🟢 Green | P25 미만 (하위 25%) |

## Example Output

### Default (--no-tui)

```
claude · 7 skills
  Always loaded  ~362 tokens
  On-demand max  ~22 tokens

  Add -v for the 10 largest descriptions
```

### Verbose

```
skillshare analyze --verbose

claude · 7 skills
  Always loaded  ~362 tokens
  On-demand max  ~22 tokens

  Largest descriptions
    my-big-skill   ~180 tokens
    another-skill  ~120 tokens
    …
```

### Single Target

target 이름을 전달하면 자동으로 verbose 출력이 활성화됩니다:

```bash
skillshare analyze claude
```

### Filter by Group

```bash
# See total token cost of all frontend skills
skillshare analyze claude --json --filter frontend

# Pre-populate the search box in TUI
skillshare analyze --filter marketing
```

## Options

| Flag | 설명 |
|------|-------------|
| `[target]` | 단일 target의 상세 정보 표시 (자동으로 verbose 활성화) |
| `--verbose`, `-v` | target별 상위 10개 가장 큰 description 표시 |
| `--no-tui` | interactive TUI 비활성화, 일반 텍스트 출력 |
| `--project`, `-p` | 프로젝트 레벨 skill 분석 (`.skillshare/`) |
| `--global`, `-g` | 전역 skill 분석 (`~/.config/skillshare`) |
| `--filter <text>` | 이름/경로 부분 문자열로 skill 필터링 |
| `--json` | JSON으로 출력 (스크립팅/CI용) |
| `--help`, `-h` | 도움말 표시 |

:::tip Auto-detection
`--project`와 `--global` 모두 지정하지 않으면, skillshare는 자동으로 감지합니다: 현재 디렉터리에 `.skillshare/config.yaml`이 존재하면 프로젝트 모드가 기본값이 되고, 그렇지 않으면 전역 모드가 기본값이 됩니다.
:::

## JSON Output

```bash
skillshare analyze --json
```

```json
{
  "targets": [
    {
      "name": "claude",
      "skill_count": 7,
      "always_loaded": {
        "chars": 1448,
        "estimated_tokens": 362
      },
      "on_demand_max": {
        "chars": 88,
        "estimated_tokens": 22
      },
      "skills": [
        {
          "name": "my-skill",
          "description_chars": 180,
          "description_tokens": 45,
          "body_chars": 400,
          "body_tokens": 100,
          "lint_issues": [
            {
              "rule": "no-trigger-phrase",
              "severity": "warning",
              "category": "format",
              "message": "Description lacks trigger phrases (e.g. 'Use when...'); agents may not know when to invoke this skill"
            }
          ]
        }
      ]
    }
  ]
}
```

lint 문제가 없는 skill은 `lint_issues` 필드를 생략합니다.

## Project Mode

```bash
skillshare analyze -p                  # Interactive TUI for project skills
skillshare analyze -p --verbose        # Verbose text output
skillshare analyze -p claude           # Single target details
skillshare analyze -p --json           # JSON output
```

## Filtering

`--filter`를 사용하여 결과를 skill의 부분 집합으로 좁힙니다. 이 필터는 skill의 상대 경로(group 디렉터리를 포함)에 대해 대소문자를 구분하지 않는 부분 문자열 매칭을 수행합니다.

예를 들어, `--into frontend`로 skill을 설치했다면:
- `--filter frontend`는 `frontend/` group의 모든 skill과 일치
- `--filter react`는 경로에 "react"가 포함된 모든 skill과 일치

TUI 모드에서는 `--filter`가 필터 입력란을 미리 채웁니다. `/` 키를 사용해 대화형으로 필터링을 시작할 수도 있습니다.

JSON 모드에서는 출력에 집계된 토큰 수를 포함하는 `filtered_summary`가 포함됩니다:

```json
{
  "filter": "frontend",
  "matched_count": 5,
  "total_count": 50,
  "filtered_summary": {
    "always_loaded": { "chars": 2400, "tokens": 600 },
    "on_demand": { "chars": 8000, "tokens": 2000 },
    "total": { "chars": 10400, "tokens": 2600 }
  },
  "skills": [...]
}
```

Web UI도 검색이나 필터가 활성화되면 동적 토큰 요약 바를 표시합니다.

## Budget Warnings

`context_budget` 임계값이 설정되어 있으면, `analyze`는 target이 예산을 초과할 경우 경고를 표시합니다. 설정 세부 정보는 [sync — Context Cost](/docs/reference/commands/sync#context-cost)를 참고하세요.

## See Also

- [list](/docs/reference/commands/list) — 설치된 skill 확인
- [audit](/docs/reference/commands/audit) — 보안 위협에 대해 skill 스캔
- [tui](/docs/reference/commands/tui) — interactive TUI 켜기/끄기
