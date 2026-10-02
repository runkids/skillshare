---
sidebar_position: 5
---

# Skill Format

skillshare skill의 구조와 메타데이터.

:::tip 언제 중요한가요?
SKILL.md 형식은 AI CLI가 여러분의 skill을 어떻게 발견하고 로드하는지를 결정합니다. `description` 필드는 특히 중요합니다 — AI가 언제 여러분의 skill을 활성화할지 결정하는 데 사용하는 필드입니다.
:::

## 개요

skill은 최소한 `SKILL.md` 파일을 포함하는 디렉터리입니다:

```
my-skill/
└── SKILL.md
```

`SKILL.md` 파일은 두 부분으로 구성됩니다:
1. **YAML frontmatter** — 메타데이터
2. **Markdown 본문** — AI를 위한 지침

---

## 기본 구조

```markdown
---
name: my-skill
description: Brief description of what this skill does
---

# My Skill

Instructions for the agent when this skill is activated.

## When to Use

Describe when this skill should be used.

## Instructions

1. First step
2. Second step
3. Additional steps as needed
```

---

## 필수 필드

### `name`

skill 식별자입니다. 다음에 사용됩니다:
- skill 호출 (예: `/skill:my-skill`)
- 충돌 감지
- skill 목록에서의 표시

```yaml
name: my-skill
```

**규칙:**
- 소문자, 숫자, 하이픈, 언더스코어
- 문자나 숫자로 시작해야 함
- 모든 skill 사이에서 고유해야 함

**예시:**
```yaml
name: code-review
name: pdf-tools
name: acme-frontend-ui  # Namespaced for teams
```

---

## 선택 필드

### `description`

skill 목록과 검색 결과에 표시되는 간단한 설명입니다.

```yaml
description: Reviews code for bugs, style issues, and improvements
```

---

## 선택 필드

### `tags`

hub 인덱스에서 필터링과 그룹화를 위한 분류 태그입니다. `skillshare hub index`를 실행하면 SKILL.md frontmatter의 태그가 생성된 `skillshare-hub.json`에 포함됩니다.

```yaml
tags: git, workflow
```

태그는 검색도 가능합니다 — `skillshare search workflow --hub ...`는 "workflow" 태그가 붙은 skill과 일치합니다.

### `targets`

이 skill이 sync되는 target을 제한합니다. 생략하면 skill은 **모든** target에 sync됩니다.

두 가지 배치 스타일을 지원합니다 — `metadata:` 아래 (권장) 또는 최상위 레벨:

```yaml
# Recommended: under metadata
metadata:
  targets: [claude, cursor]

# Legacy: top-level (still fully supported)
targets: [claude, cursor]
```

:::info 우선순위 규칙
둘 다 존재하면 `metadata.targets`가 최상위 `targets`보다 우선합니다. 이를 통해 점진적으로 마이그레이션할 수 있습니다 — `metadata:`를 추가해도 남아 있는 최상위 필드와 충돌하지 않습니다.
:::

| 값 | 동작 |
|-------|-------|
| *(생략됨)* | 모든 target에 sync됨 (기본값) |
| `[claude]` | "claude"와 일치하는 target에만 sync됨 |
| `[claude, cursor]` | 둘 중 하나의 이름과 일치하는 target에 sync됨 |

**모드 간 매칭:** `targets: [claude]`를 선언한 skill은 project target `claude`와도 일치합니다. 둘 다 동일한 AI CLI를 가리키기 때문입니다. 매칭은 [target registry](/docs/reference/targets/supported-targets)를 사용합니다.

**Config 필터와의 상호작용:** skill 레벨의 `targets`는 config 레벨의 `include`/`exclude` **이후에** 적용됩니다. skill이 sync되려면 둘 다 통과해야 합니다. 자세한 내용은 [Configuration](/docs/reference/targets/configuration#skill-level-targets)을 참고하세요.

**예시 — Claude 전용 skill:**

```markdown
---
name: claude-prompts
description: Prompt patterns for Claude Code
metadata:
  targets: [claude]
---

# Claude Prompts
...
```

이 skill은 Pi, Codex, 그 외 target이 구성되어 있어도 Claude Code의 skill 디렉터리에만 나타납니다.

### `pattern`

이 skill이 사용하는 구조적 설계 패턴입니다. `skillshare new -P <pattern>`으로 자동 생성됩니다.

```yaml
pattern: reviewer
```

사용 가능한 패턴: `tool-wrapper`, `generator`, `reviewer`, `inversion`, `pipeline`. 각각에 대한 자세한 내용은 [Skill Design Patterns](/docs/understand/philosophy/skill-design-patterns)를 참고하세요.

### `category`

이 skill의 유스케이스 카테고리입니다. `skillshare new` 중 대화형으로 설정하거나 완전히 생략할 수 있습니다.

```yaml
category: quality
```

사용 가능한 카테고리: `library`, `verification`, `data`, `automation`, `scaffold`, `quality`, `cicd`, `runbook`, `infra`.

### `license`

skill의 라이선스 식별자입니다. 준수 여부 판단을 돕기 위해 설치 중에 표시됩니다.

```yaml
license: MIT
```

값이 있으면 `skillshare install`은 skill 선택 프롬프트와 확인 화면에 라이선스를 표시합니다:

- **단일 skill**: skill 정보 박스에 `License: MIT`로 표시됨
- **다중 skill repo**: 선택 목록의 skill 이름 뒤에 추가됨 (예: `my-skill (MIT)`)

이는 순수하게 정보 제공용이며 설치를 막지 않습니다. 흔한 값: `MIT`, `Apache-2.0`, `GPL-3.0`, `BSD-3-Clause`, `ISC`.

---

## `metadata` 블록

`metadata:` 블록은 배포 및 동작 관련 필드를 위한 구조화된 YAML 객체입니다. 이는 30개 이상의 AI CLI 도구에서 사용되는 [Agent Skills 생태계 관례](https://developers.googleblog.com/en/5-agent-skill-design-patterns-every-adk-developer-should-know/)와 일치합니다.

```yaml
---
name: my-skill
description: My custom skill
metadata:
  targets: [claude]
  pattern: reviewer
  domain: python
---
```

현재 `targets`는 skillshare가 처리하는 유일한 `metadata` 필드입니다. 다른 필드(`pattern`, `domain`, `interaction` 같은)는 frontmatter에 보존되지만 skillshare가 사용하지는 않습니다 — 생태계 내 다른 도구가 소비할 수 있습니다.

하위 호환성을 위해 skillshare는 최상위 `targets` 필드도 읽습니다. 둘 다 존재하면 `metadata.targets`가 우선합니다.

## 커스텀 필드

최상위 레벨에 원하는 커스텀 필드를 추가할 수 있습니다:

```yaml
---
name: my-skill
description: My custom skill
author: Your Name
version: 1.0.0
---
```

커스텀 최상위 필드는 frontmatter에 저장되지만 skillshare 자체에서는 사용되지 않습니다.

---

## Markdown 본문

본문에는 AI를 위한 지침이 담깁니다. 사람 조수에게 지시하듯이 작성하세요.

**좋은 관행:**
- 명확하고 구체적인 지침
- 입력과 기대 출력의 예시
- 엣지 케이스와 오류 처리
- 언제 사용해야 하는지 (그리고 언제 사용하면 안 되는지)

**예시:**
```markdown
# Code Review

You are a code reviewer. Analyze code for:
- Bugs and potential issues
- Style and consistency
- Performance concerns
- Security vulnerabilities

## When to Use

Use this skill when the user asks you to review code, find bugs, or improve code quality.

## Instructions

1. Read the provided code carefully
2. Identify issues in order of severity
3. Suggest specific improvements with code examples
4. Be constructive and explain your reasoning

## Example

User: "Review this function"
```python
def add(a, b):
  return a + b
```

Response: "The function looks correct but could benefit from type hints..."
```

---

## 중앙화된 메타데이터

skill을 설치하면 skillshare는 그 메타데이터를 `.metadata.json`에 기록합니다 (모든 skill에 대해 중앙화됨):

```json
{
  "skills": [
    {
      "name": "pdf",
      "source": "anthropics/skills/skills/pdf",
      "type": "github",
      "installed_at": "2026-01-20T15:30:00Z",
      "repo_url": "https://github.com/anthropics/skills.git",
      "subdir": "skills/pdf",
      "version": "abc1234"
    }
  ]
}
```

각 skill 항목은 다음을 포함합니다:

| 필드 | 설명 |
|-------|-------------|
| `name` | skill 디렉터리 이름 |
| `source` | 원래 install 소스 입력값 |
| `type` | 소스 유형 (`github`, `local` 등) |
| `installed_at` | 설치 타임스탬프 |
| `repo_url` | git clone URL (git 소스만) |
| `subdir` | 하위 디렉터리 경로 (모노레포 소스만) |
| `version` | 설치 시점의 git 커밋 해시 |

이는 `skillshare update`와 `skillshare check`가 업데이트를 어디서 가져올지 알기 위해 사용됩니다.

**이 파일을 수동으로 편집하지 마세요.**

---

## Skill 생성하기

```bash
skillshare new my-skill
```

다음이 생성됩니다:
```
~/.config/skillshare/skills/my-skill/
└── SKILL.md  (with template)
```

생성된 `SKILL.md`를 편집하고 `skillshare sync`를 실행해 배포하세요.

---

## Skill 검증하기

```bash
skillshare doctor
```

다음을 확인합니다:
- 유효한 SKILL.md 형식
- 필수 `name` 필드
- 유효한 frontmatter YAML
- 이름 충돌

---

## 참고

- [new](/docs/reference/commands/new) — 올바른 템플릿으로 skill 생성
- [Creating Skills](/docs/how-to/daily-tasks/creating-skills) — skill 작성 전체 가이드
- [Best Practices](/docs/how-to/daily-tasks/best-practices) — 이름 짓기와 정리 팁
